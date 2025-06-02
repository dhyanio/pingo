package main

import (
	"context"
	"io"
	"log"
	"net"
	"sync"
	"time"

	pb "github.com/dhyanio/pingo/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type workerServer struct {
	pb.UnimplementedWorkerServiceServer
	mu sync.Mutex

	streams      map[string]pb.WorkerService_StreamTasksServer
	taskQueues   map[string][]*pb.TaskCommand
	loggerClient pb.LoggerServiceClient
}

func newWorkerServer(loggerClient pb.LoggerServiceClient) *workerServer {
	return &workerServer{
		streams:      make(map[string]pb.WorkerService_StreamTasksServer),
		taskQueues:   make(map[string][]*pb.TaskCommand),
		loggerClient: loggerClient,
	}
}

func (s *workerServer) StreamTasks(stream pb.WorkerService_StreamTasksServer) error {
	// First message must include worker_id for registration
	firstUpdate, err := stream.Recv()
	if err != nil {
		log.Printf("Failed to receive first update: %v", err)
		return err
	}
	workerID := firstUpdate.WorkerId
	log.Printf("Worker connected: %s", workerID)

	s.mu.Lock()
	s.streams[workerID] = stream
	s.mu.Unlock()

	// Goroutine to send taks to worker
	go func() {
		for {
			s.mu.Lock()
			queue := s.taskQueues[workerID]
			if len(queue) > 0 {
				task := queue[0]
				s.taskQueues[workerID] = queue[1:]
				s.mu.Unlock()

				log.Printf("Sending task %v to worker %s", task.TaskId, workerID)
				if err := stream.Send(task); err != nil {
					log.Printf("Error sending task to %s: %v", workerID, err)
					return
				}
			} else {
				s.mu.Unlock()
				time.Sleep(1 * time.Second)
			}
		}
	}()

	// Receive updates from worker
	for {
		update, err := stream.Recv()
		if err == io.EOF {
			log.Printf("Worker %s disconnected", workerID)
			break
		}
		if err != nil {
			log.Printf("Error receiving update from %s: %v", workerID, err)
			break
		}
		log.Printf("Update from worker %s: task %s status %s log %s time %v",
			update.WorkerId, update.TaskId, update.Status, update.Timestamp.AsTime())
	}

	s.mu.Lock()
	delete(s.streams, workerID)
	delete(s.taskQueues, workerID)
	s.mu.Unlock()

	return nil
}
func (s *workerServer) AddTask(ctx context.Context, req *pb.AddTaskRequest) (*pb.AddTaskRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.streams[req.WorkerId]; !ok {
		return nil, grpc.Errorf(14, "Worker %s not connected", req.WorkerId)
	}

	s.taskQueues[req.WorkerId] = append(s.taskQueues[req.WorkerId], req.Task)
	log.Printf("Task %s queued for worker %s", req.Task.TaskId, req.WorkerId)

	return &pb.AddTaskRequest{}, nil
}
func main() {
	conn, err := grpc.NewClient("localhost:50053", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to LoggerService: %v", err)
	}

	defer conn.Close()

	loggerClient := pb.NewLoggerServiceClient(conn)

	lis, err := net.Listen("tcp", ":50052")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterWorkerServiceServer(grpcServer, newWorkerServer(loggerClient))

	log.Println("WorkerService gRPC server listening on :50052")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
