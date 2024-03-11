package main

import (
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/streadway/amqp"
)

const (
	rabbitMQBlueURL  = "amqp://admin:admin@localhost:5672/"
	rabbitMQGreenURL = "amqp://admin:admin@localhost:5673/"
	concurrency      = 100
	sleepDuration    = 500 * time.Millisecond
)

func main() {
	var cluster string
	var queue string
	var rabbitMQURL string

	flag.StringVar(&cluster, "cluster", "", "--cluster=blue")
	flag.StringVar(&queue, "queue", "customers.de", "--queue=customers.de")
	flag.Parse()

	switch strings.ToLower(cluster) {
	case "blue":
		rabbitMQURL = rabbitMQBlueURL
	case "green":
		rabbitMQURL = rabbitMQGreenURL
	default:
		log.Fatalf("Unknown RabbitMQ cluster: %v", cluster)
	}

	if queue == "" {
		log.Fatalln("Queue name cannot be empty")
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

	err = ch.Qos(
		concurrency, // prefetch count
		0,           // prefetch size
		false,       // global
	)
	if err != nil {
		log.Fatalf("Failed to set QoS: %v", err)
	}

	// args := make(map[string]interface{})
	// args["x-priority"] = 10
	msgs, err := ch.Consume(
		queue, // queue
		"",    // consumer
		false, // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		log.Fatalf("Failed to register a consumer: %v", err)
	}

	forever := make(chan bool)

	go func() {
		for d := range msgs {
			fmt.Printf("Received a message: %s\n", d.Body)
			time.Sleep(sleepDuration) // Simulating long-running job
			fmt.Println("Job completed")
			d.Ack(false)
		}
	}()

	fmt.Println("Waiting for messages...")
	<-forever
}
