package claudecode

import "github.com/alphatechini/multi-plex/internal/agent"

// classifyTool maps a Claude Code tool name to a neutral tool class. Unknown
// tools, MCP tools (mcp__*), WebFetch, WebSearch, Task and TodoWrite are
// "other".
func classifyTool(name string) agent.ToolClass {
	switch name {
	case "Bash", "PowerShell":
		return agent.ToolShell
	case "Edit", "MultiEdit", "Write", "NotebookEdit":
		return agent.ToolWrite
	case "Read", "Grep", "Glob", "LS", "NotebookRead":
		return agent.ToolRead
	}
	return agent.ToolOther
}

// toolPaths returns the files a tool reads or writes, as Claude Code gave
// them (absolute, native separators). Search patterns are not paths.
func toolPaths(class agent.ToolClass, input map[string]any) []string {
	var keys []string
	switch class {
	case agent.ToolWrite:
		keys = []string{"file_path", "notebook_path"}
	case agent.ToolRead:
		keys = []string{"file_path", "notebook_path", "path"}
	default:
		return nil
	}
	var out []string
	for _, k := range keys {
		if s, ok := input[k].(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}
