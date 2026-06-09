package event

import (
	"context"

	amqp "github.com/rabbitmq/amqp091-go"
)

type EmailPublisher struct {
	conn *amqp.Connection
}

func NewEmailPublisher(conn *amqp.Connection) (*EmailPublisher, error) {
	p := &EmailPublisher{conn: conn}

	ch, err := conn.Channel()
	if err != nil {
		return nil, err
	}
	defer ch.Close()

	if err := ch.ExchangeDeclare(exchangeName, "topic", true, false, false, false, nil); err != nil {
		return nil, err
	}

	return p, nil
}

// Publish sends payload to the email_events exchange with the given routing key.
func (p *EmailPublisher) Publish(_ context.Context, payload []byte, routingKey string) error {
	ch, err := p.conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	return ch.Publish(
		exchangeName,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         payload,
		},
	)
}
