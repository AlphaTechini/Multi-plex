package daemon

import (
	"context"
	"time"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

// diffPollEvery is how often sessions in the working state are checked for
// changed files. Polling is cheap and avoids a file-watcher dependency.
const diffPollEvery = 2 * time.Second

// startBackground starts the daemon's background loops once.
func (d *Daemon) startBackground() {
	d.bgOnce.Do(func() {
		d.background(d.watchDiffs)
	})
}

// watchDiffs polls ChangedFiles for working sessions and publishes
// diff.changed when the set of files or their line counts change.
func (d *Daemon) watchDiffs() {
	t := time.NewTicker(diffPollEvery)
	defer t.Stop()
	for {
		select {
		case <-d.ctx.Done():
			return
		case <-t.C:
		}
		for _, s := range d.sessionsOf("") {
			if d.ctx.Err() != nil {
				return
			}
			if s.snap().State != core.StateWorking {
				continue
			}
			d.pollDiff(s)
		}
	}
}

// pollDiff refreshes one session's change list and announces a difference.
func (d *Daemon) pollDiff(s *session) {
	ctx, cancel := context.WithTimeout(d.ctx, 10*time.Second)
	defer cancel()
	info := s.snap()
	ch, err := d.vcs.ChangedFiles(ctx, info.WorktreePath, info.BaseBranch)
	if err != nil {
		d.log.Debug("poll changed files", "session", info.ID, "err", err)
		return
	}
	sig := changeSignature(ch)
	s.mu.Lock()
	changed := sig != s.diffSig
	s.diffSig = sig
	s.changes, s.changesAt = ch, time.Now()
	s.mu.Unlock()
	if changed {
		d.publish(protocol.TopicDiffChanged, protocol.SessionRef{SessionID: s.id})
	}
}
