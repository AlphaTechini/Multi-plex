package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/alphatechini/multi-plex/internal/client"
	"github.com/alphatechini/multi-plex/internal/config"
	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/mcpserver"
)

// runMCP is `mpx mcp`: the stdio MCP server an agent CLI launches. stdout
// carries the protocol, so diagnostics go to stderr.
func runMCP(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("takes no arguments (got %q)", args[0])
	}
	sid := core.SessionID(os.Getenv("MPX_SESSION_ID"))
	if sid == "" {
		fmt.Fprintln(os.Stderr, "mpx mcp: MPX_SESSION_ID is not set; the Multiplex tools will report that they are unavailable")
	}
	addr, err := daemonAddress()
	if err != nil {
		return err
	}
	fwd := mcpserver.NewForwarder(func(ctx context.Context) (*client.Client, error) {
		dctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		return client.Dial(dctx, addr)
	})
	defer fwd.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return mcpserver.RunStdio(ctx, mcpserver.New(version, sid, fwd))
}

// daemonAddress is the daemon's socket address: MPX_SOCKET, else the default.
func daemonAddress() (string, error) {
	paths, err := config.DefaultPaths()
	if err != nil {
		return "", err
	}
	return config.SocketAddress(paths), nil
}
