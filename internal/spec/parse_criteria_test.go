package spec

import (
	"reflect"
	"testing"
)

func TestParseCriteria(t *testing.T) {
	got := ParseCriteria(requirementsMD)
	want := []Criterion{
		{ID: "R1.1", Text: "WHEN a request is made with a token that expires within 60 seconds, THE SYSTEM SHALL refresh the token before sending the request."},
		{ID: "R1.2", Text: "IF the refresh fails, THEN THE SYSTEM SHALL return an authentication error and SHALL NOT retry more than once."},
		{ID: "R2.1", Text: "WHEN a refresh succeeds, THE SYSTEM SHALL log the token family and expiry time without logging the token itself."},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseCriteria mismatch\n got: %+v\nwant: %+v", got, want)
	}
}

func TestParseCriteriaNoRequirements(t *testing.T) {
	tests := map[string]string{
		"empty":            "",
		"no headings":      "1. stray item\n2. another\n",
		"items after h2":   "### Requirement 1: X\n## Notes\n1. not a criterion\n",
		"other h3 heading": "### Background\n1. not a criterion\n",
	}
	for name, md := range tests {
		t.Run(name, func(t *testing.T) {
			if got := ParseCriteria(md); len(got) != 0 {
				t.Errorf("ParseCriteria = %+v, want none", got)
			}
		})
	}
}

func TestParseCriteriaNormalisesRequirementNumber(t *testing.T) {
	md := "### Requirement 03: Padded\n1. Something\n"
	got := ParseCriteria(md)
	if len(got) != 1 || got[0].ID != "R3.1" {
		t.Errorf("ParseCriteria = %+v, want ID R3.1", got)
	}
}
