package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/alphatechini/multi-plex/internal/agent"
	"github.com/alphatechini/multi-plex/internal/approval/polkit"
	"github.com/alphatechini/multi-plex/internal/approval/windowshello"
	"github.com/alphatechini/multi-plex/internal/client"
	"github.com/alphatechini/multi-plex/internal/config"
)

// runDoctor prints what Multiplex sees on this machine and returns 1 if
// something it cannot work without is missing.
func runDoctor(w io.Writer) int {
	tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
	failed := false
	row := func(k, v string) { fmt.Fprintf(tw, "%s\t%s\n", k, v) }

	row("mpx", fmt.Sprintf("%s (%s/%s, %s)", version, runtime.GOOS, runtime.GOARCH, runtime.Version()))
	if exe, err := os.Executable(); err == nil {
		row("binary", exe)
	}

	paths, err := config.DefaultPaths()
	if err != nil {
		row("paths", "FAIL "+err.Error())
		_ = tw.Flush()
		return 1
	}
	if home := os.Getenv(config.EnvHome); home != "" {
		row("MPX_HOME", home)
	}
	row("data", paths.DataDir)
	row("config", paths.ConfigDir)
	row("runtime", paths.RuntimeDir)
	row("worktrees", paths.WorktreesDir)
	row("logs", filepath.Join(paths.LogDir, "daemon.log"))
	row("database", describeFile(paths.DBPath))

	settingsPath := filepath.Join(paths.ConfigDir, "settings.json")
	if _, err := config.LoadSettings(settingsPath); err != nil {
		row("settings", "WARN "+err.Error())
	} else if _, err := os.Stat(settingsPath); err != nil {
		row("settings", "defaults (no "+settingsPath+")")
	} else {
		row("settings", "ok "+settingsPath)
	}

	addr := config.SocketAddress(paths)
	row("socket", addr)
	row("daemon", describeDaemon(addr))

	gitLine, gitOK := describeGit()
	row("git", gitLine)
	failed = failed || !gitOK

	for _, a := range agentLines() {
		row("agent "+a.name, a.status)
	}

	row("approval", describeVerifiers())
	row("terminal", describeTerminal())
	_ = tw.Flush()
	if failed {
		return 1
	}
	return 0
}

func describeFile(path string) string {
	st, err := os.Stat(path)
	if err != nil {
		return path + " (not created yet)"
	}
	return fmt.Sprintf("%s (%d KiB)", path, st.Size()>>10)
}

// describeDaemon dials the daemon and asks hello.
func describeDaemon(addr string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	c, err := client.Dial(ctx, addr)
	if err != nil {
		return "not running (it starts when you run mpx)"
	}
	defer c.Close()
	h, err := c.Hello(ctx, "doctor")
	if err != nil {
		return "answering, but hello failed: " + err.Error()
	}
	return fmt.Sprintf("running: version %s, pid %d, approval via %s", h.Version, h.PID, h.Approver)
}

var gitVersionRe = regexp.MustCompile(`(\d+)\.(\d+)`)

// describeGit reports the git version and whether it is usable. Conflict
// prediction needs 2.38 or newer; older versions only warn.
func describeGit() (string, bool) {
	out, err := exec.Command("git", "--version").Output()
	if err != nil {
		return "FAIL git was not found on PATH; Multiplex needs it for worktrees", false
	}
	ver := strings.TrimSpace(string(out))
	if m := gitVersionRe.FindStringSubmatch(ver); m != nil {
		major, _ := strconv.Atoi(m[1])
		minor, _ := strconv.Atoi(m[2])
		if major < 2 || (major == 2 && minor < 38) {
			return "WARN " + ver + " (conflict prediction needs 2.38 or newer)", true
		}
	}
	return ver, true
}

type agentLine struct{ name, status string }

// agentLines reports, for each agent CLI, whether it is on PATH.
func agentLines() []agentLine {
	var out []agentLine
	look := func(label string, exes ...string) {
		for _, e := range exes {
			if p, err := exec.LookPath(e); err == nil {
				out = append(out, agentLine{label, "found " + p})
				return
			}
		}
		out = append(out, agentLine{label, "not found on PATH (" + exes[0] + ")"})
	}
	if _, ok := agent.Get("claude-code"); ok {
		if runtime.GOOS == "windows" {
			look("claude-code", "claude.exe", "claude.cmd", "claude")
		} else {
			look("claude-code", "claude")
		}
	}
	if a, ok := agent.Get("shell"); ok {
		if l, err := a.Prepare(agent.LaunchSpec{}); err == nil && len(l.Argv) > 0 {
			out = append(out, agentLine{"shell", l.Argv[0]})
		} else {
			out = append(out, agentLine{"shell", "unavailable: " + errString(err)})
		}
	}
	return out
}

func errString(err error) string {
	if err == nil {
		return "unknown"
	}
	return err.Error()
}

// describeVerifiers lists which approval methods work on this machine.
func describeVerifiers() string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var parts []string
	hello := windowshello.New(windowshello.Find())
	parts = append(parts, fmt.Sprintf("%s %s", hello.Name(), yesNo(hello.Available(ctx))))
	pk := polkit.New()
	parts = append(parts, fmt.Sprintf("%s %s", pk.Name(), yesNo(pk.Available(ctx))))
	parts = append(parts, "typed-code available (fallback)")
	return strings.Join(parts, ", ")
}

func yesNo(ok bool) string {
	if ok {
		return "available"
	}
	return "unavailable"
}

// describeTerminal reports the terminal environment the TUI would see.
func describeTerminal() string {
	var parts []string
	for _, k := range []string{"TERM", "COLORTERM", "TERM_PROGRAM", "WT_SESSION"} {
		if v := os.Getenv(k); v != "" {
			parts = append(parts, k+"="+v)
		}
	}
	if len(parts) == 0 {
		return "no terminal variables set (TERM, COLORTERM)"
	}
	return strings.Join(parts, " ")
}
