package pathpolicy

// CheckRead decides whether a file tool may read path without approval.
// Agents may read broadly; only credential stores (Layer 3) are denied. The
// check applies to the path as given and with symlinks resolved.
func (p Policy) CheckRead(path string) Verdict {
	cands, err := p.candidates(path)
	if err != nil {
		return Verdict{Rule: RuleUnresolvable, Reason: "the path could not be resolved safely", Path: path}
	}
	for _, c := range cands {
		if p.matchDeny(p.os(), c) {
			return Verdict{Rule: RuleDenyRead, Reason: "this is a credential store", Path: cands[0]}
		}
	}
	return Verdict{Allowed: true, Path: cands[0]}
}
