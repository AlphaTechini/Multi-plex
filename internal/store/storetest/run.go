// Package storetest is a reusable conformance suite for store.Store
// implementations. Every implementation's tests call Run with a constructor
// that returns a fresh, empty store.
package storetest

import (
	"testing"

	"github.com/alphatechini/multi-plex/internal/store"
)

// Run executes the full conformance suite. newStore must return a new empty
// store on every call; Run closes it when the subtest ends.
func Run(t *testing.T, newStore func(t *testing.T) store.Store) {
	t.Helper()
	groups := []struct {
		name string
		fn   func(*testing.T, store.Store)
	}{
		{"Workspaces", testWorkspaces},
		{"Sessions", testSessions},
		{"Summaries", testSummaries},
		{"Notes", testNotes},
		{"Comments", testComments},
		{"Checkpoints", testCheckpoints},
		{"Approvals", testApprovals},
		{"DeleteSession", testDeleteSession},
		{"Isolation", testIsolation},
		{"ConcurrentAddNote", testConcurrentAddNote},
	}
	for _, g := range groups {
		t.Run(g.name, func(t *testing.T) {
			s := newStore(t)
			t.Cleanup(func() {
				if err := s.Close(); err != nil {
					t.Errorf("Close: %v", err)
				}
			})
			g.fn(t, s)
		})
	}
}
