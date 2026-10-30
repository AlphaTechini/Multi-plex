// Command cc-01-pty checks that Multiplex can host a shell in a pseudo-terminal
// from Go: Unix PTY on Linux and macOS, ConPTY on Windows (docs/13, CC-01).
//
// It spawns the platform shell through internal/pty/xptyimpl, types a command,
// reads the output, resizes the terminal, asks the shell for its size, exits
// with code 7 and checks that Wait reports 7. It prints PASS or FAIL with the
// time each step took. Standard library plus the repository's own packages;
// no `sh -c`.
//
//	go run ./capability-checks/cc-01-pty
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/alphatechini/multi-plex/internal/pty"
	"github.com/alphatechini/multi-plex/internal/pty/xptyimpl"
)

const stepTimeout = 20 * time.Second

var ansiRe = regexp.MustCompile(`\x1b(\[[0-9;?<>=]*[ -/]*[@-~]|\][^\x07\x1b]*(\x07|\x1b\\)|[@-Z\\-_])`)

// output collects everything the process prints.
type output struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (o *output) add(b []byte) {
	o.mu.Lock()
	o.buf.Write(b)
	o.mu.Unlock()
}

// lines returns the output so far as plain-text lines.
func (o *output) lines() []string {
	o.mu.Lock()
	s := o.buf.String()
	o.mu.Unlock()
	s = ansiRe.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.Split(s, "\n")
}

func (o *output) raw() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.buf.String()
}

// waitLine waits until some output line ends with want. A shell prompt may
// precede the text on the same line, so only the end is compared.
func (o *output) waitLine(want string) error {
	deadline := time.Now().Add(stepTimeout)
	for time.Now().Before(deadline) {
		for _, l := range o.lines() {
			if strings.HasSuffix(strings.TrimSpace(l), want) {
				return nil
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for a line %q; output so far: %q", want, o.raw())
}

type step struct {
	name string
	took time.Duration
	err  error
}

func main() {
	steps, err := run()
	failed := err != nil
	for _, s := range steps {
		status := "ok  "
		if s.err != nil {
			status = "FAIL"
			failed = true
		}
		fmt.Printf("  %s %-34s %8.1f ms", status, s.name, float64(s.took.Microseconds())/1000)
		if s.err != nil {
			fmt.Printf("  %v", s.err)
		}
		fmt.Println()
	}
	if failed {
		if err != nil {
			fmt.Println("error:", err)
		}
		fmt.Println("CC-01 FAIL")
		os.Exit(1)
	}
	fmt.Printf("CC-01 PASS (%s/%s)\n", runtime.GOOS, runtime.GOARCH)
}

// shellSpec returns the command for the platform shell.
func shellSpec() pty.Spec {
	if runtime.GOOS == "windows" {
		argv := []string{"pwsh.exe", "-NoProfile", "-NoLogo"}
		if _, err := exec.LookPath("pwsh.exe"); err != nil {
			argv[0] = "powershell.exe"
		}
		return pty.Spec{Argv: argv, Cols: 80, Rows: 24}
	}
	return pty.Spec{Argv: []string{"/bin/sh"}, Cols: 80, Rows: 24}
}

// echoCommand prints "hello" without the typed command containing that word,
// so the terminal's own echo of the keystrokes cannot be mistaken for output.
func echoCommand() string {
	if runtime.GOOS == "windows" {
		return "echo ('hel'+'lo')"
	}
	return `echo hel""lo`
}

func sizeCommand() string {
	if runtime.GOOS == "windows" {
		return `Write-Output ("{0} {1}" -f [Console]::WindowHeight, [Console]::WindowWidth)`
	}
	return "stty size"
}

func run() ([]step, error) {
	var steps []step
	record := func(name string, f func() error) error {
		t0 := time.Now()
		err := f()
		steps = append(steps, step{name, time.Since(t0), err})
		return err
	}

	var p pty.Process
	if err := record("spawn shell", func() error {
		var err error
		p, err = xptyimpl.New().Spawn(context.Background(), shellSpec())
		return err
	}); err != nil {
		return steps, nil
	}
	defer p.Close()

	out := &output{}
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		buf := make([]byte, 4096)
		for {
			n, err := p.Read(buf)
			if n > 0 {
				out.add(buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()

	send := func(s string) error { _, err := p.Write([]byte(s)); return err }

	if err := record("type echo, read hello back", func() error {
		if err := send(echoCommand() + "\r"); err != nil {
			return err
		}
		return out.waitLine("hello")
	}); err != nil {
		return steps, nil
	}

	_ = record("resize to 100x30, shell sees it", func() error {
		if err := p.Resize(100, 30); err != nil {
			return err
		}
		if err := send(sizeCommand() + "\r"); err != nil {
			return err
		}
		return out.waitLine("30 100")
	})

	_ = record("exit 7, Wait returns 7", func() error {
		if err := send("exit 7\r"); err != nil {
			return err
		}
		type result struct {
			code int
			err  error
		}
		ch := make(chan result, 1)
		go func() { c, err := p.Wait(); ch <- result{c, err} }()
		select {
		case r := <-ch:
			if r.err != nil {
				return r.err
			}
			if r.code != 7 {
				return fmt.Errorf("exit code = %d, want 7", r.code)
			}
			return nil
		case <-time.After(stepTimeout):
			return fmt.Errorf("Wait did not return; output so far: %q", out.raw())
		}
	})

	_ = record("close ends the reader", func() error {
		// ConPTY keeps the output pipe open after the exit until the console
		// is closed; on Unix the reader has usually ended already.
		_ = p.Close()
		select {
		case <-readerDone:
			return nil
		case <-time.After(stepTimeout):
			return fmt.Errorf("reader goroutine still blocked after Close")
		}
	})
	return steps, nil
}
