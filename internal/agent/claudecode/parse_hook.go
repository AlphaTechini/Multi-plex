package claudecode

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/alphatechini/multi-plex/internal/agent"
)

// hookPayload is the part of Claude Code's hook stdin JSON that Multiplex
// reads. Fields not listed are ignored.
type hookPayload struct {
	SessionID        string         `json:"session_id"`
	Cwd              string         `json:"cwd"`
	HookEventName    string         `json:"hook_event_name"`
	ToolName         string         `json:"tool_name"`
	ToolInput        map[string]any `json:"tool_input"`
	Message          string         `json:"message"`
	NotificationType string         `json:"notification_type"`
}

// hookKind resolves the event name passed on the command line (`mpx hook
// <event>`): the neutral names Prepare writes, or Claude Code's own names.
func hookKind(event string) (kind agent.HookKind, permission bool, ok bool) {
	switch strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(event, "-", ""), "_", "")) {
	case "pretool", "pretooluse":
		return agent.HookPreTool, false, true
	case "posttool", "posttooluse":
		return agent.HookPostTool, false, true
	case "permissionrequest":
		return agent.HookNotification, true, true
	case "notification":
		return agent.HookNotification, false, true
	case "stop":
		return agent.HookStop, false, true
	case "promptsubmit", "userpromptsubmit":
		return agent.HookPromptSubmit, false, true
	case "sessionstart":
		return agent.HookSessionStart, false, true
	case "sessionend":
		return agent.HookSessionEnd, false, true
	}
	return "", false, false
}

// ParseHook translates the stdin JSON of a Claude Code hook into a neutral
// event. event is the name given to `mpx hook`; if empty, the payload's
// hook_event_name is used.
//
// A PermissionRequest hook becomes a HookNotification whose Message starts
// with PermissionPrefix: Claude Code is about to show its own permission
// prompt, so the session is blocked on the user. The tool name, input,
// command and paths are filled in as for a PreToolUse.
func (Adapter) ParseHook(event string, payload []byte) (agent.HookEvent, error) {
	var p hookPayload
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &p); err != nil {
			return agent.HookEvent{}, fmt.Errorf("claudecode: parse %s hook payload: %w", event, err)
		}
	}
	if event == "" {
		event = p.HookEventName
	}
	kind, permission, ok := hookKind(event)
	if !ok {
		return agent.HookEvent{}, fmt.Errorf("claudecode: unknown hook event %q", event)
	}

	ev := agent.HookEvent{Kind: kind, AgentRef: p.SessionID, Cwd: p.Cwd}
	switch kind {
	case agent.HookPreTool, agent.HookPostTool:
		fillTool(&ev, p)
		if kind == agent.HookPreTool {
			ev.ToolInput = p.ToolInput // needed to build updatedInput
		} else {
			ev.Edits = editRanges(p.ToolName, p.ToolInput, p.Cwd)
		}
	case agent.HookNotification:
		ev.Message = p.Message
		if permission {
			fillTool(&ev, p)
			ev.ToolInput = p.ToolInput
			ev.Message = permissionMessage(p.ToolName)
		} else if p.NotificationType == "permission_prompt" {
			ev.Message = PermissionPrefix + " " + p.Message
		}
	}
	return ev, nil
}

// fillTool sets the tool fields of ev from the payload.
func fillTool(ev *agent.HookEvent, p hookPayload) {
	ev.ToolName = p.ToolName
	ev.ToolClass = classifyTool(p.ToolName)
	if ev.ToolClass == agent.ToolShell {
		ev.Command, _ = p.ToolInput["command"].(string)
	}
	ev.Paths = toolPaths(ev.ToolClass, p.ToolInput)
}

func permissionMessage(tool string) string {
	if tool == "" {
		return PermissionPrefix + " waiting for permission"
	}
	return PermissionPrefix + " waiting for permission to use " + tool
}
