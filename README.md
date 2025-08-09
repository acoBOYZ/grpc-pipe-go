# grpc-pipe-go

> Ultra-fast, strongly-typed, multiplexed messaging over gRPC — in Go.  
> Pairs perfectly with the TypeScript implementation: https://github.com/acoBOYZ/grpc-pipe

- ⚡ High-throughput bidirectional streaming
- 🧩 Schema-driven (Protobuf) or schema-less (JSON)
- 🗜️ Optional gzip compression
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
)

func main() {
  reg := /* your SchemaRegistry (protobuf) or nil for JSON */
  srv := gs.New(gs.Options{
    Host:          "0.0.0.0",
    Port:          50061,
    Serialization: pipe.SerializationProtobuf, // or pipe.SerializationJSON
    Registry:      reg,
    Compression:   false,
    Heartbeat:     false,

    OnConnection: func(ph *pipe.PipeHandler) {
      log.Printf("[SERVER] client connected")
      ph.On("ping", func(v any) {
        if ping, ok := v.(*pb.Ping); ok {
          _ = ph.Post("pong", &pb.Pong{Message: ping.Message})
        }
      })
    },
    OnError: func(where string, err error) {
      log.Printf("[SERVER][%s] %v", where, err)
    },
  })

  log.Printf("[SERVER] Ready on :%d", 50061)
  if err := srv.Start(); err != nil {
    log.Fatalf("server failed: %v", err)
  }
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
)

func main() {
  reg := /* your SchemaRegistry (protobuf) or nil for JSON */

  client, err := gc.New("localhost:50061", gc.Options{
    Insecure: true,
    Metadata: map[string]string{
      "clientId": "client_go:123",
    },
    Serialization: pipe.SerializationProtobuf, // or JSON
    Registry:      reg,
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

| Field                       | Type                     | Default          | Purpose |
|----------------------------|--------------------------|------------------|---------|
| `DialOptions`              | `[]grpc.DialOption`      | –                | gRPC tuning |
| `Insecure`                 | `bool`                   | `false`          | Dev mode |
| `Serialization`            | `pipe.Serialization`     | `Protobuf`       | or JSON |
| `Registry`                 | `*pipe.SchemaRegistry`   | `nil`            | Required for Protobuf |
| `Compression`              | `bool`                   | `false`          | gzip |
| `BackpressureThresholdBytes` | `int`                  | `5<<20`          | Throttle |
| `Heartbeat`                | `bool`                   | `false`          | Enable heartbeat |
| `OnConnected`              | `func(*pipe.PipeHandler)`| –                | Connected hook |
| `OnDisconnected`           | `func()`                 | –                | Disconnected hook |
| `OnError`                  | `func(string, error)`    | –                | Error hook |
| `ReconnectBaseDelay`       | `time.Duration`          | `2s`             | Reconnect |
| `MaxReconnectDelay`        | `time.Duration`          | `30s`            | Reconnect cap |
| `Metadata`                 | `map[string]string`      | –                | Metadata |
| `IncomingWorkers`          | `int`                    | auto             | Worker pool |
| `IncomingQueueSize`        | `int`                    | `8192`           | Queue size |
| `MaxInFlight`              | `int`                    | 0                | Window size |
| `WindowReleaseOn`          | `[]string`               | –                | Release triggers |

Same semantics in the TS version: https://github.com/acoBOYZ/grpc-pipe

---

## JSON vs Protobuf

- JSON mode: no schema, human-readable
- Protobuf mode: registry-driven, compact & fast

---

## Interop with TypeScript

✅ Go ↔ Go  
✅ Go ↔ TS  
✅ TS ↔ TS  

TS repo: https://github.com/acoBOYZ/grpc-pipe

---

## Benchmarks (100k msgs, ~9 KB JSON payload)

### Protobuf, no compression

| Topology                                | Messages | Min (ms) | Avg (ms) | Max (ms) | Throughput |
|-----------------------------------------|---------:|---------:|---------:|---------:|-----------:|
| 3× Go servers → 1 Go client             | 3×33,333 | 0–1      | 6.86–7.74| 28–31    | ~24.3k msg/s per server |
| 3× Go servers → 1 TS client             | 99,999   | 21       | 2613     | 5192     | 19,186 msg/s |
| 3× TS servers → 1 Go client             | 3×33,333 | 1        | 71.5–82.6| 101–123  | ~12.3k msg/s per server |
| 3× TS servers → 1 TS client             | 99,999   | 23       | 2498     | 4949     | 20,108 msg/s |

### JSON, no compression (TS↔TS)

| Topology                        | Messages | Min (ms) | Avg (ms) | Max (ms) |
|--------------------------------|---------:|---------:|---------:|---------:|
| 3× TS servers → 1 TS client    | 99,999   | 62       | 2626     | 5117     |

---

## 📜 License
MIT — do whatever you want, but keep it fast ⚡
© ACO