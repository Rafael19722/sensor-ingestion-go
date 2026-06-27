/*
 * PharmaTrack - Firmware do sensor (ESP32)
 *
 * Filosofia: o device é um SENSOR BURRO E CONFIÁVEL.
 *   - Lê os sensores e carimba a hora REAL via NTP.
 *   - Entrega com garantia: se a rede/servidor cair, guarda em flash (LittleFS)
 *     e reenvia quando voltar (store-and-forward).
 *   - NÃO decide o que é "significativo": essa política mora no backend (Go).
 *
 * Dependências (Library Manager): OneWire, DallasTemperature, DHT sensor library.
 * LittleFS e WiFi/HTTPClient já vêm no core do ESP32.
 */

#include <WiFi.h>
#include <HTTPClient.h>
#include <OneWire.h>
#include <DallasTemperature.h>
#include <DHT.h>
#include <LittleFS.h>
#include <time.h>

// ================= CONFIGURAÇÕES =================
const char* ssid = "nome";
const char* password = "senha";
const String serverName = "link-go/ingest";  // ex: http://192.168.1.15:8080/ingest
const String jwtToken = "jwt";

// NTP (hora real). Fuso não importa: enviamos sempre em UTC ("Z").
const char* ntpServer = "pool.ntp.org";

// Sensor de metal (geladeira) - DS18B20
const int pinoDallas = 15;
OneWire oneWire(pinoDallas);
DallasTemperature sensorGeladeira(&oneWire);

// Sensor DHT22 (umidade da sala)
const int pinoDHT = 4;
#define DHTTYPE DHT22
DHT sensorSala(pinoDHT, DHTTYPE);

// Store-and-forward
const char* BUFFER_PATH = "/buffer.jsonl";
const size_t MAX_BUFFER_LINES = 2000;  // ~5h de outage a 10s; descarta as mais antigas além disso
const unsigned long INTERVALO_MS = 10000;

// ================= SETUP =================
void setup() {
  Serial.begin(115200);
  delay(1000);
  Serial.println("Iniciando Sensores IoT...");

  sensorGeladeira.begin();
  sensorSala.begin();

  if (!LittleFS.begin(true)) {  // true = formata na primeira vez
    Serial.println("ERRO: Falha ao montar LittleFS (buffer offline indisponível).");
  }

  conectarWiFi();
  configTime(0, 0, ntpServer);  // 0,0 = UTC
  aguardarHora();
}

// ================= LOOP =================
void loop() {
  float tempGeladeira = lerTemperatura();
  float umidadeSala = lerUmidade();

  if (tempGeladeira == DEVICE_DISCONNECTED_C) {
    Serial.println("Erro: Sensor da geladeira não encontrado!");
    delay(INTERVALO_MS);
    return;
  }

  String payload = montarPayload(tempGeladeira, umidadeSala);
  Serial.println(payload);

  if (WiFi.status() != WL_CONNECTED) {
    conectarWiFi();
  }

  if (WiFi.status() == WL_CONNECTED) {
    drenarBuffer();              // 1º tenta esvaziar o que ficou pendente
    if (!enviarPayload(payload)) {
      bufferizar(payload);       // falhou agora? guarda pra próxima
    }
  } else {
    Serial.println("Offline: guardando leitura no buffer.");
    bufferizar(payload);
  }

  delay(INTERVALO_MS);
}

// ================= LEITURA =================
float lerTemperatura() {
  sensorGeladeira.requestTemperatures();
  return sensorGeladeira.getTempCByIndex(0);
}

// Retorna NAN quando o DHT22 falha (será serializado como null, não como 0.0).
float lerUmidade() {
  float u = sensorSala.readHumidity();
  if (isnan(u)) {
    Serial.println("Aviso: Falha ao ler a umidade do DHT22 (enviando null).");
    return NAN;
  }
  return u;
}

// ================= PAYLOAD =================
String montarPayload(float temp, float umidade) {
  String json = "{";
  json += "\"macAddress\":\"" + WiFi.macAddress() + "\",";
  json += "\"temperature\":" + String(temp, 2) + ",";
  if (isnan(umidade)) {
    json += "\"humidity\":null,";
  } else {
    json += "\"humidity\":" + String(umidade, 2) + ",";
  }
  json += "\"timestamp\":\"" + getISOTimestamp() + "\"";
  // Sem isSignificant: a significância é decidida pelo backend.
  json += "}";
  return json;
}

