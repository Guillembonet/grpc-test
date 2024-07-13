package rabbitmq

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/rabbitmq/amqp091-go"
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

// PushMessage pushes a message to the queue for the given user ID.
func (c *Client) PushMessage(userId int64, msg string) error {
	if err := c.pushChannel.Publish("", c.queueName, false, false, amqp091.Publishing{
		ContentType: "text/plain",
		Body:        []byte(fmt.Sprintf("%d:%s", userId, msg)),
	}); err != nil {
		return fmt.Errorf("error publishing message: %w", err)
	}

	return nil
}

type Message struct {
	UserId   int64
	Msg      string
	delivery amqp091.Delivery
}

func (m *Message) Ack() error {
	if err := m.delivery.Ack(false); err != nil {
		return fmt.Errorf("error acknowledging message: %w", err)
	}

	return nil
}

func (m *Message) Reject() error {
	if err := m.delivery.Reject(true); err != nil {
		return fmt.Errorf("error rejecting message: %w", err)
	}

	return nil
}

// ConsumeMessages consumes messages from the queue and returns a channel for the client to consume.
// The context is expected to expire when the client is done consuming messages so the channel can be closed.
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

				userId, message := int64(0), ""

				body := string(d.Body)
				bodySplit := strings.Split(body, ":")
				if len(bodySplit) == 2 {
					message = bodySplit[1]
					var err error
					userId, err = strconv.ParseInt(bodySplit[0], 10, 64)
					if err != nil {
						slog.Error("error parsing user ID, returning empty", slog.Any("err", err))
					}
				} else {
					slog.Error("error parsing message, returning empty")
				}

				msg := Message{
					delivery: d,
					UserId:   userId,
					Msg:      message,
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
