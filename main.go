package main

import (
	"encoding/json"
	"log"
	"net/http"

	"sensor-ingestion-go/config"
	"sensor-ingestion-go/middleware"
	"sensor-ingestion-go/rabbitmq"

	"github.com/gin-gonic/gin"
)

type SensorPayload struct {
	MacAddress		string		`json:"macAddress" binding:"required"`
	Temperature		float64 	`json:"temperature" binding:"required"`
	Humidity		float64		`json:"humidity" binding:"required"`
	Timestamp		string		`json:"timestamp" binding:"required"`
	IsSignificant	*bool		`json:"isSignificant" binding:"required"`
}

type QueueMessage struct {
	SensorPayload
	TenantID string `json:"tenantId"`
}

func main() {
	log.Println("[App] Starting Sensor Ingestion...")

	config.LoadConfig()

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

  message := QueueMessage{
    SensorPayload: payload,
    TenantID:      tenantID,
  }

  messageBytes, err := json.Marshal(message)
  if err != nil {
    c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to serialize message"})
    return
  }

  isSig := false
  if payload.IsSignificant != nil {
    isSig = *payload.IsSignificant
  }

  if err := rabbitmq.GlobalPublisher.Publish(messageBytes, isSig); err != nil {
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