package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/streadway/amqp"
)

const (
	rabbitMQBlueURL  = "amqp://admin:admin@localhost:5672/"
	rabbitMQGreenURL = "amqp://admin:admin@localhost:5673/"
	exchange         = "customers"
)

func main() {

	var cluster string
	var rabbitMQURL string

	flag.StringVar(&cluster, "cluster", "", "--cluster=blue")
	flag.Parse()

	switch strings.ToLower(cluster) {
	case "blue":
		rabbitMQURL = rabbitMQBlueURL
	case "green":
		rabbitMQURL = rabbitMQGreenURL
	default:
		log.Fatalf("Unknown RabbitMQ cluster: %v", cluster)
	}

	conn, err := amqp.Dial(rabbitMQURL)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Failed to open a channel: %v", err)
	}
	defer ch.Close()

	err = ch.ExchangeDeclare(
		exchange, // name
		"direct", // type
		true,     // durable
		false,    // auto-deleted
		false,    // internal
		false,    // no-wait
		nil,      // arguments
	)
	if err != nil {
		log.Fatalf("Failed to declare exchange: %v", err)
	}

	for i := 0; i <= 5000; i++ {
		message := fmt.Sprintf("customer-%s-%d", cluster, i)
		routingKey := "us"
		if i%2 == 0 {
			routingKey = "de"
		}

		err = ch.Publish(
			exchange,   // exchange
			routingKey, // routing key
			false,      // mandatory
			false,      // immediate
			amqp.Publishing{
				ContentType: "text/plain",
				Body:        []byte(message),
			})
		if err != nil {
			log.Fatalf("Failed to publish a message: %v", err)
		}

		fmt.Printf("Published message '%s' with routing key '%s'\n", message, routingKey)
	}

	fmt.Println("All messages published.")
}
