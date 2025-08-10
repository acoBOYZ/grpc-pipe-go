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
	// out := buf.Bytes()
	// LogSize("GZIP original", in)
	// LogSize("GZIP compressed", out)
	// return out, nil
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
	// out := snappy.Encode(nil, in)
	// LogSize("SNAPPY original", in)
	// LogSize("SNAPPY compressed", out)
	// return out, nil
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
