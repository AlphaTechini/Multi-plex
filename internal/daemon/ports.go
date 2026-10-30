package daemon

import (
	"sync"

	"github.com/alphatechini/multi-plex/internal/core"
)

// Port blocks handed to sessions as MPX_PORT_BASE (docs/05-sessions-and-worktrees.md).
const (
	portStart     = 20000
	portBlockSize = 100
	portEnd       = 65000
)

// portAlloc hands out unique 100-port blocks to live sessions.
type portAlloc struct {
	mu   sync.Mutex
	used map[int]core.SessionID
	by   map[core.SessionID]int
}

// Alloc returns the session's block, reusing it if it already has one.
// It returns 0 when every block is taken.
func (p *portAlloc) Alloc(id core.SessionID) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if base, ok := p.by[id]; ok {
		return base
	}
	if p.used == nil {
		p.used = map[int]core.SessionID{}
		p.by = map[core.SessionID]int{}
	}
	for base := portStart; base+portBlockSize <= portEnd; base += portBlockSize {
		if _, taken := p.used[base]; !taken {
			p.used[base] = id
			p.by[id] = base
			return base
		}
	}
	return 0
}

// Release frees the session's block.
func (p *portAlloc) Release(id core.SessionID) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if base, ok := p.by[id]; ok {
		delete(p.used, base)
		delete(p.by, id)
	}
}
