// Package protocol defines the messages between the daemon and its clients
// (TUI, `mpx hook`, `mpx mcp`, later remote and mobile clients).
//
// A connection carries Frames in both directions: requests and responses
// matched by ID, and events pushed by the daemon after a subscribe call.
// How frames are encoded on the wire is the Codec's job, so the encoding
// (JSON today, see docs/15-open-questions.md Q3) can change in one place.
package protocol

import "encoding/json"

// Version is bumped when the protocol changes incompatibly.
const Version = 1

// FrameType says what a frame is.
type FrameType string

// Frame types.
const (
	TypeRequest  FrameType = "req"
	TypeResponse FrameType = "res"
	TypeEvent    FrameType = "evt"
)

// Frame is one message on a connection.
type Frame struct {
	Type   FrameType       `json:"t"`
	ID     uint64          `json:"id,omitempty"`    // request/response correlation
	Method string          `json:"m,omitempty"`     // request method
	Topic  string          `json:"topic,omitempty"` // event topic
	Data   json.RawMessage `json:"d,omitempty"`     // params, result or event payload
	Error  *Error          `json:"err,omitempty"`   // response error
}

// Error is a request failure.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error codes.
const (
	CodeNotFound   = "not_found"
	CodeBadRequest = "bad_request"
	CodeInternal   = "internal"
	CodeDenied     = "denied"
	// CodeNotARepo answers workspace.open for a directory outside any git
	// repository; the TUI shows a dedicated screen for it.
	CodeNotARepo = "not_a_repo"
)

func (e *Error) Error() string { return e.Code + ": " + e.Message }
