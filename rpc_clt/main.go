package main

import (
	"context"
	"flag"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	rabbitMQBlueURL  = "amqp://admin:admin@localhost:5672/"
	rabbitMQGreenURL = "amqp://admin:admin@localhost:5673/"
)

func failOnError(err error, msg string) {
	if err != nil {
		log.Panicf("%s: %s", msg, err)
	}
}

func fibonacciRPC(rabbitMQURL string, n int) (res int, err error) {
	conn, err := amqp.Dial(rabbitMQURL)
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	// Declare the exchange for answer
	err = ch.ExchangeDeclare(
		"rpc_answerexchange", // name
		"direct",             // type
		true,                 // durable
		false,                // auto-deleted
		false,                // internal
		false,                // no-wait
		nil,                  // arguments
	)
	failOnError(err, "Failed to declare the exchange")

	q, err := ch.QueueDeclare(
		"rpc_answer", // name
		false,        // durable
		false,        // delete when unused
		false,        // exclusive
		false,        // noWait
		nil,          // arguments
	)
	failOnError(err, "Failed to declare a queue")

	// Bind the queue to the exchange
	err = ch.QueueBind(
		q.Name,               // queue name
		q.Name,               // routing key
		"rpc_answerexchange", // exchange name
		false,                // no-wait
		nil,                  // arguments
	)
	failOnError(err, "Failed to bind the queue to the exchange")

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	failOnError(err, "Failed to register a consumer")

	corrId := uuid.NewString()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	log.Printf("publish %d correlationId:%v replyTo:%v", n, corrId, q.Name)
	err = ch.PublishWithContext(ctx,
		"rpc_requestexchange", // exchange
		"rpc_request",         // routing key
		false,                 // mandatory
		false,                 // immediate
		amqp.Publishing{
			ContentType:   "text/plain",
			CorrelationId: corrId,
			ReplyTo:       q.Name,
			Body:          []byte(strconv.Itoa(n)),
		})
	failOnError(err, "Failed to publish a message")

	for d := range msgs {
		log.Printf("Received message with correlationId: %s", d.CorrelationId)
		if corrId == d.CorrelationId {
			res, err = strconv.Atoi(string(d.Body))
			failOnError(err, "Failed to convert body to integer")
			break
		}
	}

	return
}

func main() {
	var cluster string
	var rabbitMQURL string
	var n int

	flag.StringVar(&cluster, "cluster", "blue", "--cluster=blue")
	flag.IntVar(&n, "iter", 30, "--iter=10")
	flag.Parse()

	switch strings.ToLower(cluster) {
	case "blue":
		rabbitMQURL = rabbitMQBlueURL
	case "green":
		rabbitMQURL = rabbitMQGreenURL
	default:
		log.Fatalf("Unknown RabbitMQ cluster: %v", cluster)
	}

	log.Printf(" [x] Requesting fib(%d)", n)
	res, err := fibonacciRPC(rabbitMQURL, n)
	failOnError(err, "Failed to handle RPC request")

	log.Printf(" [.] Got %d", res)
}
