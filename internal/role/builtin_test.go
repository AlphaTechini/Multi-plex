package role

import (
	"strings"
	"testing"
)

func TestBuiltinRoles(t *testing.T) {
	builtins, err := builtinRoles()
	if err != nil {
		t.Fatalf("builtinRoles: %v", err)
	}
	byName := map[string]Role{}
	for _, r := range builtins {
		byName[r.Name] = r
	}

	// researcher must not change code. planner writes spec files, so it
	// keeps file tools and its prompt limits it to .multiplex/specs/.
	if r := byName["researcher"]; !contains(r.DenyTools, "Edit") || !contains(r.DenyTools, "Write") {
		t.Errorf("researcher must deny Edit and Write, has %v", r.DenyTools)
	}
	if r := byName["planner"]; !strings.Contains(r.Prompt, ".multiplex/specs/") {
		t.Errorf("planner prompt must limit writes to .multiplex/specs/")
	}
	// Every built-in must report at the end of a turn, and must have a description.
	for _, r := range builtins {
		if r.Prompt == "" {
			t.Errorf("%s has an empty prompt", r.Name)
		}
		if r.Description == "" {
			t.Errorf("%s has no description", r.Name)
		}
		if !strings.Contains(r.Prompt, "report_progress") {
			t.Errorf("%s prompt does not mention report_progress", r.Name)
		}
		if !strings.Contains(r.Prompt, "note_to_user") {
			t.Errorf("%s prompt does not mention note_to_user", r.Name)
		}
	}
	// Each role names the tools its job depends on.
	want := map[string][]string{
		"orchestrator": {"session_spawn", "session_status", "session_wait"},
		"reviewer":     {"criterion_verify", "spec_get"},
		"implementer":  {"task_update"},
	}
	for name, needles := range want {
		for _, n := range needles {
			if !strings.Contains(byName[name].Prompt, n) {
				t.Errorf("%s prompt does not mention %s", name, n)
			}
		}
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
