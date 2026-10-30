package protocol

import "io"

// Encoder writes frames to a stream. Implementations must be safe for
// concurrent use (responses and events are written from several goroutines).
type Encoder interface {
	Encode(f *Frame) error
}

// Decoder reads frames from a stream. It is used from one goroutine.
type Decoder interface {
	Decode(f *Frame) error
}

// Codec creates encoders and decoders for a wire format.
type Codec interface {
	Name() string
	NewEncoder(w io.Writer) Encoder
	NewDecoder(r io.Reader) Decoder
}
