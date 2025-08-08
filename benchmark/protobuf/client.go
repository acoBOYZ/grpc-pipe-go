// benchmark/protobuf/client.go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	pb "github.com/acoBOYZ/grpc-pipe-go/benchmark/protobuf/gen"
	com "github.com/acoBOYZ/grpc-pipe-go/gen"
	"github.com/acoBOYZ/grpc-pipe-go/pipe"
	"google.golang.org/grpc"
)

func newBenchmarkClientRegistry() *pipe.SchemaRegistry {
	reg := pipe.NewSchemaRegistry()
	// register typed messages from benchmark.proto with our schema registry
	pipe.RegisterMessage(reg, "Ping", func() *pb.Ping { return &pb.Ping{} })
	pipe.RegisterMessage(reg, "Pong", func() *pb.Pong { return &pb.Pong{} })
	return reg
}

func main() {
	serverAddresses := []string{
		"localhost:50061",
	}

	reg := newBenchmarkClientRegistry()
	latencies := []int64{}
	messagesPerClient := 1000

	for _, addr := range serverAddresses {
		conn, err := grpc.Dial(addr, grpc.WithInsecure())
		if err != nil {
			log.Fatalf("Failed to connect %s: %v", addr, err)
		}
		defer conn.Close()

		// USE OUR grpc-pipe transport service (com.PipeService), not benchmark.PipeService
		client := com.NewPipeServiceClient(conn)

		stream, err := client.Communicate(context.Background())
		if err != nil {
			log.Fatalf("Failed to open stream: %v", err)
		}

		// recv loop
		go func() {
			for {
				msg, err := stream.Recv()
				if err != nil {
					log.Println("Recv error:", err)
					return
				}
				start := time.Now()
				decoded, _, _ := reg.Decode(msg.Type, msg.Payload)
				if _, ok := decoded.(*pb.Pong); ok {
					latencies = append(latencies, time.Since(start).Milliseconds())
				}
			}
		}()

		// send loop
		for i := 0; i < messagesPerClient; i++ {
			payload := generateBigPayload(fmt.Sprintf("%s-%d", addr, i))
			data, _, err := reg.Encode("Ping", &pb.Ping{Message: payload})
			if err != nil {
				log.Fatalf("encode error: %v", err)
			}
			if err := stream.Send(&com.PipeMessage{Type: "Ping", Payload: data}); err != nil {
				log.Fatalf("send error: %v", err)
			}
		}
	}

	// give some time to receive replies, then print
	time.Sleep(3 * time.Second)
	printResults(latencies)
}

func printResults(latencies []int64) {
	if len(latencies) == 0 {
		fmt.Println("No latencies measured.")
		return
	}
	min, max, sum := latencies[0], latencies[0], int64(0)
	for _, l := range latencies {
		if l < min {
			min = l
		}
		if l > max {
			max = l
		}
		sum += l
	}
	fmt.Printf("Messages: %d\nMin: %d ms\nAvg: %.2f ms\nMax: %d ms\n",
		len(latencies), min, float64(sum)/float64(len(latencies)), max)
}

func generateBigPayload(id string) *pb.UserProfile {
	return &pb.UserProfile{
		Id:       id,
		Username: "user_" + id,
		Email:    "user" + id + "@example.com",
		Bio:      "Lorem ipsum dolor sit amet",
		// Fill the rest if you want to stress size:
		// Settings: &pb.UserProfile_Settings{
		// 	Theme: "dark",
		// 	Notifications: &pb.UserProfile_Settings_Notifications{
		// 		Email: true, Sms: false, Push: true,
		// 	},
		// },
		// Stats: &pb.UserProfile_Stats{
		// 	Posts: 100, Followers: 1000, Following: 50, CreatedAt: time.Now().Format(time.RFC3339),
		// },
		// Posts: []*pb.Post{ ... },
	}
}
