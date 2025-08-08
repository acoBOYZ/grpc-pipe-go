package protobuf

import (
	"fmt"
	"log"
	"net"

	pb "github.com/acoBOYZ/grpc-pipe-go/benchmark/protobuf/gen"
	"github.com/acoBOYZ/grpc-pipe-go/pipe"
	"google.golang.org/grpc"
)

type server struct {
	pb.UnimplementedPipeServiceServer
	registry *pipe.SchemaRegistry
}

func (s *server) Communicate(stream pb.PipeService_CommunicateServer) error {
	for {
		msg, err := stream.Recv()
		if err != nil {
			return err
		}

		decoded, _, _ := s.registry.Decode(msg.Type, msg.Payload)
		switch v := decoded.(type) {
		case *pb.Ping:
			stream.Send(&pb.PipeMessage{
				Type:    "Pong",
				Payload: mustEncode(s.registry, "Pong", &pb.Pong{Message: v.Message}),
			})
		}
	}
}

func mustEncode(reg *pipe.SchemaRegistry, t string, v any) []byte {
	b, _, err := reg.Encode(t, v)
	if err != nil {
		panic(err)
	}
	return b
}

func main() {
	port := 50061
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	s := grpc.NewServer()
	reg := newBenchmarkServerRegistry()
	pb.RegisterPipeServiceServer(s, &server{registry: reg})

	log.Printf("[SERVER %d] Ready.", port)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
