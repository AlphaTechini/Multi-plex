package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/alphatechini/multi-plex/internal/client"
	"github.com/alphatechini/multi-plex/internal/core"
)

// runPush is `mpx push [session]`, the push broker's client (docs/04-safety.md
// Layer 3): it asks the daemon to push the session's branch, which needs a
// human-present approval, and prints git's output. The session is the
// argument (ID or name), else MPX_SESSION_ID, else the session whose
// worktree contains the current directory.
func runPush(args []string) error {
	fs := flag.NewFlagSet("push", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 1 {
		return errors.New("usage: mpx push [session]")
	}
	addr, err := daemonAddress()
	if err != nil {
		return err
	}
	dctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	c, err := client.Dial(dctx, addr)
	cancel()
	if err != nil {
		return fmt.Errorf("the Multiplex daemon is not running (%v); start Multiplex first", err)
	}
	defer c.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	sess, err := findPushSession(ctx, c, fs.Arg(0))
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Pushing %s: waiting for your approval in Multiplex...\n", sess.Branch)
	res, err := c.Push(ctx, sess.ID)
	if err != nil {
		return err
	}
	if res.Output != "" {
		fmt.Println(res.Output)
	}
	fmt.Printf("Pushed %s to %s.\n", res.Branch, res.Remote)
	return nil
}

// findPushSession picks the session to push.
func findPushSession(ctx context.Context, c *client.Client, ref string) (core.Session, error) {
	list, err := c.Sessions(ctx, "")
	if err != nil {
		return core.Session{}, err
	}
	if ref == "" {
		ref = os.Getenv("MPX_SESSION_ID")
	}
	if ref != "" {
		for _, s := range list {
			if string(s.ID) == ref || s.Slug == ref {
				return s, nil
			}
		}
		return core.Session{}, fmt.Errorf("no session %q", ref)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return core.Session{}, err
	}
	for _, s := range list {
		if insideDir(s.WorktreePath, cwd) {
			return s, nil
		}
	}
	return core.Session{}, errors.New("the current directory is not inside a session's worktree; name the session: mpx push <session>")
}

// insideDir reports whether path is dir or lies under it, resolving symlinks.
func insideDir(dir, path string) bool {
	if r, err := filepath.EvalSymlinks(dir); err == nil {
		dir = r
	}
	if r, err := filepath.EvalSymlinks(path); err == nil {
		path = r
	}
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
