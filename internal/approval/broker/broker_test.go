package broker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alphatechini/multi-plex/internal/approval"
	"github.com/alphatechini/multi-plex/internal/approval/typedcode"
	"github.com/alphatechini/multi-plex/internal/core"
)

// fakeVerifier returns scripted results, one per Verify call (the last repeats).
type fakeVerifier struct {
	name      string
	available bool
	mu        sync.Mutex
	results   []error
	calls     int
	block     chan struct{} // if set, Verify waits for it (or ctx)
	messages  []string
}

func (f *fakeVerifier) Name() string                   { return f.name }
func (f *fakeVerifier) Available(context.Context) bool { return f.available }
func (f *fakeVerifier) Verify(ctx context.Context, _ core.ApprovalRequest, msg string) error {
	f.mu.Lock()
	i := f.calls
	f.calls++
	f.messages = append(f.messages, msg)
	if i >= len(f.results) {
		i = len(f.results) - 1
	}
	var err error
	if i >= 0 {
		err = f.results[i]
	}
	block := f.block
	f.mu.Unlock()
	if block != nil {
		select {
		case <-block:
		case <-ctx.Done():
			return approval.ErrTimeout
		}
	}
	return err
}

type pendingEvent struct {
	req          core.ApprovalRequest
	method, code string
}

type harness struct {
	b        *Broker
	pending  chan pendingEvent
	mu       sync.Mutex
	audited  []core.ApprovalDecision
	resolved []core.ApprovalDecision
}

func newHarness(vs []approval.Verifier, typed *typedcode.Verifier) *harness {
	h := &harness{pending: make(chan pendingEvent, 16)}
	h.b = New(vs, typed, func(_ core.ApprovalRequest, d core.ApprovalDecision) {
		h.mu.Lock()
		h.audited = append(h.audited, d)
		h.mu.Unlock()
	})
	h.b.OnPending = func(r core.ApprovalRequest, m, c string) { h.pending <- pendingEvent{r, m, c} }
	h.b.OnResolved = func(_ core.ApprovalRequest, d core.ApprovalDecision) {
		h.mu.Lock()
		h.resolved = append(h.resolved, d)
		h.mu.Unlock()
	}
	return h
}

func (h *harness) wait(t *testing.T) pendingEvent {
	t.Helper()
	select {
	case e := <-h.pending:
		return e
	case <-time.After(3 * time.Second):
		t.Fatal("no OnPending")
		return pendingEvent{}
	}
}

type reqResult struct {
	dec core.ApprovalDecision
	err error
}

func (h *harness) request(id string) <-chan reqResult {
	ch := make(chan reqResult, 1)
	go func() {
		d, err := h.b.Request(context.Background(), core.ApprovalRequest{ID: id, Kind: core.ApprovalCommand, Command: "git push --force", At: time.Now()})
		ch <- reqResult{d, err}
	}()
	return ch
}

func get(t *testing.T, ch <-chan reqResult) reqResult {
	t.Helper()
	select {
	case r := <-ch:
		return r
	case <-time.After(3 * time.Second):
		t.Fatal("Request did not return")
		return reqResult{}
	}
}

func TestApprove(t *testing.T) {
	f := &fakeVerifier{name: "windows-hello", available: true, results: []error{nil}}
	h := newHarness([]approval.Verifier{f}, typedcode.New())
	res := h.request("r1")
	ev := h.wait(t)
	if ev.method != "windows-hello" || ev.code != "" {
		t.Fatalf("event %+v", ev)
	}
	if got := h.b.Pending(); len(got) != 1 || got[0].ID != "r1" {
		t.Fatalf("pending %v", got)
	}
	d, err := h.b.Respond("r1", core.Approved, "", "")
	if err != nil || d.Outcome != core.Approved || d.Method != "windows-hello" {
		t.Fatalf("respond %+v %v", d, err)
	}
	r := get(t, res)
	if r.err != nil || r.dec.Outcome != core.Approved {
		t.Fatalf("request %+v", r)
	}
	if len(h.b.Pending()) != 0 || len(h.audited) != 1 || len(h.resolved) != 1 {
		t.Fatalf("pending=%d audited=%d resolved=%d", len(h.b.Pending()), len(h.audited), len(h.resolved))
	}
	if f.messages[0] == "" {
		t.Fatal("verifier got no message")
	}
}

func TestDenyWithMessageNeedsNoVerification(t *testing.T) {
	f := &fakeVerifier{name: "windows-hello", available: true, results: []error{nil}}
	h := newHarness([]approval.Verifier{f}, typedcode.New())
	res := h.request("r1")
	h.wait(t)
	d, err := h.b.Respond("r1", core.Denied, "use a branch instead", "")
	if err != nil || d.Outcome != core.Denied || d.Message != "use a branch instead" {
		t.Fatalf("respond %+v %v", d, err)
	}
	if r := get(t, res); r.dec.Outcome != core.Denied || r.dec.Message != "use a branch instead" {
		t.Fatalf("request %+v", r)
	}
	if f.calls != 0 {
		t.Fatal("deny must not verify")
	}
}

