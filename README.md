# grpc-pipe-go

> Ultra-fast, strongly-typed, multiplexed messaging over gRPC — in Go.  
> Pairs perfectly with the TypeScript implementation: https://github.com/acoBOYZ/grpc-pipe

- ⚡ High-throughput bidirectional streaming
- 🧩 Schema-driven (Protobuf) or schema-less (JSON)
- 🗜️ Optional snappy/gzip compression
- 🫧 Built-in backpressure & in-flight windowing
- ❤️ Automatic reconnect (client) with exponential backoff
- 🔌 Drop-in interop with the TypeScript library

---

## Install

```bash
go get github.com/acoBOYZ/grpc-pipe-go
```

---

## Quick Start

### Server (Go → Go or TS clients)

```go
package main

import (
  "log"

  pb "github.com/acoBOYZ/grpc-pipe-go/gen"
  "github.com/acoBOYZ/grpc-pipe-go/pipe"
  gs "github.com/acoBOYZ/grpc-pipe-go/server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

func main() {
  reg := /* your SchemaRegistry (protobuf) or nil for JSON */
  srv := gs.New(gs.Options{
    Host:          "0.0.0.0",
    Port:          50061,
    Serialization: pipe.SerializationProtobuf, // or pipe.SerializationJSON
    Registry:      reg,
    Compression:   pipe.CompressionSnappy, // false | pipe.Snappy | pipe.Gzip (true means pipe.Snappy)
    Heartbeat:     false,
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
      log.Printf("[SERVER] client connected")
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

	log.Printf("[SERVER] Ready.")
	srv.Start()
}
```

### Client (Go → Go or TS servers)

```go
package main

import (
  "context"
  "log"
  "time"

  pb "github.com/acoBOYZ/grpc-pipe-go/gen"
  gc "github.com/acoBOYZ/grpc-pipe-go/client"
  "github.com/acoBOYZ/grpc-pipe-go/pipe"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

func main() {
  reg := /* your SchemaRegistry (protobuf) or nil for JSON */

  client, err := gc.New("localhost:50061", gc.Options{
    DialOptions: []grpc.DialOption{
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
    Insecure: true,
    Metadata: map[string]string{
      "clientId": "client_go:123",
    },
    Serialization: pipe.SerializationProtobuf, // or JSON
    Registry:      reg,
    Compression:   pipe.CompressionSnappy, // false | pipe.Snappy | pipe.Gzip (true means pipe.Snappy)
    BackpressureThresholdBytes: 5 << 20,
    OnConnected: func(ph *pipe.PipeHandler) {
      log.Println("[CLIENT] connected")
      ph.On("pong", func(v any) {
        if pong, ok := v.(*pb.Pong); ok {
          log.Printf("got pong: %s", pong.Message)
        }
      })
      _ = ph.Post("ping", &pb.Ping{Message: "Hello from Go"})
    },
    OnDisconnected: func() {
      log.Println("[CLIENT] disconnected")
    },
    OnError: func(where string, err error) {
      log.Printf("[CLIENT][%s] %v", where, err)
    },

		// reconnection behavior
    ReconnectBaseDelay: 2 * time.Second,
    MaxReconnectDelay:  30 * time.Second,
  })
  if err != nil {
    log.Fatal(err)
  }

  client.Start(context.Background())
  select {}
}
```

---

## Options (Client)

