package pipe

import (
	"context"
	"errors"
	"log"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	defaultBackpressureBytes = 5 * 1024 * 1024 // 5MB
	defaultIncomingQueue     = 8192
)

// SendFunc is provided by the transport (client/server stream) to actually write bytes.
type SendFunc func(msgType string, payload []byte) error

type PipeHandlerOptions struct {
	Compression                bool // enable compression (default snappy unless Codec set)
	Codec                      CompressionCodec
	BackpressureThresholdBytes int           // queue cap; 0 => default (5MB)
	Heartbeat                  bool          // send system_heartbeat
	HeartbeatInterval          time.Duration // default 5s

	// Advanced: control incoming worker pool & queue
	IncomingWorkers   int // 0 => auto (2 * GOMAXPROCS)
	IncomingQueueSize int // 0 => default (8192)

	// NEW: Application-level in-flight window (request/response style throttling)
	// If > 0, Post() will block while inFlight >= MaxInFlight.
	// A slot is released automatically when a message with type ∈ WindowReleaseOn arrives,
	// or immediately if the transport send fails.
	MaxInFlight     int
	WindowReleaseOn []string // e.g., []string{"pong"} for ping->pong patterns
}

type incoming struct {
	msgType string
	payload []byte
}

type outgoing struct {
	msgType string
	bytes   []byte
	size    int
}

type Handler func(data any)

type PipeHandler struct {
	// config
	compression    bool
	codec          CompressionCodec
	queueLimit     int
	heartbeat      bool
	heartbeatEvery time.Duration

	// NEW: window config
	maxInFlight int
	releaseOn   map[string]struct{}
	inFlight    int64 // current in-flight (increment on Post, decrement on release)

	// schema
	registry *SchemaRegistry

	// plumbing
	send          SendFunc
	serialization Serialization
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup

	// queues
	outQ  chan outgoing
	outMu sync.Mutex
	outSz int

	inQ            chan incoming
	incomingOnce   sync.Once
	incomingClosed bool

	// callbacks
	mu       sync.RWMutex
	handlers map[string][]Handler
	onAny    func(msgType string, data any)

	// ---- debug / metrics ----
	dbg struct {
		enqIn     int64 // incoming enqueued
		decOK     int64 // decode ok
		decFail   int64 // decode fail
		decompErr int64 // decompress fail
		sysDrop   int64 // system_* dropped
	}
}

func NewPipeHandler(
	parentCtx context.Context,
	serialization Serialization,
	registry *SchemaRegistry,
	send SendFunc,
	opt PipeHandlerOptions,
) *PipeHandler {
	if !serialization.IsValid() {
		panic("invalid serialization type: " + serialization.String())
	}
	ctx, cancel := context.WithCancel(parentCtx)

	inqSize := opt.IncomingQueueSize
	if inqSize <= 0 {
		inqSize = defaultIncomingQueue
	}
	workers := opt.IncomingWorkers
	if workers <= 0 {
		workers = 2 * runtime.GOMAXPROCS(0) // good default for CPU-bound decode
	}

	releaseSet := map[string]struct{}{}
	for _, t := range opt.WindowReleaseOn {
		t = strings.TrimSpace(t)
		if t != "" {
			releaseSet[t] = struct{}{}
		}
	}

	h := &PipeHandler{
		compression: opt.Compression,
		codec: func() CompressionCodec {
			if opt.Codec != "" {
				return opt.Codec
			}
			return Snappy
		}(),
		queueLimit:     ifZero(opt.BackpressureThresholdBytes, defaultBackpressureBytes),
		heartbeat:      opt.Heartbeat,
		heartbeatEvery: ifZeroDur(opt.HeartbeatInterval, 5*time.Second),

		maxInFlight: opt.MaxInFlight,
		releaseOn:   releaseSet,

		registry:      registry,
		send:          send,
		serialization: serialization,
		ctx:           ctx,
		cancel:        cancel,

		outQ:     make(chan outgoing, 1024),
		inQ:      make(chan incoming, inqSize),
		handlers: map[string][]Handler{},
	}

	// sender
	h.wg.Add(1)
	go h.runSender()

	// heartbeat
	if h.heartbeat {
		h.wg.Add(1)
		go h.runHeartbeat()
	}

	// incoming worker pool
	h.wg.Add(workers)
	for i := 0; i < workers; i++ {
		go h.runIncomingWorker()
	}

	// optional debug ticker
	if v := strings.TrimSpace(strings.ToLower(getenv("PIPE_DEBUG"))); v == "1" || v == "true" || v == "yes" {
		h.wg.Add(1)
		go func() {
			defer h.wg.Done()
			t := time.NewTicker(2 * time.Second)
			defer t.Stop()
			for {
				select {
				case <-h.ctx.Done():
					return
				case <-t.C:
					inQLen := len(h.inQ)
					outQLen := len(h.outQ)
					eIn := atomicLoad(&h.dbg.enqIn)
					ok := atomicLoad(&h.dbg.decOK)
					df := atomicLoad(&h.dbg.decFail)
					de := atomicLoad(&h.dbg.decompErr)
					sd := atomicLoad(&h.dbg.sysDrop)
					curIF := atomic.LoadInt64(&h.inFlight)
					println("[GO][PIPE] inQ=", inQLen, " outQ=", outQLen,
						" enqIn=", eIn, " decOK=", ok, " decFail=", df,
						" decompErr=", de, " sysDrop=", sd, " inFlight=", curIF, "/", h.maxInFlight)
				}
			}
		}()
	}

	return h
}

