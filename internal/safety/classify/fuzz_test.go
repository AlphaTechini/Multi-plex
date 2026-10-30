package classify

import "testing"

// FuzzClassify checks that arbitrary input never panics or hangs and that
// classification is deterministic.
func FuzzClassify(f *testing.F) {
	for _, s := range []string{
		"git push -f", "rm -rf ~", `bash -c "echo $(rm -rf /)"`, "cat <<EOF\nx\nEOF", `echo "unterminated`, "a | b && c || d; e &",
		`$'\x72m' -rf /`, "(((((", ")))", "`", "$(", "<(", "<<", ">", "2>&1", `powershell -enc AAAA`, "@'\nx\n'@", "'", `\`, "{", "}", "for x in; do", "if", "!",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, cmd string) {
		a := Classify(cmd, posixCtx)
		b := Classify(cmd, posixCtx)
		if a != b {
			t.Fatalf("nondeterministic result for %q: %+v vs %+v", cmd, a, b)
		}
		Classify(cmd, winCtx)
		Flatten(cmd, posixCtx)
	})
}
