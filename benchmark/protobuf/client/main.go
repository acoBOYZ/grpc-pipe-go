// benchmark/protobuf/client/main.go
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/acoBOYZ/grpc-pipe-go/benchmark/protobuf"
	pb "github.com/acoBOYZ/grpc-pipe-go/benchmark/protobuf/gen"
	gc "github.com/acoBOYZ/grpc-pipe-go/client"
	"github.com/acoBOYZ/grpc-pipe-go/pipe"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

var (
	defaultAddrs      = []string{"localhost:50061", "localhost:50062", "localhost:50063"} // add more if you want
	messagesPerClient = 33333
)

// per-session (per reconnect) state
type runState struct {
	runId         int64
	addr          string
	start         time.Time
	expectedTotal int64

	// latency bookkeeping
	pending   sync.Map // id -> sendTimeMs
	latencies []int64
	latMu     sync.Mutex

	// counters
	totalReceived int64
}

func (rs *runState) nowMs() int64 { return time.Now().UnixNano() / int64(time.Millisecond) }

func (rs *runState) printResults(serialization pipe.Serialization, compression bool) {
	rs.latMu.Lock()
	defer rs.latMu.Unlock()

	if len(rs.latencies) == 0 {
		fmt.Printf("\n[Session %d @ %s] mode=%s compression=%v — no latencies measured.\n",
			rs.runId, rs.addr, serialization, compression)
		return
	}

	min := rs.latencies[0]
	max := rs.latencies[0]
	var sum int64
	for _, l := range rs.latencies {
		if l < min {
			min = l
		}
		if l > max {
			max = l
		}
		sum += l
	}
	avg := float64(sum) / float64(len(rs.latencies))
	elapsed := time.Since(rs.start).Seconds()
	throughput := float64(rs.expectedTotal) / elapsed

	fmt.Printf(
		"\n[Session %d @ %s] mode=%s compression=%v\nMessages sent: %d\nMessages received: %d\nMin latency: %d ms\nAvg latency: %.2f ms\nMax latency: %d ms\nThroughput: %.0f msg/s\n",
		rs.runId, rs.addr, serialization, compression,
		rs.expectedTotal, len(rs.latencies), min, avg, max, throughput,
	)
}

