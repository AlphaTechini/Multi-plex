package protocol

import (
	"bufio"
	"encoding/json"
	"io"
	"sync"
)

// JSON is the newline-delimited JSON codec.
var JSON Codec = jsonCodec{}

type jsonCodec struct{}

func (jsonCodec) Name() string { return "json" }

func (jsonCodec) NewEncoder(w io.Writer) Encoder {
	bw := bufio.NewWriterSize(w, 64<<10)
	return &jsonEncoder{w: bw, enc: json.NewEncoder(bw)}
}

func (jsonCodec) NewDecoder(r io.Reader) Decoder {
	dec := json.NewDecoder(bufio.NewReaderSize(r, 64<<10))
	return &jsonDecoder{dec: dec}
}

type jsonEncoder struct {
	mu  sync.Mutex
	w   *bufio.Writer
	enc *json.Encoder
}

func (e *jsonEncoder) Encode(f *Frame) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.enc.Encode(f); err != nil {
		return err
	}
	return e.w.Flush()
}

type jsonDecoder struct{ dec *json.Decoder }

func (d *jsonDecoder) Decode(f *Frame) error { return d.dec.Decode(f) }
