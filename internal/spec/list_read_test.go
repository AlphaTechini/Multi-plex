package spec

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// writeSpec creates .multiplex/specs/<slug>/ with the given files.
func writeSpec(t *testing.T, root, slug string, files map[string]string) {
	t.Helper()
	dir := filepath.Join(root, ".multiplex", "specs", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestListAndRead(t *testing.T) {
	root := t.TempDir()
	writeSpec(t, root, "auth", map[string]string{
		"requirements.md": requirementsMD,
		"design.md":       "# Design\n",
		"tasks.md":        tasksMD,
	})
	writeSpec(t, root, "empty", nil)
	// A stray file in the specs folder is not a spec.
	if err := os.WriteFile(filepath.Join(root, ".multiplex", "specs", "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	list, err := List(root)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []Info{
		{Slug: "auth", Title: "Auth refresh", TasksDone: 3, TasksTotal: 7, CriteriaTotal: 3},
		{Slug: "empty", Title: "empty"},
	}
	if !reflect.DeepEqual(list, want) {
		t.Errorf("List = %+v\nwant %+v", list, want)
	}

	doc, err := Read(root, "auth")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if doc.Slug != "auth" || doc.Requirements != requirementsMD || doc.Design != "# Design\n" || doc.Tasks != tasksMD {
		t.Errorf("Read returned unexpected content for auth")
	}

	empty, err := Read(root, "empty")
	if err != nil {
		t.Fatalf("Read empty: %v", err)
	}
	if empty.Requirements != "" || empty.Design != "" || empty.Tasks != "" {
		t.Errorf("missing files should read as empty, got %+v", empty)
	}
}

func TestListNoSpecsFolder(t *testing.T) {
	list, err := List(t.TempDir())
	if err != nil || list != nil {
		t.Errorf("List = %v, %v; want nil, nil", list, err)
	}
}

func TestReadRejectsBadSlugs(t *testing.T) {
	root := t.TempDir()
	writeSpec(t, root, "auth", map[string]string{"requirements.md": "x"})
	for _, slug := range []string{"", "..", "../auth", "auth/..", "a/b", `a\b`, "c:d", ".hidden", "x..y"} {
		if _, err := Read(root, slug); !errors.Is(err, ErrInvalidSlug) {
			t.Errorf("Read(%q) err = %v, want ErrInvalidSlug", slug, err)
		}
	}
}

func TestReadMissingSpec(t *testing.T) {
	_, err := Read(t.TempDir(), "nope")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("err = %v, want fs.ErrNotExist", err)
	}
}
