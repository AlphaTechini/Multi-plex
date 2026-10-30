package bench

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/alphatechini/multi-plex/internal/term"
	"github.com/alphatechini/multi-plex/internal/vt/charmvt"
)

// renderOpts are the flags of `mpx bench render`.
type renderOpts struct {
	mb   int
	file string
	cols int
	rows int
}

func renderFlags(fs *flag.FlagSet) *renderOpts {
	o := &renderOpts{}
	fs.IntVar(&o.mb, "mb", 10, "size of the generated stream in MiB")
	fs.StringVar(&o.file, "file", "", "replay this recorded PTY stream instead of a generated one")
	fs.IntVar(&o.cols, "cols", 120, "terminal width")
	fs.IntVar(&o.rows, "rows", 40, "terminal height")
	return o
}

// runRender replays a terminal stream through the emulator the way a pane
// host does, and reports throughput and per-frame cost (docs/03: keep up with
// a `cat` of a 10 MB log).
func runRender(o *renderOpts) (*Result, error) {
	var data []byte
	if o.file != "" {
		b, err := os.ReadFile(o.file)
		if err != nil {
			return nil, err
		}
		data = b
	} else {
		data = generateLog(o.mb << 20)
	}
	emu := charmvt.New(o.cols, o.rows)
	defer emu.Close()

	t0 := time.Now()
	frames, maxFrame, p50, p99 := term.Replay(bytes.NewReader(data), emu, o.cols, o.rows)
	total := time.Since(t0)

	res := newResult("render")
	res.note("stream: %s, %d KiB, %dx%d; frame budget is one 16 ms window", streamName(o), len(data)>>10, o.cols, o.rows)
	res.addValue("total time", ms(total), "ms")
	res.addValue("throughput", float64(len(data))/(1<<20)/total.Seconds(), "MiB/s")
	res.addValue("frames", float64(frames), "")
	res.addValue("frame cost p50", ms(p50), "ms")
	res.addValue("frame cost p99", ms(p99), "ms")
	res.addValue("frame cost max", ms(maxFrame), "ms")
	return res, nil
}

func streamName(o *renderOpts) string {
	if o.file != "" {
		return o.file
	}
	return "generated colored log, " + strconv.Itoa(o.mb) + " MiB"
}

// generateLog builds about n bytes of build-log-like terminal output with
// colors, long lines and the odd cursor movement.
func generateLog(n int) []byte {
	var b bytes.Buffer
	b.Grow(n + 256)
	levels := []struct{ name, sgr string }{{"INFO", "32"}, {"WARN", "33"}, {"ERROR", "1;31"}, {"DEBUG", "36"}}
	for i := 0; b.Len() < n; i++ {
		lv := levels[i%len(levels)]
		fmt.Fprintf(&b, "\x1b[90m2026-10-30T12:%02d:%02d.%03dZ\x1b[0m \x1b[%sm%-5s\x1b[0m internal/pkg%d/file%d.go:%d: processed %d items in %dms (cache hit ratio %.2f)\r\n",
			(i/60)%60, i%60, i%1000, lv.sgr, lv.name, i%37, i%11, 10+i%900, i%5000, i%250, float64(i%100)/100)
		if i%500 == 499 {
			io.WriteString(&b, "\x1b[2K\rprogress: "+strconv.Itoa(i/500)+"%\r\n")
		}
	}
	return b.Bytes()
}
