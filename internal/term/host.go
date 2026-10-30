// Package term hosts one terminal pane: a PTY process whose output feeds a
// VT emulator, and the coalesced screen updates (protocol.PaneFrame) derived
// from it (docs/01-architecture.md "Terminal hosting").
//
// The data path never blocks on a consumer: the reader goroutine only writes
// into the emulator, and frames are computed by a separate goroutine from the
// emulator's current state. A slow sink therefore drops intermediate frames
// and never input or output (docs/03-performance.md).
package term

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/pty"
	"github.com/alphatechini/multi-plex/internal/vt"
)

const (
	// readChunk is the size of one read from the PTY.
	readChunk = 32 << 10
	// frameInterval is the minimum time between two frames (60 Hz).
	frameInterval = 16 * time.Millisecond
	// exitGrace is how long, after the process exits, the host waits for the
	// PTY to report end of output before closing it (ConPTY never does).
	exitGrace = 150 * time.Millisecond
	// closeTimeout bounds how long Close waits for the goroutines to stop.
	closeTimeout = 5 * time.Second
)

// Host runs one pane.
type Host struct {
	id   core.PaneID
	proc pty.Process
	emu  vt.Emulator
	sink func(protocol.PaneFrame)
	out  *lockedWriter // serialised writes to the PTY (input and emulator replies)

	dirty    chan struct{} // output arrived since the last frame (capacity 1)
	stopLoop chan struct{} // closed to flush a last frame and stop the frame loop
	loopDone chan struct{}
	readDone chan struct{}
	done     chan struct{} // closed once the process has exited and output is drained

	fmu sync.Mutex // guards fr, and orders Resize against frame computation
	fr  *framer

	mu      sync.Mutex // guards the fields below
	exited  bool
	code    int
	waitErr error
	onExit  []func(code int)

	closeOnce sync.Once
}

// Start spawns spec in a pseudo-terminal and begins hosting it. Every screen
// update is passed to sink from a single goroutine; the first frame is a full
// one. The sink may be slow (frames are then coalesced) but must not call
// Close. Cancelling ctx closes the host. The caller must call Close when it
// is finished with the pane, even after the process has exited.
func Start(ctx context.Context, sp pty.Spawner, emu vt.Factory, spec pty.Spec, id core.PaneID, sink func(protocol.PaneFrame)) (*Host, error) {
	if spec.Cols <= 0 || spec.Rows <= 0 {
		return nil, fmt.Errorf("term: invalid size %dx%d", spec.Cols, spec.Rows)
	}
	if sink == nil {
		sink = func(protocol.PaneFrame) {}
	}
	proc, err := sp.Spawn(ctx, spec)
	if err != nil {
		return nil, err
	}
	e := emu(spec.Cols, spec.Rows)
	h := &Host{
		id:       id,
		proc:     proc,
		emu:      e,
		sink:     sink,
		out:      &lockedWriter{w: proc},
		dirty:    make(chan struct{}, 1),
		stopLoop: make(chan struct{}),
		loopDone: make(chan struct{}),
		readDone: make(chan struct{}),
		done:     make(chan struct{}),
		fr:       newFramer(id, e),
	}
	e.SetReplyWriter(h.out)

	h.markDirty() // the first frame is a full one
	go h.readLoop()
	go h.frameLoop()
	go h.lifecycle()
	go func() {
		select {
		case <-ctx.Done():
			_ = h.Close()
		case <-h.done:
		}
	}()
	return h, nil
}

// markDirty tells the frame loop that the screen may have changed.
func (h *Host) markDirty() {
	select {
	case h.dirty <- struct{}{}:
	default:
	}
}

// readLoop copies PTY output into the emulator.
func (h *Host) readLoop() {
	defer close(h.readDone)
	buf := make([]byte, readChunk)
	for {
		n, err := h.proc.Read(buf)
		if n > 0 {
			// An emulator error (closed, or a recovered library panic)
			// must not stop the PTY from being drained.
			_, _ = h.emu.Write(buf[:n])
			h.markDirty()
		}
		if err != nil {
			return
		}
	}
}

