package gate

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/safety/classify"
	"github.com/alphatechini/multi-plex/internal/safety/pathpolicy"
)

const maxScriptSize = 256 << 10

// scanScripts looks inside shell scripts a command runs ("bash clean.sh",
// "./deploy.sh", "pwsh -File x.ps1", "source env.sh") when the script lives in
// the worktree, and classifies its text. This covers docs/04's "a script the
// agent writes and then runs". Scripts outside the worktree, interpreters
// other than shells, and scripts created in the same command line are not
// seen; Layers 1 to 3 cover those.
func (g *Gate) scanScripts(an classify.Analysis, rules *classify.Rules, ctx classify.Context) (Verdict, bool) {
	for i := range an.Commands {
		s := &an.Commands[i]
		path, shell := scriptOf(s)
		if path == "" {
			continue
		}
		abs, ok := an.Resolver.Resolve(path, s.Cwd, s.CwdKnown)
		if !ok || !g.inWritable(abs) {
			continue
		}
		fi, err := os.Stat(filepath.FromSlash(abs))
		if err != nil || !fi.Mode().IsRegular() || fi.Size() > maxScriptSize {
			continue
		}
		data, err := os.ReadFile(filepath.FromSlash(abs))
		if err != nil {
			continue
		}
		c := ctx
		c.Shell = shell
		c.Cwd = s.Cwd
		res := rules.Classify(string(data), c)
		if res.Class != classify.Dangerous {
			continue
		}
		shown := g.Policy.Display(abs)
		kind := core.ApprovalCommand
		if res.Kind == "push" {
			kind = core.ApprovalPush
		}
		return Verdict{
			Decision: DecisionGate, Kind: kind, Rule: res.RuleID, Layer: LayerCommand,
			Reason: "script " + shown + " contains a dangerous command: " + res.Reason, Paths: []string{shown},
		}, true
	}
	return Verdict{}, false
}

var scriptExt = map[string]string{".sh": "bash", ".bash": "bash", ".zsh": "bash", ".ps1": "powershell", ".bat": "cmd", ".cmd": "cmd"}

// scriptOf returns the script file a command runs and its shell flavour.
func scriptOf(s *classify.Simple) (path, shell string) {
	name := s.Name()
	args := s.Argv[1:]
	switch name {
	case "sh", "bash", "zsh", "dash", "ksh", "source", ".":
		for _, a := range args {
			if !strings.HasPrefix(a, "-") {
				return a, "bash"
			}
		}
	case "pwsh", "powershell":
		for i, a := range args {
			if l := strings.ToLower(a); (l == "-file" || l == "-f") && i+1 < len(args) {
				return args[i+1], "powershell"
			}
		}
	}
	if ext := strings.ToLower(filepath.Ext(s.Argv[0])); scriptExt[ext] != "" && name != "cmd" {
		return s.Argv[0], scriptExt[ext]
	}
	return "", ""
}

// inWritable reports whether abs is in the worktree or the writable
// allow-list, the places an agent can have written a script.
func (g *Gate) inWritable(abs string) bool {
	for _, d := range g.Policy.WritableDirs() {
		if pathpolicy.WithinOS(g.Policy.OS(), d, abs) {
			return true
		}
	}
	return false
}
