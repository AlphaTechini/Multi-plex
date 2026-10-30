package bench

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSummarize(t *testing.T) {
	var s []time.Duration
	for i := 1; i <= 100; i++ {
		s = append(s, time.Duration(i)*time.Millisecond)
	}
	st := summarize(s)
	if st.N != 100 || st.Min != 1 || st.Max != 100 || st.P50 != 50 || st.P95 != 95 || st.P99 != 99 {
		t.Errorf("stats = %+v", st)
	}
	if st.Mean < 50 || st.Mean > 51 {
		t.Errorf("mean = %v", st.Mean)
	}
	if z := summarize(nil); z.N != 0 {
		t.Errorf("empty = %+v", z)
	}
	if one := summarize([]time.Duration{3 * time.Millisecond}); one.P99 != 3 || one.P50 != 3 {
		t.Errorf("single = %+v", one)
	}
}

func TestParseFirstFrame(t *testing.T) {
	tests := []struct {
		in   string
		want time.Duration
		ok   bool
	}{
		{"first_frame=84.3ms\n", 84300 * time.Microsecond, true},
		{"noise\n  first_frame=1.2s trailing\nmore", 1200 * time.Millisecond, true},
		{"first_frame=oops", 0, false},
		{"nothing", 0, false},
	}
	for _, tt := range tests {
		got, ok := parseFirstFrame(tt.in)
		if got != tt.want || ok != tt.ok {
			t.Errorf("parseFirstFrame(%q) = %v, %v", tt.in, got, ok)
		}
	}
}

func TestRenderBench(t *testing.T) {
	var out, errb bytes.Buffer
	code := Run([]string{"render", "-mb", "1", "--json"}, &out, &errb)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	var r Result
	if err := json.Unmarshal(out.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	if r.Name != "render" || r.Machine.GoVersion == "" || r.Machine.CPUs == 0 || len(r.Metrics) < 4 {
		t.Errorf("result = %+v", r)
	}
}

// TestHookBenchInProcess runs the hook benchmark against an in-process daemon
// (without spawning mpx processes, which would run this test binary).
func TestHookBenchInProcess(t *testing.T) {
	dir := t.TempDir()
	var out, errb bytes.Buffer
	code := Run([]string{"hook", "-n", "30", "-procs", "0", "-out", dir + string(os.PathSeparator)}, &out, &errb)
	if code != 0 {
		t.Fatalf("exit %d: %s%s", code, out.String(), errb.String())
	}
	if !strings.Contains(out.String(), "round trip") {
		t.Errorf("table:\n%s", out.String())
	}
	b, err := os.ReadFile(filepath.Join(dir, "hook.json"))
	if err != nil {
		t.Fatal(err)
	}
	var r Result
	if err := json.Unmarshal(b, &r); err != nil || r.Metrics[0].Stats == nil || r.Metrics[0].Stats.N != 30 {
		t.Errorf("saved result: %v %+v", err, r)
	}
	if r.Metrics[0].Stats.P50 > 10 {
		t.Logf("in-process hook round trip p50 = %.2fms (slow machine?)", r.Metrics[0].Stats.P50)
	}
}

func TestUsage(t *testing.T) {
	var out, errb bytes.Buffer
	if code := Run(nil, &out, &errb); code != 2 {
		t.Errorf("no args: exit %d", code)
	}
	if code := Run([]string{"nope"}, &out, &errb); code != 2 {
		t.Errorf("unknown: exit %d", code)
	}
	if code := Run([]string{"hook", "-tool", "bogus"}, &out, &errb); code != 1 {
		t.Errorf("bad tool: exit %d", code)
	}
}
