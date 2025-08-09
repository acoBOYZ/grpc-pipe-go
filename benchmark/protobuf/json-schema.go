// benchmark/protobuf/registry_json.go
package protobuf

import (
	pb "github.com/acoBOYZ/grpc-pipe-go/benchmark/protobuf/gen"
	"github.com/acoBOYZ/grpc-pipe-go/pipe"
)

func NewBenchmarkRegistryJSON() *pipe.SchemaRegistry {
	reg := pipe.NewSchemaRegistry()

	reg.Register("ping",
		func(v any) ([]byte, error) { return pipe.JSONEncode(v) },
		func(b []byte) (any, error) {
			var m pb.Ping
			if err := pipe.JSONDecode(b, &m); err != nil {
				return nil, err
			}
			return &m, nil
		},
	)

	reg.Register("pong",
		func(v any) ([]byte, error) { return pipe.JSONEncode(v) },
		func(b []byte) (any, error) {
			var m pb.Pong
			if err := pipe.JSONDecode(b, &m); err != nil {
				return nil, err
			}
			return &m, nil
		},
	)

	return reg
}
