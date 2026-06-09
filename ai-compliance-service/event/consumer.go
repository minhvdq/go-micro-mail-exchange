package event

import amqp "github.com/rabbitmq/amqp091-go"

type Consumer struct {
	conn *amqp.Connection
}

func NewConsumer(conn *amqp.Connection) (*Consumer, error) {
	return &Consumer{conn: conn}, nil
}
