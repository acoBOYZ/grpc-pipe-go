// benchmark/protobuf/schema.go
package protobuf

import (
	pb "github.com/acoBOYZ/grpc-pipe-go/benchmark/protobuf/gen"
	"github.com/acoBOYZ/grpc-pipe-go/pipe"
)

func NewBenchmarkClientRegistry() *pipe.SchemaRegistry {
	reg := pipe.NewSchemaRegistry()
	pipe.RegisterMessage(reg, "ping", func() *pb.Ping { return &pb.Ping{} })
	pipe.RegisterMessage(reg, "pong", func() *pb.Pong { return &pb.Pong{} })
	return reg
}

func NewBenchmarkServerRegistry() *pipe.SchemaRegistry {
	reg := pipe.NewSchemaRegistry()
	pipe.RegisterMessage(reg, "ping", func() *pb.Ping { return &pb.Ping{} })
	pipe.RegisterMessage(reg, "pong", func() *pb.Pong { return &pb.Pong{} })
	return reg
}
