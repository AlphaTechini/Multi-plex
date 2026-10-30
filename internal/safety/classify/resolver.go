package classify

// Resolver resolves paths written in commands (~, $HOME, %USERPROFILE%,
// relative paths, Windows drive and UNC forms) to absolute cleaned paths
// without touching the file system.
type Resolver struct{ x cctx }

// NewResolver returns a Resolver for ctx.
func NewResolver(ctx Context) *Resolver { return &Resolver{x: newCctx(ctx)} }

// Resolve turns tok into an absolute path relative to cwd. ok is false when
// the path depends on something unknown, such as a variable.
func (r *Resolver) Resolve(tok, cwd string, cwdKnown bool) (abs string, ok bool) {
	return r.x.resolve(tok, cwd, cwdKnown)
}

// Home is the home directory the resolver uses.
func (r *Resolver) Home() string { return r.x.home }
