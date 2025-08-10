package client

import (
	"context"
	"time"

	pb "github.com/acoBOYZ/grpc-pipe-go/gen"
	"github.com/acoBOYZ/grpc-pipe-go/pipe"
	"google.golang.org/grpc"
)

type Options struct {
	DialOptions []grpc.DialOption
	Insecure    bool

	// Pipe features
	Serialization              pipe.Serialization
	Registry                   *pipe.SchemaRegistry // required if protobuf
	Compression                bool
	Codec                      pipe.CompressionCodec
	BackpressureThresholdBytes int
	Heartbeat                  bool
	HeartbeatInterval          time.Duration

	OnConnected    func(ph *pipe.PipeHandler)
	OnDisconnected func()
	OnError        func(where string, err error)

	ReconnectBaseDelay time.Duration // default: 2s
	MaxReconnectDelay  time.Duration // default: 30s

	// Optional per-RPC metadata (TS `metadata`)
	Metadata map[string]string

	// ➜ Newly exposed tuning knobs (0/empty => PipeHandler defaults)
	IncomingWorkers   int      // 0 => 2 * GOMAXPROCS
	IncomingQueueSize int      // 0 => 8192
	MaxInFlight       int      // 0 => disabled
	WindowReleaseOn   []string // e.g., []string{"pong"}
}

type Client struct {
	addr string
	opts Options

	conn   *grpc.ClientConn
	cc     pb.PipeServiceClient
	stream pb.PipeService_CommunicateClient

	ctx    context.Context
	cancel context.CancelFunc

	shouldReconnect bool
	reconnectDelay  time.Duration

	handler *pipe.PipeHandler
}
