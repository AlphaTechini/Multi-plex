// Command cc-02-vt records a program's PTY output to a file and replays a
// recording through Multiplex's VT emulator (internal/vt/charmvt), printing
// the final screen. It is the tool for CC-02 (docs/13): which emulator
// renders agent TUIs correctly.
//
//	go run ./capability-checks/cc-02-vt record -o out.bin [-size 100x30] [-send '"\x1b"' -send-after 1s] -- <cmd> [args...]
//	go run ./capability-checks/cc-02-vt replay [-size 100x30] [-ansi] out.bin
//
// A recording is the raw bytes the program wrote to the terminal. While
// recording, an emulator answers the program's terminal queries (device
// attributes, cursor position) so full-screen programs start normally.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/alphatechini/multi-plex/internal/pty"
	"github.com/alphatechini/multi-plex/internal/pty/xptyimpl"
	"github.com/alphatechini/multi-plex/internal/term"
	"github.com/alphatechini/multi-plex/internal/vt/charmvt"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch os.Args[1] {
	case "record":
		err = record(os.Args[2:])
	case "replay":
		err = replay(os.Args[2:])
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage:\n  cc-02-vt record -o out.bin [-size COLSxROWS] [-send QUOTED -send-after DUR] [-timeout DUR] -- cmd [args...]\n  cc-02-vt replay [-size COLSxROWS] [-ansi] out.bin")
	os.Exit(2)
}

// parseSize parses "100x30".
func parseSize(s string) (cols, rows int, err error) {
	c, r, ok := strings.Cut(s, "x")
	if ok {
		cols, err = strconv.Atoi(c)
		if err == nil {
			rows, err = strconv.Atoi(r)
		}
	}
	if !ok || err != nil || cols < 1 || rows < 1 {
		return 0, 0, fmt.Errorf("bad size %q, want COLSxROWS", s)
	}
	return cols, rows, nil
}

func record(args []string) error {
	fs := flag.NewFlagSet("record", flag.ExitOnError)
	out := fs.String("o", "", "output file (required)")
	size := fs.String("size", "100x30", "terminal size, COLSxROWS")
	send := fs.String("send", "", `input to type once, as a Go quoted string, e.g. '":q!\r"'`)
	sendAfter := fs.Duration("send-after", time.Second, "delay before -send")
	timeout := fs.Duration("timeout", 30*time.Second, "kill the program after this long")
	_ = fs.Parse(args)
	cmd := fs.Args()
	if *out == "" || len(cmd) == 0 {
		usage()
	}
	cols, rows, err := parseSize(*size)
	if err != nil {
		return err
	}
	var input []byte
	if *send != "" {
		s, err := strconv.Unquote(*send)
		if err != nil {
			return fmt.Errorf("-send must be a Go quoted string: %w", err)
		}
		input = []byte(s)
	}

	f, err := os.Create(*out)
	if err != nil {
		return err
	}
	defer f.Close()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	p, err := xptyimpl.New().Spawn(ctx, pty.Spec{Argv: cmd, Cols: cols, Rows: rows})
	if err != nil {
		return err
	}
	defer p.Close()

	// An emulator that only answers queries, so programs that ask the
	// terminal who it is do not wait.
	emu := charmvt.New(cols, rows)
	defer emu.Close()
	emu.SetReplyWriter(p)

	if len(input) > 0 {
		time.AfterFunc(*sendAfter, func() { _, _ = p.Write(input) })
	}

	readDone := make(chan int64, 1)
	go func() {
		var n int64
		buf := make([]byte, 32<<10)
		for {
			m, err := p.Read(buf)
			if m > 0 {
				_, _ = f.Write(buf[:m])
				_, _ = emu.Write(buf[:m])
				n += int64(m)
			}
			if err != nil {
				readDone <- n
				return
			}
		}
	}()

	code, err := p.Wait()
	if err != nil {
		return err
	}
	// Let the last output drain; ConPTY needs the console closed to end.
	var n int64
	select {
	case n = <-readDone:
	case <-time.After(300 * time.Millisecond):
		_ = p.Close()
		n = <-readDone
	}
	fmt.Printf("recorded %d bytes to %s (exit code %d, %dx%d)\n", n, *out, code, cols, rows)
	return nil
}

func replay(args []string) error {
	fs := flag.NewFlagSet("replay", flag.ExitOnError)
	size := fs.String("size", "100x30", "terminal size, COLSxROWS (use the size it was recorded at)")
	useANSI := fs.Bool("ansi", false, "print the screen with its colours")
	_ = fs.Parse(args)
	if fs.NArg() != 1 {
		usage()
	}
	cols, rows, err := parseSize(*size)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(fs.Arg(0))
	if err != nil {
		return err
	}

	emu := charmvt.New(cols, rows)
	defer emu.Close()
	t0 := time.Now()
	frames, maxFrame, p50, p99 := term.Replay(bytes.NewReader(data), emu, cols, rows)
	elapsed := time.Since(t0)

	bad := 0
	fmt.Printf("+%s+\n", strings.Repeat("-", cols))
	for y := 0; y < rows; y++ {
		line := emu.Line(y)
		if ansi.StringWidth(line) != cols {
			bad++
		}
		if *useANSI {
			fmt.Printf("|%s|\n", line)
		} else {
			fmt.Printf("|%s|\n", ansi.Strip(line))
		}
	}
	fmt.Printf("+%s+\n", strings.Repeat("-", cols))
	cur := emu.Cursor()
	fmt.Printf("cursor (%d,%d) visible=%v  alt screen=%v  title=%q\n", cur.X, cur.Y, cur.Visible, emu.AltScreen(), emu.Title())
	mb := float64(len(data)) / (1 << 20)
	fmt.Printf("%d bytes in %v (%.2f MB/s), %d frames, frame cost p50 %v p99 %v max %v\n",
		len(data), elapsed.Round(time.Microsecond), mb/elapsed.Seconds(), frames, p50, p99, maxFrame)
	if bad > 0 {
		return fmt.Errorf("%d rows were not exactly %d cells wide", bad, cols)
	}
	return nil
}
