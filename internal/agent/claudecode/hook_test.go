package claudecode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/alphatechini/multi-plex/internal/agent"
)

func TestParseHookEvents(t *testing.T) {
	tests := []struct {
		name    string
		event   string
		payload string
		want    agent.HookEvent
	}{
		{
			name:  "pre-tool bash",
			event: "pre-tool",
			payload: `{"session_id":"abc123","prompt_id":"550e8400","transcript_path":"/t.jsonl","cwd":"/home/u/proj",
				"permission_mode":"default","hook_event_name":"PreToolUse","tool_name":"Bash",
				"tool_input":{"command":"npm test","description":"Run test suite","timeout":120000,"run_in_background":false},
				"tool_use_id":"toolu_01ABC"}`,
			want: agent.HookEvent{
				Kind: agent.HookPreTool, AgentRef: "abc123", Cwd: "/home/u/proj", ToolName: "Bash",
				ToolClass: agent.ToolShell, Command: "npm test",
				ToolInput: map[string]any{"command": "npm test", "description": "Run test suite", "timeout": float64(120000), "run_in_background": false},
			},
		},
		{
			name:    "pre-tool powershell",
			event:   "pre-tool",
			payload: `{"session_id":"s","cwd":"C:\\proj","hook_event_name":"PreToolUse","tool_name":"PowerShell","tool_input":{"command":"Remove-Item -Recurse .\\build"}}`,
			want: agent.HookEvent{
				Kind: agent.HookPreTool, AgentRef: "s", Cwd: `C:\proj`, ToolName: "PowerShell", ToolClass: agent.ToolShell,
				Command: `Remove-Item -Recurse .\build`, ToolInput: map[string]any{"command": `Remove-Item -Recurse .\build`},
			},
		},
		{
			name:    "pre-tool write",
			event:   "pre-tool",
			payload: `{"session_id":"s","cwd":"/p","tool_name":"Write","tool_input":{"file_path":"/p/a.go","content":"package a\n"}}`,
			want: agent.HookEvent{
				Kind: agent.HookPreTool, AgentRef: "s", Cwd: "/p", ToolName: "Write", ToolClass: agent.ToolWrite,
				Paths: []string{"/p/a.go"}, ToolInput: map[string]any{"file_path": "/p/a.go", "content": "package a\n"},
			},
		},
		{
			name:    "pre-tool edit",
			event:   "pre-tool",
			payload: `{"tool_name":"Edit","tool_input":{"file_path":"/p/a.go","old_string":"x","new_string":"y","replace_all":false}}`,
			want: agent.HookEvent{
				Kind: agent.HookPreTool, ToolName: "Edit", ToolClass: agent.ToolWrite, Paths: []string{"/p/a.go"},
				ToolInput: map[string]any{"file_path": "/p/a.go", "old_string": "x", "new_string": "y", "replace_all": false},
			},
		},
		{
			name:    "pre-tool notebook edit",
			event:   "pre-tool",
			payload: `{"tool_name":"NotebookEdit","tool_input":{"notebook_path":"/p/n.ipynb","new_source":"1"}}`,
			want: agent.HookEvent{
				Kind: agent.HookPreTool, ToolName: "NotebookEdit", ToolClass: agent.ToolWrite, Paths: []string{"/p/n.ipynb"},
				ToolInput: map[string]any{"notebook_path": "/p/n.ipynb", "new_source": "1"},
			},
		},
		{
			name:    "pre-tool read",
			event:   "pre-tool",
			payload: `{"tool_name":"Read","tool_input":{"file_path":"/p/a.go","offset":10,"limit":50}}`,
			want: agent.HookEvent{
				Kind: agent.HookPreTool, ToolName: "Read", ToolClass: agent.ToolRead, Paths: []string{"/p/a.go"},
				ToolInput: map[string]any{"file_path": "/p/a.go", "offset": float64(10), "limit": float64(50)},
			},
		},
		{
			name:    "pre-tool grep (pattern is not a path)",
			event:   "pre-tool",
			payload: `{"tool_name":"Grep","tool_input":{"pattern":"TODO.*","path":"/p/src","output_mode":"content"}}`,
			want: agent.HookEvent{
				Kind: agent.HookPreTool, ToolName: "Grep", ToolClass: agent.ToolRead, Paths: []string{"/p/src"},
				ToolInput: map[string]any{"pattern": "TODO.*", "path": "/p/src", "output_mode": "content"},
			},
		},
		{
			name:    "pre-tool glob without path",
			event:   "pre-tool",
			payload: `{"tool_name":"Glob","tool_input":{"pattern":"**/*.go"}}`,
			want: agent.HookEvent{
				Kind: agent.HookPreTool, ToolName: "Glob", ToolClass: agent.ToolRead,
				ToolInput: map[string]any{"pattern": "**/*.go"},
			},
		},
		{
			name:    "pre-tool mcp",
			event:   "pre-tool",
			payload: `{"tool_name":"mcp__multiplex__report_progress","mcp_server":{"name":"multiplex"},"tool_input":{"done":["x"]}}`,
			want: agent.HookEvent{
				Kind: agent.HookPreTool, ToolName: "mcp__multiplex__report_progress", ToolClass: agent.ToolOther,
				ToolInput: map[string]any{"done": []any{"x"}},
			},
		},
		{
			name:    "pre-tool webfetch",
			event:   "pre-tool",
			payload: `{"tool_name":"WebFetch","tool_input":{"url":"https://example.com"}}`,
			want: agent.HookEvent{
				Kind: agent.HookPreTool, ToolName: "WebFetch", ToolClass: agent.ToolOther,
				ToolInput: map[string]any{"url": "https://example.com"},
			},
		},
		{
			name:    "post-tool bash has no input",
			event:   "post-tool",
			payload: `{"session_id":"s","cwd":"/p","tool_name":"Bash","tool_input":{"command":"ls"},"tool_response":{"stdout":"a"},"tool_use_id":"t","duration_ms":12}`,
			want:    agent.HookEvent{Kind: agent.HookPostTool, AgentRef: "s", Cwd: "/p", ToolName: "Bash", ToolClass: agent.ToolShell, Command: "ls"},
		},
		{
			name:    "post-tool write",
			event:   "post-tool",
			payload: `{"cwd":"/p","tool_name":"Write","tool_input":{"file_path":"/p/a.txt","content":"one\ntwo\nthree\n"},"tool_response":{}}`,
			want: agent.HookEvent{
				Kind: agent.HookPostTool, Cwd: "/p", ToolName: "Write", ToolClass: agent.ToolWrite, Paths: []string{"/p/a.txt"},
				Edits: []agent.EditRange{{Path: "/p/a.txt", StartLine: 1, EndLine: 3}},
			},
		},
		{
			name:    "notification",
			event:   "notification",
			payload: `{"session_id":"s","hook_event_name":"Notification","message":"Claude is waiting for your input","title":"Claude Code","notification_type":"idle_prompt"}`,
			want:    agent.HookEvent{Kind: agent.HookNotification, AgentRef: "s", Message: "Claude is waiting for your input"},
		},
		{
			name:    "notification permission prompt",
			event:   "notification",
			payload: `{"session_id":"s","message":"Claude needs your permission to use Bash","notification_type":"permission_prompt"}`,
			want:    agent.HookEvent{Kind: agent.HookNotification, AgentRef: "s", Message: "permission: Claude needs your permission to use Bash"},
		},
		{
			name:    "permission request",
			event:   "permission-request",
			payload: `{"session_id":"s","cwd":"/p","hook_event_name":"PermissionRequest","tool_name":"Bash","tool_input":{"command":"rm -rf build"},"permission_suggestions":[]}`,
			want: agent.HookEvent{
				Kind: agent.HookNotification, AgentRef: "s", Cwd: "/p", ToolName: "Bash", ToolClass: agent.ToolShell,
				Command: "rm -rf build", ToolInput: map[string]any{"command": "rm -rf build"},
				Message: "permission: waiting for permission to use Bash",
			},
		},
		{
			name:    "stop",
			event:   "stop",
			payload: `{"session_id":"s","cwd":"/p","hook_event_name":"Stop","stop_hook_active":false,"last_assistant_message":"Done."}`,
			want:    agent.HookEvent{Kind: agent.HookStop, AgentRef: "s", Cwd: "/p"},
		},
		{
			name:    "prompt submit",
			event:   "prompt-submit",
			payload: `{"session_id":"s","hook_event_name":"UserPromptSubmit","prompt":"fix the bug"}`,
			want:    agent.HookEvent{Kind: agent.HookPromptSubmit, AgentRef: "s"},
		},
		{
			name:    "session start",
			event:   "session-start",
			payload: `{"session_id":"s","hook_event_name":"SessionStart","source":"startup","model":"claude-opus"}`,
			want:    agent.HookEvent{Kind: agent.HookSessionStart, AgentRef: "s"},
		},
		{
			name:    "session end",
			event:   "session-end",
			payload: `{"session_id":"s","hook_event_name":"SessionEnd","reason":"prompt_input_exit"}`,
			want:    agent.HookEvent{Kind: agent.HookSessionEnd, AgentRef: "s"},
		},
		{
			name:    "claude event name on the command line",
			event:   "PreToolUse",
			payload: `{"tool_name":"Bash","tool_input":{"command":"ls"}}`,
			want: agent.HookEvent{
				Kind: agent.HookPreTool, ToolName: "Bash", ToolClass: agent.ToolShell, Command: "ls",
				ToolInput: map[string]any{"command": "ls"},
			},
		},
		{
			name:    "event taken from payload when omitted",
			event:   "",
			payload: `{"hook_event_name":"Stop","session_id":"z"}`,
			want:    agent.HookEvent{Kind: agent.HookStop, AgentRef: "z"},
		},
		{
			name:    "empty payload",
			event:   "stop",
			payload: ``,
			want:    agent.HookEvent{Kind: agent.HookStop},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Adapter{}.ParseHook(tt.event, []byte(tt.payload))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("event =\n %+v\nwant\n %+v", got, tt.want)
			}
		})
	}
}

