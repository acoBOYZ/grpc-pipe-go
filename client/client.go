package client

import (
	"context"
	"log"

	pb "github.com/acoBOYZ/grpc-pipe-go/gen"
	"google.golang.org/grpc"
)

type Stream struct {
	s pb.PipeService_CommunicateClient
}

func (st *Stream) Post(msgType string, payload []byte) error {
	return st.s.Send(&pb.PipeMessage{Type: msgType, Payload: payload})
}

func (st *Stream) Listen(onMessage func(msgType string, payload []byte)) error {
	for {
		msg, err := st.s.Recv()
		if err != nil {
			return err
		}
		onMessage(msg.Type, msg.Payload)
	}
}

type Client struct {
	cc pb.PipeServiceClient
}

func New(addr string) (*Client, error) {
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	return &Client{cc: pb.NewPipeServiceClient(conn)}, nil
}

func (c *Client) Connect(ctx context.Context) (*Stream, error) {
	s, err := c.cc.Communicate(ctx)
	if err != nil {
		return nil, err
	}
	log.Println("[CLIENT] Connected")
	return &Stream{s: s}, nil
}
