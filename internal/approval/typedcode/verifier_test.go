package typedcode

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alphatechini/multi-plex/internal/approval"
	"github.com/alphatechini/multi-plex/internal/core"
)

func req(id string) core.ApprovalRequest { return core.ApprovalRequest{ID: id} }

func verifyAsync(v *Verifier, ctx context.Context, id string) <-chan error {
	ch := make(chan error, 1)
	go func() { ch <- v.Verify(ctx, req(id), "") }()
	return ch
}

func TestCodeFormat(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 500; i++ {
		c := newCode()
		if len(c) != CodeLength {
			t.Fatalf("len %d", len(c))
		}
		if strings.ContainsAny(c, "0O1IL") {
			t.Fatalf("ambiguous char in %q", c)
		}
		for _, r := range c {
			if !strings.ContainsRune(Alphabet, r) {
				t.Fatalf("char %q outside alphabet", r)
			}
		}
		seen[c] = true
	}
	if len(seen) < 490 {
		t.Fatalf("codes not random enough: %d unique of 500", len(seen))
	}
}

func TestIssueIdempotent(t *testing.T) {
	v := New()
	a := v.Issue("r1")
	if b := v.Issue("r1"); a != b {
		t.Fatal("same request must keep its code")
	}
	if c := v.Issue("r2"); c == a {
		t.Fatal("different requests should differ")
	}
}

func TestSubmitSuccess(t *testing.T) {
	v := New()
	code := v.Issue("r1")
	done := verifyAsync(v, context.Background(), "r1")
	time.Sleep(20 * time.Millisecond)
	// Lower case, spaces and dashes are tolerated.
	typed := strings.ToLower(code[:3]) + " - " + code[3:]
	if err := v.Submit("r1", typed); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("Verify = %v", err)
	}
	// Single use.
	if err := v.Submit("r1", code); !errors.Is(err, approval.ErrFailed) {
		t.Fatalf("reuse = %v", err)
	}
}

func TestAttemptsExhausted(t *testing.T) {
	v := New()
	v.Issue("r1")
	done := verifyAsync(v, context.Background(), "r1")
	for i := 0; i < MaxAttempts; i++ {
		if err := v.Submit("r1", "ZZZZZZ"); !errors.Is(err, approval.ErrFailed) {
			t.Fatalf("attempt %d = %v", i, err)
		}
	}
	if err := <-done; !errors.Is(err, approval.ErrFailed) {
		t.Fatalf("Verify = %v", err)
	}
}

func TestCorrectCodeAfterWrongOne(t *testing.T) {
	v := New()
	code := v.Issue("r1")
	done := verifyAsync(v, context.Background(), "r1")
	_ = v.Submit("r1", "ZZZZZZ")
	if err := v.Submit("r1", code); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestExpiry(t *testing.T) {
	v := New()
	v.ttl = 50 * time.Millisecond
	code := v.Issue("r1")
	if err := v.Verify(context.Background(), req("r1"), ""); !errors.Is(err, approval.ErrTimeout) {
		t.Fatalf("Verify = %v", err)
	}
	if err := v.Submit("r1", code); !errors.Is(err, approval.ErrFailed) && !errors.Is(err, approval.ErrTimeout) {
		t.Fatalf("Submit after expiry = %v", err)
	}
}

func TestSubmitExpiredBeforeVerify(t *testing.T) {
	v := New()
	v.ttl = 10 * time.Millisecond
	code := v.Issue("r1")
	time.Sleep(30 * time.Millisecond)
	if err := v.Submit("r1", code); !errors.Is(err, approval.ErrTimeout) {
		t.Fatalf("Submit = %v", err)
	}
}

func TestContextEnds(t *testing.T) {
	v := New()
	v.Issue("r1")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if err := v.Verify(ctx, req("r1"), ""); !errors.Is(err, approval.ErrTimeout) {
		t.Fatalf("deadline: %v", err)
	}
	v.Issue("r2")
	ctx2, cancel2 := context.WithCancel(context.Background())
	cancel2()
	if err := v.Verify(ctx2, req("r2"), ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
}

func TestSubmitUnknown(t *testing.T) {
	if err := New().Submit("nope", "ABCDEF"); !errors.Is(err, approval.ErrFailed) {
		t.Fatalf("got %v", err)
	}
}

func TestConcurrent(t *testing.T) {
	v := New()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		id := string(rune('a' + i))
		code := v.Issue(id)
		done := verifyAsync(v, context.Background(), id)
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = v.Submit(id, "WRONG1")
			if err := v.Submit(id, code); err != nil {
				t.Errorf("%s: %v", id, err)
			}
			if err := <-done; err != nil {
				t.Errorf("%s verify: %v", id, err)
			}
		}()
	}
	wg.Wait()
}

func TestInterface(t *testing.T) {
	var _ approval.Verifier = New()
	if !New().Available(context.Background()) || New().Name() != "typed-code" {
		t.Fatal("basics")
	}
}
