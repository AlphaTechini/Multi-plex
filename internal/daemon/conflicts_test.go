package daemon

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/alphatechini/multi-plex/internal/agent"
	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

// TestConflictPrediction: two sessions edit the same lines of the same file
// (uncommitted); when a turn ends each is told about the other, and a third
// session editing another file is not involved.
func TestConflictPrediction(t *testing.T) {
	h := newHarness(t)
	a := h.newSession("helper", "alpha")
	b := h.newSession("helper", "beta")
	c := h.newSession("helper", "gamma")

	writeFile(t, filepath.Join(a.WorktreePath, "shared.go"), "package main\n\nfunc shared() int { return 100 }\n")
	writeFile(t, filepath.Join(b.WorktreePath, "shared.go"), "package main\n\nfunc shared() int { return 200 }\n")
	writeFile(t, filepath.Join(c.WorktreePath, "other.go"), "package main\n")

	h.hook(a.ID, agent.HookEvent{Kind: agent.HookPromptSubmit})
	h.hook(b.ID, agent.HookEvent{Kind: agent.HookPromptSubmit})

	// Alpha finishes first: nothing to compare against yet but base and beta.
	h.hook(a.ID, agent.HookEvent{Kind: agent.HookStop})
	ev := decode[protocol.ConflictsEvent](t, h.ev.wait(t, protocol.TopicConflicts, func(raw json.RawMessage) bool {
		e := decode[protocol.ConflictsEvent](t, raw)
		return e.SessionID == a.ID && len(e.Conflicts) > 0
	}))
	if len(ev.Conflicts) != 1 || ev.Conflicts[0].OtherSession != b.ID || ev.Conflicts[0].OtherBranch != "mpx/beta" ||
		len(ev.Conflicts[0].Files) != 1 || ev.Conflicts[0].Files[0] != "shared.go" {
		t.Errorf("alpha conflicts = %+v", ev.Conflicts)
	}
	// Beta learns of it too, without finishing a turn.
	ev = decode[protocol.ConflictsEvent](t, h.ev.wait(t, protocol.TopicConflicts, func(raw json.RawMessage) bool {
		e := decode[protocol.ConflictsEvent](t, raw)
		return e.SessionID == b.ID && len(e.Conflicts) > 0
	}))
	if ev.Conflicts[0].OtherSession != a.ID {
		t.Errorf("beta conflicts = %+v", ev.Conflicts)
	}

	if got := h.detail(a.ID).Conflicts; len(got) != 1 || got[0].Files[0] != "shared.go" {
		t.Errorf("session.get conflicts for alpha = %+v", got)
	}
	if got := h.detail(c.ID).Conflicts; len(got) != 0 {
		t.Errorf("gamma has conflicts: %+v", got)
	}

	// Beta reverts its edit and finishes: the conflict clears for both.
	writeFile(t, filepath.Join(b.WorktreePath, "shared.go"), "package main\n\nfunc shared() int { return 1 }\n")
	h.hook(b.ID, agent.HookEvent{Kind: agent.HookStop})
	h.ev.wait(t, protocol.TopicConflicts, func(raw json.RawMessage) bool {
		e := decode[protocol.ConflictsEvent](t, raw)
		return e.SessionID == a.ID && len(e.Conflicts) == 0
	})
	if got := h.detail(a.ID).Conflicts; len(got) != 0 {
		t.Errorf("alpha conflicts after revert = %+v", got)
	}
	_ = core.StateDone
}