| Field                         | Type                      | Default          | Purpose |
|--------------------------------|---------------------------|------------------|---------|
| `DialOptions`                  | `[]grpc.DialOption`       | –                | gRPC tuning |
| `Insecure`                     | `bool`                    | `false`          | Dev mode |
| `Serialization`                | `pipe.Serialization`      | `Protobuf`       | or JSON |
| `Registry`                     | `*pipe.SchemaRegistry`    | `nil`            | Required for Protobuf |
| `Compression`                  | `pipe.Compression`        | `false`          | snappy or disabled |
| `Codec`                        | `pipe.CompressionCodec`   | Snappy / Gzip    | snappy or gzip |
| `BackpressureThresholdBytes`   | `int`                     | `5<<20`          | Throttle |
| `Heartbeat`                    | `bool`                    | `false`          | Enable heartbeat |
| `OnConnected`                  | `func(*pipe.PipeHandler)` | –                | Connected hook |
| `OnDisconnected`               | `func()`                  | –                | Disconnected hook |
| `OnError`                      | `func(string, error)`     | –                | Error hook |
| `ReconnectBaseDelay`           | `time.Duration`           | `2s`             | Reconnect |
| `MaxReconnectDelay`            | `time.Duration`           | `30s`            | Reconnect cap |
| `Metadata`                     | `map[string]string`       | –                | Metadata |
| `IncomingWorkers`              | `int`                     | auto             | Worker pool |
| `IncomingQueueSize`            | `int`                     | `8192`           | Queue size |
| `MaxInFlight`                  | `int`                     | 0                | Window size |
| `WindowReleaseOn`              | `[]string`                | –                | Release triggers |

---

## JSON vs Protobuf

- **JSON mode:** no schema, human-readable
- **Protobuf mode:** registry-driven, compact & fast

---

## Interop with TypeScript

✅ Go ↔ Go  
✅ Go ↔ TS  
✅ TS ↔ TS  

- TS repo: https://github.com/acoBOYZ/grpc-pipe

---

## Benchmarks (100k msgs, ~9 KB JSON payload)

> **Note:** All tests are **3 servers → 1 client**.  
> **Thpt\***: When Go is the client → throughput per server. When TS is the client → combined throughput from all servers.

---

### Protobuf (no compression)

| Servers→Client | Messages   | Min   | Avg        | Max     | Thpt*    |
|----------------|-----------:|------:|-----------:|--------:|---------:|
| Go→Go          | 3×33,333   | 0–1   | 6.86–7.74  | 28–31   | ~24.3k/s |
| Go→TS          | 99,999     | 21    | 2613       | 5192    | 19.2k/s  |
| TS→Go          | 3×33,333   | 1     | 71.5–82.6  | 101–123 | ~12.3k/s |
| TS→TS          | 99,999     | 25    | 2501       | 4931    | 20.2k/s  |

---

### JSON (GO↔GO)

| Servers→Client | Compression | Messages   | Min   | Avg    | Max       | Thpt*    |
|----------------|-------------|-----------:|------:|-------:|----------:|---------:|
| Go→Go          | none        | 3×33,333   | 0–3   | 50–62  | 142–169   | ~13.8k/s |
| Go→Go          | snappy      | 3×33,333   | 0     | 15–18  | 69–91     | ~13.3k/s |

---

### JSON (TS↔TS)

| Servers→Client | Compression | Messages   | Min   | Avg    | Max     | Thpt*   |
|----------------|-------------|-----------:|------:|-------:|--------:|--------:|
| TS→TS          | none        | 99,999     | 62    | 2626   | 5117    |  —      |
| TS→TS          | snappy      | 99,999     | 67    | 2574   | 4964    |  —      |

---

### JSON (TS↔GO)

| Servers→Client | Compression | Messages   | Min   | Avg    | Max       | Thpt*    |
|----------------|-------------|-----------:|------:|-------:|----------:|---------:|
| TS→Go          | snappy      | 3×33,333   | 2     | 70     | 100–118   | ~14.4k/s |
| TS→Go          | none        | 3×33,333   | 1     | 70     | 87        | ~13.3k/s |

---

### JSON (GO↔TS)

| Servers→Client | Compression | Messages   | Min   | Avg    | Max     | Thpt*   |
|----------------|-------------|-----------:|------:|-------:|--------:|--------:|
| Go→TS          | snappy      | 99,999     | 69    | 2592   | 5133    |  —      |
| Go→TS          | none        | 99,999     | 61    | 2370   | 4601    |  —      |

---

## 📜 License
MIT — do whatever you want, but keep it fast ⚡  
© ACO