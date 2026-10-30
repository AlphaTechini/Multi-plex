package role

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeRole(t *testing.T, dir, file, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, file), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadPrecedence(t *testing.T) {
	ws := t.TempDir()
	global := t.TempDir()

	// Global overrides built-in "implementer" and adds "shared".
	writeRole(t, global, "implementer.md", "---\ndescription: global impl\n---\nglobal body\n")
	writeRole(t, global, "shared.md", "---\ndescription: from global\n---\nshared body\n")
	// Workspace overrides global "implementer" and built-in "reviewer".
	writeRole(t, filepath.Join(ws, ".multiplex", "roles"), "implementer.md", "---\ndescription: ws impl\n---\nws body\n")
	writeRole(t, filepath.Join(ws, ".multiplex", "roles"), "reviewer.md", "---\ndescription: ws reviewer\n---\nws reviewer\n")
	writeRole(t, filepath.Join(ws, ".multiplex", "roles"), "notes.txt", "ignored")

	roles, err := Load(ws, global)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	tests := []struct {
		name, desc, prompt, source string
	}{
		{"implementer", "ws impl", "ws body", filepath.Join(".multiplex", "roles", "implementer.md")},
		{"reviewer", "ws reviewer", "ws reviewer", filepath.Join(".multiplex", "roles", "reviewer.md")},
		{"shared", "from global", "shared body", filepath.Join(global, "shared.md")},
		{"planner", "", "", "builtin"}, // built-in: only the source is checked
	}
	for _, tt := range tests {
		r, ok := roles[tt.name]
		if !ok {
			t.Errorf("role %q missing", tt.name)
			continue
		}
		if tt.desc != "" && r.Description != tt.desc {
			t.Errorf("%s description = %q, want %q", tt.name, r.Description, tt.desc)
		}
		if tt.prompt != "" && r.Prompt != tt.prompt {
			t.Errorf("%s prompt = %q, want %q", tt.name, r.Prompt, tt.prompt)
		}
		if !strings.HasSuffix(r.Source, tt.source) {
			t.Errorf("%s Source = %q, want suffix %q", tt.name, r.Source, tt.source)
		}
	}
	if _, ok := roles["notes"]; ok {
		t.Error("non-.md file was loaded as a role")
	}
}

func TestLoadBuiltinsOnly(t *testing.T) {
	roles, err := Load("", "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, name := range []string{"orchestrator", "planner", "implementer", "researcher", "reviewer", "resolver"} {
		r, ok := roles[name]
		if !ok {
			t.Errorf("built-in %q missing", name)
			continue
		}
		if r.Source != "builtin" {
			t.Errorf("%s Source = %q, want builtin", name, r.Source)
		}
	}
	if len(roles) != 6 {
		t.Errorf("got %d roles, want 6", len(roles))
	}
}

func TestLoadReportsBadFileAndKeepsGoodOnes(t *testing.T) {
	ws := t.TempDir()
	dir := filepath.Join(ws, ".multiplex", "roles")
	writeRole(t, dir, "broken.md", "no front matter here")
	writeRole(t, dir, "good.md", "---\ndescription: ok\n---\nbody\n")

	roles, err := Load(ws, "")
	if err == nil {
		t.Fatal("Load returned no error for a broken role file")
	}
	if !strings.Contains(err.Error(), "broken.md") {
		t.Errorf("error does not name the broken file: %v", err)
	}
	if _, ok := roles["good"]; !ok {
		t.Error("good role was dropped because another file failed")
	}
	if _, ok := roles["resolver"]; !ok {
		t.Error("built-ins were dropped because a file failed")
	}
	var joined interface{ Unwrap() []error }
	if !errors.As(err, &joined) {
		t.Errorf("error is not a joined error: %T", err)
	}
}

func TestLoadMissingDirsAreFine(t *testing.T) {
	roles, err := Load(t.TempDir(), filepath.Join(t.TempDir(), "absent"))
	if err != nil {
		t.Fatalf("Load with missing dirs: %v", err)
	}
	if len(roles) != 6 {
		t.Errorf("got %d roles, want 6 built-ins", len(roles))
	}
}
