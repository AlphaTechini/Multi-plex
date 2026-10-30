package role

// Role is a reusable definition of how an agent session behaves.
type Role struct {
	// Name identifies the role. It comes from the front matter, or the file name.
	Name string
	// Description says what the role is for, in one line.
	Description string
	// Agent names the agent CLI, such as "claude-code". Empty means the user's default.
	Agent string
	// Model is the model the CLI should use, or "default". Empty means the CLI's default.
	Model string
	// Permissions is "read-only", "standard" or "custom". Empty means the default.
	Permissions string
	// Report says how much the session reports, such as "summary" or "summary-only".
	Report string
	// AllowTools lists tools the role may use (from tools.allow).
	AllowTools []string
	// DenyTools lists tools the role may not use (from tools.deny).
	DenyTools []string
	// Prompt is the body after the front matter, trimmed.
	Prompt string
	// Source is the file path the role was read from, or "builtin".
	Source string
}
