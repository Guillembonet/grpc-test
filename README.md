# grpc-test

## Description

This is a simple gRPC server and client implementation in Go which receives a message from the client using an RPC, adds it to a RabbitMQ queue and publishes it to a `"messages"` subject in Nats. The client can then request the processed messages from the server using a server streaming RPC. The client can also subscribe to a Nats subject to receive the messages sent for processing.

I created this project to learn more about gRPC, RabbitMQ and NATS.

## How to use

1. Start the docker compose using `docker-compose up -d`
2. Run the server using `go run main.go`

You can now send messages for processing using `go run cli/main.go pm helloworld` with an optional `-n <note>` flag.

You can request the processed messages using `go run cli/main.go gpm` and the server will return all the messages processed.

You can also subscribe to a Nats subject using `go run cli/main.go stn <subject>` and the client will subscribe to the subject and print the messages received.

### Disclaimer

This is very simplified and doesn't make any sense because the proessing is just encoding a message, also, in case the processing was something more complex, there should be a job which takes the messages from the queue, processes them and adds them to a "processed" queue. Adding Nats to the mix is also unnecessary, but I wanted to learn more about it.
