package daemon

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/vcs"
)

// diffDebounce is how long after the last edit a diff.changed event waits.
const diffDebounce = 300 * time.Millisecond

// changesOf returns the session's changed files, using the cached list while
// it is fresh.
func (d *Daemon) changesOf(ctx context.Context, s *session) ([]vcs.FileChange, error) {
	s.mu.Lock()
	if s.changes != nil && time.Since(s.changesAt) < changesFresh {
		out := slices.Clone(s.changes)
		s.mu.Unlock()
		return out, nil
	}
	s.mu.Unlock()
	info := s.snap()
	ch, err := d.vcs.ChangedFiles(ctx, info.WorktreePath, info.BaseBranch)
	if err != nil {
		return nil, err
	}
	if ch == nil {
		ch = []vcs.FileChange{}
	}
	s.mu.Lock()
	s.changes, s.changesAt = ch, time.Now()
	s.mu.Unlock()
	return slices.Clone(ch), nil
}

// changeSignature summarises a change list so two lists compare cheaply.
func changeSignature(ch []vcs.FileChange) string {
	var b strings.Builder
	for _, c := range ch {
		b.WriteString(c.Path)
		b.WriteByte(0)
		b.WriteString(c.OldPath)
		b.WriteByte(0)
		b.WriteString(string(c.Status))
		b.WriteByte(0)
		b.WriteString(strconv.Itoa(c.Added))
		b.WriteByte('/')
		b.WriteString(strconv.Itoa(c.Deleted))
		b.WriteByte(';')
	}
	return b.String()
}

// announceDiff publishes diff.changed for a session at once.
func (d *Daemon) announceDiff(s *session) {
	s.mu.Lock()
	s.changesAt = time.Time{} // the cached list is stale now
	s.mu.Unlock()
	d.publish(protocol.TopicDiffChanged, protocol.SessionRef{SessionID: s.id})
}

// scheduleDiffChanged publishes diff.changed once edits have been quiet for
// 300 ms, so a burst of edits makes one event.
func (d *Daemon) scheduleDiffChanged(s *session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.diffTimer != nil {
		s.diffTimer.Reset(diffDebounce)
		return
	}
	s.diffTimer = time.AfterFunc(diffDebounce, func() {
		if d.ctx.Err() != nil {
			return
		}
		d.announceDiff(s)
	})
}
