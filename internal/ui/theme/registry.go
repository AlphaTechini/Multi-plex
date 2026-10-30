package theme

import (
	"slices"
	"sync"

	"github.com/charmbracelet/colorprofile"
)

var (
	registryMu sync.RWMutex
	registry   = map[string]Builder{}
)

// Register adds a theme builder under name, replacing any earlier one. Theme
// files call it from init, so adding a theme means adding one file.
func Register(name string, build Builder) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[name] = build
}

// ByName returns the named theme resolved for truecolor, and whether it
// exists. Call For on the result to resolve it for another profile.
func ByName(name string) (*Theme, bool) {
	registryMu.RLock()
	build, ok := registry[name]
	registryMu.RUnlock()
	if !ok {
		return nil, false
	}
	t := build(colorprofile.TrueColor)
	t.build = build
	return t, true
}

// Names returns the registered theme names in sorted order.
func Names() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	slices.Sort(names)
	return names
}
