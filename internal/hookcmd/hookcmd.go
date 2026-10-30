// Package hookcmd is `mpx hook <event>`: the command an agent CLI runs on its
// hook events. It forwards the event to the daemon and prints the decision.
//
// It must start and exit fast (docs/03-performance.md: under 10 ms on Linux),
// so it imports only the client, protocol, config and the Claude Code adapter,
// and never the TUI, SQLite or syntax-highlighting packages. Check with
//
//	go list -deps ./internal/hookcmd | grep -E 'bubbletea|lipgloss|sqlite|chroma'
//
// A failure while handling a pre-tool event fails closed (docs/04-safety.md):
// the tool call is blocked. For every other event the command stays silent and
// exits 0, so a broken daemon never stops an agent that was not asking
// permission.
package hookcmd

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/alphatechini/multi-plex/internal/agent"
	_ "github.com/alphatechini/multi-plex/internal/agent/claudecode" // registers the claude-code adapter
	"github.com/alphatechini/multi-plex/internal/client"
	"github.com/alphatechini/multi-plex/internal/config"
	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

// Limits and timeouts.
const (
	// MaxStdin caps how much of the hook payload is read.
	MaxStdin = 4 << 20
	// ConnectTimeout bounds dialing the daemon.
	ConnectTimeout = 2 * time.Second
	// CallTimeout bounds every call except pre-tool, which waits as long as
	// an approval takes (the agent CLI's own hook timeout bounds it).
	CallTimeout = 5 * time.Second
)

// Environment variables the daemon sets for the agent's process.
const (
	EnvSession = "MPX_SESSION_ID"
	EnvAdapter = "MPX_ADAPTER"
	EnvSocket  = config.EnvSocket
	// DefaultAdapter is used when MPX_ADAPTER is not set.
	DefaultAdapter = "claude-code"
)

// Run executes `mpx hook <event>`. args are the arguments after "hook". It
// returns the exit code for the process.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer, getenv func(string) string) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: mpx hook <event>")
		return 0
	}
	event := args[0]
	sessionID := getenv(EnvSession)
	if sessionID == "" {
		return 0 // not launched by Multiplex: do not interfere
	}
	gating := isPreTool(event)

	payload, _ := io.ReadAll(io.LimitReader(stdin, MaxStdin))

	name := getenv(EnvAdapter)
	if name == "" {
		name = DefaultAdapter
	}
	ad, ok := agent.Get(name)
	if !ok {
		return failure(gating, nil, fmt.Sprintf("unknown agent adapter %q", name), stdout, stderr)
	}
	ev, err := ad.ParseHook(event, payload)
	if err != nil {
		return failure(gating, ad, "could not read the hook payload", stdout, stderr)
	}
	// The adapter may know the event by another name (Claude Code's own).
	gating = ev.Kind == agent.HookPreTool

	addr := getenv(EnvSocket)
	if addr == "" {
		paths, err := config.DefaultPaths()
		if err != nil {
			return failure(gating, ad, "cannot locate the Multiplex daemon: "+err.Error(), stdout, stderr)
		}
		addr = config.SocketAddress(paths)
	}

	dctx, cancel := context.WithTimeout(context.Background(), ConnectTimeout)
	c, err := client.Dial(dctx, addr)
	cancel()
	if err != nil {
		return failure(gating, ad, "cannot reach the Multiplex daemon ("+shortErr(err)+")", stdout, stderr)
	}
	defer c.Close()

	ctx := context.Background()
	if !gating {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, CallTimeout)
		defer cancel()
	}
	resp, err := c.Hook(ctx, protocol.HookParams{SessionID: core.SessionID(sessionID), Adapter: name, Event: ev})
	if err != nil {
		return failure(gating, ad, "the Multiplex daemon failed: "+shortErr(err), stdout, stderr)
	}

	out, code := ad.EncodeHookResponse(ev, resp)
	_, _ = stdout.Write(out)
	if code != 0 && resp.Reason != "" {
		fmt.Fprintln(stderr, resp.Reason)
	}
	return code
}

// failure handles an error: a pre-tool event fails closed, with the reason on
// stderr as well (Claude Code reads an exit-2 reason from there); any other
// event exits silently.
func failure(gating bool, ad agent.Adapter, reason string, stdout, stderr io.Writer) int {
	if !gating {
		return 0
	}
	if ad == nil {
		fmt.Fprintln(stderr, "Blocked by Multiplex: "+reason)
		return 2
	}
	out, code := ad.FailClosed(reason)
	_, _ = stdout.Write(out)
	fmt.Fprintln(stderr, "Blocked by Multiplex: "+reason)
	return code
}

// isPreTool reports whether an event name is a pre-tool event, in the
// neutral spelling or Claude Code's.
func isPreTool(event string) bool {
	e := strings.ToLower(strings.NewReplacer("-", "", "_", "").Replace(event))
	return e == "pretool" || e == "pretooluse"
}

// shortErr trims an error to one line.
func shortErr(err error) string {
	msg, _, _ := strings.Cut(err.Error(), "\n")
	return msg
}
