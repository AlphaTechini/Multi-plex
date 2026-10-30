package polkit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/alphatechini/multi-plex/internal/approval"
	"github.com/alphatechini/multi-plex/internal/core"
)

// agentProcesses are process names (/proc/<pid>/comm, truncated to 15
// characters by the kernel) of desktop components that provide a polkit
// authentication agent.
var agentProcesses = map[string]bool{
	"polkit-gnome-au": true, // polkit-gnome-authentication-agent-1
	"polkit-kde-auth": true, // polkit-kde-authentication-agent-1
	"polkit-mate-aut": true,
	"lxqt-policykit-": true,
	"lxpolkit":        true,
	"xfce-polkit":     true,
	"gnome-shell":     true, // built-in agent
	"plasmashell":     true, // KDE Plasma starts its agent via kded/plasma
	"cosmic-osd":      true,
	"ukui-polkit":     true,
	"hyprpolkitagent": true,
	"soteria":         true,
}

// Verifier authenticates through pkexec.
type Verifier struct {
	// pkexec and trueBin are resolved lazily; tests set them directly.
	pkexec  string
	trueBin string
	// procDir is /proc, replaceable in tests. onlyLinux is false in tests.
	procDir   string
	onlyLinux bool
	getenv    func(string) string
}

// New returns a polkit Verifier.
func New() *Verifier {
	return &Verifier{procDir: "/proc", onlyLinux: true, getenv: os.Getenv}
}

// Name implements approval.Verifier.
func (v *Verifier) Name() string { return "polkit" }

// Available implements approval.Verifier. It is conservative: Linux only,
// pkexec on PATH, a graphical session (DISPLAY or WAYLAND_DISPLAY), and a
// running process that is known to provide a polkit agent.
func (v *Verifier) Available(context.Context) bool {
	if v.onlyLinux && runtime.GOOS != "linux" {
		return false
	}
	if v.getenv("DISPLAY") == "" && v.getenv("WAYLAND_DISPLAY") == "" {
		return false
	}
	if _, _, ok := v.resolve(); !ok {
		return false
	}
	return agentRunning(v.procDir)
}

// Verify implements approval.Verifier. The message is not shown by pkexec
// (see the package doc); it is accepted for interface compatibility.
func (v *Verifier) Verify(ctx context.Context, _ core.ApprovalRequest, _ string) error {
	pk, tr, ok := v.resolve()
	if !ok {
		return approval.ErrUnavailable
	}
	// --disable-internal-agent: never fall back to a text prompt on a tty,
	// which an agent in the same terminal could answer.
	cmd := exec.CommandContext(ctx, pk, "--disable-internal-agent", tr)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		return approval.ErrTimeout
	}
	return mapExit(err, stderr.String())
}

// resolve finds pkexec and an absolute path to a harmless program for it to run.
func (v *Verifier) resolve() (pkexecPath, trueBin string, ok bool) {
	pk, tr := v.pkexec, v.trueBin
	var err error
	if pk == "" {
		if pk, err = exec.LookPath("pkexec"); err != nil {
			return "", "", false
		}
	}
	if tr == "" {
		if tr, err = exec.LookPath("true"); err != nil {
			return "", "", false
		}
		if tr, err = filepath.Abs(tr); err != nil {
			return "", "", false
		}
	}
	return pk, tr, true
}

// mapExit converts pkexec's exit status to neutral errors.
func mapExit(runErr error, stderr string) error {
	if runErr == nil {
		return nil
	}
	var ee *exec.ExitError
	if !errors.As(runErr, &ee) {
		return fmt.Errorf("%w: %v", approval.ErrUnavailable, runErr)
	}
	switch ee.ExitCode() {
	case 126:
		return approval.ErrCancelled
	case 127:
		low := strings.ToLower(stderr)
		if strings.Contains(low, "no authentication agent") || strings.Contains(low, "error getting authority") {
			return fmt.Errorf("%w: no polkit agent", approval.ErrUnavailable)
		}
		return approval.ErrFailed
	}
	return fmt.Errorf("%w: pkexec exit %d", approval.ErrFailed, ee.ExitCode())
}

// agentRunning scans procDir for a known agent process.
func agentRunning(procDir string) bool {
	ents, err := os.ReadDir(procDir)
	if err != nil {
		return false
	}
	for _, e := range ents {
		if !e.IsDir() || e.Name()[0] < '0' || e.Name()[0] > '9' {
			continue
		}
		b, err := os.ReadFile(filepath.Join(procDir, e.Name(), "comm"))
		if err == nil && agentProcesses[strings.TrimSpace(string(b))] {
			return true
		}
	}
	return false
}
