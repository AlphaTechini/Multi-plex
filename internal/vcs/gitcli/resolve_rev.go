package gitcli

import "context"

// ResolveRev returns the full commit hash of rev in dir.
func (g *Git) ResolveRev(ctx context.Context, dir, rev string) (string, error) {
	if err := checkArg("revision", rev); err != nil {
		return "", err
	}
	return g.runString(ctx, runOpts{dir: dir, readOnly: true}, "rev-parse", "--verify", rev+"^{commit}")
}
