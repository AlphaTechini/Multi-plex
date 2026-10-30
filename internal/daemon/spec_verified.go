package daemon

import (
	"os"
	"regexp"
)

// verifiedLineRe matches a recorded criterion in verification.md.
var verifiedLineRe = regexp.MustCompile(`(?m)^- (R\d+\.\d+):`)

// verifiedCriteria counts the distinct criterion IDs that criterion_verify
// recorded for a spec, across the given checkouts (the workspace root and the
// session worktrees, where agents write verification.md).
func verifiedCriteria(roots []string, slug string) int {
	seen := map[string]struct{}{}
	for _, root := range roots {
		text, err := os.ReadFile(specFile(root, slug, verificationFile))
		if err != nil {
			continue
		}
		for _, m := range verifiedLineRe.FindAllSubmatch(text, -1) {
			seen[string(m[1])] = struct{}{}
		}
	}
	return len(seen)
}
