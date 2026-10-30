package classify

// Flatten parses cmd the way Classify does and returns every simple command
// it would run, including those nested in bash -c, sudo, xargs, command
// substitutions and the like, each with its effective working directory. The
// gate uses it to find paths in commands for Layers 2 and 3.
func Flatten(cmd string, ctx Context) []Simple {
	w := newWalker(ctx)
	var out []Simple
	w.visit = func(s *Simple) bool {
		out = append(out, *s)
		return false
	}
	w.run(cmd)
	return out
}
