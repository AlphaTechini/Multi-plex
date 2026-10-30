// Command mpx is Multiplex: the TUI, the daemon, the agent hook and MCP
// entry points, benchmarks and diagnostics in one binary
// (docs/01-architecture.md "Binary roles").
package main

import (
	"fmt"
	"os"

	"github.com/alphatechini/multi-plex/internal/bench"
	"github.com/alphatechini/multi-plex/internal/hookcmd"
)

func main() {
	// `mpx hook` runs on every agent tool call and must be as fast as
	// possible, so it is dispatched before anything else.
	if len(os.Args) > 1 && os.Args[1] == "hook" {
		os.Exit(hookcmd.Run(os.Args[2:], os.Stdin, os.Stdout, os.Stderr, os.Getenv))
	}
	os.Exit(run(os.Args[1:]))
}

// run dispatches the subcommand and returns the exit code.
func run(args []string) int {
	cmd, rest := "", args
	if len(args) > 0 && len(args[0]) > 0 && args[0][0] != '-' {
		cmd, rest = args[0], args[1:]
	}
	var err error
	switch cmd {
	case "", "tui":
		err = runTUI(rest)
	case "daemon":
		err = runDaemon(rest)
	case "mcp":
		err = runMCP(rest)
	case "bench":
		return bench.Run(rest, os.Stdout, os.Stderr)
	case "version":
		runVersion(os.Stdout)
	case "doctor":
		return runDoctor(os.Stdout)
	case "push":
		err = runPush(rest)
	case "help":
		fmt.Fprint(os.Stdout, usage)
	default:
		fmt.Fprintf(os.Stderr, "mpx: unknown command %q\n\n%s", cmd, usage)
		return 2
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "mpx %s: %v\n", cmdName(cmd), err)
		return 1
	}
	return 0
}

func cmdName(cmd string) string {
	if cmd == "" {
		return "tui"
	}
	return cmd
}

const usage = `usage: mpx [command]

  (none)         open the terminal UI (starts the daemon if needed)
  daemon         run the daemon in the foreground
  hook <event>   agent hook entry point (called by the agent CLI)
  mcp            MCP server for an agent (called by the agent CLI)
  push [session] push a session's branch after a human-present approval
  bench <name>   performance measurements: startup, hook, render
  doctor         check the installation
  version        print the version
`
