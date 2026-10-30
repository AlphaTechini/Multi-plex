package pane

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/keys"
)

// Key forwards a key press to the pane's process as the bytes a terminal
// would send. It reports false, sending nothing, when the pane is not
// attached or its process has exited, so the owner can handle the key.
func (p *Pane) Key(k tea.KeyPressMsg) bool {
	if p.id == "" || p.exited {
		return false
	}
	if b := keys.Encode(k); len(b) > 0 {
		p.send(b)
	}
	return true
}

// Paste forwards pasted text, in bracketed-paste form. It reports false when
// the pane cannot take input.
func (p *Pane) Paste(text string) bool {
	if p.id == "" || p.exited {
		return false
	}
	p.send(keys.EncodePaste(text, true))
	return true
}

// Send forwards raw bytes to the pane's process.
func (p *Pane) Send(b []byte) bool {
	if p.id == "" || p.exited {
		return false
	}
	p.send(b)
	return true
}

// send queues bytes for the daemon. One goroutine at a time drains the queue
// in order, merging whatever piled up into a single call, so keystrokes can
// never be reordered and the UI loop never waits on the daemon.
func (p *Pane) send(b []byte) {
	p.inMu.Lock()
	p.pending = append(p.pending, b...)
	if p.draining {
		p.inMu.Unlock()
		return
	}
	p.draining = true
	p.inMu.Unlock()
	go p.drain(p.id)
}

func (p *Pane) drain(id core.PaneID) {
	for {
		p.inMu.Lock()
		data := p.pending
		p.pending = nil
		if len(data) == 0 {
			p.draining = false
			p.inMu.Unlock()
			return
		}
		p.inMu.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = p.be.PaneInput(ctx, id, data)
		cancel()
	}
}