func getenv(k string) string {
	v, _ := syscall.Getenv(k)
	return v
}
func atomicLoad(p *int64) int64   { return atomic.LoadInt64(p) }
func atomicAdd(p *int64, d int64) { atomic.AddInt64(p, d) }

func ifZero(v, d int) int {
	if v == 0 {
		return d
	}
	return v
}

func ifZeroDur(v, d time.Duration) time.Duration {
	if v == 0 {
		return d
	}
	return v
}

func (h *PipeHandler) compressPayload(b []byte) ([]byte, error) {
	return CompressWith(b, h.codec)
}

func (h *PipeHandler) decompressPayload(b []byte) ([]byte, error) {
	return DecompressWith(b, h.codec)
}

func (h *PipeHandler) Serialization() Serialization { return h.serialization }

func (h *PipeHandler) On(msgType string, cb Handler) {
	h.mu.Lock()
	h.handlers[msgType] = append(h.handlers[msgType], cb)
	h.mu.Unlock()
}

func (h *PipeHandler) OnAny(cb func(msgType string, data any)) {
	h.mu.Lock()
	h.onAny = cb
	h.mu.Unlock()
}

// Post: encode (protobuf/JSON), optional snappy or gzip,
// apply application-level in-flight window (if configured),
// and enqueue with byte backpressure.
func (h *PipeHandler) Post(msgType string, v any) error {
	// --- window gating (application-level) ---
	if h.maxInFlight > 0 && !strings.HasPrefix(msgType, "system_") {
		for atomic.LoadInt64(&h.inFlight) >= int64(h.maxInFlight) {
			select {
			case <-h.ctx.Done():
				return h.ctx.Err()
			case <-time.After(50 * time.Microsecond):
			}
		}
	}

	b, err := h.encode(msgType, v)
	if err != nil {
		return err
	}
	if h.compression {
		if b, err = h.compressPayload(b); err != nil {
			return err
		}
	}

	// --- byte backpressure accounting ---
	h.outMu.Lock()
	if h.outSz+len(b) > h.queueLimit {
		for h.outSz+len(b) > h.queueLimit {
			h.outMu.Unlock()
			select {
			case <-h.ctx.Done():
				return h.ctx.Err()
			case <-time.After(100 * time.Microsecond):
			}
			h.outMu.Lock()
		}
	}
	h.outSz += len(b)
	h.outMu.Unlock()

	// increment in-flight *after* we know we’ll queue (non-system only)
	if h.maxInFlight > 0 && !strings.HasPrefix(msgType, "system_") {
		atomic.AddInt64(&h.inFlight, 1)
	}

	select {
	case h.outQ <- outgoing{msgType: msgType, bytes: b, size: len(b)}:
		return nil
	case <-h.ctx.Done():
		// rollback inFlight if queueing failed
		if h.maxInFlight > 0 && !strings.HasPrefix(msgType, "system_") {
			atomic.AddInt64(&h.inFlight, -1)
		}
		return h.ctx.Err()
	}
}

// Called by the transport reader when a raw message arrives.
// This MUST NOT block forever; use buffered channel.
func (h *PipeHandler) HandleIncoming(msgType string, payload []byte) {
	select {
	case h.inQ <- incoming{msgType: msgType, payload: payload}:
		atomicAdd(&h.dbg.enqIn, 1)
	case <-h.ctx.Done():
	default:
		// if full, block briefly (prefer not to drop)
		select {
		case h.inQ <- incoming{msgType: msgType, payload: payload}:
			atomicAdd(&h.dbg.enqIn, 1)
		case <-h.ctx.Done():
		}
	}
}

