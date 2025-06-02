package main

import (
	"context"
	"log"
	"time"

	pb "github.com/dhyanio/pingo/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func main() {
	conn, err := grpc.NewClient("localhost:50052", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to dial worker service: %v", err)
	}
	defer conn.Close()

	client := pb.NewWorkerServiceClient(conn)

	stream, err := client.StreamTasks(context.Background())
	if err != nil {
		log.Fatalf("Failed to start stream: %v", err)
	}

	workerID := "worker-1"

	// Send first update to register
	err = stream.Send(&pb.WorkerUpdate{
		WorkerId:  workerID,
		Timestamp: timestamppb.Now(),
	})
	if err != nil {
		log.Fatalf("Failed to send first update: %v", err)

	}

	// Listen for tasks from server
	go func() {
		for {
			taskCmd, err := stream.Recv()
			if err != nil {
				log.Fatalf("Error receiving task: %v", err)
			}
			log.Printf("Received task: %s command: %s", taskCmd.TaskId, taskCmd.Commnad)

			// Simulate task running
			time.Sleep(2 * time.Second)
			// Send update task finished
			err = stream.Send(&pb.WorkerUpdate{
				WorkerId:  workerID,
				TaskId:    taskCmd.TaskId,
				Status:    "SUCCESS",
				Log:       "Task completed successfully",
				Timestamp: timestamppb.Now(),
			})
			if err != nil {
				log.Fatalf("Failed to send update: %v", err)
			}
		}
	}()

	// Keep client alive
	select {}

}
