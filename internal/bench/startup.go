package bench

import (
	"github.com/alphatechini/multi-plex/internal/daemon"

	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/alphatechini/multi-plex/internal/client"
	"github.com/alphatechini/multi-plex/internal/config"
	"github.com/alphatechini/multi-plex/internal/pty"
	"github.com/alphatechini/multi-plex/internal/pty/xptyimpl"
	"github.com/alphatechini/multi-plex/internal/term"
	"github.com/alphatechini/multi-plex/internal/vt/charmvt"
)

// startupOpts are the flags of `mpx bench startup`.
type startupOpts struct {
	n    int
	cold int
	warm int
	tui  bool
}

func startupFlags(fs *flag.FlagSet) *startupOpts {
	o := &startupOpts{}
	fs.IntVar(&o.n, "n", 30, "runs of 'mpx version' and of a no-op 'mpx hook'")
	fs.IntVar(&o.cold, "cold", 3, "TUI runs with no daemon running (each starts one)")
	fs.IntVar(&o.warm, "warm", 10, "TUI runs attaching to a running daemon")
	fs.BoolVar(&o.tui, "tui", true, "also time the TUI's first frame (needs the TUI to support "+EnvFirstFrame+"=1)")
	return o
}

// runStartup times process start and, when the TUI supports it, the time to
// its first frame (docs/03: cold < 150 ms, attach < 50 ms).
func runStartup(o *startupOpts) (*Result, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	res := newResult("startup")

	version, err := timeRuns(o.n, func() error { return exec.Command(exe, "version").Run() })
	if err != nil {
		return nil, fmt.Errorf("mpx version: %w", err)
	}
	res.addTiming("'mpx version' process (runtime + package init + exit)", version, "", 0, nil)

	noop, err := timeRuns(o.n, func() error {
		argv := append(daemon.HookCommand(exe), "pre-tool")
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Env = cleanEnv(os.Environ()) // no MPX_SESSION_ID: exits at once
		return cmd.Run()
	})
	if err != nil {
		return nil, fmt.Errorf("mpx hook: %w", err)
	}
	res.addTiming("hook process that exits at once (the floor for hook latency)", noop, "", 0, nil)

	if o.tui {
		if err := startupTUI(res, exe, o); err != nil {
			res.note("TUI first frame not measured: %v", err)
		}
	}
	return res, nil
}

// timeRuns calls fn n times (after one warm-up) and times each call.
func timeRuns(n int, fn func() error) ([]time.Duration, error) {
	if err := fn(); err != nil {
		return nil, err
	}
	out := make([]time.Duration, 0, n)
	for i := 0; i < n; i++ {
		t0 := time.Now()
		if err := fn(); err != nil {
			return nil, err
		}
		out = append(out, time.Since(t0))
	}
	return out, nil
}

// startupTUI measures the TUI's first frame cold and warm, each in a private
// MPX_HOME so the user's real daemon is never touched.
func startupTUI(res *Result, exe string, o *startupOpts) error {
	var cold, warm []time.Duration
	for i := 0; i < o.cold; i++ {
		home, err := os.MkdirTemp("", "mpxstart")
		if err != nil {
			return err
		}
		d, err := tuiFirstFrame(exe, home)
		stopDaemonAt(home)
		_ = os.RemoveAll(home)
		if err != nil {
			return err
		}
		cold = append(cold, d)
	}
	if o.warm > 0 {
		home, err := os.MkdirTemp("", "mpxstart")
		if err != nil {
			return err
		}
		defer os.RemoveAll(home)
		defer stopDaemonAt(home)
		if _, err := tuiFirstFrame(exe, home); err != nil { // starts the daemon
			return err
		}
		for i := 0; i < o.warm; i++ {
			d, err := tuiFirstFrame(exe, home)
			if err != nil {
				return err
			}
			warm = append(warm, d)
		}
	}
	res.addTiming("TUI first frame, cold (no daemon yet)", cold, "< 150ms p50", BudgetColdStart, func(s Stats) float64 { return s.P50 })
	res.addTiming("TUI first frame, attach to running daemon", warm, "< 50ms p50", BudgetWarmAttach, func(s Stats) float64 { return s.P50 })
	return nil
}

// tuiFirstFrame runs the TUI once in a pseudo-terminal with
// MPX_BENCH_FIRST_FRAME=1 and returns the time it reports.
func tuiFirstFrame(exe, home string) (time.Duration, error) {
	env := append(cleanEnv(os.Environ()), "MPX_HOME="+home, EnvFirstFrame+"=1")
	spec := pty.Spec{Argv: []string{exe}, Env: env, Dir: home, Cols: 100, Rows: 30}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	// The host answers the terminal queries a TUI sends at startup.
	h, err := term.Start(ctx, xptyimpl.New(), charmvt.Factory, spec, "bench", nil)
	if err != nil {
		return 0, err
	}
	defer h.Close()
	done := make(chan int, 1)
	go func() { code, _ := h.Wait(); done <- code }()
	select {
	case code := <-done:
		out := strings.Join(h.LastLines(spec.Rows), "\n")
		d, ok := parseFirstFrame(out)
		if !ok {
			if code != 0 {
				return 0, fmt.Errorf("the TUI exited with code %d and printed no %s line (%s)", code, strings.TrimSuffix(FirstFramePrefix, "="), lastLine(out))
			}
			return 0, fmt.Errorf("the TUI printed no %s line; it must support %s=1", strings.TrimSuffix(FirstFramePrefix, "="), EnvFirstFrame)
		}
		return d, nil
	case <-ctx.Done():
		return 0, fmt.Errorf("the TUI did not exit within 20s of %s=1", EnvFirstFrame)
	}
}

// parseFirstFrame finds "first_frame=<duration>" in the TUI's output.
func parseFirstFrame(out string) (time.Duration, bool) {
	for _, line := range strings.Split(out, "\n") {
		_, rest, ok := strings.Cut(line, FirstFramePrefix)
		if !ok {
			continue
		}
		tok := strings.Fields(rest)
		if len(tok) == 0 {
			continue
		}
		if d, err := time.ParseDuration(tok[0]); err == nil {
			return d, true
		}
	}
	return 0, false
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

// stopDaemonAt shuts down the daemon that a TUI run started under home.
func stopDaemonAt(home string) {
	oldHome, hadHome := os.LookupEnv(config.EnvHome)
	oldSock, hadSock := os.LookupEnv(config.EnvSocket)
	_ = os.Setenv(config.EnvHome, home)
	_ = os.Unsetenv(config.EnvSocket)
	defer func() {
		restoreEnv(config.EnvHome, oldHome, hadHome)
		restoreEnv(config.EnvSocket, oldSock, hadSock)
	}()
	paths, err := config.DefaultPaths()
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	c, err := client.Dial(ctx, config.SocketAddress(paths))
	if err != nil {
		return
	}
	defer c.Close()
	_ = c.Shutdown(ctx)
	select { // give it a moment to remove its socket
	case <-c.Done():
	case <-time.After(2 * time.Second):
	}
}

func restoreEnv(k, v string, had bool) {
	if had {
		_ = os.Setenv(k, v)
	} else {
		_ = os.Unsetenv(k)
	}
}
