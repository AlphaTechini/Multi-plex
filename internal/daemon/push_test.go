package daemon

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

// TestPushBroker: a worktree of a repository with an origin has a disabled
// push URL, direct pushes fail, and `session.push` pushes only after approval.
func TestPushBroker(t *testing.T) {
	h := newHarnessWithOrigin(t)

	sess := h.newSession("shell", "pushy")
	if got := gitRun(t, sess.WorktreePath, "config", "--get", "remote.origin.pushurl"); got != disabledPush {
		t.Fatalf("worktree pushurl = %q, want %q", got, disabledPush)
	}
	probe := exec.Command("git", "config", "--get-all", "remote.origin.pushurl")
	probe.Dir = h.repo
	if out, _ := probe.Output(); strings.Contains(string(out), disabledPush) {
		t.Error("the main repository's push URL was disabled too")
	}

	writeFile(t, filepath.Join(sess.WorktreePath, "feature.txt"), "feature\n")
	gitRun(t, sess.WorktreePath, "add", ".")
	gitRun(t, sess.WorktreePath, "commit", "-m", "feature")

	// A direct push from the worktree cannot work.
	cmd := exec.Command("git", "push", "origin", sess.Branch)
	cmd.Dir = sess.WorktreePath
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("direct push from the worktree succeeded:\n%s", out)
	}

	// Through the broker: denied first.
	errCh := make(chan error, 1)
	go func() { errCh <- h.call(protocol.MethodSessionPush, protocol.SessionRef{SessionID: sess.ID}, nil) }()
	p := decode[protocol.ApprovalPending](t, h.ev.wait(t, protocol.TopicApprovalPending, nil))
	if p.Request.Kind != core.ApprovalPush || p.Request.Rule != "push-broker" || p.Request.Layer != "layer-3-credentials" ||
		p.Request.Command != "git push origin "+sess.Branch {
		t.Errorf("push request = %+v", p.Request)
	}
	h.mustCall(protocol.MethodApprovalRespond, protocol.ApprovalRespondParams{RequestID: p.Request.ID, Outcome: core.Denied, Message: "not yet"}, nil)
	if err := <-errCh; err == nil || !strings.Contains(err.Error(), "not yet") {
		t.Errorf("denied push error = %v", err)
	}
	if out := gitRun(t, h.origin, "branch", "--list", sess.Branch); out != "" {
		t.Fatalf("a denied push reached the remote: %q", out)
	}

	// Then approved.
	type result struct {
		res protocol.PushResult
		err error
	}
	resCh := make(chan result, 1)
	go func() {
		var res protocol.PushResult
		err := h.call(protocol.MethodSessionPush, protocol.SessionRef{SessionID: sess.ID}, &res)
		resCh <- result{res, err}
	}()
	p2 := decode[protocol.ApprovalPending](t, h.ev.wait(t, protocol.TopicApprovalPending, func(raw json.RawMessage) bool {
		return decode[protocol.ApprovalPending](t, raw).Request.ID != p.Request.ID
	}))
	h.mustCall(protocol.MethodApprovalRespond, protocol.ApprovalRespondParams{RequestID: p2.Request.ID, Outcome: core.Approved, Code: p2.Code}, nil)
	r := <-resCh
	if r.err != nil {
		t.Fatalf("approved push failed: %v", r.err)
	}
	if r.res.Remote != "origin" || r.res.Branch != sess.Branch {
		t.Errorf("push result = %+v", r.res)
	}
	if out := gitRun(t, h.origin, "branch", "--list", sess.Branch); !strings.Contains(out, sess.Branch) {
		t.Errorf("remote branches after push: %q", out)
	}
}

// newHarnessWithOrigin is newHarness with a bare repository as origin.
func newHarnessWithOrigin(t *testing.T) *harness {
	t.Helper()
	origin := filepath.Join(t.TempDir(), "origin.git")
	gitRun(t, filepath.Dir(origin), "init", "--bare", "-b", "main", origin)
	h := newHarnessAt(t, func(repo string) { gitRun(t, repo, "remote", "add", "origin", origin) })
	h.origin = origin
	return h
}
