package config

// DefaultSettings returns the settings used when no file exists or a key is absent.
func DefaultSettings() Settings {
	return Settings{
		Theme:              "dark-plus",
		PrefixKey:          "ctrl+space",
		AgentPanelWidth:    0.30,
		AgentPanelMinCols:  60,
		ReduceMotion:       false,
		DefaultAgent:       "claude-code",
		DefaultRole:        "implementer",
		ConflictResolution: "ask",
		Approval:           "auto",
		ExtraDenyRead:      nil,
		ExtraScrubEnv:      nil,
		Keymap:             map[string]string{},
	}
}
