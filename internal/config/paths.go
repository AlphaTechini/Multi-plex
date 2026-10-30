package config

// Paths lists the directories Multiplex uses. See docs/01-architecture.md
// "Storage" for what lives where.
type Paths struct {
	// DataDir holds the database, worktrees and logs.
	DataDir string
	// ConfigDir holds settings.json and the global roles folder.
	ConfigDir string
	// RuntimeDir holds the daemon socket (not used on Windows).
	RuntimeDir string
	// DBPath is the SQLite database file.
	DBPath string
	// WorktreesDir holds session worktrees, outside the repositories.
	WorktreesDir string
	// LogDir holds daemon and session logs.
	LogDir string
}
