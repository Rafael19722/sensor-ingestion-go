# Sensor Ingestion Service (Golang)

A high-performance IoT sensor ingestion microservice built with **Go** and **Gin-Gonic**. It authenticates incoming telemetries using **JWT** and publishes them to **RabbitMQ** for asynchronous processing.

## Tech Stack
- **Go** (1.22+)
- **Gin-Gonic** (HTTP Router)
- **RabbitMQ** via AMQP 0.9.1
- **JWT-Go** (v5)

## Features
- **Ultra-fast response times** (microsecond-level latency)
- **JWT Authentication middleware** with automatic tenant isolation (`tenant_id` extraction)
- **RabbitMQ queue declaration and publishing**
- **Resilient RabbitMQ connection retry mechanism**
- **Graceful resource shutdown**

## Getting Started

### 1. Requirements
Ensure you have **Go 1.22+** installed and **RabbitMQ** running on `localhost:5672` (e.g., via Docker).

### 2. Configuration
Create a `.env` file in the root directory:
```env
PORT=8080
JWT_SECRET=super-secret-jwt-key-change-in-prod-2026
RABBITMQ_URL=amqp://admin:secret@localhost:5672/
```

### 3. Install Dependencies
```bash
go mod tidy
```

### 4. Running the Service
```bash
go run main.go
```

## API Specification

### Health Check
`GET /health`

**Response (200 OK):**
```json
{
  "service": "sensor-ingestion-go",
  "status": "healthy"
}
```

### Ingest Telemetry
`POST /ingest`

**Headers:**
- `Content-Type: application/json`
- `Authorization: Bearer <JWT_TOKEN>`

**Request Body:**
```json
{
  "macAddress": "00:1B:44:11:3A:B7",
  "temperature": 4.5,
  "humidity": 42.0,
  "timestamp": "2026-06-09T14:15:00Z"
}
```

**Response (202 Accepted):**
```json
{
  "message": "Sensor read recieved with success and send to process",
  "tenantId": "tenant_filial_rj_123"
}
```
