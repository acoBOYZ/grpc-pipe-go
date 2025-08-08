package pipe

import (
	"fmt"

	"google.golang.org/protobuf/proto"
)

type Encoder func(any) ([]byte, error)
type Decoder func([]byte) (any, error)

type SchemaRegistry struct {
	enc map[string]Encoder
	dec map[string]Decoder
}

func NewSchemaRegistry() *SchemaRegistry {
	return &SchemaRegistry{enc: map[string]Encoder{}, dec: map[string]Decoder{}}
}

func (r *SchemaRegistry) Register[T proto.Message](msgType string, factory func() T) {
	r.enc[msgType] = func(v any) ([]byte, error) {
		pb, ok := v.(proto.Message)
		if !ok {
			return nil, fmt.Errorf("type %T is not proto.Message for %s", v, msgType)
		}
		return proto.Marshal(pb)
	}
	r.dec[msgType] = func(b []byte) (any, error) {
		m := factory()
		if err := proto.Unmarshal(b, m); err != nil {
			return nil, err
		}
		return m, nil
	}
}

func (r *SchemaRegistry) Encode(t string, v any) ([]byte, bool, error) {
	enc, ok := r.enc[t]
	if !ok {
		return nil, false, nil
	}
	b, err := enc(v)
	return b, true, err
}

func (r *SchemaRegistry) Decode(t string, b []byte) (any, bool, error) {
	dec, ok := r.dec[t]
	if !ok {
		return nil, false, nil
	}
	m, err := dec(b)
	return m, true, err
}