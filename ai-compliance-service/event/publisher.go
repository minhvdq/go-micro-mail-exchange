package event

import (
	"context"

	amqp "github.com/rabbitmq/amqp091-go"
)

type EmailPublisher struct {
	conn *amqp.Connection
}

func NewEmailPublisher(conn *amqp.Connection) (*EmailPublisher, error) {
	return &EmailPublisher{conn: conn}, nil
}

func (p *EmailPublisher) Publish(ctx context.Context, payload []byte, routingKey string) error {
	return nil
}
