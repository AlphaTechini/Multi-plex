package main

// The agent adapters register themselves with the agent registry from init.
import (
	_ "github.com/alphatechini/multi-plex/internal/agent/claudecode"
	_ "github.com/alphatechini/multi-plex/internal/agent/shell"
)
