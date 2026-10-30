package agent

// HookArgv returns the command line an agent CLI runs for a hook event:
// HookCommand plus the event, or `<MpxPath> hook <event>` when HookCommand
// is empty.
func (s LaunchSpec) HookArgv(event string) []string {
	prefix := s.HookCommand
	if len(prefix) == 0 {
		prefix = []string{s.MpxPath, "hook"}
	}
	out := make([]string, 0, len(prefix)+1)
	out = append(out, prefix...)
	return append(out, event)
}
