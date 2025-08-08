package server

import (
	"fmt"
	"log"
	"net"

	pb "github.com/acoBOYZ/grpc-pipe-go/gen"
	"google.golang.org/grpc"
)

type Pipe struct {
	post func(msgType string, payload []byte) error
}

func (p *Pipe) Post(msgType string, payload []byte) error {
	return p.post(msgType, payload)
}

type Options struct {
	Host         string
	Port         int
	OnConnection func(pipe *Pipe)
	OnMessage    func(pipe *Pipe, msgType string, payload []byte)
}

type Server struct {
	opts Options
}

func New(opts Options) *Server { return &Server{opts: opts} }

func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.opts.Host, s.opts.Port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	grpcServer := grpc.NewServer()
	pb.RegisterPipeServiceServer(grpcServer, &pipeService{
		parent: s,
	})
	log.Printf("[SERVER] Listening on %s", addr)
	return grpcServer.Serve(lis)
}

type pipeService struct {
	pb.UnimplementedPipeServiceServer
	parent *Server
}

func (p *pipeService) Communicate(stream pb.PipeService_CommunicateServer) error {
	pipe := &Pipe{
		post: func(msgType string, payload []byte) error {
			return stream.Send(&pb.PipeMessage{Type: msgType, Payload: payload})
		},
	}
	if p.parent.opts.OnConnection != nil {
		p.parent.opts.OnConnection(pipe)
	}

	for {
		msg, err := stream.Recv()
		if err != nil {
			return err
		}
		if p.parent.opts.OnMessage != nil {
			p.parent.opts.OnMessage(pipe, msg.Type, msg.Payload)
		}
	}
}
