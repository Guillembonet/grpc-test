# grpc-test

## Description

This is a simple gRPC server and client implementation in Go which receives a message from the client using an RPC and adds it to a RabbitMQ queue. The client can then request the processed messages from the server using a server streaming RPC.

I created this project to learn more about gRPC and RabbitMQ.

## How to use

1. Start the docker compose using `docker-compose up -d`
2. Run the server using `go run main.go`

You can now send messages for processing using `go run cli/main.go pm helloworld` with an optional `-n <note>` flag.

You can request the processed messages using `go run cli/main.go gpm` and the server will return all the messages processed.

### Disclaimer

This is very simplified and doesn't make any sense because the proessing is just encoding a message, also, in case the processing was something more complex, there should be a job which takes the messages from the queue, processes them and adds them to a "processed" queue.
