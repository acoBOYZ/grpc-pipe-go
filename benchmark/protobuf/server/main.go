// benchmark/protobuf/server/main.go
package main

import (
	"flag"
	"log"
	"time"

	"github.com/acoBOYZ/grpc-pipe-go/benchmark/protobuf"
	pb "github.com/acoBOYZ/grpc-pipe-go/benchmark/protobuf/gen"
	"github.com/acoBOYZ/grpc-pipe-go/pipe"
	gs "github.com/acoBOYZ/grpc-pipe-go/server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

func main() {
	port := flag.Int("port", 50061, "server port")
	useJSON := flag.Bool("json", false, "use JSON serialization")
	useCompression := flag.Bool("compression", false, "enable gzip compression")
	flag.Parse()

	var reg *pipe.SchemaRegistry
	serialization := pipe.SerializationProtobuf
	if *useJSON {
		reg = protobuf.NewBenchmarkRegistryJSON()
		serialization = pipe.SerializationJSON
	} else {
		reg = protobuf.NewBenchmarkServerRegistry()
	}

	srv := gs.New(gs.Options{
		Host:          "0.0.0.0",
		Port:          *port,
		Serialization: serialization,
		Registry:      reg,
		Compression:   false,
		// Codec:             pipe.Snappy,
		Heartbeat:         false, // keep disabled for clean benchmark numbers
		HeartbeatInterval: 5 * time.Second,
		ServerOptions: []grpc.ServerOption{
			grpc.WriteBufferSize(1 << 20),
			grpc.ReadBufferSize(1 << 20),
			grpc.InitialWindowSize(1 << 20),
			grpc.InitialConnWindowSize(1 << 20),
			grpc.KeepaliveParams(keepalive.ServerParameters{
				Time:    10 * time.Second,
				Timeout: 5 * time.Second,
			}),
			grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
				MinTime:             20 * time.Second,
				PermitWithoutStream: true,
			}),
		},

		OnConnection: func(ph *pipe.PipeHandler) {
			log.Printf("[SERVER] Client connected; serialization=%s compression=%v", ph.Serialization(), *useCompression)
			ph.On("ping", func(v any) {
				if ping, ok := v.(*pb.Ping); ok {
					_ = ph.Post("pong", &pb.Pong{Message: ping.Message})
				}
			})
		},
		OnDisconnect: func(ph *pipe.PipeHandler) {
			log.Printf("[SERVER] Client disconnected")
		},
		OnError: func(where string, err error) {
			log.Printf("[SERVER][%s] %v", where, err)
		},
	})

	log.Printf("[SERVER] Ready on port %d (mode=%s compression=%v).", *port, serialization, *useCompression)
	srv.Start()
}
