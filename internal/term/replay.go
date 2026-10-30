package term

import (
	"io"
	"slices"
	"time"

	"github.com/alphatechini/multi-plex/internal/vt"
)

// ReplayFrameBytes is how much recorded output Replay treats as arriving
// within one 16 ms frame window. 64 KiB is 4 MB/s, a busy program such as a
// build log; a plain `cat` of a big file is faster, so the frames it would
// really produce are fewer and each larger.
var ReplayFrameBytes = 64 << 10

// Replay feeds a recorded PTY byte stream through emu at the given size,
// taking one frame per ReplayFrameBytes (the coalescing a Host does per 16 ms)
// and measuring what each frame costs: the time to find the changed rows and
// render them. Writing into the emulator is not part of a frame's cost. It
// backs `mpx bench render`.
func Replay(r io.Reader, emu vt.Emulator, cols, rows int) (frames int, maxFrame, p50, p99 time.Duration) {
	emu.Resize(cols, rows)
	fr := newFramer("replay", emu)

	var costs []time.Duration
	frame := func() {
		t0 := time.Now()
		_, ok := fr.next()
		d := time.Since(t0)
		if ok {
			costs = append(costs, d)
		}
	}

	buf := make([]byte, readChunk)
	pending := 0
	for {
		n, err := r.Read(buf)
		if n > 0 {
			_, _ = emu.Write(buf[:n])
			pending += n
			if pending >= ReplayFrameBytes {
				frame()
				pending = 0
			}
		}
		if err != nil {
			break
		}
	}
	if pending > 0 {
		frame()
	}

	if len(costs) == 0 {
		return 0, 0, 0, 0
	}
	slices.Sort(costs)
	return len(costs), costs[len(costs)-1], percentile(costs, 50), percentile(costs, 99)
}

// percentile returns the p-th percentile of sorted durations (nearest rank).
func percentile(sorted []time.Duration, p int) time.Duration {
	i := (len(sorted)*p + 99) / 100
	if i < 1 {
		i = 1
	}
	return sorted[min(i, len(sorted))-1]
}