func (h *PipeHandler) runSender() {
	defer h.wg.Done()
	for {
		select {
		case <-h.ctx.Done():
			return
		case out := <-h.outQ:
			if err := h.send(out.msgType, out.bytes); err != nil {
				// If send fails, free the in-flight slot for non-system messages
				if h.maxInFlight > 0 && !strings.HasPrefix(out.msgType, "system_") {
					atomic.AddInt64(&h.inFlight, -1)
				}
				// optionally log/send error metrics
			}
			h.outMu.Lock()
			h.outSz -= out.size
			if h.outSz < 0 {
				h.outSz = 0
			}
			h.outMu.Unlock()
		}
	}
}

func (h *PipeHandler) runHeartbeat() {
	defer h.wg.Done()
	t := time.NewTicker(h.heartbeatEvery)
	defer t.Stop()
	for {
		select {
		case <-h.ctx.Done():
			return
		case <-t.C:
			_ = h.Post("system_heartbeat", []byte{}) // tiny; ignored by the peer
		}
	}
}

// multiple workers consume inQ concurrently
func (h *PipeHandler) runIncomingWorker() {
	defer h.wg.Done()
	for {
		select {
		case <-h.ctx.Done():
			return
		case in, ok := <-h.inQ:
			if !ok {
				return
			}

			// system_*: drop, but also release slot if someone (mis)counted on it
			if strings.HasPrefix(in.msgType, "system_") {
				atomicAdd(&h.dbg.sysDrop, 1)
				continue
			}

			raw := in.payload
			var err error
			if h.compression {
				raw, err = h.decompressPayload(raw)
				if err != nil {
					atomicAdd(&h.dbg.decompErr, 1)
					if v := getenv("PIPE_DEBUG_DECOMP"); v != "" {
						log.Printf("[GO][DECOMP][%s] err=%v len=%d", in.msgType, err, len(in.payload))
					}
					continue
				}
			}

			val, err := h.decode(in.msgType, raw)
			if err != nil {
				atomicAdd(&h.dbg.decFail, 1)
				if v := getenv("PIPE_DEBUG_DECODE"); v != "" {
					n := 32
					if len(raw) < n {
						n = len(raw)
					}
					log.Printf("[GO][DECODE][%s] err=%v len=%d first=% x", in.msgType, err, len(raw), raw[:n])
				}
				continue
			}

			atomicAdd(&h.dbg.decOK, 1)

			// NEW: release window slot if this type is configured
			if h.maxInFlight > 0 {
				if _, ok := h.releaseOn[in.msgType]; ok {
					atomic.AddInt64(&h.inFlight, -1)
				}
			}

			h.dispatch(in.msgType, val)
		}
	}
}

func (h *PipeHandler) dispatch(msgType string, v any) {
	h.mu.RLock()
	onAny := h.onAny
	hs := append([]Handler(nil), h.handlers[msgType]...)
	h.mu.RUnlock()

	if onAny != nil {
		onAny(msgType, v)
	}
	for _, cb := range hs {
		cb(v)
	}
}

// encode: prefer registry if present (JSON or Protobuf). Fallback to generic JSON only if no registry/entry.
func (h *PipeHandler) encode(msgType string, v any) ([]byte, error) {
	if h.registry != nil {
		b, ok, err := h.registry.Encode(msgType, v)
		if err != nil {
			return nil, err
		}
		if ok {
			return b, nil
		}
		if h.serialization == SerializationProtobuf {
			return nil, errors.New("no encoder for type: " + msgType)
		}
	} else if h.serialization == SerializationProtobuf {
		return nil, errors.New("protobuf serialization without registry")
	}
	return JSONEncode(v)
}

// decode: prefer registry if present. Fallback to generic JSON map only in JSON mode.
func (h *PipeHandler) decode(msgType string, b []byte) (any, error) {
	if h.registry != nil {
		v, ok, err := h.registry.Decode(msgType, b)
		if err != nil {
			return nil, err
		}
		if ok {
			return v, nil
		}
		if h.serialization == SerializationProtobuf {
			return nil, errors.New("no decoder for type: " + msgType)
		}
	} else if h.serialization == SerializationProtobuf {
		return nil, errors.New("protobuf serialization without registry")
	}
	var m map[string]any
	if err := JSONDecode(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func (h *PipeHandler) Close() {
	h.cancel()

	// close incoming so workers can exit if blocked on read
	h.incomingOnce.Do(func() {
		h.incomingClosed = true
		close(h.inQ)
	})

	h.wg.Wait()
}
