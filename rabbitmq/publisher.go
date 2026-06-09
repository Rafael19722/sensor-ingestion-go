package rabbitmq

import (
	"context"
	"log"
	"time"

	"sensor-ingestion-go/config"

	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQConn struct {
	conn *amqp.Connection
	channel *amqp.Channel
}

var GlobalPublisher *RabbitMQConn

func InitRabbitMQ() {
	url := config.GlobalConfig.RabbitMQURL

	var err error
	var connection *amqp.Connection

	for i := 1; i <= 5; i++ {
		connection, err = amqp.Dial(url)
		if err == nil {
      break
    }
    log.Printf("[RabbitMQ] Try %d/5 of connection failed. New try in 3 seconds...", i)
    time.Sleep(3 * time.Second)
	}

  if err != nil {
    log.Fatalf("CRITIAL ERROR: Fail to connect with RabbitMQ after 5 attempts: %v", err)
  }

  log.Println("[RabbitMQ] Connected with success!")

  channel, err := connection.Channel()
  if err != nil {
    connection.Close()
    log.Fatalf("CRITIAL ERROR: Fail to open an channel in RabbitMQ: %v", err)
  }

  _, err = channel.QueueDeclare(
    "sensor_data_queue",
    true,
    false,
    false,
    false,
    nil,
  )
  if err != nil {
    channel.Close()
    connection.Close()
    log.Fatalf("CRITIAL ERROR: Failed to declair a queue in RabbitMQ: %v", err)
  }

  GlobalPublisher = &RabbitMQConn{
    conn: connection,
    channel: channel,
  }
}

func (r *RabbitMQConn) Publish(body []byte) error {
  ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
  defer cancel()

  err := r.channel.PublishWithContext(
    ctx,
    "",
    "sensor_data_queue",
    false,
    false,
    amqp.Publishing{
      ContentType: "application/json",
      Body:        body,
    },
  )

  return err
}

func (r *RabbitMQConn) Close() {
  if r.channel != nil {
    r.channel.Close()
  }
  if r.conn != nil {
    r.conn.Close()
  }
}