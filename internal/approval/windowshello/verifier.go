package windowshello

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/alphatechini/multi-plex/internal/approval"
	"github.com/alphatechini/multi-plex/internal/core"
)

const (
	availabilityTTL = 60 * time.Second
	defaultTimeout  = 60 * time.Second
	maxMessageRunes = 200
)

// Verifier runs the Windows Hello helper executable.
type Verifier struct {
	helper  string
	timeout time.Duration
	// onlyWindows is false in tests so the helper logic runs on any OS.
	onlyWindows bool

	mu       sync.Mutex
	checked  time.Time
	cachedOK bool
}

// New returns a Verifier that runs the helper at helperPath. An empty path
// means the method is never available.
func New(helperPath string) *Verifier {
	return &Verifier{helper: helperPath, timeout: defaultTimeout, onlyWindows: true}
}

// Name implements approval.Verifier.
func (v *Verifier) Name() string { return "windows-hello" }

// Available implements approval.Verifier. It is true only on Windows, when
// the helper exists and its "check" command succeeds. The answer is cached
// for 60 seconds.
func (v *Verifier) Available(ctx context.Context) bool {
	if v.helper == "" || (v.onlyWindows && runtime.GOOS != "windows") {
		return false
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if !v.checked.IsZero() && time.Since(v.checked) < availabilityTTL {
		return v.cachedOK
	}
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	res, _ := run(cctx, v.helper, checkArgs())
	v.cachedOK = res.Result == "available" && res.exit == 0
	v.checked = time.Now()
	return v.cachedOK
}

// Verify implements approval.Verifier. It shows the Windows Hello prompt and
// maps the helper's answer to nil, ErrCancelled, ErrUnavailable, ErrTimeout or
// ErrFailed.
func (v *Verifier) Verify(ctx context.Context, _ core.ApprovalRequest, message string) error {
	if v.helper == "" {
		return approval.ErrUnavailable
	}
	res, err := run(ctx, v.helper, verifyArgs(message, v.timeout))
	if err != nil && ctx.Err() != nil {
		return approval.ErrTimeout
	}
	return mapResult(res, err)
}

var errBadOutput = errors.New("windows-hello helper: bad output")

// helperResult is the helper's single JSON line plus its exit code.
type helperResult struct {
	Result string `json:"result"`
	Detail string `json:"detail"`
	exit   int
}

func checkArgs() []string { return []string{"check"} }

// verifyArgs builds the arguments for "verify". The message is cut to a
// length the Hello dialog can show; the full text is in the TUI.
func verifyArgs(message string, timeout time.Duration) []string {
	secs := int(timeout / time.Second)
	if secs < 1 {
		secs = 1
	}
	return []string{"verify", "--message", truncate(message, maxMessageRunes), "--timeout", strconv.Itoa(secs)}
}

func truncate(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max-1]) + "…"
}

// run executes the helper directly (no shell) and parses its JSON line. A
// helper that exits non-zero but printed valid JSON still yields a result.
func run(ctx context.Context, path string, args []string) (helperResult, error) {
	cmd := exec.CommandContext(ctx, path, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	var res helperResult
	if cmd.ProcessState != nil {
		res.exit = cmd.ProcessState.ExitCode()
	}
	if jerr := json.Unmarshal(lastLine(out.Bytes()), &res); jerr != nil && err == nil {
		err = fmt.Errorf("%w: %v", errBadOutput, jerr)
	}
	return res, err
}

func lastLine(b []byte) []byte {
	b = bytes.TrimSpace(b)
	if i := bytes.LastIndexByte(b, '\n'); i >= 0 {
		b = b[i+1:]
	}
	return b
}

// mapResult converts a helper answer to the neutral errors.
func mapResult(res helperResult, runErr error) error {
	switch res.Result {
	case "verified":
		if res.exit == 0 {
			return nil
		}
		return approval.ErrFailed
	case "canceled":
		return approval.ErrCancelled
	case "not_available", "not_configured", "disabled_by_policy", "device_busy":
		return fmt.Errorf("%w: windows hello %s", approval.ErrUnavailable, res.Result)
	case "retries_exhausted":
		return fmt.Errorf("%w: too many failed attempts", approval.ErrFailed)
	case "error":
		if res.exit == 4 || res.Detail == "timeout" {
			return approval.ErrTimeout
		}
		return fmt.Errorf("%w: %s", approval.ErrFailed, res.Detail)
	}
	if errors.Is(runErr, errBadOutput) {
		return fmt.Errorf("%w: %v", approval.ErrFailed, runErr)
	}
	if runErr != nil {
		var ee *exec.ExitError
		if errors.As(runErr, &ee) {
			return fmt.Errorf("%w: helper exit %d", approval.ErrFailed, ee.ExitCode())
		}
		return fmt.Errorf("%w: %v", approval.ErrUnavailable, runErr)
	}
	return fmt.Errorf("%w: unknown result %q", approval.ErrFailed, res.Result)
}
