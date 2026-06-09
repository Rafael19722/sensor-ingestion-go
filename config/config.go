package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port		string
	JWTSecret	string
	RabbitMQURL string
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
	}

	if GlobalConfig.JWTSecret == "" {
		log.Fatal("Critial Error: JWT_SECRET is not defined!")
	}
}

func getEnv(key, defaultValue string) string {
  value := os.Getenv(key)
  if value = "" {
    return defaultValue
  }
  return value
}