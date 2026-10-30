package classify

// Class is the Layer 4 classification of a command.
type Class string

// Classes (docs/04-safety.md, Layer 4).
const (
	Safe      Class = "safe"      // known safe: allow, no prompt
	Dangerous Class = "dangerous" // known dangerous: gate
	Unknown   Class = "unknown"   // anything else: allow, rely on Layers 1 to 3
)

// Context is what the classifier needs to know about where a command runs.
type Context struct {
	// WorktreeRoot is the session's worktree. Deletes inside it are
	// reversible (Layer 1) and are not flagged.
	WorktreeRoot string
	// Cwd is the directory the command starts in; empty means WorktreeRoot.
	Cwd string
	// Shell is "bash", "powershell", "cmd" or "" (unknown, parsed as POSIX
	// with Windows paths tolerated).
	Shell string
	// ProtectedBranches are shared branches; defaults to main and master.
	ProtectedBranches []string
	// CurrentBranch is the branch checked out in the worktree, if known. It
	// lets rules gate a bare "git push" or "git reset --hard" on a shared branch.
	CurrentBranch string
	// Home is the user's home directory for "~" and $HOME; defaults to the
	// current user's.
	Home string
	// SafeDirs are extra directories (temp, tool caches) whose contents may
	// be deleted without a prompt.
	SafeDirs []string
}

// Result is the classification of a command.
type Result struct {
	Class  Class
	RuleID string // the rule that decided, empty for plain unknown
	Reason string // the rule's description
	Kind   string // optional approval-kind hint from the rule, e.g. "push"
}
