package role

import (
	"errors"
	"fmt"
	"strings"
)

// Parse reads one role file. name is the file's stem, used when the front
// matter has no name. The returned Role has an empty Source; Load sets it.
func Parse(name string, data []byte) (Role, error) {
	header, body, err := splitFrontMatter(string(data))
	if err != nil {
		return Role{}, fmt.Errorf("role %s: %w", name, err)
	}
	values, err := parseFrontMatter(header)
	if err != nil {
		return Role{}, fmt.Errorf("role %s: %w", name, err)
	}

	r := Role{Name: name, Prompt: strings.TrimSpace(body)}
	scalars := map[string]*string{
		"name":        &r.Name,
		"description": &r.Description,
		"agent":       &r.Agent,
		"model":       &r.Model,
		"permissions": &r.Permissions,
		"report":      &r.Report,
	}
	for key, dst := range scalars {
		v, ok := values[key]
		if !ok {
			continue
		}
		if v.isList {
			return Role{}, fmt.Errorf("role %s: %s must be a single value", name, key)
		}
		if key == "name" && v.text == "" {
			continue
		}
		*dst = v.text
	}
	if r.Name == "" {
		return Role{}, errors.New("role: empty name")
	}
	r.AllowTools = toolList(values["tools.allow"])
	r.DenyTools = toolList(values["tools.deny"])
	return r, nil
}

// toolList returns a tool list from a front matter value. A bare scalar is read
// as a comma-separated list.
func toolList(v fmValue) []string {
	if v.isList {
		return v.list
	}
	var out []string
	for _, part := range strings.Split(v.text, ",") {
		if s := strings.TrimSpace(part); s != "" {
			out = append(out, s)
		}
	}
	return out
}
