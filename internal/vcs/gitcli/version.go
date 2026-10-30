package gitcli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// Version returns the git version string, for example "git version 2.43.0".
func (g *Git) Version(ctx context.Context) (string, error) {
	return g.runString(ctx, runOpts{readOnly: true}, "version")
}

// versionAtLeast reports whether git is at least major.minor. The result of
// the first successful probe is cached.
func (g *Git) versionAtLeast(ctx context.Context, major, minor int) (bool, error) {
	g.verMu.Lock()
	defer g.verMu.Unlock()
	if g.verMaj == 0 {
		v, err := g.Version(ctx)
		if err != nil {
			return false, err
		}
		maj, min, err := parseVersion(v)
		if err != nil {
			return false, err
		}
		g.verMaj, g.verMin = maj, min
	}
	return g.verMaj > major || (g.verMaj == major && g.verMin >= minor), nil
}

// parseVersion extracts major and minor from "git version 2.39.2.windows.1"
// and similar strings.
func parseVersion(s string) (major, minor int, err error) {
	f := strings.Fields(s)
	if len(f) < 3 {
		return 0, 0, fmt.Errorf("gitcli: cannot parse git version %q", s)
	}
	parts := strings.Split(f[2], ".")
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("gitcli: cannot parse git version %q", s)
	}
	if major, err = strconv.Atoi(parts[0]); err != nil {
		return 0, 0, fmt.Errorf("gitcli: cannot parse git version %q", s)
	}
	if minor, err = strconv.Atoi(parts[1]); err != nil {
		return 0, 0, fmt.Errorf("gitcli: cannot parse git version %q", s)
	}
	return major, minor, nil
}
