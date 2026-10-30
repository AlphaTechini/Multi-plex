package spec

import (
	"regexp"
	"strconv"
	"strings"
)

// Criterion is one acceptance criterion from requirements.md.
type Criterion struct {
	// ID combines the requirement and criterion numbers, such as "R1.2".
	ID string
	// Text is the criterion's sentence, with wrapped lines joined by spaces.
	Text string
}

// requirementHeadRe matches "### Requirement N" and captures N.
var requirementHeadRe = regexp.MustCompile(`^###\s+Requirement\s+(\d+)\b`)

// anyHeadRe matches any Markdown heading, which ends the current requirement.
var anyHeadRe = regexp.MustCompile(`^#{1,6}\s`)

// criterionRe matches a numbered list item and captures its number and text.
var criterionRe = regexp.MustCompile(`^\s*(\d+)\.\s+(.*\S)\s*$`)

// ParseCriteria returns the acceptance criteria under each "### Requirement N"
// heading, with IDs R<N>.<k> where k is the item's number in the list.
func ParseCriteria(md string) []Criterion {
	var out []Criterion
	req := ""  // number of the current requirement, "" outside one
	last := -1 // index in out of the last criterion, for wrapped lines
	for _, line := range strings.Split(md, "\n") {
		line = strings.TrimRight(line, "\r")
		if m := requirementHeadRe.FindStringSubmatch(line); m != nil {
			n, _ := strconv.Atoi(m[1])
			req, last = strconv.Itoa(n), -1
			continue
		}
		if anyHeadRe.MatchString(line) {
			req, last = "", -1
			continue
		}
		if req == "" {
			continue
		}
		if m := criterionRe.FindStringSubmatch(line); m != nil {
			out = append(out, Criterion{ID: "R" + req + "." + m[1], Text: m[2]})
			last = len(out) - 1
			continue
		}
		// An indented, non-empty line after an item continues that criterion.
		if last >= 0 && strings.TrimSpace(line) != "" && leadingSpace(line) > 0 {
			out[last].Text += " " + strings.TrimSpace(line)
		}
	}
	return out
}
