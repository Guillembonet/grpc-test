# grpc-test

## Description

This is a simple gRPC server and client implementation in Go which receives a message from the client using an RPC and adds it to a RabbitMQ queue. The client can then request the processed messages from the server using a server streaming RPC.

I created this project to learn more about gRPC and RabbitMQ.