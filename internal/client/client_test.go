package client

import (
	"context"
	"encoding/json"
	"net"
	"testing"

	"github.com/alphatechini/multi-plex/internal/protocol"
)

// serve answers requests on one end of a pipe with echo results.
func serve(t *testing.T, nc net.Conn, closeAfterFirst bool) {
	t.Helper()
	enc, dec := protocol.JSON.NewEncoder(nc), protocol.JSON.NewDecoder(nc)
	go func() {
		defer nc.Close()
		for {
			var f protocol.Frame
			if dec.Decode(&f) != nil {
				return
			}
			if f.Method == "fail" {
				_ = enc.Encode(&protocol.Frame{Type: protocol.TypeResponse, ID: f.ID, Error: &protocol.Error{Code: protocol.CodeNotFound, Message: "nope"}})
			} else {
				_ = enc.Encode(&protocol.Frame{Type: protocol.TypeResponse, ID: f.ID, Data: f.Data})
			}
			if closeAfterFirst {
				return
			}
		}
	}()
}

func TestCallRoundTrip(t *testing.T) {
	a, b := net.Pipe()
	serve(t, b, false)
	c := New(a, protocol.JSON)
	defer c.Close()

	var out protocol.SessionRef
	if err := c.Call(context.Background(), "echo", protocol.SessionRef{SessionID: "s1"}, &out); err != nil || out.SessionID != "s1" {
		t.Fatalf("echo = %+v, %v", out, err)
	}
	err := c.Call(context.Background(), "fail", protocol.Empty{}, nil)
	pe, ok := err.(*protocol.Error)
	if !ok || pe.Code != protocol.CodeNotFound {
		t.Errorf("error = %v", err)
	}
}

// TestResponseBeforeCloseIsNotLost: a daemon that answers and immediately
// closes (shutdown, or the hook client of a one-shot server) must not turn
// that answer into ErrClosed.
func TestResponseBeforeCloseIsNotLost(t *testing.T) {
	for i := 0; i < 300; i++ {
		a, b := net.Pipe()
		serve(t, b, true)
		c := New(a, protocol.JSON)
		var out json.RawMessage
		if err := c.Call(context.Background(), "echo", map[string]int{"n": i}, &out); err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		c.Close()
	}
}

func TestCallAfterCloseFails(t *testing.T) {
	a, b := net.Pipe()
	b.Close()
	c := New(a, protocol.JSON)
	<-c.Done()
	if err := c.Call(context.Background(), "echo", protocol.Empty{}, nil); err == nil {
		t.Error("call on a closed client succeeded")
	}
}
