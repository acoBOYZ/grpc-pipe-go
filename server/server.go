package server

import (
	"fmt"
	"log"
	"net"

	pb "github.com/acoBOYZ/grpc-pipe-go/gen"
	"github.com/acoBOYZ/grpc-pipe-go/pipe"
	"google.golang.org/grpc"
)

func New(opts Options) *Server { return &Server{opts: opts} }

func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.opts.Host, s.opts.Port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		if s.opts.OnError != nil {
			s.opts.OnError("listen", err)
		}
		return err
	}
	grpcServer := grpc.NewServer(s.opts.ServerOptions...)
	pb.RegisterPipeServiceServer(grpcServer, &pipeService{parent: s})
	log.Printf("[SERVER] Listening on %s", addr)

	if err := grpcServer.Serve(lis); err != nil {
		if s.opts.OnError != nil {
			s.opts.OnError("serve", err)
		}
		return err
	}
	return nil
}

type pipeService struct {
	pb.UnimplementedPipeServiceServer
	parent *Server
}

func (p *pipeService) Communicate(stream pb.PipeService_CommunicateServer) error {
	ctx := stream.Context()

	// Build PipeHandler
	handler := pipe.NewPipeHandler(
		ctx,
		p.parent.opts.Serialization,
		p.parent.opts.Registry,
		func(msgType string, payload []byte) error {
			return stream.Send(&pb.PipeMessage{Type: msgType, Payload: payload})
		},
		pipe.PipeHandlerOptions{
			Compression:                p.parent.opts.Compression,
			BackpressureThresholdBytes: p.parent.opts.BackpressureThresholdBytes,
			Heartbeat:                  p.parent.opts.Heartbeat,
			HeartbeatInterval:          p.parent.opts.HeartbeatInterval,
			IncomingWorkers:            p.parent.opts.IncomingWorkers,
			IncomingQueueSize:          p.parent.opts.IncomingQueueSize,
			MaxInFlight:                p.parent.opts.MaxInFlight,
			WindowReleaseOn:            p.parent.opts.WindowReleaseOn,
		},
	)

	if p.parent.opts.OnConnection != nil {
		p.parent.opts.OnConnection(handler)
	}

	if err := stream.Send(&pb.PipeMessage{Type: "system_ready", Payload: []byte{}}); err != nil {
		if p.parent.opts.OnError != nil {
			p.parent.opts.OnError("system_ready_send", err)
		}
		if p.parent.opts.OnDisconnect != nil {
			p.parent.opts.OnDisconnect(handler)
		}
		handler.Close()
		return err
	}

	// recv loop -> handler.HandleIncoming
	for {
		msg, err := stream.Recv()
		if err != nil {
			if p.parent.opts.OnError != nil {
				p.parent.opts.OnError("recv", err)
			}
			if p.parent.opts.OnDisconnect != nil {
				p.parent.opts.OnDisconnect(handler)
			}
			handler.Close()
			return err
		}
		handler.HandleIncoming(msg.Type, msg.Payload)
	}
}
