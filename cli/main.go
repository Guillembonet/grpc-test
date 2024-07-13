package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"gihub.com/guillembonet/grpc-test/message"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	noteFlag = "note"
)

var rootCmd = cobra.Command{
	Use:   "grpc-message-cli",
	Short: "grpc-message-cli - a simple CLI to send messages to a gRPC server",
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		slog.Error("error executing root command", slog.Any("err", err))
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(processMessageCmd, getProcessedMessagedCmd)

	processMessageCmd.Flags().StringP(noteFlag, "n", "", "note to attach to the message")
}

func initClient() (message.MessengerClient, *grpc.ClientConn, error) {
	conn, err := grpc.NewClient("localhost:8080", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, fmt.Errorf("error creating client: %w", err)
	}

	return message.NewMessengerClient(conn), conn, nil
}

var processMessageCmd = &cobra.Command{
	Use:     "process-message",
	Aliases: []string{"pm"},
	Short:   "process-message - sends a message for processing to a gRPC server",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		note, err := cmd.Flags().GetString(noteFlag)
		if err != nil {
			slog.Error("error getting flag", slog.String("flag_name", noteFlag), slog.Any("err", err))
			os.Exit(1)
		}

		client, conn, err := initClient()
		if err != nil {
			slog.Error("error initializing client", slog.Any("err", err))
			os.Exit(1)
		}
		defer conn.Close()

		msg := &message.Message{
			Message: args[0],
			Note:    note,
		}
		resp, err := client.ProcessMessage(context.Background(), msg)
		if err != nil {
			slog.Error("error sending message", slog.Any("err", err))
			os.Exit(1)
		}

		slog.Info("message sent", slog.Int("response_status", int(resp.GetStatus())))
	},
}

var getProcessedMessagedCmd = &cobra.Command{
	Use:     "get-processed-messages",
	Aliases: []string{"gpm"},
	Short:   "get-processed-messages - get processed messages from a gRPC server",
	Run: func(cmd *cobra.Command, args []string) {
		client, conn, err := initClient()
		if err != nil {
			slog.Error("error initializing client", slog.Any("err", err))
			os.Exit(1)
		}
		defer conn.Close()

		resp, err := client.GetProcessedMessages(context.Background(), nil)
		if err != nil {
			slog.Error("error sending message", slog.Any("err", err))
			os.Exit(1)
		}

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		defer close(sigChan)

		stopped := make(chan struct{})
		go func() {
			defer close(stopped)

			for {
				msg, err := resp.Recv()
				if err != nil {
					if err == io.EOF {
						break
					}

					slog.Error("error receiving message", slog.Any("err", err))
					break
				}

				slog.Info("received message",
					slog.String("message", msg.GetMessage()),
					slog.String("note", msg.GetNote()),
					slog.String("base64_message", msg.GetBase64Message()))
			}
		}()

		select {
		case <-sigChan:
			slog.Info("stopping gracefully")
		case <-stopped:
			slog.Info("done receiving processed messages")
		}
	},
}
