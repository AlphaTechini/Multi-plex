package claudecode

import (
	"bytes"
	"encoding/json"

	"github.com/alphatechini/multi-plex/internal/agent"
)

// preToolOutput is the stdout of a PreToolUse hook.
type preToolOutput struct {
	Specific preToolDecision `json:"hookSpecificOutput"`
}

type preToolDecision struct {
	HookEventName string         `json:"hookEventName"`
	Decision      string         `json:"permissionDecision"`
	Reason        string         `json:"permissionDecisionReason,omitempty"`
	UpdatedInput  map[string]any `json:"updatedInput,omitempty"`
}

// EncodeHookResponse turns the daemon's answer into what Claude Code expects
// on stdout, plus the exit code.
//
// For PreToolUse it prints a hookSpecificOutput object with permissionDecision
// allow, deny or ask (a missing decision is treated as ask, the safe choice;
// "defer" is not used, it only works in non-interactive mode) and exits 0.
// UpdatedInput replaces the tool's whole input in Claude Code, so the
// original input from ev is kept and the response's fields are laid over it.
// Every other event prints nothing and exits 0.
func (Adapter) EncodeHookResponse(ev agent.HookEvent, r agent.HookResponse) ([]byte, int) {
	if ev.Kind != agent.HookPreTool {
		return nil, 0
	}
	d := preToolDecision{HookEventName: "PreToolUse", Reason: r.Reason}
	switch r.Decision {
	case agent.Allow:
		d.Decision = "allow"
	case agent.Deny:
		d.Decision = "deny"
		if d.Reason == "" {
			d.Reason = "Blocked by Multiplex."
		}
	default:
		d.Decision = "ask"
		if d.Reason == "" {
			d.Reason = "Multiplex has no decision for this action."
		}
	}
	if len(r.UpdatedInput) > 0 {
		d.UpdatedInput = make(map[string]any, len(ev.ToolInput)+len(r.UpdatedInput))
		for k, v := range ev.ToolInput {
			d.UpdatedInput[k] = v
		}
		for k, v := range r.UpdatedInput {
			d.UpdatedInput[k] = v
		}
	}
	return mustJSON(preToolOutput{Specific: d}), 0
}

// FailClosed returns the output that blocks a tool call when the daemon
// cannot be reached: a deny decision on stdout and exit code 2. Exit code 2
// blocks on its own (Claude Code then reads the reason from stderr, which
// this interface cannot carry, so `mpx hook` should also print reason to
// stderr), and the deny JSON gives the same verdict and reason where Claude
// Code does read stdout. The hooks are also registered with onFailure
// "block", so a crash or timeout of `mpx hook` blocks as well.
func (Adapter) FailClosed(reason string) ([]byte, int) {
	if reason == "" {
		reason = "Multiplex is not reachable."
	}
	out := preToolOutput{Specific: preToolDecision{
		HookEventName: "PreToolUse",
		Decision:      "deny",
		Reason:        "Blocked by Multiplex: " + reason,
	}}
	return mustJSON(out), 2
}

// mustJSON encodes v as a single-line JSON object plus newline, without HTML
// escaping, so stdout holds exactly one JSON object. The types used are all
// marshalable, so an error is a programming mistake.
func mustJSON(v any) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		panic("claudecode: " + err.Error())
	}
	return buf.Bytes()
}
