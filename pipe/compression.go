package pipe

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"

	"github.com/golang/snappy"
)

type CompressionCodec string

const (
	Gzip   CompressionCodec = "gzip"
	Snappy CompressionCodec = "snappy"
)

var ErrUnknownCodec = errors.New("unknown compression codec")

// ---- GZIP ----

func GzipCompress(in []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(in); err != nil {
		_ = zw.Close()
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func GzipDecompress(in []byte) ([]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(in))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	return io.ReadAll(zr)
}

// ---- SNAPPY ----

func SnappyCompress(in []byte) ([]byte, error) {
	// snappy.Encode never returns an error
	return snappy.Encode(nil, in), nil
}

func SnappyDecompress(in []byte) ([]byte, error) {
	return snappy.Decode(nil, in)
}

// ---- generic dispatchers ----

func CompressWith(in []byte, codec CompressionCodec) ([]byte, error) {
	switch codec {
	case Gzip:
		return GzipCompress(in)
	case Snappy:
		return SnappyCompress(in)
	default:
		return nil, ErrUnknownCodec
	}
}

func DecompressWith(in []byte, codec CompressionCodec) ([]byte, error) {
	switch codec {
	case Gzip:
		return GzipDecompress(in)
	case Snappy:
		return SnappyDecompress(in)
	default:
		return nil, ErrUnknownCodec
	}
}