func TestParseHookErrors(t *testing.T) {
	if _, err := (Adapter{}).ParseHook("pre-tool", []byte(`{not json`)); err == nil {
		t.Error("malformed JSON should fail")
	}
	if _, err := (Adapter{}).ParseHook("frobnicate", []byte(`{}`)); err == nil {
		t.Error("unknown event should fail")
	}
	if _, err := (Adapter{}).ParseHook("", []byte(`{}`)); err == nil {
		t.Error("no event name anywhere should fail")
	}
}

func TestClassifyTool(t *testing.T) {
	tests := map[string]agent.ToolClass{
		"Bash": agent.ToolShell, "PowerShell": agent.ToolShell,
		"Edit": agent.ToolWrite, "MultiEdit": agent.ToolWrite, "Write": agent.ToolWrite, "NotebookEdit": agent.ToolWrite,
		"Read": agent.ToolRead, "Grep": agent.ToolRead, "Glob": agent.ToolRead, "LS": agent.ToolRead, "NotebookRead": agent.ToolRead,
		"WebFetch": agent.ToolOther, "WebSearch": agent.ToolOther, "Task": agent.ToolOther, "Agent": agent.ToolOther,
		"TodoWrite": agent.ToolOther, "mcp__github__create_issue": agent.ToolOther, "": agent.ToolOther, "bash": agent.ToolOther,
	}
	for name, want := range tests {
		if got := classifyTool(name); got != want {
			t.Errorf("classifyTool(%q) = %q, want %q", name, got, want)
		}
	}
}

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "f.txt")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestEditRanges(t *testing.T) {
	file := "l1\nl2\nfunc a() {\n\treturn 1\n}\nl6\nreturn 1\n"
	tests := []struct {
		name  string
		tool  string
		input func(path string) map[string]any
		want  [][2]int // start, end
	}{
		{"write", "Write", func(p string) map[string]any { return map[string]any{"file_path": p, "content": "a\nb\n"} }, [][2]int{{1, 2}}},
		{"write without trailing newline", "Write", func(p string) map[string]any { return map[string]any{"file_path": p, "content": "a\nb"} }, [][2]int{{1, 2}}},
		{"write empty", "Write", func(p string) map[string]any { return map[string]any{"file_path": p, "content": ""} }, [][2]int{{1, 1}}},
		{"edit multi-line", "Edit", func(p string) map[string]any {
			return map[string]any{"file_path": p, "old_string": "x", "new_string": "func a() {\n\treturn 1\n}"}
		}, [][2]int{{3, 5}}},
		{"edit single line", "Edit", func(p string) map[string]any {
			return map[string]any{"file_path": p, "old_string": "x", "new_string": "l2"}
		}, [][2]int{{2, 2}}},
		{"edit with trailing newline", "Edit", func(p string) map[string]any {
			return map[string]any{"file_path": p, "old_string": "x", "new_string": "l1\nl2\n"}
		}, [][2]int{{1, 2}}},
		{"edit first occurrence only", "Edit", func(p string) map[string]any {
			return map[string]any{"file_path": p, "new_string": "return 1"}
		}, [][2]int{{4, 4}}},
		{"edit replace_all", "Edit", func(p string) map[string]any {
			return map[string]any{"file_path": p, "new_string": "return 1", "replace_all": true}
		}, [][2]int{{4, 4}, {7, 7}}},
		{"edit text not found", "Edit", func(p string) map[string]any { return map[string]any{"file_path": p, "new_string": "nope"} }, nil},
		{"edit deletion", "Edit", func(p string) map[string]any {
			return map[string]any{"file_path": p, "old_string": "l2", "new_string": ""}
		}, nil},
		{"multiedit", "MultiEdit", func(p string) map[string]any {
			return map[string]any{"file_path": p, "edits": []any{
				map[string]any{"old_string": "a", "new_string": "l1"},
				map[string]any{"old_string": "b", "new_string": "l6"},
			}}
		}, [][2]int{{1, 1}, {6, 6}}},
		{"unknown tool", "NotebookEdit", func(p string) map[string]any { return map[string]any{"file_path": p, "new_string": "l1"} }, nil},
		{"no path", "Edit", func(string) map[string]any { return map[string]any{"new_string": "l1"} }, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTemp(t, file)
			got := editRanges(tt.tool, tt.input(path), filepath.Dir(path))
			var ranges [][2]int
			for _, e := range got {
				if e.Path != path && tt.input(path)["file_path"] != nil {
					t.Errorf("range path = %q, want %q", e.Path, path)
				}
				ranges = append(ranges, [2]int{e.StartLine, e.EndLine})
			}
			if !reflect.DeepEqual(ranges, tt.want) {
				t.Errorf("ranges = %v, want %v", ranges, tt.want)
			}
		})
	}
}

