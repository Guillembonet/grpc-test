package rabbitmq

import (
	"context"
	"fmt"
	"log/slog"

	"gihub.com/guillembonet/grpc-test/message"
	"github.com/rabbitmq/amqp091-go"
	"google.golang.org/protobuf/proto"
)

type Client struct {
	conn        *amqp091.Connection
	pushChannel *amqp091.Channel
	queueName   string
}

func NewClient(url string) (*Client, error) {
	conn, err := amqp091.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("error dialing rabbitmq: %w", err)
	}

	channel, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("error creating channel: %w", err)
	}

	queue, err := channel.QueueDeclare("messages", false, false, false, false, nil)
	if err != nil {
		return nil, fmt.Errorf("error declaring queue: %w", err)
	}

	return &Client{
		conn:        conn,
		pushChannel: channel,
		queueName:   queue.Name,
	}, nil
}

// PushMessage pushes a message to the queue.
func (c *Client) PushMessage(msg *message.Message) error {
	body, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	if err := c.pushChannel.Publish("", c.queueName, false, false, amqp091.Publishing{
		ContentType: "application/protobuf",
		Body:        body,
	}); err != nil {
		return fmt.Errorf("error publishing message: %w", err)
	}

	return nil
}

// Message represents a message consumed from the queue.
type Message struct {
	Data     *message.Message
	delivery amqp091.Delivery
}

// Ack acknowledges the message.
func (m *Message) Ack() error {
	if err := m.delivery.Ack(false); err != nil {
		return fmt.Errorf("error acknowledging message: %w", err)
	}

	return nil
}

// Reject rejects the message and requeues it.
func (m *Message) Reject() error {
	if err := m.delivery.Reject(true); err != nil {
		return fmt.Errorf("error rejecting message: %w", err)
	}

	return nil
}

// ConsumeMessages consumes messages from the queue and returns a channel for the client to consume.
// The context is expected to expire when the client is done consuming messages so the channel can be closed.
// When the context expires, messages will fail to be acknowledged or rejected.
func (c *Client) ConsumeMessages(ctx context.Context) (<-chan Message, error) {
	channel, err := c.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("error creating channel: %w", err)
	}

	deliveries, err := channel.Consume(c.queueName, "", false, false, false, false, nil)
	if err != nil {
		return nil, fmt.Errorf("error consuming messages: %w", err)
	}

	msgs := make(chan Message)
	go func() {
		defer channel.Close()
		defer close(msgs)

		for {
			select {
			case <-ctx.Done():
				return
			case d, ok := <-deliveries:
				if !ok {
					return
				}

				pbMsg := &message.Message{}
				err := proto.Unmarshal(d.Body, pbMsg)
				if err != nil {
					slog.Error("error unmarshaling message", slog.Any("err", err))
					pbMsg = nil
				}

				msg := Message{
					delivery: d,
					Data:     pbMsg,
				}

				select {
				case <-ctx.Done():
					err := d.Reject(true)
					if err != nil {
						slog.Error("failed to reject message before exit", slog.Any("err", err))
					}
					return
				case msgs <- msg:
					// do nothing
				}
			}
		}
	}()

	return msgs, nil
}

func (c *Client) Close() error {
	if err := c.pushChannel.Close(); err != nil {
		return fmt.Errorf("error closing channel: %w", err)
	}

	if err := c.conn.Close(); err != nil {
		return fmt.Errorf("error closing connection: %w", err)
	}

	return nil
}