// frameLoop sends at most one frame per frameInterval, and only while the
// screen is changing: with nothing to show it parks on the dirty channel and
// uses no CPU.
func (h *Host) frameLoop() {
	defer close(h.loopDone)
	timer := time.NewTimer(time.Hour)
	timer.Stop()
	defer timer.Stop()
	var last time.Time
	for {
		select {
		case <-h.dirty:
		case <-h.stopLoop:
			h.emitFrame()
			return
		}
		if wait := frameInterval - time.Since(last); wait > 0 {
			timer.Reset(wait)
			select {
			case <-timer.C:
			case <-h.stopLoop:
				timer.Stop()
				h.emitFrame()
				return
			}
		}
		h.emitFrame()
		last = time.Now() // measured after the sink, so a slow sink is never hit back to back
	}
}

// emitFrame computes the next frame and hands it to the sink.
func (h *Host) emitFrame() {
	h.fmu.Lock()
	fr, ok := h.fr.next()
	h.fmu.Unlock()
	if ok {
		h.sink(fr)
	}
}

// lifecycle waits for the process, lets the last output drain, and publishes
// the exit.
func (h *Host) lifecycle() {
	code, err := h.proc.Wait()

	// On Unix the reader sees end of output by itself once the child side is
	// closed; ConPTY needs the console closed. Give pending output a moment.
	select {
	case <-h.readDone:
	case <-time.After(exitGrace):
	}
	_ = h.proc.Close()
	<-h.readDone

	close(h.stopLoop) // flush the final screen
	<-h.loopDone

	h.mu.Lock()
	h.exited, h.code, h.waitErr = true, code, err
	cbs := h.onExit
	h.onExit = nil
	h.mu.Unlock()
	close(h.done)
	for _, cb := range cbs {
		cb(code)
	}
}

// Input writes keyboard or paste bytes straight to the PTY. It adds no queue
// and no allocation to the keystroke path.
func (h *Host) Input(b []byte) error {
	if len(b) == 0 {
		return nil
	}
	_, err := h.out.Write(b)
	return err
}

// Resize changes the pane size. The emulator is resized first so the
// program's redraw after SIGWINCH lands on a screen of the new size; the next
// frame is a full one.
func (h *Host) Resize(cols, rows int) error {
	if cols <= 0 || rows <= 0 {
		return fmt.Errorf("term: invalid size %dx%d", cols, rows)
	}
	h.fmu.Lock()
	h.emu.Resize(cols, rows)
	err := h.proc.Resize(cols, rows)
	h.fr.forceFull()
	h.fmu.Unlock()
	h.markDirty()
	return err
}

// Snapshot returns a full frame of the current screen, for a client that is
// attaching or has lost sync. It does not disturb the frames being sent.
func (h *Host) Snapshot() protocol.PaneFrame {
	h.fmu.Lock()
	defer h.fmu.Unlock()
	return snapshot(h.id, h.emu)
}

// Wait blocks until the process has exited and its output is drained, then
// returns its exit code.
func (h *Host) Wait() (int, error) {
	<-h.done
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.code, h.waitErr
}

// Exited reports whether the process has exited.
func (h *Host) Exited() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.exited
}

// OnExit registers fn to be called, once, with the exit code after the
// process has exited and the final frame has been sent. If that has already
// happened fn is called immediately, on the caller's goroutine.
func (h *Host) OnExit(fn func(code int)) {
	h.mu.Lock()
	if h.exited {
		code := h.code
		h.mu.Unlock()
		fn(code)
		return
	}
	h.onExit = append(h.onExit, fn)
	h.mu.Unlock()
}

// Close kills the process if it is still running, stops the goroutines and
// releases the PTY and the emulator. It is safe to call more than once and
// from OnExit callbacks, but not from the sink.
func (h *Host) Close() error {
	var err error
	h.closeOnce.Do(func() {
		_ = h.proc.Kill()
		err = h.proc.Close()
		select {
		case <-h.done:
		case <-time.After(closeTimeout):
			err = errors.Join(err, errors.New("term: timed out stopping pane"))
		}
		_ = h.emu.Close()
	})
	return err
}

// lockedWriter serialises writes so keystrokes and emulator replies never
// interleave inside one another.
type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}
