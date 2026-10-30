package config

// Settings are the user's preferences, read from settings.json in the config
// directory. JSON keys are the camelCase field names.
type Settings struct {
	// Theme names a colour theme. The default is VS Code's Dark+.
	Theme string `json:"theme"`
	// PrefixKey is the prefix key for multiplexer commands.
	// TODO(CC-09): placeholder until the capability check settles the default;
	// see docs/15-open-questions.md Q2.
	PrefixKey string `json:"prefixKey"`
	// AgentPanelWidth is the agent panel's share of the window width (0 to 1).
	AgentPanelWidth float64 `json:"agentPanelWidth"`
	// AgentPanelMinCols is the narrowest the agent panel may be, in columns.
	AgentPanelMinCols int `json:"agentPanelMinCols"`
	// ReduceMotion turns off short UI animations (docs/06-ui-layout.md).
	ReduceMotion bool `json:"reduceMotion"`
	// DefaultAgent is the agent CLI used when a session does not name one.
	DefaultAgent string `json:"defaultAgent"`
	// DefaultRole is the role used when a session does not name one.
	DefaultRole string `json:"defaultRole"`
	// ConflictResolution is "off", "ask" or "auto" (docs/05-sessions-and-worktrees.md).
	ConflictResolution string `json:"conflictResolution"`
	// Approval selects the approval verifier; "auto" picks the best available.
	Approval string `json:"approval"`
	// ExtraDenyRead lists extra path patterns the agent may not read.
	ExtraDenyRead []string `json:"extraDenyRead"`
	// ExtraScrubEnv lists extra environment variables removed before an agent starts.
	ExtraScrubEnv []string `json:"extraScrubEnv"`
	// Keymap maps action names to key bindings.
	Keymap map[string]string `json:"keymap"`
}
