package main

import (
	"flag"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gihub.com/guillembonet/grpc-test/message"
	"gihub.com/guillembonet/grpc-test/nats"
	"gihub.com/guillembonet/grpc-test/rabbitmq"
	"gihub.com/guillembonet/grpc-test/server"
	"google.golang.org/grpc"
)

//go:generate ./generate_protobuf.sh

var (
	rabbitMqUrlFlag      = flag.String("rabbitmq-url", "amqp://guest:guest@localhost:5672/", "RabbitMQ URL")
	natsUrlFlag          = flag.String("nats-url", "nats://localhost:4222", "NATS URL")
	tcpListenAddressFlag = flag.String("tcp-listen-address", "localhost:8080", "TCP listen address")
)

func main() {
	flag.Parse()

	if rabbitMqUrlFlag == nil || tcpListenAddressFlag == nil || natsUrlFlag == nil ||
		*rabbitMqUrlFlag == "" || *tcpListenAddressFlag == "" || *natsUrlFlag == "" {
		slog.Error("missing required flags")
		os.Exit(1)
	}

	lis, err := net.Listen("tcp", *tcpListenAddressFlag)
	if err != nil {
		slog.Error("error listening", slog.Any("err", err))
		os.Exit(1)
	}

	rabbitMqClient, err := rabbitmq.NewClient(*rabbitMqUrlFlag)
	if err != nil {
		slog.Error("error creating rabbitmq client", slog.Any("err", err))
		os.Exit(1)
	}

	natsClient, err := nats.NewClient(*natsUrlFlag)
	if err != nil {
		slog.Error("error creating nats client", slog.Any("err", err))
		os.Exit(1)
	}

	server := server.NewServer(rabbitMqClient, natsClient)
	grpcServer := grpc.NewServer()

	message.RegisterMessengerServer(grpcServer, server)

	serverChan := make(chan struct{})
	go func() {
		slog.Info("starting server")
		err := grpcServer.Serve(lis)
		if err != nil {
			slog.Error("error serving server", slog.Any("err", err))
		}
		close(serverChan)
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case <-sigChan:
		slog.Info("shutting down server gracefully")

		stopped := make(chan struct{})
		go func() {
			grpcServer.GracefulStop()
			close(stopped)
		}()

		t := time.NewTimer(10 * time.Second)
		select {
		case <-t.C:
			slog.Error("server did not stop gracefully, stopping forcefully")
			grpcServer.Stop()
		case <-stopped:
			t.Stop()
		}
	case <-serverChan:
		slog.Error("server stopped unexpectedly")
	}

	slog.Info("bye!")
}
