package app

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/alphatechini/multi-plex/internal/core"
)

// steeringItem is one entry of the sidebar's STEERING section.
type steeringItem struct {
	// Label is what the sidebar shows ("AGENTS.md", "~/.claude/CLAUDE.md").
	Label string
	// Path is what the Code view opens: workspace-relative and slash-separated
	// for repository files, absolute for the user's global files.
	Path string
	// Dir marks the .multiplex/ directory.
	Dir bool
}

// workspaceInfo is what the sidebar shows besides sessions and specs.
type workspaceInfo struct {
	Steering           []steeringItem
	Roles, Skills, MCP int
}

// probeWorkspace looks at the workspace on disk: steering files that exist,
// and the number of skills and MCP servers. It does file I/O, so the shell
// calls it only from a command. The roles count comes from the daemon.
func probeWorkspace(ws core.Workspace) workspaceInfo {
	var info workspaceInfo
	if ws.Root == "" {
		return info
	}
	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		if isFile(filepath.Join(ws.Root, name)) {
			info.Steering = append(info.Steering, steeringItem{Label: name, Path: name})
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		for _, rel := range globalInstructionFiles {
			full := filepath.Join(home, filepath.FromSlash(rel))
			if isFile(full) {
				info.Steering = append(info.Steering, steeringItem{Label: "~/" + rel, Path: full})
			}
		}
	}
	if isDir(filepath.Join(ws.Root, ".multiplex")) {
		info.Steering = append(info.Steering, steeringItem{Label: ".multiplex/", Path: ".multiplex", Dir: true})
	}
	info.Skills = countEntries(filepath.Join(ws.Root, ".multiplex", "skills")) + countEntries(filepath.Join(ws.Root, ".claude", "skills"))
	info.MCP = countMCPServers(filepath.Join(ws.Root, ".mcp.json"))
	return info
}

// globalInstructionFiles are the user-level instruction files of the agent
// CLIs, relative to the home directory, slash-separated.
var globalInstructionFiles = []string{
	".claude/CLAUDE.md",
	".codex/AGENTS.md",
	".config/opencode/AGENTS.md",
	".gemini/GEMINI.md",
}

func isFile(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.Mode().IsRegular()
}

func isDir(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}

func countEntries(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	return len(entries)
}

func countMCPServers(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	var f struct {
		Servers map[string]json.RawMessage `json:"mcpServers"`
	}
	if json.Unmarshal(data, &f) != nil {
		return 0
	}
	return len(f.Servers)
}
