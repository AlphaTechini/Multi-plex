package term

import (
	"bytes"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/alphatechini/multi-plex/internal/vt/charmvt"
)

// syntheticLog builds about size bytes of build-log-like output: coloured
// prefixes, varying text, the occasional wide character and cursor movement.
func syntheticLog(size int) []byte {
	var buf bytes.Buffer
	buf.Grow(size + 256)
	for i := 0; buf.Len() < size; i++ {
		switch i % 7 {
		case 0:
			fmt.Fprintf(&buf, "\x1b[32m[ok]\x1b[0m   pkg/module%d/file_%d.go compiled in %dms\r\n", i%50, i, i%900)
		case 1:
			fmt.Fprintf(&buf, "\x1b[33m[warn]\x1b[0m line %d: unused variable \x1b[1mx%d\x1b[0m 警告\r\n", i, i%13)
		case 2:
			fmt.Fprintf(&buf, "\x1b[38;5;%dmstep %d\x1b[0m: %s\r\n", i%256, i, "the quick brown fox jumps over the lazy dog")
		case 3:
			fmt.Fprintf(&buf, "\x1b[38;2;%d;%d;%dmtruecolor row %d\x1b[0m\r\n", i%256, (i*3)%256, (i*7)%256, i)
		case 4:
			fmt.Fprintf(&buf, "progress %3d%%\r", i%100)
		default:
			fmt.Fprintf(&buf, "plain output line number %d with some padding to make it longer ........\r\n", i)
		}
	}
	return buf.Bytes()
}

func TestReplay(t *testing.T) {
	log := syntheticLog(512 << 10)
	em := charmvt.New(80, 24)
	defer em.Close()
	frames, maxFrame, p50, p99 := Replay(bytes.NewReader(log), em, 100, 30)
	if frames < 5 {
		t.Fatalf("frames = %d, want many for a %d byte log", frames, len(log))
	}
	if !(p50 > 0 && p50 <= p99 && p99 <= maxFrame) {
		t.Errorf("percentiles out of order: p50=%v p99=%v max=%v", p50, p99, maxFrame)
	}
	if c, r := em.Size(); c != 100 || r != 30 {
		t.Errorf("emulator size = %dx%d, want 100x30", c, r)
	}
	t.Logf("%d frames, p50 %v, p99 %v, max %v", frames, p50, p99, maxFrame)
}

func TestReplayEmpty(t *testing.T) {
	em := charmvt.New(10, 3)
	defer em.Close()
	if f, _, _, _ := Replay(bytes.NewReader(nil), em, 10, 3); f != 0 {
		t.Errorf("empty stream produced %d frames", f)
	}
}

func TestPercentile(t *testing.T) {
	d := func(n ...int) []time.Duration {
		out := make([]time.Duration, len(n))
		for i, v := range n {
			out[i] = time.Duration(v)
		}
		return out
	}
	tests := []struct {
		in   []time.Duration
		p    int
		want time.Duration
	}{
		{d(5), 50, 5}, {d(5), 99, 5},
		{d(1, 2, 3, 4), 50, 2}, {d(1, 2, 3, 4), 99, 4},
		{d(1, 2, 3, 4, 5, 6, 7, 8, 9, 10), 50, 5}, {d(1, 2, 3, 4, 5, 6, 7, 8, 9, 10), 99, 10},
	}
	for _, tt := range tests {
		if got := percentile(tt.in, tt.p); got != tt.want {
			t.Errorf("percentile(%v, %d) = %v, want %v", tt.in, tt.p, got, tt.want)
		}
	}
}

// BenchmarkReplay10MB feeds a synthetic 10 MB log through the emulator and
// frame builder and reports throughput (MB/s) plus frame cost percentiles.
func BenchmarkReplay10MB(b *testing.B) {
	log := syntheticLog(10 << 20)
	b.SetBytes(int64(len(log)))
	b.ResetTimer()
	var frames int
	var maxF, p50, p99 time.Duration
	for i := 0; i < b.N; i++ {
		em := charmvt.New(120, 40)
		frames, maxF, p50, p99 = Replay(bytes.NewReader(log), em, 120, 40)
		em.Close()
	}
	b.ReportMetric(float64(frames), "frames")
	b.ReportMetric(float64(p50.Microseconds()), "p50-µs")
	b.ReportMetric(float64(p99.Microseconds()), "p99-µs")
	b.ReportMetric(float64(maxF.Microseconds()), "max-µs")
}

// BenchmarkEmulatorWrite10MB isolates the emulator's parse speed.
func BenchmarkEmulatorWrite10MB(b *testing.B) {
	log := syntheticLog(10 << 20)
	b.SetBytes(int64(len(log)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		em := charmvt.New(120, 40)
		r := bytes.NewReader(log)
		buf := make([]byte, readChunk)
		for {
			n, err := r.Read(buf)
			_, _ = em.Write(buf[:n])
			if err == io.EOF {
				break
			}
		}
		em.Close()
	}
}