func main() {
	jsonMode := flag.Bool("json", false, "use JSON serialization (server must match)")
	compress := flag.Bool("compression", false, "enable gzip compression")
	addrCSV := flag.String("addrs", strings.Join(defaultAddrs, ","), "comma-separated server addresses")
	flag.Parse()

	addrs := splitAddrs(*addrCSV)
	if len(addrs) == 0 {
		log.Fatal("no addresses provided")
	}

	// Choose registry + serialization
	var reg *pipe.SchemaRegistry
	serialization := pipe.SerializationProtobuf
	if *jsonMode {
		reg = protobuf.NewBenchmarkRegistryJSON()
		serialization = pipe.SerializationJSON
	} else {
		reg = protobuf.NewBenchmarkClientRegistry() // registers "ping","pong" (lowercase)
	}

	// dial/channel options
	dialOpts := []grpc.DialOption{
		grpc.WithWriteBufferSize(4 << 20),
		grpc.WithReadBufferSize(4 << 20),
		grpc.WithInitialWindowSize(32 << 20),     // per-stream
		grpc.WithInitialConnWindowSize(64 << 20), // per-connection
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(64<<20),
			grpc.MaxCallSendMsgSize(64<<20),
		),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                20 * time.Second,
			Timeout:             10 * time.Second,
			PermitWithoutStream: true,
		}),
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var clients []*gc.Client
	var globalRunId int64 // incremented per (re)connect per address

	for _, addr := range addrs {
		addr := addr // capture

		client, err := gc.New(addr, gc.Options{
			DialOptions: dialOpts,
			Insecure:    true,
			Metadata: map[string]string{
				"clientId": "client_go:123",
			},

			Serialization:              serialization,
			Registry:                   reg,
			Compression:                *compress,
			BackpressureThresholdBytes: 1 << 30,
			Heartbeat:                  false,

			// on every connection (including reconnects), run a fresh session
			OnConnected: func(ph *pipe.PipeHandler) {
				runId := atomic.AddInt64(&globalRunId, 1)
				rs := &runState{
					runId:         runId,
					addr:          addr,
					start:         time.Now(),
					expectedTotal: int64(messagesPerClient),
				}

				log.Printf("[CLIENT] Connected to %s (session=%d, mode=%s, compression=%v)",
					addr, runId, serialization, *compress)

				// single handler for this connection (new handler on each reconnect)
				ph.On("pong", func(v any) {
					pong, ok := v.(*pb.Pong)
					if !ok || pong.Message == nil {
						return
					}
					id := pong.Message.Id

					// only count if it belongs to this session
					if !strings.HasPrefix(id, fmt.Sprintf("%s|%d|", addr, runId)) {
						return
					}

					if sentTimeAny, found := rs.pending.Load(id); found {
						if sentTime, ok := sentTimeAny.(int64); ok {
							elapsed := rs.nowMs() - sentTime
							rs.latMu.Lock()
							rs.latencies = append(rs.latencies, elapsed)
							rs.latMu.Unlock()
						}
						rs.pending.Delete(id)
					}

					if atomic.AddInt64(&rs.totalReceived, 1) == rs.expectedTotal {
						// print results for this session
						rs.printResults(serialization, *compress)
					}
				})

				// small delay to let server attach handlers
				time.AfterFunc(200*time.Millisecond, func() {
					for i := 0; i < messagesPerClient; i++ {
						id := fmt.Sprintf("%s|%d|%d", addr, runId, i) // addr|runId|i
						rs.pending.Store(id, rs.nowMs())
						if err := ph.Post("ping", &pb.Ping{Message: generateBigPayload(id)}); err != nil {
							log.Printf("[post][%s] %v", id, err)
						}
					}
				})
			},

			OnDisconnected: func() {
				log.Printf("[CLIENT] Disconnected from %s", addr)
			},
			OnError: func(where string, err error) {
				log.Printf("[CLIENT][%s][%s] %v", addr, where, err)
			},

			// reconnection behavior
			ReconnectBaseDelay: 2 * time.Second,
			MaxReconnectDelay:  30 * time.Second,
		})
		if err != nil {
			log.Printf("WARN: skip %s: client init error: %v", addr, err)
			continue
		}
		clients = append(clients, client)
		client.Start(ctx)
	}

	// run forever until Ctrl-C
	<-ctx.Done()
	log.Println("[MAIN] shutting down…")

	for _, c := range clients {
		c.Destroy()
	}
}

// ---------- helpers ----------

func splitAddrs(csv string) []string {
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		s := strings.TrimSpace(p)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// payload gen
func generateBigPayload(id string) *pb.UserProfile {
	return &pb.UserProfile{
		Id:       id,
		Username: "user_" + id,
		Email:    "user" + id + "@example.com",
		Bio:      "Lorem ipsum dolor sit amet, consectetur adipiscing elit.",
		Settings: &pb.UserProfile_Settings{
			Theme: func() string {
				if rand.Intn(2) == 0 {
					return "light"
				}
				return "dark"
			}(),
			Notifications: &pb.UserProfile_Settings_Notifications{
				Email: true,
				Sms:   false,
				Push:  true,
			},
		},
		Stats: &pb.UserProfile_Stats{
			Posts:     int32(rand.Intn(1000)),
			Followers: int32(rand.Intn(10000)),
			Following: int32(rand.Intn(500)),
			CreatedAt: time.Now().Format(time.RFC3339),
		},
		Posts: func() []*pb.Post {
			posts := make([]*pb.Post, 10)
			for i := 0; i < 10; i++ {
				posts[i] = &pb.Post{
					Id:      fmt.Sprintf("%s-%d", id, i),
					Title:   fmt.Sprintf("Post Title %d", i),
					Content: repeat("Content here...", 50),
					Likes:   int32(rand.Intn(500)),
					Tags:    []string{"benchmark", "test", "data"},
				}
			}
			return posts
		}(),
	}
}

func repeat(s string, n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteString(s)
	}
	return b.String()
}
