package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/spec"
)

// verificationFile is where verified criteria are recorded, next to the
// spec's other files in the session's worktree (docs/09 does not name a
// format; one "- R1.2: evidence" line per criterion keeps it diffable).
const verificationFile = "verification.md"

var criterionIDRe = regexp.MustCompile(`^R\d+\.\d+$`)

// mcpCriterionVerify records that an acceptance criterion is met, with
// evidence, in the session's worktree.
func (d *Daemon) mcpCriterionVerify(ctx context.Context, s *session, raw json.RawMessage) (string, error) {
	var a protocol.CriterionVerifyArgs
	if err := decodeArgs(raw, &a); err != nil {
		return "", err
	}
	id := strings.TrimSpace(a.Criterion)
	if !criterionIDRe.MatchString(id) {
		return "", badRequest("criterion must look like R1.2, got %q", a.Criterion)
	}
	evidence := strings.Join(strings.Fields(clipText(a.Evidence, maxEvidece)), " ")
	if evidence == "" {
		return "", badRequest("evidence is required")
	}
	doc, err := d.readSpec(ctx, s, a.Slug)
	if err != nil {
		return "", err
	}
	known := false
	for _, c := range spec.ParseCriteria(doc.Requirements) {
		known = known || c.ID == id
	}
	if !known {
		return "", notFound("spec %q has no criterion %s", a.Slug, id)
	}

	info := s.snap()
	path := specFile(info.WorktreePath, a.Slug, verificationFile)
	existing, _ := os.ReadFile(path)
	line := fmt.Sprintf("- %s: %s (session %s, %s)", id, evidence, info.Slug, d.now().UTC().Format(time.RFC3339))
	if err := writeFileAtomic(path, []byte(upsertVerification(string(existing), id, line))); err != nil {
		return "", fmt.Errorf("write %s: %w", verificationFile, err)
	}
	d.scheduleDiffChanged(s)
	return fmt.Sprintf("criterion %s recorded as verified", id), nil
}

// upsertVerification replaces the line for id in the file's text, or appends
// it, keeping the heading.
func upsertVerification(text, id, line string) string {
	if strings.TrimSpace(text) == "" {
		return "# Verified criteria\n\n" + line + "\n"
	}
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	prefix := "- " + id + ":"
	for i, l := range lines {
		if strings.HasPrefix(l, prefix) {
			lines[i] = line
			return strings.Join(lines, "\n") + "\n"
		}
	}
	return strings.Join(append(lines, line), "\n") + "\n"
}
