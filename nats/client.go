package nats

import (
	"context"
	"fmt"
	"log/slog"

	"gihub.com/guillembonet/grpc-test/message"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

type Client struct {
	client *nats.Conn
}

func NewClient(url string) (*Client, error) {
	client, err := nats.Connect(url)
	if err != nil {
		return nil, err
	}

	return &Client{
		client: client,
	}, nil
}

// PublishMessage publishes a message to the NATS server.
func (c *Client) PublishMessage(subject string, message *message.Message) error {
	data, err := proto.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	err = c.client.Publish(subject, data)
	if err != nil {
		return fmt.Errorf("error publishing message: %w", err)
	}

	return nil
}

// Message represents a message consumed from the subject.
type Message struct {
	Data *message.Message
	msg  *nats.Msg
}

// Ack acknowledges the message.
func (m *Message) Ack() error {
	return m.msg.Ack()
}

// Nak negatively acknowledges the message.
func (m *Message) Nak() error {
	return m.msg.Nak()
}

// Subscribe subscribes to a subject and returns a channel of messages.
// The context is expected to expire when the client is done consuming messages so the channel can be closed.
func (c *Client) Subscribe(ctx context.Context, subject string) (<-chan Message, error) {
	natsMsgs := make(chan *nats.Msg)
	sub, err := c.client.ChanSubscribe(subject, natsMsgs)
	if err != nil {
		return nil, fmt.Errorf("error subscribing to subject: %w", err)
	}

	msgs := make(chan Message)
	go func() {
		defer sub.Drain()
		defer sub.Unsubscribe()
		defer close(msgs)

		for {
			select {
			case <-ctx.Done():
				return
			case m, ok := <-natsMsgs:
				if !ok {
					return
				}

				pbMsg := &message.Message{}
				err := proto.Unmarshal(m.Data, pbMsg)
				if err != nil {
					slog.Error("error unmarshaling message", slog.Any("err", err))
					pbMsg = nil
				}

				msg := Message{
					msg:  m,
					Data: pbMsg,
				}

				select {
				case <-ctx.Done():
					err := m.Nak()
					if err != nil {
						slog.Error("failed to unack message before exit", slog.Any("err", err))
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

// Close closes the client.
func (c *Client) Close() {
	c.client.Close()
}
