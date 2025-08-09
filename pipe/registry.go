package pipe

import (
	"fmt"

	"google.golang.org/protobuf/proto"
)

// Encoder encodes any value into bytes
type Encoder func(any) ([]byte, error)

// Decoder decodes bytes into a value
type Decoder func([]byte) (any, error)

// SchemaRegistry holds encoders and decoders for different message types
type SchemaRegistry struct {
	enc map[string]Encoder
	dec map[string]Decoder
}

// NewSchemaRegistry creates a new SchemaRegistry
func NewSchemaRegistry() *SchemaRegistry {
	return &SchemaRegistry{
		enc: map[string]Encoder{},
		dec: map[string]Decoder{},
	}
}

// Register is a non-generic method that stores the encoder/decoder
func (r *SchemaRegistry) Register(msgType string, enc Encoder, dec Decoder) {
	r.enc[msgType] = enc
	r.dec[msgType] = dec
}

// RegisterMessage is a generic helper function for proto.Message types
func RegisterMessage[T proto.Message](r *SchemaRegistry, msgType string, factory func() T) {
	r.Register(
		msgType,
		func(v any) ([]byte, error) {
			pb, ok := v.(proto.Message)
			if !ok {
				return nil, fmt.Errorf("type %T is not proto.Message for %s", v, msgType)
			}
			return proto.Marshal(pb)
		},
		func(b []byte) (any, error) {
			m := factory()
			if err := proto.Unmarshal(b, m); err != nil {
				return nil, err
			}
			return m, nil
		},
	)
}

// Encode encodes a registered message type
func (r *SchemaRegistry) Encode(t string, v any) ([]byte, bool, error) {
	enc, ok := r.enc[t]
	if !ok {
		return nil, false, nil
	}
	b, err := enc(v)
	return b, true, err
}

// Decode decodes a registered message type
func (r *SchemaRegistry) Decode(t string, b []byte) (any, bool, error) {
	dec, ok := r.dec[t]
	if !ok {
		// fmt.Printf("[GO][SCHEMA] no decoder registered for type=%q (have: %v)\n", t, keys(r.dec))
		return nil, false, nil
	}
	m, err := dec(b)
	if err != nil {
		// fmt.Printf("[GO][SCHEMA] decoder error type=%q err=%v\n", t, err)
		return nil, true, err
	}
	return m, true, err
}

// func keys[M ~map[string]V, V any](m M) []string {
// 	out := make([]string, 0, len(m))
// 	for k := range m {
// 		out = append(out, k)
// 	}
// 	sort.Strings(out)
// 	return out
// }
