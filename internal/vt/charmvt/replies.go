package charmvt

import (
	"io"
	"sync"

	xvt "github.com/charmbracelet/x/vt"
)

// maxPendingReplies bounds queued reply bytes. Replies to a program that is
// not reading its input are useless, so the excess is dropped.
const maxPendingReplies = 64 << 10

// replyPump moves the emulator's replies (DA, DSR, mode reports, ...) to the
// reply writer. x/vt writes them synchronously into an unbuffered pipe from
// inside Write, so something must always be reading that pipe or Write blocks
// forever. Two goroutines keep that safe: one only drains the pipe into a
// queue, the other writes the queue out, so a slow writer never stalls Write.
type replyPump struct {
	mu      sync.Mutex
	pending []byte
	w       io.Writer
	wake    chan struct{}
	quit    chan struct{}
	drained chan struct{}
}

func startReplyPump(e *xvt.Emulator) *replyPump {
	p := &replyPump{
		wake:    make(chan struct{}, 1),
		quit:    make(chan struct{}),
		drained: make(chan struct{}),
	}
	go p.drain(e)
	go p.flush()
	return p
}

func (p *replyPump) setWriter(w io.Writer) {
	p.mu.Lock()
	p.w = w
	p.mu.Unlock()
}

// drain reads the emulator's pipe until it is closed.
func (p *replyPump) drain(e *xvt.Emulator) {
	defer close(p.drained)
	buf := make([]byte, 512)
	for {
		n, err := e.Read(buf)
		if n > 0 {
			p.mu.Lock()
			if len(p.pending)+n <= maxPendingReplies && p.w != nil {
				p.pending = append(p.pending, buf[:n]...)
			}
			p.mu.Unlock()
			select {
			case p.wake <- struct{}{}:
			default:
			}
		}
		if err != nil {
			return
		}
	}
}

// flush writes queued replies to the writer.
func (p *replyPump) flush() {
	var out []byte
	for {
		select {
		case <-p.wake:
		case <-p.quit:
			return
		}
		p.mu.Lock()
		out, p.pending = append(out[:0], p.pending...), p.pending[:0]
		w := p.w
		p.mu.Unlock()
		if w != nil && len(out) > 0 {
			_, _ = w.Write(out)
		}
	}
}

// stop ends both goroutines. The emulator's pipe must already be closed.
// The flush goroutine may still be inside a blocked Write; it exits when that
// returns.
func (p *replyPump) stop() {
	<-p.drained
	close(p.quit)
}
