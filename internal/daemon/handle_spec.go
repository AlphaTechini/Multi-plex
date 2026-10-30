package daemon

import (
	"context"
	"errors"
	"io/fs"

	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/spec"
)

// hSpecList lists the specs in a workspace.
func (d *Daemon) hSpecList(ctx context.Context, _ *conn, p protocol.WorkspaceRef) (any, error) {
	ws, err := d.workspace(ctx, p.WorkspaceID)
	if err != nil {
		return nil, err
	}
	infos, err := spec.List(ws.Root)
	if err != nil {
		return nil, err
	}
	roots := []string{ws.Root}
	for _, s := range d.sessionsOf(ws.ID) {
		roots = append(roots, s.snap().WorktreePath)
	}
	out := make([]protocol.SpecInfo, len(infos))
	for i, in := range infos {
		out[i] = protocol.SpecInfo{
			Slug: in.Slug, Title: in.Title,
			TasksDone: in.TasksDone, TasksTotal: in.TasksTotal, CriteriaTotal: in.CriteriaTotal,
			CriteriaVerified: verifiedCriteria(roots, in.Slug),
		}
	}
	return out, nil
}

// hSpecGet reads one spec's three files.
func (d *Daemon) hSpecGet(ctx context.Context, _ *conn, p protocol.SpecGetParams) (any, error) {
	ws, err := d.workspace(ctx, p.WorkspaceID)
	if err != nil {
		return nil, err
	}
	doc, err := spec.Read(ws.Root, p.Slug)
	if err != nil {
		if errors.Is(err, spec.ErrInvalidSlug) {
			return nil, badRequest("%v", err)
		}
		if errors.Is(err, fs.ErrNotExist) {
			return nil, notFound("no such spec %q", p.Slug)
		}
		return nil, err
	}
	return protocol.SpecDoc{Slug: doc.Slug, Requirements: doc.Requirements, Design: doc.Design, Tasks: doc.Tasks}, nil
}