// Hora real em ISO8601 UTC, ex: "2026-06-27T12:34:56Z".
String getISOTimestamp() {
  time_t now = time(nullptr);
  struct tm t;
  gmtime_r(&now, &t);
  char buf[25];
  strftime(buf, sizeof(buf), "%Y-%m-%dT%H:%M:%SZ", &t);
  return String(buf);
}

// ================= ENVIO =================
// Retorna true em sucesso (HTTP 2xx).
bool enviarPayload(const String& payload) {
  HTTPClient http;
  http.begin(serverName);
  http.addHeader("Content-Type", "application/json");
  http.addHeader("Authorization", "Bearer " + jwtToken);

  int code = http.POST(payload);
  http.end();

  Serial.printf("Resposta do servidor (Go): %d\n", code);
  return code >= 200 && code < 300;
}

// ================= STORE-AND-FORWARD =================
void bufferizar(const String& payload) {
  File f = LittleFS.open(BUFFER_PATH, FILE_APPEND);
  if (!f) {
    Serial.println("ERRO: não consegui abrir o buffer para escrita.");
    return;
  }
  f.println(payload);
  f.close();
  podarBuffer();
}

// Reenvia as leituras guardadas, em ordem. Para no primeiro erro e mantém o resto.
void drenarBuffer() {
  if (!LittleFS.exists(BUFFER_PATH)) {
    return;
  }
  File f = LittleFS.open(BUFFER_PATH, FILE_READ);
  if (!f) {
    return;
  }

  String pendentes = "";
  bool falhou = false;
  while (f.available()) {
    String linha = f.readStringUntil('\n');
    linha.trim();
    if (linha.length() == 0) {
      continue;
    }
    if (falhou) {
      pendentes += linha + "\n";  // já falhou antes: preserva o resto
    } else if (!enviarPayload(linha)) {
      falhou = true;
      pendentes += linha + "\n";
    }
  }
  f.close();

  if (pendentes.length() == 0) {
    LittleFS.remove(BUFFER_PATH);
    Serial.println("Buffer drenado por completo.");
  } else {
    File w = LittleFS.open(BUFFER_PATH, FILE_WRITE);  // reescreve só o que sobrou
    if (w) {
      w.print(pendentes);
      w.close();
    }
  }
}

// Mantém o buffer dentro de MAX_BUFFER_LINES, descartando as leituras mais antigas.
void podarBuffer() {
  File f = LittleFS.open(BUFFER_PATH, FILE_READ);
  if (!f) {
    return;
  }
  size_t total = 0;
  while (f.available()) {
    if (f.readStringUntil('\n').length() > 0) {
      total++;
    }
  }
  f.close();

  if (total <= MAX_BUFFER_LINES) {
    return;
  }

  size_t descartar = total - MAX_BUFFER_LINES;
  File r = LittleFS.open(BUFFER_PATH, FILE_READ);
  String mantidas = "";
  size_t i = 0;
  while (r.available()) {
    String linha = r.readStringUntil('\n');
    linha.trim();
    if (linha.length() == 0) {
      continue;
    }
    if (i++ < descartar) {
      continue;  // pula as mais antigas
    }
    mantidas += linha + "\n";
  }
  r.close();

  File w = LittleFS.open(BUFFER_PATH, FILE_WRITE);
  if (w) {
    w.print(mantidas);
    w.close();
  }
  Serial.printf("Buffer podado: descartadas %u leituras antigas.\n", (unsigned)descartar);
}

// ================= WIFI / NTP =================
void conectarWiFi() {
  if (WiFi.status() == WL_CONNECTED) {
    return;
  }
  WiFi.begin(ssid, password);
  Serial.print("Conectando no Wi-Fi");
  unsigned long inicio = millis();
  while (WiFi.status() != WL_CONNECTED && millis() - inicio < 15000) {  // timeout 15s
    delay(500);
    Serial.print(".");
  }
  if (WiFi.status() == WL_CONNECTED) {
    Serial.println("\nWi-Fi conectado! IP: " + WiFi.localIP().toString());
  } else {
    Serial.println("\nFalha ao conectar (seguindo offline; leituras vão pro buffer).");
  }
}

void aguardarHora() {
  Serial.print("Sincronizando hora (NTP)");
  unsigned long inicio = millis();
  while (time(nullptr) < 1000000000 && millis() - inicio < 10000) {  // ~ano 2001 como piso
    delay(500);
    Serial.print(".");
  }
  Serial.println(time(nullptr) < 1000000000 ? "\nAviso: NTP não sincronizou ainda." : "\nHora sincronizada.");
}