func TestEditRangesRelativePathAndCRLF(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "w.txt"), []byte("a\r\nb\r\nc\r\nd\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := editRanges("Edit", map[string]any{"file_path": "w.txt", "new_string": "b\nc"}, dir)
	if len(got) != 1 || got[0].StartLine != 2 || got[0].EndLine != 3 || got[0].Path != "w.txt" {
		t.Errorf("ranges = %+v, want one range 2-3 for w.txt", got)
	}
}

func TestEditRangesSkipsHugeAndMissingFiles(t *testing.T) {
	dir := t.TempDir()
	big := filepath.Join(dir, "big.txt")
	if err := os.WriteFile(big, []byte(strings.Repeat("x\n", maxEditFileSize/2+10)), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := editRanges("Edit", map[string]any{"file_path": big, "new_string": "x"}, dir); got != nil {
		t.Errorf("file over the cap should be skipped, got %v", got)
	}
	if got := editRanges("Edit", map[string]any{"file_path": filepath.Join(dir, "missing"), "new_string": "x"}, dir); got != nil {
		t.Errorf("missing file should give no ranges, got %v", got)
	}
	// Write never needs the file.
	if got := editRanges("Write", map[string]any{"file_path": big, "content": "a\nb\nc\n"}, dir); len(got) != 1 || got[0].EndLine != 3 {
		t.Errorf("Write ranges = %v", got)
	}
}

func TestParsedPostToolEditUsesDisk(t *testing.T) {
	path := writeTemp(t, "one\ntwo\nthree\n")
	payload, _ := json.Marshal(map[string]any{
		"cwd": filepath.Dir(path), "tool_name": "Edit",
		"tool_input": map[string]any{"file_path": path, "old_string": "x", "new_string": "two"},
	})
	ev, err := Adapter{}.ParseHook("post-tool", payload)
	if err != nil {
		t.Fatal(err)
	}
	want := []agent.EditRange{{Path: path, StartLine: 2, EndLine: 2}}
	if !reflect.DeepEqual(ev.Edits, want) || !reflect.DeepEqual(ev.Paths, []string{path}) {
		t.Errorf("edits = %+v paths = %v", ev.Edits, ev.Paths)
	}
}

func decode(t *testing.T, b []byte) map[string]any {
	t.Helper()
	if len(b) == 0 || b[len(b)-1] != '\n' || strings.Count(string(b), "\n") != 1 {
		t.Fatalf("stdout must be exactly one JSON line, got %q", b)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("stdout is not JSON: %v: %q", err, b)
	}
	return m
}

func TestEncodeHookResponsePreTool(t *testing.T) {
	ev := agent.HookEvent{Kind: agent.HookPreTool, ToolName: "Bash", ToolInput: map[string]any{"command": "rm -rf /", "timeout": float64(5)}}
	tests := []struct {
		name         string
		resp         agent.HookResponse
		wantDecision string
		wantReason   string
		wantUpdated  map[string]any
	}{
		{"allow", agent.HookResponse{Decision: agent.Allow}, "allow", "", nil},
		{"allow with reason", agent.HookResponse{Decision: agent.Allow, Reason: "approved by Touch ID"}, "allow", "approved by Touch ID", nil},
		{"deny", agent.HookResponse{Decision: agent.Deny, Reason: "irreversible"}, "deny", "irreversible", nil},
		{"deny without reason", agent.HookResponse{Decision: agent.Deny}, "deny", "Blocked by Multiplex.", nil},
		{"ask", agent.HookResponse{Decision: agent.Ask, Reason: "please confirm"}, "ask", "please confirm", nil},
		{"no decision is ask", agent.HookResponse{}, "ask", "Multiplex has no decision for this action.", nil},
		{"updated input is merged over the original", agent.HookResponse{Decision: agent.Allow, UpdatedInput: map[string]any{"command": "echo safe"}},
			"allow", "", map[string]any{"command": "echo safe", "timeout": float64(5)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, code := Adapter{}.EncodeHookResponse(ev, tt.resp)
			if code != 0 {
				t.Errorf("exit code = %d, want 0", code)
			}
			m := decode(t, out)
			if len(m) != 1 {
				t.Errorf("top level must hold only hookSpecificOutput: %v", m)
			}
			h, _ := m["hookSpecificOutput"].(map[string]any)
			if h["hookEventName"] != "PreToolUse" || h["permissionDecision"] != tt.wantDecision {
				t.Errorf("hookSpecificOutput = %v", h)
			}
			reason, _ := h["permissionDecisionReason"].(string)
			if reason != tt.wantReason {
				t.Errorf("reason = %q, want %q", reason, tt.wantReason)
			}
			if !reflect.DeepEqual(h["updatedInput"], anyMap(tt.wantUpdated)) {
				t.Errorf("updatedInput = %v, want %v", h["updatedInput"], tt.wantUpdated)
			}
		})
	}
}

// anyMap makes a nil map compare equal to a missing JSON field.
func anyMap(m map[string]any) any {
	if m == nil {
		return nil
	}
	return m
}

func TestEncodeHookResponseExactJSON(t *testing.T) {
	out, code := Adapter{}.EncodeHookResponse(agent.HookEvent{Kind: agent.HookPreTool},
		agent.HookResponse{Decision: agent.Deny, Reason: "no <rm> & friends"})
	want := `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"no <rm> & friends"}}` + "\n"
	if string(out) != want || code != 0 {
		t.Errorf("got %q (%d), want %q (0)", out, code, want)
	}
}

func TestEncodeHookResponseOtherEventsAreEmpty(t *testing.T) {
	for _, k := range []agent.HookKind{agent.HookPostTool, agent.HookNotification, agent.HookStop, agent.HookPromptSubmit, agent.HookSessionStart, agent.HookSessionEnd} {
		out, code := Adapter{}.EncodeHookResponse(agent.HookEvent{Kind: k}, agent.HookResponse{Decision: agent.Deny, Reason: "x"})
		if len(out) != 0 || code != 0 {
			t.Errorf("%s: got %q (%d), want empty and 0", k, out, code)
		}
	}
}

func TestFailClosed(t *testing.T) {
	out, code := Adapter{}.FailClosed("daemon not running")
	if code != 2 {
		t.Errorf("exit code = %d, want 2 (blocks the tool call)", code)
	}
	h, _ := decode(t, out)["hookSpecificOutput"].(map[string]any)
	if h["permissionDecision"] != "deny" || !strings.Contains(h["permissionDecisionReason"].(string), "daemon not running") {
		t.Errorf("hookSpecificOutput = %v", h)
	}
	if out, code := (Adapter{}).FailClosed(""); code != 2 || len(out) == 0 {
		t.Errorf("empty reason: %q (%d)", out, code)
	}
}

func TestTypeMessage(t *testing.T) {
	tests := []struct{ in, want string }{
		{"hello", "hello\r"},
		{"", "\r"},
		{"line one\nline two", "\x1b[200~line one\nline two\x1b[201~\r"},
		{"a\r\nb", "\x1b[200~a\nb\x1b[201~\r"},
		{"a\rb", "\x1b[200~a\nb\x1b[201~\r"},
		{"x\ny\x1b[201~z", "\x1b[200~x\nyz\x1b[201~\r"},
	}
	for _, tt := range tests {
		if got := string(Adapter{}.TypeMessage(tt.in)); got != tt.want {
			t.Errorf("TypeMessage(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestInterruptAndSideQuestion(t *testing.T) {
	if got := string(Adapter{}.Interrupt()); got != "\x1b" {
		t.Errorf("Interrupt = %q", got)
	}
	b, ok := Adapter{}.SideQuestion("what is\nthis  file?")
	if !ok || string(b) != "/btw what is this file?\r" {
		t.Errorf("SideQuestion = %q, %v", b, ok)
	}
}
