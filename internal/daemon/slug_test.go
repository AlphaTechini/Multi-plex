package daemon

import (
	"strings"
	"testing"

	"github.com/alphatechini/multi-plex/internal/core"
)

func TestSlugify(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Fix the Auth refresh!", "fix-the-auth-refresh"},
		{"  ", "session"},
		{"", "session"},
		{"___", "session"},
		{"ÄÖ über", "ber"},
		{"a--b__c", "a-b-c"},
		{"Add token cache to Session.Refresh and update every caller in api/ and worker/ packages", "add-token-cache-to-session-refresh-and"},
		{strings.Repeat("x", 100), strings.Repeat("x", 40)},
	}
	for _, tt := range tests {
		got := slugify(tt.in)
		if got != tt.want {
			t.Errorf("slugify(%q) = %q, want %q", tt.in, got, tt.want)
		}
		if len(got) > 40 || strings.ContainsAny(got, "ABCDEFGHIJKLMNOPQRSTUVWXYZ _/") || strings.HasSuffix(got, "-") || strings.HasPrefix(got, "-") {
			t.Errorf("slugify(%q) = %q is not a valid slug", tt.in, got)
		}
	}
}

func TestUniqueSlug(t *testing.T) {
	taken := map[string]bool{"a": true, "a-2": true, "a-3": true}
	if got := uniqueSlug("a", func(s string) bool { return taken[s] }); got != "a-4" {
		t.Errorf("got %q", got)
	}
	long := strings.Repeat("y", 40)
	takenLong := map[string]bool{long: true}
	got := uniqueSlug(long, func(s string) bool { return takenLong[s] })
	if len(got) > 40 || got != strings.Repeat("y", 38)+"-2" {
		t.Errorf("long slug = %q (%d)", got, len(got))
	}
}

func TestPortAlloc(t *testing.T) {
	var p portAlloc
	a, b := p.Alloc("s1"), p.Alloc("s2")
	if a != 20000 || b != 20100 {
		t.Errorf("blocks = %d, %d", a, b)
	}
	if p.Alloc("s1") != a {
		t.Error("a session got a second block")
	}
	p.Release("s1")
	if c := p.Alloc(core.SessionID("s3")); c != 20000 {
		t.Errorf("released block not reused: %d", c)
	}
	seen := map[int]bool{}
	var q portAlloc
	for i := 0; i < 400; i++ {
		base := q.Alloc(core.SessionID(string(rune('a'+i%26)) + string(rune('a'+i/26))))
		if base == 0 || seen[base] {
			t.Fatalf("block %d reused or exhausted at %d", base, i)
		}
		seen[base] = true
	}
}
