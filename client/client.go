package client

import (
	"context"
	"log"
	"time"

	pb "github.com/acoBOYZ/grpc-pipe-go/gen"
	"github.com/acoBOYZ/grpc-pipe-go/pipe"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

func New(addr string, opts Options) (*Client, error) {
	if opts.ReconnectBaseDelay == 0 {
		opts.ReconnectBaseDelay = 2 * time.Second
	}
	if opts.MaxReconnectDelay == 0 {
		opts.MaxReconnectDelay = 30 * time.Second
	}
	return &Client{
		addr:            addr,
		opts:            opts,
		shouldReconnect: true,
		reconnectDelay:  opts.ReconnectBaseDelay,
	}, nil
}

func (c *Client) Start(parent context.Context) {
	if c.ctx != nil {
		return
	}
	c.ctx, c.cancel = context.WithCancel(parent)
	go c.connectLoop()
}

func (c *Client) connectLoop() {
	for c.shouldReconnect {
		if err := c.connectOnce(); err != nil {
			c.emitErr("connect", err)
			c.sleepBackoff()
			continue
		}
		// Block reading from stream until error/EOF
		if err := c.recvLoop(); err != nil {
			c.emitErr("recv", err)
		}
		c.teardownStream()

		if c.opts.OnDisconnected != nil {
			c.opts.OnDisconnected()
		}
		c.sleepBackoff()
	}
}

func (c *Client) connectOnce() error {
	// creds + dial options
	dialOpts := append([]grpc.DialOption(nil), c.opts.DialOptions...)
	if c.opts.Insecure {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	// dial
	conn, err := grpc.DialContext(c.ctx, c.addr, dialOpts...)
	if err != nil {
		return err
	}
	c.conn = conn
	c.cc = pb.NewPipeServiceClient(conn)

	// metadata
	md := metadata.New(nil)
	for k, v := range c.opts.Metadata {
		md.Set(k, v)
	}
	ctx := metadata.NewOutgoingContext(c.ctx, md)

	// open bidi
	stream, err := c.cc.Communicate(ctx)
	if err != nil {
		return err
	}
	c.stream = stream

	// build pipe handler
	ph := pipe.NewPipeHandler(
		c.ctx,
		c.opts.Serialization,
		c.opts.Registry,
		func(msgType string, payload []byte) error {
			return c.stream.Send(&pb.PipeMessage{Type: msgType, Payload: payload})
		},
		pipe.PipeHandlerOptions{
			Compression:                c.opts.Compression,
			BackpressureThresholdBytes: c.opts.BackpressureThresholdBytes,
			Heartbeat:                  c.opts.Heartbeat,
			HeartbeatInterval:          c.opts.HeartbeatInterval,
			IncomingWorkers:            c.opts.IncomingWorkers,
			IncomingQueueSize:          c.opts.IncomingQueueSize,
			MaxInFlight:                c.opts.MaxInFlight,
			WindowReleaseOn:            c.opts.WindowReleaseOn,
		},
	)

	c.handler = ph
	return nil
}

func (c *Client) recvLoop() error {
	readyFired := false
	for {
		msg, err := c.stream.Recv()
		if err != nil {
			return err
		}
		// fire OnConnected only after server says ready
		if !readyFired && msg.Type == "system_ready" {
			readyFired = true
			c.reconnectDelay = c.opts.ReconnectBaseDelay
			if c.opts.OnConnected != nil && c.handler != nil {
				c.opts.OnConnected(c.handler)
			}
			continue // do not forward system_* to handler
		}
		c.onIncoming(msg)
	}
}

func (c *Client) onIncoming(msg *pb.PipeMessage) {
	if c.handler != nil {
		c.handler.HandleIncoming(msg.Type, msg.Payload)
	}
}

func (c *Client) sleepBackoff() {
	if !c.shouldReconnect || c.ctx.Err() != nil {
		return
	}
	d := c.reconnectDelay
	// exponential backoff capped at MaxReconnectDelay
	c.reconnectDelay *= 2
	if c.reconnectDelay > c.opts.MaxReconnectDelay {
		c.reconnectDelay = c.opts.MaxReconnectDelay
	}
	timer := time.NewTimer(d)
	select {
	case <-timer.C:
	case <-c.ctx.Done():
	}
}

func (c *Client) emitErr(where string, err error) {
	if c.opts.OnError != nil {
		c.opts.OnError(where, err)
	} else {
		log.Printf("[CLIENT][%s] %v", where, err)
	}
}

func (c *Client) teardownStream() {
	if c.stream != nil {
		_ = c.stream.CloseSend()
		c.stream = nil
	}
	if c.handler != nil {
		c.handler.Close()
		c.handler = nil
	}
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
}

// Destroy stops reconnection and tears everything down.
func (c *Client) Destroy() {
	c.shouldReconnect = false
	if c.cancel != nil {
		c.cancel()
	}
	c.teardownStream()
}