func TestCancelledThenApproved(t *testing.T) {
	f := &fakeVerifier{name: "windows-hello", available: true, results: []error{approval.ErrCancelled, approval.ErrFailed, nil}}
	h := newHarness([]approval.Verifier{f}, nil)
	res := h.request("r1")
	h.wait(t)
	if _, err := h.b.Respond("r1", core.Approved, "", ""); !errors.Is(err, approval.ErrCancelled) {
		t.Fatalf("first: %v", err)
	}
	if _, err := h.b.Respond("r1", core.Approved, "", ""); !errors.Is(err, approval.ErrFailed) {
		t.Fatalf("second: %v", err)
	}
	if len(h.b.Pending()) != 1 {
		t.Fatal("must stay pending after failures")
	}
	d, err := h.b.Respond("r1", core.Approved, "", "")
	if err != nil || d.Outcome != core.Approved {
		t.Fatalf("third: %+v %v", d, err)
	}
	if r := get(t, res); r.dec.Outcome != core.Approved {
		t.Fatalf("%+v", r)
	}
}

func TestFailuresExhausted(t *testing.T) {
	f := &fakeVerifier{name: "polkit", available: true, results: []error{approval.ErrFailed}}
	h := newHarness([]approval.Verifier{f}, nil)
	res := h.request("r1")
	h.wait(t)
	for i := 0; i < DefaultMaxFailures-1; i++ {
		if d, err := h.b.Respond("r1", core.Approved, "", ""); !errors.Is(err, approval.ErrFailed) {
			t.Fatalf("attempt %d: %+v %v", i, d, err)
		}
	}
	d, err := h.b.Respond("r1", core.Approved, "", "")
	if err != nil || d.Outcome != core.Denied || d.Method != "polkit" {
		t.Fatalf("final: %+v %v", d, err)
	}
	if r := get(t, res); r.dec.Outcome != core.Denied || r.dec.Method != "polkit" {
		t.Fatalf("%+v", r)
	}
}

func TestTimeout(t *testing.T) {
	h := newHarness([]approval.Verifier{&fakeVerifier{name: "x", available: true}}, nil)
	h.b.Timeout = 50 * time.Millisecond
	r := get(t, h.request("r1"))
	if r.err != nil || r.dec.Outcome != core.Denied || r.dec.Message != "approval timed out" {
		t.Fatalf("%+v", r)
	}
	if len(h.b.Pending()) != 0 || len(h.audited) != 1 {
		t.Fatal("timeout must clear and audit")
	}
	if _, err := h.b.Respond("r1", core.Approved, "", ""); !errors.Is(err, ErrUnknownRequest) {
		t.Fatalf("late respond: %v", err)
	}
}

func TestContextCancel(t *testing.T) {
	h := newHarness(nil, typedcode.New())
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan reqResult, 1)
	go func() {
		d, err := h.b.Request(ctx, core.ApprovalRequest{ID: "r1"})
		ch <- reqResult{d, err}
	}()
	h.wait(t)
	cancel()
	r := get(t, ch)
	if !errors.Is(r.err, context.Canceled) || r.dec.Outcome != core.Denied {
		t.Fatalf("%+v", r)
	}
}

func TestTypedCodePath(t *testing.T) {
	h := newHarness([]approval.Verifier{&fakeVerifier{name: "windows-hello", available: false}}, typedcode.New())
	res := h.request("r1")
	ev := h.wait(t)
	if ev.method != "typed-code" || len(ev.code) != typedcode.CodeLength {
		t.Fatalf("event %+v", ev)
	}
	if d := h.b.PendingDetails(); len(d) != 1 || d[0].Code != ev.code {
		t.Fatalf("details %+v", d)
	}
	if _, err := h.b.Respond("r1", core.Approved, "", "ZZZZZZ"); !errors.Is(err, approval.ErrFailed) {
		t.Fatalf("wrong code: %v", err)
	}
	d, err := h.b.Respond("r1", core.Approved, "", ev.code)
	if err != nil || d.Outcome != core.Approved || d.Method != "typed-code" {
		t.Fatalf("right code: %+v %v", d, err)
	}
	if r := get(t, res); r.dec.Outcome != core.Approved {
		t.Fatalf("%+v", r)
	}
}

func TestTypedCodeExhausted(t *testing.T) {
	h := newHarness(nil, typedcode.New())
	res := h.request("r1")
	h.wait(t)
	var d core.ApprovalDecision
	for i := 0; i < 3; i++ {
		d, _ = h.b.Respond("r1", core.Approved, "", "ZZZZZZ")
	}
	if d.Outcome != core.Denied || d.Method != "typed-code" {
		t.Fatalf("%+v", d)
	}
	if r := get(t, res); r.dec.Outcome != core.Denied {
		t.Fatalf("%+v", r)
	}
}

