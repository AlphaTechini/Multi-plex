package classify

import (
	"bytes"
	_ "embed" // default_rules.json
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sync"
)

// The rule files are JSON, read with the standard library's encoding/json.
// JSON was chosen over YAML or TOML to avoid a dependency (AGENTS.md).

//go:embed default_rules.json
var defaultRulesJSON []byte

// RuleFile is the on-disk format of a rules file.
type RuleFile struct {
	Version int    `json:"version"`
	Rules   []Rule `json:"rules"`
}

var (
	defaultOnce  sync.Once
	defaultRules *Rules
	defaultIDs   map[string]bool
)

// Default returns the built-in rules. It panics only if the embedded file is
// invalid, which the tests rule out.
func Default() *Rules {
	defaultOnce.Do(func() {
		list, err := parseRules(defaultRulesJSON, "default_rules.json")
		if err != nil {
			panic("classify: embedded rules invalid: " + err.Error())
		}
		defaultRules = newRules(list)
		defaultIDs = map[string]bool{}
		for _, r := range list {
			defaultIDs[r.ID] = true
		}
	})
	return defaultRules
}

// Load returns the default rules extended with the rules in the given files
// (typically a user-level file, then ".multiplex/safety-rules.json" in the
// workspace). Files that do not exist are skipped.
//
// Files can only ADD rules. A rule whose id equals a default rule's id is an
// error: defaults can be neither replaced nor disabled, and unknown fields
// such as "disabled" are rejected, because docs/04-safety.md forbids weakening
// a default. A "safe" rule cannot override a dangerous one either, because the
// most dangerous match always wins.
//
// If a file is unreadable or invalid, Load still returns the defaults plus the
// files that did load, together with the error, so the caller can warn the
// user and keep running with a safe configuration.
func Load(paths ...string) (*Rules, error) {
	Default()
	list := append([]*Rule(nil), defaultRules.list...)
	ids := map[string]bool{}
	for k := range defaultIDs {
		ids[k] = true
	}
	var errs []error
	for _, p := range paths {
		if p == "" {
			continue
		}
		data, err := os.ReadFile(p)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				errs = append(errs, fmt.Errorf("%s: %w", p, err))
			}
			continue
		}
		rules, err := parseRules(data, p)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		var dup error
		seen := map[string]bool{}
		for _, r := range rules {
			if ids[r.ID] || seen[r.ID] {
				dup = fmt.Errorf("%s: rule id %q already exists; default rules cannot be replaced or disabled", p, r.ID)
				break
			}
			seen[r.ID] = true
		}
		if dup != nil {
			errs = append(errs, dup)
			continue
		}
		for _, r := range rules {
			ids[r.ID] = true
			list = append(list, r)
		}
	}
	return newRules(list), errors.Join(errs...)
}

// ParseRules compiles rules from JSON data (a RuleFile).
func ParseRules(data []byte) (*Rules, error) {
	list, err := parseRules(data, "rules")
	if err != nil {
		return nil, err
	}
	return newRules(list), nil
}

func parseRules(data []byte, origin string) ([]*Rule, error) {
	var f RuleFile
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("%s: %w", origin, err)
	}
	if f.Version != 0 && f.Version != 1 {
		return nil, fmt.Errorf("%s: unsupported rules version %d", origin, f.Version)
	}
	out := make([]*Rule, 0, len(f.Rules))
	seen := map[string]bool{}
	for i := range f.Rules {
		r := f.Rules[i]
		if err := r.compile(); err != nil {
			return nil, fmt.Errorf("%s: %w", origin, err)
		}
		if seen[r.ID] {
			return nil, fmt.Errorf("%s: duplicate rule id %q", origin, r.ID)
		}
		seen[r.ID] = true
		out = append(out, &r)
	}
	return out, nil
}
