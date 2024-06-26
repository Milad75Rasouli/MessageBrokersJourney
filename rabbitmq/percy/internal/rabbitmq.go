package internal

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"

	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitClient struct {
	// the connection used by the client
	conn *amqp.Connection
	// the channel is used to process/ send message
	ch *amqp.Channel
}

func ConnectRabbitMQ(username, password, host, vhost string) (*amqp.Connection, error) {
	return amqp.Dial(fmt.Sprintf("amqp://%s:%s@%s/%s", username, password, host, vhost))
}

func ConnectRabbitMQWithTLS(username, password, host, vhost, caCert, clientCert, clientKey string) (*amqp.Connection, error) {
	ca, err := os.ReadFile(caCert)
	if err != nil {
		return nil, err
	}

	// load keypair
	cert, err := tls.LoadX509KeyPair(clientCert, clientKey)
	if err != nil {
		return nil, err
	}

	// add the rootca the cert pool
	rootCAs := x509.NewCertPool()
	rootCAs.AppendCertsFromPEM(ca)
	tlsCfg := &tls.Config{
		RootCAs:      rootCAs,
		Certificates: []tls.Certificate{cert},
	}
	return amqp.DialTLS(fmt.Sprintf("amqps://%s:%s@%s/%s", username, password, host, vhost), tlsCfg)
}

func NewRabbitMQClient(conn *amqp.Connection) (RabbitClient, error) {
	ch, err := conn.Channel()
	if err != nil {
		return RabbitClient{}, err
	}
	// for waiting for ack need to active the confirm mode
	err = ch.Confirm(false)
	if err != nil {
		return RabbitClient{}, err
	}
	return RabbitClient{
		conn: conn,
		ch:   ch,
	}, nil
}

func (rc RabbitClient) CreateQueue(queueName string, durable, autoDelete bool) (amqp.Queue, error) {
	q, err := rc.ch.QueueDeclare(queueName, durable, autoDelete, false, false, nil)
	if err != nil {
		return amqp.Queue{}, err
	}
	return q, err
}

func (rc RabbitClient) CreateBinding(name, binding, exchange string) error {
	// leaving nowait false, will make the channel return an error if it fails.
	return rc.ch.QueueBind(name, binding, exchange, false, nil)
}

func (rc RabbitClient) Send(ctx context.Context, exchange, routingKey string, options amqp.Publishing) error {
	// return rc.ch.PublishWithContext(ctx,
	// 	exchange,
	// 	routingKey,
	// 	true,  // Mandatory is used to determine an error should be returned upon failure
	// 	false, //immediate
	// 	options,
	// )
	confirmation, err := rc.ch.PublishWithDeferredConfirmWithContext(ctx,
		exchange,
		routingKey,
		true,  // Mandatory is used to determine an error should be returned upon failure
		false, //immediate
		options,
	)
	if err != nil {
		return err
	}
	log.Println(confirmation.Wait())
	return nil

}

func (rc RabbitClient) Consume(queue, consumer string, autoAck bool) (<-chan amqp.Delivery, error) {
	return rc.ch.Consume(queue, consumer, autoAck, false, false, false, nil)
}

// prefetched count - an int of how many unacknowledged messages the server can send
// prefetch size - an int of how many bytes
// global - determines if the rule should be applied globally or not
func (rc RabbitClient) Qos(prefetchCount, prefetchSize int, global bool) error {
	return rc.ch.Qos(prefetchCount, prefetchSize, global)
}

func (rc RabbitClient) Close() error {
	return rc.ch.Close()
}
