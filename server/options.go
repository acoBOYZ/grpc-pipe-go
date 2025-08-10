package server

import (
	"time"

	"github.com/acoBOYZ/grpc-pipe-go/pipe"
	"google.golang.org/grpc"
)

type Options struct {
	Host          string
	Port          int
	OnConnection  func(ph *pipe.PipeHandler)
	OnDisconnect  func(ph *pipe.PipeHandler)
	ServerOptions []grpc.ServerOption

	OnError func(where string, err error)

	// Pipe features
	Serialization              pipe.Serialization
	Registry                   *pipe.SchemaRegistry // required if protobuf
	Compression                bool
	Codec                      pipe.CompressionCodec
	BackpressureThresholdBytes int
	Heartbeat                  bool
	HeartbeatInterval          time.Duration

	// ➜ Newly exposed tuning knobs (0/empty => PipeHandler defaults)
	IncomingWorkers   int      // 0 => 2 * GOMAXPROCS
	IncomingQueueSize int      // 0 => 8192
	MaxInFlight       int      // 0 => disabled
	WindowReleaseOn   []string // e.g., []string{"pong"}
}

type Server struct{ opts Options }
