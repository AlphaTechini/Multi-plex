package agent

import (
	"sort"
	"sync"
)

var (
	regMu    sync.RWMutex
	registry = map[string]Adapter{}
)

// Register makes an adapter available by name. Adapters call it from init.
func Register(a Adapter) {
	regMu.Lock()
	defer regMu.Unlock()
	registry[a.Name()] = a
}

// Get returns the adapter registered under name.
func Get(name string) (Adapter, bool) {
	regMu.RLock()
	defer regMu.RUnlock()
	a, ok := registry[name]
	return a, ok
}

// Names lists registered adapter names, sorted.
func Names() []string {
	regMu.RLock()
	defer regMu.RUnlock()
	out := make([]string, 0, len(registry))
	for n := range registry {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
