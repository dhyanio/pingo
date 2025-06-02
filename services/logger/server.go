package main

import (
	"io"
	"log"
	"net"
	"time"

	pb "github.com/dhyanio/pingo/proto"
	"google.golang.org/grpc"
)

type loggerServer struct {
	pb.UnimplementedLoggerServiceServer
}

func (s *loggerServer) Log(stream pb.LoggerService_LogServer) error {
	for {
		entry, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&pb.LogAck{Success: true})
		}
		if err != nil {
			return err
		}

		t := time.Unix(entry.Timestamp, 0).Format(time.RFC3339)
		log.Printf("[Task: %s | %s] %s", entry.TaskId, t, entry.Message)
	}
}

func main() {
	lis, err := net.Listen("tcp", ":50053")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	pb.RegisterLoggerServiceServer(grpcServer, &loggerServer{})

	log.Printf("LoggerService gRPC server listening on :50053")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to server: %v", err)
	}
}
