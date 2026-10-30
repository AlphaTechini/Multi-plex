package spec

import (
	"errors"
	"strings"
	"testing"
)

func TestUpdateTask(t *testing.T) {
	tests := []struct {
		name  string
		id    string
		state TaskState
		want  string // the line that must change, with its index in tasksMD
		line  int
	}{
		{"open to done, nested", "1.2", TaskDone, "  - [x] 1.2 Persist expiry times", 6},
		{"in progress to done", "2", TaskDone, "- [x] 2. Refresh logic", 8},
		{"done to open", "2.1", TaskOpen, "  - [ ] 2.1 Refresh before expiry", 9},
		{"to in progress", "2.2", TaskInProgress, "  - [~] 2.2 Retry once", 11},
		{"optional star kept", "3", TaskDone, "* [x] 3. Docs", 14},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := UpdateTask(tasksMD, tt.id, tt.state)
			if err != nil {
				t.Fatalf("UpdateTask: %v", err)
			}
			in := strings.Split(tasksMD, "\n")
			got := strings.Split(out, "\n")
			if len(got) != len(in) {
				t.Fatalf("line count changed: %d -> %d", len(in), len(got))
			}
			for i := range in {
				if i == tt.line-1 {
					if got[i] != tt.want {
						t.Errorf("line %d = %q, want %q", tt.line, got[i], tt.want)
					}
					continue
				}
				if got[i] != in[i] {
					t.Errorf("line %d changed unexpectedly: %q", i+1, got[i])
				}
			}
		})
	}
}

func TestUpdateTaskPreservesCRLF(t *testing.T) {
	md := "- [ ] 1. First\r\n- [ ] 2. Second\r\n"
	out, err := UpdateTask(md, "2", TaskDone)
	if err != nil {
		t.Fatal(err)
	}
	want := "- [ ] 1. First\r\n- [x] 2. Second\r\n"
	if out != want {
		t.Errorf("got %q, want %q", out, want)
	}
}

func TestUpdateTaskErrors(t *testing.T) {
	dup := "- [ ] 1. A\n- [ ] 1. B\n"
	tests := []struct {
		name    string
		md      string
		id      string
		state   TaskState
		wantErr error
	}{
		{"unknown id", tasksMD, "9.9", TaskDone, ErrTaskNotFound},
		{"ambiguous id", dup, "1", TaskDone, ErrAmbiguousTask},
		{"bad state", tasksMD, "1", TaskState("blocked"), ErrInvalidState},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := UpdateTask(tt.md, tt.id, tt.state)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
