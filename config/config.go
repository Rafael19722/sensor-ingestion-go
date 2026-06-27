package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port		string
	JWTSecret	string
	RabbitMQURL string
	SignificantIntervalMin int
	SignificantDeltaC      float64
}

var GlobalConfig *Config

func LoadConfig() {
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using environment variables from system")
	}

	GlobalConfig = &Config{
		Port:			getEnv("PORT", "8080"),
		JWTSecret:  	getEnv("JWT_SECRET", ""),
		RabbitMQURL:	getEnv("RABBITMQ_URL", "amqp://admin:secret@localhost:5672/"),
		SignificantIntervalMin: getEnvInt("SIGNIFICANT_INTERVAL_MINUTES", 5),
		SignificantDeltaC:      getEnvFloat("SIGNIFICANT_DELTA_C", 0.5),
	}

	if GlobalConfig.JWTSecret == "" {
		log.Fatal("Critial Error: JWT_SECRET is not defined!")
	}
}

func getEnv(key, defaultValue string) string {
  value := os.Getenv(key)
  if value == "" {
    return defaultValue
  }
  return value
}

func getEnvInt(key string, defaultValue int) int {
  if value := os.Getenv(key); value != "" {
    if parsed, err := strconv.Atoi(value); err == nil {
      return parsed
    }
  }
  return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
  if value := os.Getenv(key); value != "" {
    if parsed, err := strconv.ParseFloat(value, 64); err == nil {
      return parsed
    }
  }
  return defaultValue
}