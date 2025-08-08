package pipe

import (
	"sync"
)

type Handler func(data any)

type Pipe struct {
	mu            sync.RWMutex
	handlers      map[string][]Handler
	serialization string // "protobuf" or "json"
	Context       any
}

func NewPipe(serialization string, ctx any) *Pipe {
	return &Pipe{
		handlers:      map[string][]Handler{},
		serialization: serialization,
		Context:       ctx,
	}
}

func (p *Pipe) On(msgType string, h Handler) {
	p.mu.Lock()
	p.handlers[msgType] = append(p.handlers[msgType], h)
	p.mu.Unlock()
}

func (p *Pipe) Emit(msgType string, data any) {
	p.mu.RLock()
	hs := p.handlers[msgType]
	p.mu.RUnlock()
	for _, h := range hs {
		h(data)
	}
}

func (p *Pipe) Serialization() string { return p.serialization }
