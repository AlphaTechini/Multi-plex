package role

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Load returns the roles visible to a workspace, keyed by name. Precedence is
// workspace (.multiplex/roles) over global (globalDir) over built-in. An empty
// workspaceRoot or globalDir skips that source. A file that fails to parse is
// skipped and reported in the error, which joins every such failure; the map
// still holds the roles that did parse.
func Load(workspaceRoot, globalDir string) (map[string]Role, error) {
	builtins, err := builtinRoles()
	if err != nil {
		return nil, err
	}
	roles := make(map[string]Role, len(builtins))
	for _, r := range builtins {
		roles[r.Name] = r
	}

	var errs []error
	sources := []string{globalDir}
	if workspaceRoot != "" {
		sources = append(sources, filepath.Join(workspaceRoot, ".multiplex", "roles"))
	}
	for _, dir := range sources {
		if dir == "" {
			continue
		}
		loaded, dirErrs := loadDir(dir)
		errs = append(errs, dirErrs...)
		for _, r := range loaded {
			roles[r.Name] = r
		}
	}
	return roles, errors.Join(errs...)
}

// loadDir parses every *.md file directly inside dir. A missing dir is not an error.
func loadDir(dir string) ([]Role, []error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, []error{fmt.Errorf("role: read %s: %w", dir, err)}
	}

	var out []Role
	var errs []error
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".md") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			errs = append(errs, fmt.Errorf("role: read %s: %w", path, err))
			continue
		}
		r, err := Parse(strings.TrimSuffix(e.Name(), filepath.Ext(e.Name())), data)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, err))
			continue
		}
		r.Source = path
		out = append(out, r)
	}
	return out, errs
}
