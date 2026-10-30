package daemon

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/alphatechini/multi-plex/internal/spec"
)

// specRoots lists where a session looks for specs: its own worktree first (so
// it sees its own edits), then the workspace root (specs not yet committed).
func (d *Daemon) specRoots(ctx context.Context, s *session) []string {
	info := s.snap()
	roots := []string{info.WorktreePath}
	if ws, err := d.workspace(ctx, info.WorkspaceID); err == nil && ws.Root != info.WorktreePath {
		roots = append(roots, ws.Root)
	}
	return roots
}

// readSpec reads a spec, file by file: each of its three files comes from the
// first root that has it non-empty. A session that only changed tasks.md in
// its worktree therefore still sees requirements.md from the main checkout.
func (d *Daemon) readSpec(ctx context.Context, s *session, slug string) (spec.Doc, error) {
	var out spec.Doc
	found := false
	var firstErr error
	for _, root := range d.specRoots(ctx, s) {
		doc, err := spec.Read(root, slug)
		if err != nil {
			if errors.Is(err, spec.ErrInvalidSlug) {
				return spec.Doc{}, badRequest("%v", err)
			}
			if firstErr == nil && !errors.Is(err, fs.ErrNotExist) {
				firstErr = err
			}
			continue
		}
		found = true
		out.Slug = doc.Slug
		if out.Requirements == "" {
			out.Requirements = doc.Requirements
		}
		if out.Design == "" {
			out.Design = doc.Design
		}
		if out.Tasks == "" {
			out.Tasks = doc.Tasks
		}
	}
	if found {
		return out, nil
	}
	if firstErr != nil {
		return spec.Doc{}, firstErr
	}
	return spec.Doc{}, notFound("no such spec %q", slug)
}

// specFile is the path of one of a spec's files under a root.
func specFile(root, slug, name string) string {
	return filepath.Join(root, ".multiplex", "specs", slug, name)
}

// writeFileAtomic writes data to path through a temporary file, creating
// parent directories.
func writeFileAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if err := errors.Join(werr, cerr); err != nil {
		_ = os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		_ = os.Remove(name)
		return err
	}
	return nil
}
