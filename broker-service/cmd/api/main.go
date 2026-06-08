package main

import (
	"broker/event"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const webPort = "8080"

type MailPublisher interface {
	Push(payload string, routingKey string) error
}

type Config struct {
	Rabbit        *amqp.Connection
	MailPublisher MailPublisher
}

func main() {
	rabbitConn, err := connect()
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	emailEmitter, err := event.NewEmailEmitter(rabbitConn)
	if err != nil {
		log.Printf("failed to create email emitter: %v", err)
		os.Exit(1)
	}

	app := Config{
		Rabbit:        rabbitConn,
		MailPublisher: emailEmitter,
	}

	log.Printf("Starting broker on port %s\n", webPort)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", webPort),
		Handler: app.routes(),
	}

	err = srv.ListenAndServe()
	if err != nil {
		log.Panic(err)
	}
}

func connect() (*amqp.Connection, error) {
	var counts int64
	var backOff = 1 * time.Second
	var connection *amqp.Connection

	for {
		c, err := amqp.Dial("amqp://guest:guest@rabbitmq")
		if err != nil {
			fmt.Println("RabbitMQ is not ready")
			counts++
		} else {
			connection = c
			log.Println("Connected to RabbitMQ")
			break
		}

		if counts > 5 {
			fmt.Println(err)
			return nil, err
		}

		backOff = time.Duration(math.Pow(float64(counts), 2)) * time.Second
		log.Println("Backing off")
		time.Sleep(backOff)
		continue
	}

	return connection, nil
}
