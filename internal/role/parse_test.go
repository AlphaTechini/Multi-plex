package role

import (
	"reflect"
	"strings"
	"testing"
)

// researcherMD follows the example in docs/10-agent-roles-and-skills.md.
const researcherMD = `---
name: researcher
description: Investigates a question and reports findings without changing code.
agent: claude-code          # claude-code | opencode | codex | native
model: default              # or a specific model the CLI accepts
permissions: read-only      # read-only | standard | custom
tools:
  deny: [Edit, Write]
report: summary-only
---

You are a researcher. Find the answer, cite files and lines, and do not modify code.
Call report_progress with your findings and note_to_user for anything surprising.
`

func TestParseDocsExample(t *testing.T) {
	r, err := Parse("researcher", []byte(researcherMD))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := Role{
		Name:        "researcher",
		Description: "Investigates a question and reports findings without changing code.",
		Agent:       "claude-code",
		Model:       "default",
		Permissions: "read-only",
		Report:      "summary-only",
		DenyTools:   []string{"Edit", "Write"},
		Prompt:      "You are a researcher. Find the answer, cite files and lines, and do not modify code.\nCall report_progress with your findings and note_to_user for anything surprising.",
	}
	if !reflect.DeepEqual(r, want) {
		t.Errorf("Parse mismatch\n got: %+v\nwant: %+v", r, want)
	}
}

func TestParseValues(t *testing.T) {
	src := "---\r\n" +
		"name: \"quoted: name\"\r\n" +
		"description: 'has # not a comment' # but this is\r\n" +
		"tools:\r\n" +
		"  allow: [Read, \"Bash\", ]\r\n" +
		"  deny: Edit, Write\r\n" +
		"---\r\n" +
		"body\r\n"
	r, err := Parse("fallback", []byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if r.Name != "quoted: name" {
		t.Errorf("Name = %q", r.Name)
	}
	if r.Description != "has # not a comment" {
		t.Errorf("Description = %q", r.Description)
	}
	if want := []string{"Read", "Bash"}; !reflect.DeepEqual(r.AllowTools, want) {
		t.Errorf("AllowTools = %v, want %v", r.AllowTools, want)
	}
	if want := []string{"Edit", "Write"}; !reflect.DeepEqual(r.DenyTools, want) {
		t.Errorf("DenyTools = %v, want %v (bare comma list)", r.DenyTools, want)
	}
	if r.Prompt != "body" {
		t.Errorf("Prompt = %q", r.Prompt)
	}
}

func TestParseNameFallsBackToFileStem(t *testing.T) {
	r, err := Parse("mystem", []byte("---\ndescription: d\n---\nhi"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Name != "mystem" || r.Prompt != "hi" {
		t.Errorf("got Name %q Prompt %q", r.Name, r.Prompt)
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name, stem, src string
	}{
		{"no front matter", "x", "just a prompt"},
		{"unterminated", "x", "---\nname: x\nbody"},
		{"missing colon", "x", "---\nname x\n---\n"},
		{"nested too deep", "x", "---\nname: x\ntools:\n  deny:\n    a: b\n---\n"},
		{"unexpected indent", "x", "---\nname: x\n  stray: 1\n---\n"},
		{"list where scalar", "x", "---\nname: x\nagent: [a, b]\n---\n"},
		{"no name and empty stem", "", "---\ndescription: d\n---\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Parse(tt.stem, []byte(tt.src)); err == nil {
				t.Errorf("Parse succeeded, want error")
			}
		})
	}
}

func TestParseEmptyNameUsesStem(t *testing.T) {
	r, err := Parse("stem", []byte("---\nname: \"\"\n---\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if r.Name != "stem" {
		t.Errorf("Name = %q, want stem", r.Name)
	}
}

func TestParseUnknownKeysIgnored(t *testing.T) {
	r, err := Parse("x", []byte("---\nname: x\nfuture: [1, 2]\ntools:\n  mode: strict\n---\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if r.Name != "x" || len(r.AllowTools) != 0 || len(r.DenyTools) != 0 {
		t.Errorf("unexpected result %+v", r)
	}
}

func TestParseEmptyLists(t *testing.T) {
	r, err := Parse("x", []byte("---\ntools:\n  deny: []\n---\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.DenyTools) != 0 {
		t.Errorf("DenyTools = %v, want none", r.DenyTools)
	}
	if strings.Contains(r.Prompt, "---") {
		t.Errorf("Prompt = %q", r.Prompt)
	}
}
