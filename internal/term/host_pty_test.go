package term

import (
	"bufio"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alphatechini/multi-plex/internal/pty"
	"github.com/alphatechini/multi-plex/internal/pty/xptyimpl"
	"github.com/alphatechini/multi-plex/internal/vt/charmvt"
)

// TestHelperProcess is the child program for the real-PTY test below (the
// test binary running itself); it does nothing in a normal test run.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("MPX_TEST_HELPER") != "hostpty" {
		return
	}
	// Ask the terminal who it is: the host's emulator must answer (DA1), or
	// this blocks and the test times out.
	os.Stdout.WriteString("\x1b[31mred text\x1b[0m\r\n\x1b[c")
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		if strings.Contains(sc.Text(), "\x1b[?") {
			os.Stdout.WriteString("\r\ngot-reply")
			os.Exit(4)
		}
	}
	os.Exit(5)
}

func TestHostWithRealPTY(t *testing.T) {
	rec := &sinkRecorder{}
	spec := pty.Spec{
		Argv: []string{os.Args[0], "-test.run=^TestHelperProcess$"},
		Env:  append(os.Environ(), "MPX_TEST_HELPER=hostpty"),
		Cols: 40,
		Rows: 6,
	}
	h, err := Start(context.Background(), xptyimpl.New(), charmvt.Factory, spec, "pane_pty", rec.sink)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()

	// The line-buffered child only sees the reply once a newline follows it,
	// so keep pressing Enter until it exits.
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		for {
			select {
			case <-stop:
				return
			case <-time.After(50 * time.Millisecond):
				_ = h.Input([]byte("\r"))
			}
		}
	}()

	waited := make(chan struct{})
	var code int
	go func() { code, _ = h.Wait(); close(waited) }()
	select {
	case <-waited:
	case <-time.After(20 * time.Second):
		t.Fatalf("host did not report exit; screen %q", rec.screen())
	}
	if code != 4 {
		t.Errorf("exit code = %d, want 4", code)
	}
	// ConPTY may prepend its own setup output, so look for the text anywhere.
	screen := strings.Join(rec.screen(), "\n")
	if !strings.Contains(screen, "red text") || !strings.Contains(screen, "got-reply") {
		t.Errorf("final screen = %q", screen)
	}
}
