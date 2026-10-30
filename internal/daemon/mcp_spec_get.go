package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/spec"
)

// mcpSpecGet returns a spec's files, one file, or one requirement or
// criterion by ID.
func (d *Daemon) mcpSpecGet(ctx context.Context, s *session, raw json.RawMessage) (string, error) {
	var a protocol.SpecGetArgs
	if err := decodeArgs(raw, &a); err != nil {
		return "", err
	}
	doc, err := d.readSpec(ctx, s, a.Slug)
	if err != nil {
		return "", err
	}
	if a.Requirement != "" {
		return requirementText(doc.Requirements, a.Requirement)
	}
	switch strings.ToLower(a.File) {
	case "requirements":
		return doc.Requirements, nil
	case "design":
		return doc.Design, nil
	case "tasks":
		return doc.Tasks, nil
	case "", "all":
		return fmt.Sprintf("# requirements.md\n\n%s\n\n# design.md\n\n%s\n\n# tasks.md\n\n%s\n", doc.Requirements, doc.Design, doc.Tasks), nil
	}
	return "", badRequest("file must be requirements, design or tasks, got %q", a.File)
}

var reqIDRe = regexp.MustCompile(`^R?(\d+)(?:\.(\d+))?$`)

// requirementText extracts requirement "R1" (its whole section) or criterion
// "R1.2" from a requirements.md.
func requirementText(md, id string) (string, error) {
	m := reqIDRe.FindStringSubmatch(strings.TrimSpace(id))
	if m == nil {
		return "", badRequest("requirement must look like R1 or R1.2, got %q", id)
	}
	if m[2] != "" {
		want := "R" + m[1] + "." + m[2]
		for _, c := range spec.ParseCriteria(md) {
			if c.ID == want {
				return c.ID + ": " + c.Text, nil
			}
		}
		return "", notFound("no criterion %s", want)
	}
	headRe := regexp.MustCompile(`^###\s+Requirement\s+` + m[1] + `\b`)
	var out []string
	in := false
	for _, line := range strings.Split(md, "\n") {
		trimmed := strings.TrimRight(line, "\r")
		if headRe.MatchString(trimmed) {
			in = true
		} else if in && (strings.HasPrefix(trimmed, "### ") || strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "# ")) {
			break
		}
		if in {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		return "", notFound("no requirement R%s", m[1])
	}
	return strings.TrimSpace(strings.Join(out, "\n")), nil
}
