package spec

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseTasks(t *testing.T) {
	got := ParseTasks(tasksMD)
	want := []Task{
		{ID: "1", Title: "Token store", Done: true, Line: 3},
		{ID: "1.1", Title: "Add store interface", Done: true, Requirements: []string{"R1.1"}, Line: 4},
		{ID: "1.2", Title: "Persist expiry times", Requirements: []string{"R1.1", "R1.2"}, Line: 6},
		{ID: "2", Title: "Refresh logic", InProgress: true, Line: 8},
		{ID: "2.1", Title: "Refresh before expiry", Done: true, Requirements: []string{"R1.1"}, Line: 9},
		{ID: "2.2", Title: "Retry once", Requirements: []string{"R1.2"}, Line: 11},
		{ID: "3", Title: "Docs", Requirements: []string{"R2.1"}, Line: 14},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseTasks mismatch\n got: %+v\nwant: %+v", got, want)
	}
}

func TestParseTasksCheckboxStyles(t *testing.T) {
	tests := []struct {
		name string
		md   string
		want Task
	}{
		{"dash open", "- [ ] Write it", Task{ID: "1", Title: "Write it"}},
		{"dash lower x", "- [x] Write it", Task{ID: "1", Title: "Write it", Done: true}},
		{"star upper X", "* [X] Write it", Task{ID: "1", Title: "Write it", Done: true}},
		{"tilde in progress", "- [~] Write it", Task{ID: "1", Title: "Write it", InProgress: true}},
		{"dash in progress", "- [-] Write it", Task{ID: "1", Title: "Write it", InProgress: true}},
		{"optional marker", "- [ ]* 2.1 Extra", Task{ID: "2.1", Title: "Extra"}},
		{"numbered with dot", "- [ ] 4. Thing", Task{ID: "4", Title: "Thing"}},
		{"crlf line endings", "- [x] 5.1 Thing\r", Task{ID: "5.1", Title: "Thing", Done: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseTasks(tt.md)
			if len(got) != 1 {
				t.Fatalf("got %d tasks, want 1", len(got))
			}
			tt.want.Line = 1
			if !reflect.DeepEqual(got[0], tt.want) {
				t.Errorf("got %+v, want %+v", got[0], tt.want)
			}
		})
	}
}

func TestRequirementIDs(t *testing.T) {
	tests := map[string]string{
		"_Requirements: R1.1, R2.3_": "R1.1,R2.3",
		"R1.2 R1.2 R10.11":           "R1.2,R1.2,R10.11",
		"XR1.1 R1.10x R1.1.2 R3.4":   "R3.4",
		"no ids here 1.1":            "",
	}
	for in, want := range tests {
		var got []string
		got = append(got, requirementIDs(in)...)
		joined := strings.Join(got, ",")
		if joined != want {
			t.Errorf("requirementIDs(%q) = %q, want %q", in, joined, want)
		}
	}
}

func TestParseTasksIgnoresNonTasks(t *testing.T) {
	md := "Plain text\n- a bullet\n[ ] no bullet\n- [?] unknown marker\n"
	if got := ParseTasks(md); len(got) != 0 {
		t.Errorf("ParseTasks = %+v, want none", got)
	}
}

func TestParseTasksUnnumberedIDsUseIndex(t *testing.T) {
	md := "- [ ] Alpha\n- [ ] 7. Beta\n- [ ] Gamma\n"
	got := ParseTasks(md)
	ids := []string{got[0].ID, got[1].ID, got[2].ID}
	want := []string{"1", "7", "3"}
	if !reflect.DeepEqual(ids, want) {
		t.Errorf("IDs = %v, want %v", ids, want)
	}
}
