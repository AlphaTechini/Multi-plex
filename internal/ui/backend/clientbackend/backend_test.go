package clientbackend

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/alphatechini/multi-plex/internal/client"
	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

// server is a scripted daemon end of a pipe.
type server struct {
	enc protocol.Encoder
	dec protocol.Decoder
}

func newPair(t *testing.T) (*Backend, *server) {
	t.Helper()
	cc, sc := net.Pipe()
	c := client.New(cc, protocol.JSON)
	b := New(c)
	t.Cleanup(func() { b.Close(); sc.Close() })
	return b, &server{enc: protocol.JSON.NewEncoder(sc), dec: protocol.JSON.NewDecoder(sc)}
}

// serve answers requests with fn until the pipe closes.
func (s *server) serve(fn func(method string, params json.RawMessage) any) {
	go func() {
		for {
			var f protocol.Frame
			if s.dec.Decode(&f) != nil {
				return
			}
			res := fn(f.Method, f.Data)
			data, _ := json.Marshal(res)
			_ = s.enc.Encode(&protocol.Frame{Type: protocol.TypeResponse, ID: f.ID, Data: data})
		}
	}()
}

func (s *server) push(topic string, v any) {
	data, _ := json.Marshal(v)
	_ = s.enc.Encode(&protocol.Frame{Type: protocol.TypeEvent, Topic: topic, Data: data})
}

