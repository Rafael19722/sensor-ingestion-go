package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"sensor-ingestion-go/config"
	"sensor-ingestion-go/middleware"
	"sensor-ingestion-go/rabbitmq"

	"github.com/gin-gonic/gin"
)

type SensorPayload struct {
	MacAddress  string   `json:"macAddress" binding:"required"`
	Temperature *float64 `json:"temperature" binding:"required"`
	Humidity    *float64 `json:"humidity"`
	Timestamp   string   `json:"timestamp"`
}

type QueueMessage struct {
	MacAddress    string   `json:"macAddress"`
	Temperature   float64  `json:"temperature"`
	Humidity      *float64 `json:"humidity"`
	Timestamp     string   `json:"timestamp"`
	TenantID      string   `json:"tenantId"`
	IsSignificant bool     `json:"isSignificant"`
}

var sigTracker *significanceTracker

func main() {
	log.Println("[App] Starting Sensor Ingestion...")

	config.LoadConfig()

	sigTracker = newSignificanceTracker(
		time.Duration(config.GlobalConfig.SignificantIntervalMin)*time.Minute,
		config.GlobalConfig.SignificantDeltaC,
	)

	rabbitmq.InitRabbitMQ()

	defer rabbitmq.GlobalPublisher.Close()

	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
			"service": "sensor-ingestion-go",
		})
	})

  r.POST("/ingest", middleware.AuthMiddleware(), handleIngestion)

  port := config.GlobalConfig.Port
  log.Printf("[APP] Server boot at %s", port)

  if err := r.Run(":" + port); err != nil {
    log.Fatalf("CRITICAL ERROR: Failed to boot HTTP server: %v", err)
  }
}

func handleIngestion(c *gin.Context) {
  var payload SensorPayload

  if err := c.ShouldBindJSON(&payload); err != nil {
    c.JSON(http.StatusBadRequest, gin.H{
      "error":    "Invalid Payload or missing values",
      "details":  err.Error(),
    })
    return
  }

  tenantIDRaw, exists := c.Get("tenant_id")
  if !exists {
    c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal Error to get tenant_id"})
    return
  }
  tenantID := tenantIDRaw.(string)

  ts := time.Now().UTC()
  if payload.Timestamp != "" {
    if parsed, err := time.Parse(time.RFC3339, payload.Timestamp); err == nil {
      ts = parsed.UTC()
    } else {
      log.Printf("[WARN] Bad timestamp %q from %s, using server time", payload.Timestamp, payload.MacAddress)
    }
  }

  significant := sigTracker.evaluate(payload.MacAddress, *payload.Temperature, ts)

  message := QueueMessage{
    MacAddress:    payload.MacAddress,
    Temperature:   *payload.Temperature,
    Humidity:      payload.Humidity,
    Timestamp:     ts.Format(time.RFC3339),
    TenantID:      tenantID,
    IsSignificant: significant,
  }

  messageBytes, err := json.Marshal(message)
  if err != nil {
    c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to serialize message"})
    return
  }

  if err := rabbitmq.GlobalPublisher.Publish(messageBytes); err != nil {
    log.Printf("[ERRO] Failed to public message in queue: %v", err)
    c.JSON(http.StatusServiceUnavailable, gin.H{
      "error": "Message service unavailable temporary",
    })
    return
  }

  c.JSON(http.StatusAccepted, gin.H{
    "message": "Sensor read recieved with success and send to process",
    "tenantId": tenantID,
  })
}