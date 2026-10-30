package daemon

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/alphatechini/multi-plex/internal/agent"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

func TestAgentList(t *testing.T) {
	h := newHarness(t)
	var got []protocol.AgentInfo
	h.mustCall(protocol.MethodAgentList, protocol.Empty{}, &got)
	by := map[string]protocol.AgentInfo{}
	for _, a := range got {
		by[a.Name] = a
	}
	for _, name := range []string{"claude-code", "shell", "helper"} {
		a, ok := by[name]
		if !ok {
			t.Fatalf("agent %q missing from %+v", name, got)
		}
		if a.DisplayName == "" {
			t.Errorf("%s has no display name", name)
		}
	}
	if !by["helper"].Available {
		t.Error("an adapter without a Detector must be available")
	}
	if cc := by["claude-code"]; !cc.Available && !strings.Contains(cc.Detail, "Install") {
		t.Errorf("a missing claude must say how to install it: %+v", cc)
	}
	if len(got) != len(agent.Names()) {
		t.Errorf("listed %d agents, registry has %d", len(got), len(agent.Names()))
	}
}

func TestRoleList(t *testing.T) {
	h := newHarness(t)
	rolesDir := filepath.Join(h.repo, ".multiplex", "roles")
	writeFile(t, filepath.Join(rolesDir, "docs-writer.md"), "---\nname: docs-writer\ndescription: Writes the docs\n---\nWrite docs.\n")
	writeFile(t, filepath.Join(h.paths.ConfigDir, "roles", "mine.md"), "---\nname: mine\ndescription: A global role\n---\nHi.\n")

	var got []protocol.RoleInfo
	h.mustCall(protocol.MethodRoleList, protocol.WorkspaceRef{WorkspaceID: h.ws.ID}, &got)
	src := map[string]string{}
	for _, r := range got {
		src[r.Name] = r.Source
		if r.Description == "" {
			t.Errorf("role %s has no description", r.Name)
		}
	}
	want := map[string]string{"implementer": "builtin", "docs-writer": "workspace", "mine": "global"}
	for name, source := range want {
		if src[name] != source {
			t.Errorf("role %s source = %q, want %q (all: %v)", name, src[name], source, src)
		}
	}
	if err := h.call(protocol.MethodRoleList, protocol.WorkspaceRef{WorkspaceID: "ws_nope"}, nil); err == nil {
		t.Error("unknown workspace accepted")
	}
}

func TestDetailTurnAndReports(t *testing.T) {
	h := newHarness(t)
	sess := h.newSession("helper", "counts")
	if det := h.detail(sess.ID); det.Turn != 0 || det.Reports != 0 {
		t.Fatalf("fresh detail = turn %d, reports %d", det.Turn, det.Reports)
	}
	h.hook(sess.ID, agentPrompt())
	h.hook(sess.ID, agent.HookEvent{Kind: agent.HookStop})
	h.hook(sess.ID, agentPrompt())
	h.mcp(sess.ID, protocol.ToolReportProgress, protocol.ReportProgressArgs{Done: []string{"one"}})
	h.mcp(sess.ID, protocol.ToolReportProgress, protocol.ReportProgressArgs{Done: []string{"two"}})
	if det := h.detail(sess.ID); det.Turn != 2 || det.Reports != 2 {
		t.Errorf("detail = turn %d, reports %d, want 2 and 2", det.Turn, det.Reports)
	}
}

func TestVerifiedCriteria(t *testing.T) {
	root, wt := t.TempDir(), t.TempDir()
	writeFile(t, specFile(root, "s", verificationFile), "# Verified criteria\n\n- R1.1: a (session x, now)\n- R1.2: b\n")
	writeFile(t, specFile(wt, "s", verificationFile), "# Verified criteria\n\n- R1.2: again\n- R2.1: c\nnot a criterion line\n- R1.3 missing colon\n")
	if got := verifiedCriteria([]string{root, wt, filepath.Join(root, "nowhere")}, "s"); got != 3 {
		t.Errorf("verified = %d, want 3 distinct", got)
	}
	if got := verifiedCriteria([]string{root}, "other"); got != 0 {
		t.Errorf("other spec verified = %d", got)
	}
}

func TestSpecListCountsVerified(t *testing.T) {
	h := newHarness(t)
	specDir := filepath.Join(h.repo, ".multiplex", "specs", "auth-refresh")
	writeFile(t, filepath.Join(specDir, "requirements.md"), specRequirements)
	sess := h.newSession("helper", "verify")
	h.mcp(sess.ID, protocol.ToolCriterionVerify, protocol.CriterionVerifyArgs{Slug: "auth-refresh", Criterion: "R1.1", Evidence: "t"})
	h.mcp(sess.ID, protocol.ToolCriterionVerify, protocol.CriterionVerifyArgs{Slug: "auth-refresh", Criterion: "R1.1", Evidence: "t2"})
	h.mcp(sess.ID, protocol.ToolCriterionVerify, protocol.CriterionVerifyArgs{Slug: "auth-refresh", Criterion: "R2.1", Evidence: "t"})
	var infos []protocol.SpecInfo
	h.mustCall(protocol.MethodSpecList, protocol.WorkspaceRef{WorkspaceID: h.ws.ID}, &infos)
	if len(infos) != 1 || infos[0].CriteriaVerified != 2 || infos[0].CriteriaTotal != 3 {
		t.Errorf("spec.list = %+v", infos)
	}
}
