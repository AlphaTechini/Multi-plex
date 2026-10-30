// Package gitremote implements daemon.Remotes with the git command line.
package gitremote

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Git runs the git executable for remote operations.
type Git struct {
	// Path is the git executable; empty means "git" from PATH.
	Path string
}

func (g Git) exe() string {
	if g.Path != "" {
		return g.Path
	}
	return "git"
}

// HasRemote reports whether repo has a remote called name.
func (g Git) HasRemote(ctx context.Context, repo, name string) bool {
	if name == "" || strings.HasPrefix(name, "-") {
		return false
	}
	cmd := exec.CommandContext(ctx, g.exe(), "config", "--get", "remote."+name+".url")
	cmd.Dir = repo
	cmd.Env = cleanGitEnv(os.Environ())
	hideWindow(cmd)
	out, err := cmd.Output()
	return err == nil && len(bytes.TrimSpace(out)) > 0
}

// Push runs `git push <remote> <branch>` in repo with the user's full
// environment (credential helpers included), without ever prompting on a
// terminal the daemon does not have. It returns git's combined output.
func (g Git) Push(ctx context.Context, repo, remote, branch string) (string, error) {
	if strings.HasPrefix(remote, "-") || strings.HasPrefix(branch, "-") || remote == "" || branch == "" {
		return "", fmt.Errorf("gitremote: invalid remote or branch")
	}
	cmd := exec.CommandContext(ctx, g.exe(), "push", remote, branch)
	cmd.Dir = repo
	cmd.Env = append(cleanGitEnv(os.Environ()), "GIT_TERMINAL_PROMPT=0")
	cmd.WaitDelay = 2 * time.Second
	hideWindow(cmd)
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	err := cmd.Run()
	return strings.TrimSpace(buf.String()), err
}

// cleanGitEnv drops variables that would send git to a different repository
// than cmd.Dir.
func cleanGitEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		switch strings.ToUpper(name) {
		case "GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR", "GIT_PREFIX":
			continue
		}
		out = append(out, kv)
	}
	return out
}
