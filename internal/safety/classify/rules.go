package classify

import (
	"fmt"
	"regexp"
	"strings"
)

// Match says what a rule matches. Exactly one of Argv, Regex or All is set.
//
// Regex rules use Go's RE2 syntax, so a hostile command cannot make them
// backtrack. Scope "segment" (the default) tests each simple command, written
// as its words joined by single spaces followed by its output redirections
// ("echo x > f"); scope "full" tests the whole command text of one nesting
// level, which is what pipelines such as "curl ... | sh" need. All is a list
// of regexes that must all match (RE2 has no look-around).
//
// For a regex rule with unless_inside_worktree, the named group "path"
// supplies the paths to test: the rule fires only if some captured path is
// outside the worktree.
type Match struct {
	Argv  ArgvPatterns `json:"argv,omitempty"`
	Regex string       `json:"regex,omitempty"`
	All   []string     `json:"all,omitempty"`
	Scope string       `json:"scope,omitempty"`
}

// Rule is one classification rule. Rules are data: see default_rules.json.
type Rule struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Class       Class  `json:"class"` // "safe" or "dangerous"
	Match       Match  `json:"match"`
	// UnlessInsideWorktree makes the rule fire only when a path the command
	// works on lies outside the worktree (or cannot be shown to be inside).
	UnlessInsideWorktree bool `json:"unless_inside_worktree,omitempty"`
	// OnProtectedCurrentBranch makes the rule fire only when
	// Context.CurrentBranch is one of the protected branches.
	OnProtectedCurrentBranch bool `json:"on_protected_current_branch,omitempty"`
	// Exclude lists argv patterns that stop the rule from firing.
	Exclude ArgvPatterns `json:"exclude,omitempty"`
	// Kind is an optional hint for the approval kind, for example "push".
	Kind string `json:"kind,omitempty"`

	c compiled
}

type compiled struct {
	patterns  []pattern
	excludes  []pattern
	res       []*regexp.Regexp
	full      bool
	pathGroup int // submatch index of the "path" group in res[0], or -1
}

// compile validates the rule and prepares it for matching.
func (r *Rule) compile() error {
	if r.ID == "" {
		return fmt.Errorf("rule without id")
	}
	if r.Class != Safe && r.Class != Dangerous {
		return fmt.Errorf("rule %q: class must be %q or %q", r.ID, Safe, Dangerous)
	}
	n := 0
	if len(r.Match.Argv) > 0 {
		n++
	}
	if r.Match.Regex != "" {
		n++
	}
	if len(r.Match.All) > 0 {
		n++
	}
	if n != 1 {
		return fmt.Errorf("rule %q: match needs exactly one of argv, regex, all", r.ID)
	}
	r.c = compiled{pathGroup: -1}
	for _, raw := range r.Match.Argv {
		p, err := compilePattern(raw)
		if err != nil {
			return fmt.Errorf("rule %q: %w", r.ID, err)
		}
		r.c.patterns = append(r.c.patterns, p)
	}
	for _, raw := range r.Exclude {
		p, err := compilePattern(raw)
		if err != nil {
			return fmt.Errorf("rule %q exclude: %w", r.ID, err)
		}
		r.c.excludes = append(r.c.excludes, p)
	}
	srcs := r.Match.All
	if r.Match.Regex != "" {
		srcs = []string{r.Match.Regex}
	}
	for _, s := range srcs {
		re, err := regexp.Compile(s)
		if err != nil {
			return fmt.Errorf("rule %q: bad regex: %w", r.ID, err)
		}
		r.c.res = append(r.c.res, re)
	}
	if len(r.c.res) > 0 {
		for i, name := range r.c.res[0].SubexpNames() {
			if name == "path" {
				r.c.pathGroup = i
			}
		}
	}
	switch strings.ToLower(r.Match.Scope) {
	case "", "segment":
	case "full":
		r.c.full = true
	default:
		return fmt.Errorf("rule %q: scope must be segment or full", r.ID)
	}
	if r.UnlessInsideWorktree && len(r.c.res) > 0 && r.c.pathGroup < 0 {
		return fmt.Errorf("rule %q: unless_inside_worktree on a regex rule needs a (?P<path>...) group", r.ID)
	}
	if r.UnlessInsideWorktree && len(r.c.res) > 1 {
		return fmt.Errorf("rule %q: unless_inside_worktree cannot be combined with all", r.ID)
	}
	return nil
}

// Rules is an immutable, indexed set of rules. It is safe for concurrent use.
type Rules struct {
	list   []*Rule
	byCmd  map[string][]*Rule // argv rules keyed by literal command name
	anyCmd []*Rule            // argv rules whose command is a glob
	seg    []*Rule            // segment-scope regex rules
	full   []*Rule            // full-scope regex rules
}

func newRules(list []*Rule) *Rules {
	r := &Rules{list: list, byCmd: map[string][]*Rule{}}
	for _, rule := range list {
		switch {
		case len(rule.c.patterns) > 0:
			wild := false
			for _, p := range rule.c.patterns {
				if p.hasGlb {
					wild = true
				}
			}
			if wild {
				r.anyCmd = append(r.anyCmd, rule)
				continue
			}
			seen := map[string]bool{}
			for _, p := range rule.c.patterns {
				for _, c := range p.cmds {
					if !seen[c] {
						seen[c] = true
						r.byCmd[c] = append(r.byCmd[c], rule)
					}
				}
			}
		case rule.c.full:
			r.full = append(r.full, rule)
		default:
			r.seg = append(r.seg, rule)
		}
	}
	return r
}

// All returns a copy of the rules in evaluation order.
func (r *Rules) All() []Rule {
	out := make([]Rule, len(r.list))
	for i, x := range r.list {
		out[i] = *x
	}
	return out
}

// Len is the number of rules.
func (r *Rules) Len() int { return len(r.list) }
