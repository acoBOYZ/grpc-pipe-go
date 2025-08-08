package main

import (
	"fmt"

	"github.com/acoBOYZ/grpc-pipe-go/gen"
	"github.com/acoBOYZ/grpc-pipe-go/pipe"
)

func main() {
	reg := pipe.NewSchemaRegistry()

	// Register a protobuf message type
	pipe.RegisterMessage(reg, "PipeMessage", func() *gen.PipeMessage {
		return &gen.PipeMessage{}
	})

	// Encode
	data, _, _ := reg.Encode("PipeMessage", &gen.PipeMessage{
		Type:    "chat",
		Payload: []byte("hello"),
	})

	// Decode
	msg, _, _ := reg.Decode("PipeMessage", data)
	pm := msg.(*gen.PipeMessage)
	fmt.Println(pm.Type, string(pm.Payload))
}
