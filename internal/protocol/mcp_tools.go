package protocol

// MCP tool names and argument types shared by `mpx mcp` (which declares the
// tools) and the daemon (which runs them). The jsonschema tags become the
// descriptions an agent sees.
const (
	ToolReportProgress  = "report_progress"
	ToolNoteToUser      = "note_to_user"
	ToolSpecList        = "spec_list"
	ToolSpecGet         = "spec_get"
	ToolTaskUpdate      = "task_update"
	ToolCriterionVerify = "criterion_verify"
	ToolSessionSpawn    = "session_spawn"
	ToolSessionStatus   = "session_status"
	ToolSessionWait     = "session_wait"
	ToolSessionMessage  = "session_message"
)

// ReportProgressArgs is the argument of report_progress (docs/08).
type ReportProgressArgs struct {
	Done          []string `json:"done" jsonschema:"what was completed this turn, one short item each"`
	FilesChanged  []string `json:"files_changed,omitempty" jsonschema:"paths changed, relative to the workspace root"`
	Decisions     []string `json:"decisions,omitempty" jsonschema:"decisions made and why"`
	OpenQuestions []string `json:"open_questions,omitempty" jsonschema:"questions the user should answer"`
	Next          string   `json:"next,omitempty" jsonschema:"what you will do next"`
}

// NoteToUserArgs is the argument of note_to_user (docs/08).
type NoteToUserArgs struct {
	Title    string   `json:"title" jsonschema:"short headline of the card"`
	Body     string   `json:"body,omitempty" jsonschema:"one or two sentences with the detail"`
	Severity string   `json:"severity,omitempty" jsonschema:"info (default), warning, or decision-needed when the user must answer"`
	Refs     []string `json:"refs,omitempty" jsonschema:"code references such as src/auth/session.go:L40-L52"`
}

// SpecListArgs is the (empty) argument of spec_list.
type SpecListArgs struct{}

// SpecGetArgs is the argument of spec_get (docs/09).
type SpecGetArgs struct {
	Slug        string `json:"slug" jsonschema:"the spec's folder name under .multiplex/specs"`
	Requirement string `json:"requirement,omitempty" jsonschema:"one requirement or criterion by ID, such as R1 or R1.2; omit to read the whole spec"`
	File        string `json:"file,omitempty" jsonschema:"requirements, design or tasks; omit for all three"`
}

// TaskUpdateArgs is the argument of task_update (docs/09).
type TaskUpdateArgs struct {
	Slug   string `json:"slug" jsonschema:"the spec's folder name"`
	TaskID string `json:"task_id" jsonschema:"the task's number in tasks.md, such as 2 or 2.1"`
	Status string `json:"status" jsonschema:"open, in-progress, done or blocked"`
	Note   string `json:"note,omitempty" jsonschema:"a short note recorded under the task; required when blocked"`
}

// CriterionVerifyArgs is the argument of criterion_verify (docs/09).
type CriterionVerifyArgs struct {
	Slug      string `json:"slug" jsonschema:"the spec's folder name"`
	Criterion string `json:"criterion" jsonschema:"the criterion ID, such as R1.2"`
	Evidence  string `json:"evidence" jsonschema:"how it was verified: test name, file and lines"`
}

// SessionSpawnArgs is the argument of session_spawn (docs/10).
type SessionSpawnArgs struct {
	Task       string `json:"task" jsonschema:"what the new session should do"`
	Role       string `json:"role,omitempty" jsonschema:"role name, for example implementer, researcher or reviewer"`
	Name       string `json:"name,omitempty" jsonschema:"short name for the session's branch and worktree"`
	BaseBranch string `json:"base_branch,omitempty" jsonschema:"branch to start from; default is the repository's current branch"`
	Agent      string `json:"agent,omitempty" jsonschema:"agent CLI to run; default is the configured default"`
}

// SessionStatusArgs is the argument of session_status (docs/10).
type SessionStatusArgs struct {
	Sessions []string `json:"sessions,omitempty" jsonschema:"session IDs or names; omit for every other session in this workspace"`
}

// SessionWaitArgs is the argument of session_wait (docs/10).
type SessionWaitArgs struct {
	Sessions       []string `json:"sessions" jsonschema:"session IDs or names to wait for"`
	TimeoutSeconds int      `json:"timeout_seconds,omitempty" jsonschema:"give up after this many seconds; default 1800"`
}

// SessionMessageArgs is the argument of session_message (docs/10).
type SessionMessageArgs struct {
	Session   string `json:"session" jsonschema:"the session ID or name to message"`
	Text      string `json:"text" jsonschema:"the steer message"`
	Interrupt bool   `json:"interrupt,omitempty" jsonschema:"interrupt the session's current step first"`
}
