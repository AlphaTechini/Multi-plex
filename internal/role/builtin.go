package role

import (
	"embed"
	"fmt"
	"path"
	"strings"
)

//go:embed builtin/*.md
var builtinFS embed.FS

// builtinRoles parses the roles embedded in the binary, in file-name order.
// Their Source is "builtin".
func builtinRoles() ([]Role, error) {
	entries, err := builtinFS.ReadDir("builtin")
	if err != nil {
		return nil, fmt.Errorf("role: read built-ins: %w", err)
	}
	var out []Role
	for _, e := range entries {
		data, err := builtinFS.ReadFile(path.Join("builtin", e.Name()))
		if err != nil {
			return nil, fmt.Errorf("role: read built-in %s: %w", e.Name(), err)
		}
		r, err := Parse(strings.TrimSuffix(e.Name(), ".md"), data)
		if err != nil {
			return nil, fmt.Errorf("role: built-in %s: %w", e.Name(), err)
		}
		r.Source = "builtin"
		out = append(out, r)
	}
	return out, nil
}
