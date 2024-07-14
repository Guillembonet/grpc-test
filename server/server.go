package server

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"

	"gihub.com/guillembonet/grpc-test/message"
	"gihub.com/guillembonet/grpc-test/nats"
	"gihub.com/guillembonet/grpc-test/rabbitmq"
)

type Server struct {
	rabbitMqClient *rabbitmq.Client
	natsClient     *nats.Client

	message.UnimplementedMessengerServer
}

func NewServer(rabbitMqClient *rabbitmq.Client, natsClient *nats.Client) *Server {
	return &Server{
		rabbitMqClient: rabbitMqClient,
		natsClient:     natsClient,
	}
}

var _ message.MessengerServer = &Server{}

func (s *Server) ProcessMessage(ctx context.Context, msg *message.Message) (*message.MessageResponse, error) {
	slog.Info("received message", slog.String("message", msg.GetMessage()), slog.String("note", msg.GetNote()))

	err := s.rabbitMqClient.PushMessage(msg)
	if err != nil {
		slog.Error("error pushing message to rabbitmq", slog.Any("err", err))
		return &message.MessageResponse{
			Status: message.Status_ERROR,
		}, nil
	}

	err = s.natsClient.PublishMessage("messages", msg)
	if err != nil {
		slog.Error("error publishing message to nats", slog.Any("err", err))
		return &message.MessageResponse{
			Status: message.Status_ERROR,
		}, nil
	}

	return &message.MessageResponse{
		Status: message.Status_OK,
	}, nil
}

func (s *Server) GetProcessedMessages(_ *message.GetProcessedMessagesParams, server message.Messenger_GetProcessedMessagesServer) error {
	msgs, err := s.rabbitMqClient.ConsumeMessages(server.Context())
	if err != nil {
		return fmt.Errorf("error consuming messages: %w", err)
	}

	for msg := range msgs {
		if msg.Data == nil {
			slog.Debug("received nil message, acknowledging")
			err = msg.Ack()
			if err != nil {
				return fmt.Errorf("error acknowledging empty message: %w", err)
			}
			continue
		}

		msgString := msg.Data.GetMessage()
		base64Message := base64.StdEncoding.EncodeToString([]byte(msgString))
		err = server.Send(&message.ProcessedMessage{
			Message:       msgString,
			Note:          msg.Data.GetNote(),
			Base64Message: base64Message,
		})
		if err != nil {
			return fmt.Errorf("error sending message: %w", err)
		}

		err = msg.Ack()
		if err != nil {
			return fmt.Errorf("error acknowledging message: %w", err)
		}
	}

	return nil
}
