package classify

import (
	"regexp"
	"strings"
)

var interpreterFlags = map[string][]string{
	"python": {"-c"}, "python3": {"-c"}, "py": {"-c"}, "pypy": {"-c"}, "pypy3": {"-c"},
	"node": {"-e", "--eval", "-p", "--print"}, "nodejs": {"-e", "--eval", "-p", "--print"},
	"bun": {"-e", "--eval"}, "deno": {"eval"}, "ruby": {"-e"}, "perl": {"-e", "-E"}, "php": {"-r"},
}

var reShellCall = regexp.MustCompile(`\b(?:system|popen|exec\w*|spawn\w*|run|call|Popen|check_call|check_output|getoutput|getstatusoutput|shell_exec|passthru)\s*\(`)

// embeddedCommands finds shell commands that an interpreter one-liner runs
// ("python -c 'os.system(\"rm -rf ~\")'"), so they can be classified like any
// other command. It extracts the string literals inside calls that run
// programs and joins them with spaces, which turns both os.system("a b") and
// subprocess.run(["a","b"]) into "a b". Dynamic strings cannot be seen; that
// is the documented limit of Layer 4 (covered by Layers 1 to 3).
func embeddedCommands(name string, args []string) []string {
	flags, ok := interpreterFlags[name]
	if !ok {
		if strings.HasPrefix(name, "python") || strings.HasPrefix(name, "pypy") {
			flags = []string{"-c"}
		} else {
			return nil
		}
	}
	var code string
	for i, a := range args {
		for _, f := range flags {
			if a == f && i+1 < len(args) {
				code = args[i+1]
			}
		}
	}
	if code == "" {
		return nil
	}
	var out []string
	for _, loc := range reShellCall.FindAllStringIndex(code, -1) {
		lits := callLiterals(code, loc[1])
		if len(lits) > 0 {
			out = append(out, strings.Join(lits, " "))
		}
	}
	return out
}

// callLiterals collects the string literals between an opening parenthesis
// (just consumed) and its matching close.
func callLiterals(code string, start int) []string {
	var lits []string
	depth := 1
	for i := start; i < len(code) && depth > 0; i++ {
		switch c := code[i]; c {
		case '(':
			depth++
		case ')':
			depth--
		case '"', '\'', '`':
			var b strings.Builder
			j := i + 1
			for ; j < len(code) && code[j] != c; j++ {
				if code[j] == '\\' && j+1 < len(code) {
					j++
				}
				b.WriteByte(code[j])
			}
			lits = append(lits, b.String())
			i = j
		}
	}
	return lits
}
