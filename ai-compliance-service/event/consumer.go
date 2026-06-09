package event

import (
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	exchangeName = "email_events"
	queueName    = "email.compliance.worker"
	ingestKey    = "email.ingest"
)

type Consumer struct {
	conn *amqp.Connection
}

func NewConsumer(conn *amqp.Connection) (*Consumer, error) {
	c := &Consumer{conn: conn}
	if err := c.setup(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Consumer) setup() error {
	ch, err := c.conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	if err := ch.ExchangeDeclare(exchangeName, "topic", true, false, false, false, nil); err != nil {
		return err
	}

	if _, err := ch.QueueDeclare(queueName, true, false, false, false, nil); err != nil {
		return err
	}

	return ch.QueueBind(queueName, ingestKey, exchangeName, false, nil)
}

// Consume starts delivery on queueName and returns the channel of messages.
func (c *Consumer) Consume() (<-chan amqp.Delivery, error) {
	ch, err := c.conn.Channel()
	if err != nil {
		return nil, err
	}

	if err := ch.Qos(1, 0, false); err != nil {
		return nil, err
	}

	return ch.Consume(queueName, "", false, false, false, false, nil)
}