func TestFallsBackWhenMethodUnavailableAtVerify(t *testing.T) {
	f := &fakeVerifier{name: "windows-hello", available: true, results: []error{approval.ErrUnavailable}}
	h := newHarness([]approval.Verifier{f}, typedcode.New())
	res := h.request("r1")
	if ev := h.wait(t); ev.method != "windows-hello" {
		t.Fatalf("%+v", ev)
	}
	if _, err := h.b.Respond("r1", core.Approved, "", ""); !errors.Is(err, approval.ErrUnavailable) {
		t.Fatalf("%v", err)
	}
	ev := h.wait(t) // re-announced with the typed code
	if ev.method != "typed-code" || ev.code == "" {
		t.Fatalf("%+v", ev)
	}
	if d, err := h.b.Respond("r1", core.Approved, "", ev.code); err != nil || d.Outcome != core.Approved {
		t.Fatalf("%+v %v", d, err)
	}
	get(t, res)
}

func TestNoMethodAtAll(t *testing.T) {
	h := newHarness([]approval.Verifier{&fakeVerifier{name: "x"}}, nil)
	r := get(t, h.request("r1"))
	if !errors.Is(r.err, ErrNoMethod) || r.dec.Outcome != core.Denied || len(h.audited) != 1 {
		t.Fatalf("%+v", r)
	}
}

func TestDuplicateAndEmptyID(t *testing.T) {
	h := newHarness(nil, typedcode.New())
	h.request("r1")
	h.wait(t)
	if _, err := h.b.Request(context.Background(), core.ApprovalRequest{ID: "r1"}); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("dup: %v", err)
	}
	if _, err := h.b.Request(context.Background(), core.ApprovalRequest{}); err == nil {
		t.Fatal("empty id must fail")
	}
	if _, err := h.b.Respond("r1", "bogus", "", ""); err == nil {
		t.Fatal("bad outcome must fail")
	}
	h.b.Respond("r1", core.Denied, "", "")
}

func TestDenyWhilePromptIsUp(t *testing.T) {
	f := &fakeVerifier{name: "windows-hello", available: true, block: make(chan struct{}), results: []error{nil}}
	h := newHarness([]approval.Verifier{f}, nil)
	res := h.request("r1")
	h.wait(t)
	type rr = reqResult
	appr := make(chan rr, 1)
	go func() { d, err := h.b.Respond("r1", core.Approved, "", ""); appr <- rr{d, err} }()
	for { // wait until the prompt is up
		f.mu.Lock()
		n := f.calls
		f.mu.Unlock()
		if n > 0 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if _, err := h.b.Respond("r1", core.Approved, "", ""); !errors.Is(err, ErrBusy) {
		t.Fatalf("second approve: %v", err)
	}
	if d, _ := h.b.Respond("r1", core.Denied, "no", ""); d.Outcome != core.Denied {
		t.Fatalf("%+v", d)
	}
	close(f.block)
	if r := get(t, appr); r.dec.Outcome != core.Denied {
		t.Fatalf("approve returned %+v", r)
	}
	if r := get(t, res); r.dec.Outcome != core.Denied {
		t.Fatalf("%+v", r)
	}
}

func TestConcurrentRequests(t *testing.T) {
	f := &fakeVerifier{name: "windows-hello", available: true, results: []error{nil}}
	h := newHarness([]approval.Verifier{f}, typedcode.New())
	h.pending = make(chan pendingEvent, 100)
	const n = 40
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("r%02d", i)
		res := h.request(id)
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var d core.ApprovalDecision
			var err error
			for { // retry until the request is registered
				if i%2 == 0 {
					d, err = h.b.Respond(id, core.Approved, "", "")
				} else {
					d, err = h.b.Respond(id, core.Denied, "no", "")
				}
				if !errors.Is(err, ErrUnknownRequest) {
					break
				}
				time.Sleep(time.Millisecond)
			}
			want := core.Approved
			if i%2 == 1 {
				want = core.Denied
			}
			if err != nil || d.Outcome != want {
				t.Errorf("%s: %+v %v", id, d, err)
			}
			if r := get(t, res); r.dec.Outcome != want {
				t.Errorf("%s request: %+v", id, r)
			}
		}(i)
	}
	wg.Wait()
	if len(h.b.Pending()) != 0 || len(h.audited) != n {
		t.Fatalf("pending=%d audited=%d", len(h.b.Pending()), len(h.audited))
	}
}

func TestPendingOrder(t *testing.T) {
	h := newHarness(nil, typedcode.New())
	now := time.Now()
	for i, id := range []string{"b", "a", "c"} {
		go h.b.Request(context.Background(), core.ApprovalRequest{ID: id, At: now.Add(time.Duration(2-i) * time.Second)})
	}
	for i := 0; i < 3; i++ {
		h.wait(t)
	}
	got := h.b.Pending()
	if len(got) != 3 || got[0].ID != "c" || got[1].ID != "a" || got[2].ID != "b" {
		t.Fatalf("%v", got)
	}
	for _, r := range got {
		h.b.Respond(r.ID, core.Denied, "", "")
	}
}