func TestCalls(t *testing.T) {
	b, srv := newPair(t)
	var methods []string
	srv.serve(func(method string, params json.RawMessage) any {
		methods = append(methods, method)
		switch method {
		case protocol.MethodWorkspaceOpen:
			return core.Workspace{ID: "w1", Name: "multi-plex"}
		case protocol.MethodHello:
			return protocol.HelloResult{Protocol: protocol.Version, Approver: "typed-code"}
		case protocol.MethodAgentList:
			return []protocol.AgentInfo{{Name: "shell", DisplayName: "Shell", Available: true}}
		case protocol.MethodRoleList:
			var p protocol.WorkspaceRef
			_ = json.Unmarshal(params, &p)
			return []protocol.RoleInfo{{Name: "implementer", Source: string(p.WorkspaceID)}}
		case protocol.MethodSessionList:
			return []core.Session{{ID: "s1", Slug: "auth-refactor"}}
		case protocol.MethodSessionCreate:
			var p protocol.SessionCreateParams
			_ = json.Unmarshal(params, &p)
			return core.Session{ID: "s2", Task: p.Task, Role: p.Role}
		}
		return protocol.Empty{}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ws, err := b.OpenWorkspace(ctx, "/x")
	if err != nil || ws.ID != "w1" {
		t.Fatalf("OpenWorkspace = %v, %v", ws, err)
	}
	ss, err := b.Sessions(ctx, ws.ID)
	if err != nil || len(ss) != 1 || ss[0].Slug != "auth-refactor" {
		t.Fatalf("Sessions = %v, %v", ss, err)
	}
	if got := b.ApprovalMethod(); got != "typed-code" {
		t.Errorf("ApprovalMethod = %q, want the hello result's approver", got)
	}
	ags, err := b.Agents(ctx)
	if err != nil || len(ags) != 1 || ags[0].Name != "shell" {
		t.Fatalf("Agents = %v, %v", ags, err)
	}
	rs, err := b.Roles(ctx, ws.ID)
	if err != nil || len(rs) != 1 || rs[0].Source != "w1" {
		t.Fatalf("Roles = %v, %v (the workspace must be passed)", rs, err)
	}
	s, err := b.CreateSession(ctx, protocol.SessionCreateParams{Task: "do it", Role: "planner"})
	if err != nil || s.Task != "do it" || s.Role != "planner" {
		t.Fatalf("CreateSession = %v, %v", s, err)
	}
	if err := b.StopSession(ctx, "s1"); err != nil {
		t.Fatal(err)
	}
	want := []string{protocol.MethodWorkspaceOpen, protocol.MethodHello, protocol.MethodSubscribe, protocol.MethodSessionList,
		protocol.MethodAgentList, protocol.MethodRoleList, protocol.MethodSessionCreate, protocol.MethodSessionStop}
	if len(methods) != len(want) {
		t.Fatalf("methods = %v, want %v", methods, want)
	}
	for i := range want {
		if methods[i] != want[i] {
			t.Fatalf("methods = %v, want %v", methods, want)
		}
	}
}

func TestEventsForwardedInOrder(t *testing.T) {
	b, srv := newPair(t)
	srv.serve(func(string, json.RawMessage) any { return protocol.Empty{} })
	for i := range 5 {
		srv.push(protocol.TopicToast, protocol.Toast{Level: "info", Text: string(rune('a' + i))})
	}
	for i := range 5 {
		select {
		case ev := <-b.Events():
			var ts protocol.Toast
			_ = json.Unmarshal(ev.Data, &ts)
			if ev.Topic != protocol.TopicToast || ts.Text != string(rune('a'+i)) {
				t.Fatalf("event %d = %v %v", i, ev.Topic, ts)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout at event %d", i)
		}
	}
}

func TestMergeFrame(t *testing.T) {
	dst := &protocol.PaneFrame{PaneID: "p", Cols: 10, Rows: 3, Changed: []protocol.Row{{Y: 0, Text: "a"}, {Y: 1, Text: "b"}}}
	mergeFrame(dst, protocol.PaneFrame{PaneID: "p", Cols: 10, Rows: 3, Changed: []protocol.Row{{Y: 1, Text: "B"}, {Y: 2, Text: "C"}}, CursorX: 4})
	if len(dst.Changed) != 3 || dst.Changed[1].Text != "B" || dst.Changed[2].Text != "C" || dst.CursorX != 4 {
		t.Fatalf("merged = %+v", dst)
	}
	mergeFrame(dst, protocol.PaneFrame{PaneID: "p", Cols: 10, Rows: 3, Full: true, Changed: []protocol.Row{{Y: 0, Text: "x"}}})
	if !dst.Full || len(dst.Changed) != 1 {
		t.Fatalf("full frame must replace: %+v", dst)
	}
}

// A UI that does not read must never lose a pane's final screen state, and
// non-frame events must all arrive.
func TestFramesCoalesceNotDrop(t *testing.T) {
	b, srv := newPair(t)
	srv.serve(func(string, json.RawMessage) any { return protocol.Empty{} })
	const n = eventBuffer * 3
	go func() {
		srv.push(protocol.TopicNote, core.Note{ID: "n1"})
		for i := range n {
			srv.push(protocol.TopicPaneFrame, protocol.PaneFrame{
				PaneID: "p", Cols: 10, Rows: 2,
				Changed: []protocol.Row{{Y: i % 2, Text: string(rune('A' + i%26))}},
			})
		}
		srv.push(protocol.TopicNote, core.Note{ID: "n2"})
	}()
	time.Sleep(200 * time.Millisecond) // let the buffer fill
	rows := map[int]string{}
	notes := 0
	timeout := time.After(5 * time.Second)
	for notes < 2 {
		select {
		case ev := <-b.Events():
			switch ev.Topic {
			case protocol.TopicNote:
				notes++
			case protocol.TopicPaneFrame:
				var f protocol.PaneFrame
				_ = json.Unmarshal(ev.Data, &f)
				for _, r := range f.Changed {
					rows[r.Y] = r.Text
				}
			}
		case <-timeout:
			t.Fatalf("timeout; notes=%d", notes)
		}
	}
	// Drain what is left of the frames.
	for {
		select {
		case ev := <-b.Events():
			var f protocol.PaneFrame
			_ = json.Unmarshal(ev.Data, &f)
			for _, r := range f.Changed {
				rows[r.Y] = r.Text
			}
			continue
		case <-time.After(300 * time.Millisecond):
		}
		break
	}
	last := n - 1
	if rows[last%2] != string(rune('A'+last%26)) || rows[(last-1)%2] != string(rune('A'+(last-1)%26)) {
		t.Fatalf("final screen state wrong: %v", rows)
	}
}

func TestApprovalMethodBeforeHello(t *testing.T) {
	b, _ := newPair(t)
	if got := b.ApprovalMethod(); got != "" {
		t.Errorf("ApprovalMethod before OpenWorkspace = %q, want empty", got)
	}
}
