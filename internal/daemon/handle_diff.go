package daemon

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"

	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/vcs"
)

// maxFileRead is the largest file file.read returns.
const maxFileRead = 4 << 20

// hDiffGet returns the session's diff against its base branch.
func (d *Daemon) hDiffGet(ctx context.Context, _ *conn, p protocol.SessionRef) (any, error) {
	s, err := d.getSession(p.SessionID)
	if err != nil {
		return nil, err
	}
	info := s.snap()
	raw, err := d.vcs.Diff(ctx, info.WorktreePath, info.BaseBranch)
	if err != nil {
		return nil, err
	}
	changes, err := d.vcs.ChangedFiles(ctx, info.WorktreePath, info.BaseBranch)
	if err != nil {
		return nil, err
	}
	if changes == nil {
		changes = []vcs.FileChange{}
	}
	res := protocol.DiffResult{Raw: raw, Changes: changes}
	if res.Base, err = d.vcs.ResolveRev(ctx, info.WorktreePath, info.BaseBranch); err != nil {
		return nil, err
	}
	if res.Head, err = d.vcs.ResolveRev(ctx, info.WorktreePath, "HEAD"); err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.changes, s.changesAt, s.diffSig = changes, d.now(), changeSignature(changes)
	s.mu.Unlock()
	return res, nil
}

// hFileRead reads a file of a session's worktree, at a revision or as it is
// on disk. The path must stay inside the worktree.
func (d *Daemon) hFileRead(ctx context.Context, _ *conn, p protocol.FileReadParams) (any, error) {
	s, err := d.getSession(p.SessionID)
	if err != nil {
		return nil, err
	}
	rel := filepath.ToSlash(p.Path)
	if rel == "" || strings.HasPrefix(rel, "/") || filepath.IsAbs(p.Path) || hasDotDot(rel) {
		return nil, badRequest("path %q must be relative to the worktree", p.Path)
	}
	data, err := d.vcs.ReadFile(ctx, s.snap().WorktreePath, p.Rev, rel)
	if err != nil {
		return nil, notFound("cannot read %s: %v", rel, err)
	}
	if len(data) > maxFileRead {
		return nil, badRequest("%s is larger than %d MiB", rel, maxFileRead>>20)
	}
	res := protocol.FileReadResult{Path: rel}
	probe := data
	if len(probe) > 8000 {
		probe = probe[:8000]
	}
	if bytes.IndexByte(probe, 0) >= 0 {
		res.Binary = true
		return res, nil
	}
	res.Content = string(data)
	return res, nil
}

// hasDotDot reports whether a slash path has a ".." element.
func hasDotDot(p string) bool {
	for _, part := range strings.Split(p, "/") {
		if part == ".." {
			return true
		}
	}
	return false
}
