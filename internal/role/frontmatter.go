package role

import (
	"fmt"
	"strings"
)

// fmValue is one front matter value: a scalar, or a list in [a, b] form.
type fmValue struct {
	text   string
	list   []string
	isList bool
}

// splitFrontMatter separates the "---" delimited header from the body.
func splitFrontMatter(src string) (header []string, body string, err error) {
	src = strings.TrimPrefix(strings.ReplaceAll(src, "\r\n", "\n"), "\ufeff")
	lines := strings.Split(src, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, "", fmt.Errorf("missing front matter (file must start with ---)")
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return lines[1:i], strings.Join(lines[i+1:], "\n"), nil
		}
	}
	return nil, "", fmt.Errorf("unterminated front matter (no closing ---)")
}

// parseFrontMatter reads the YAML subset used by roles: "key: value" pairs,
// inline lists "[a, b]", quoted or bare scalars, "#" comments, and one level
// of nesting ("tools:" followed by indented "deny: [...]"). A nested key is
// stored as "parent.child".
func parseFrontMatter(lines []string) (map[string]fmValue, error) {
	out := map[string]fmValue{}
	parent := "" // the open nested block, or "" when at top level
	for i, raw := range lines {
		line := stripComment(raw)
		if strings.TrimSpace(line) == "" {
			continue
		}
		content := strings.TrimSpace(line)
		key, val, ok := splitKeyValue(content)
		if !ok {
			return nil, fmt.Errorf("line %d: expected key: value", i+2)
		}
		if leadingSpace(line) == 0 {
			parent = ""
			if val == "" {
				parent = key
				continue
			}
			out[key] = parseValue(val)
			continue
		}
		if parent == "" {
			return nil, fmt.Errorf("line %d: unexpected indentation", i+2)
		}
		if val == "" {
			return nil, fmt.Errorf("line %d: nesting deeper than one level is not supported", i+2)
		}
		out[parent+"."+key] = parseValue(val)
	}
	return out, nil
}

// stripComment removes a "#" comment that starts a word, ignoring "#" inside quotes.
func stripComment(line string) string {
	var quote byte
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '#' && (i == 0 || line[i-1] == ' ' || line[i-1] == '\t'):
			return line[:i]
		}
	}
	return line
}

// splitKeyValue splits "key: value" at the first colon. The key must be non-empty.
func splitKeyValue(content string) (key, val string, ok bool) {
	i := strings.IndexByte(content, ':')
	if i <= 0 {
		return "", "", false
	}
	return strings.TrimSpace(content[:i]), strings.TrimSpace(content[i+1:]), true
}

// parseValue turns the text after a colon into a scalar or a list.
func parseValue(val string) fmValue {
	if strings.HasPrefix(val, "[") && strings.HasSuffix(val, "]") {
		var items []string
		for _, part := range strings.Split(val[1:len(val)-1], ",") {
			if s := unquote(strings.TrimSpace(part)); s != "" {
				items = append(items, s)
			}
		}
		return fmValue{list: items, isList: true}
	}
	return fmValue{text: unquote(val)}
}

// unquote removes one pair of matching single or double quotes.
func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}

// leadingSpace returns the number of leading spaces or tabs.
func leadingSpace(line string) int {
	return len(line) - len(strings.TrimLeft(line, " \t"))
}
