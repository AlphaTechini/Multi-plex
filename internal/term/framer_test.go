package term

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/alphatechini/multi-plex/internal/vt"
	"github.com/alphatechini/multi-plex/internal/vt/charmvt"
)

// noDirty hides an emulator's TakeDirty so the framer compares every row.
type noDirty struct{ vt.Emulator }

// TestFramerDirtyTrackingMatchesFullCompare drives two framers over the same
// random output, one using damage marks and one comparing every row, and
// requires that the screens rebuilt from their frames are identical.
func TestFramerDirtyTrackingMatchesFullCompare(t *testing.T) {
	const cols, rows = 40, 12
	fast, slow := charmvt.New(cols, rows), charmvt.New(cols, rows)
	defer fast.Close()
	defer slow.Close()
	ff, sf := newFramer("a", fast), newFramer("a", slow)
	ff.rep = fast.(dirtyReporter)
	sf = newFramer("a", noDirty{slow})
	if sf.rep != nil {
		t.Fatal("noDirty must hide TakeDirty")
	}

	rng := rand.New(rand.NewSource(99))
	frags := []string{"hello ", "世界", "\r\n", "\x1b[2J", "\x1b[H", "\x1b[31m", "\x1b[0m", "\x1b[5;3H", "\x1b[K", "x", "\x1b[2L", "\x1b[3M", "\x1b[?1049h", "\x1b[?1049l", "\x1bM", "\x1b[3S", "\x1b[2T"}
	screenF, screenS := make([]string, rows), make([]string, rows)
	apply := func(screen []string, f *framer) {
		if fr, ok := f.next(); ok {
			if fr.Full {
				for i := range screen {
					screen[i] = ""
				}
			}
			for _, r := range fr.Changed {
				screen[r.Y] = r.Text
			}
		}
	}
	for i := 0; i < 4000; i++ {
		var sb strings.Builder
		for j, n := 0, 1+rng.Intn(6); j < n; j++ {
			sb.WriteString(frags[rng.Intn(len(frags))])
		}
		in := []byte(sb.String())
		_, _ = fast.Write(in)
		_, _ = slow.Write(in)
		if rng.Intn(3) == 0 { // frames are not taken after every write
			apply(screenF, ff)
			apply(screenS, sf)
		}
		if rng.Intn(1) == 0 && i%97 == 0 {
			apply(screenF, ff)
			apply(screenS, sf)
			for y := 0; y < rows; y++ {
				if screenF[y] != screenS[y] || screenF[y] != fast.Line(y) {
					t.Fatalf("iter %d row %d: dirty-tracked %q, full %q, actual %q", i, y, screenF[y], screenS[y], fast.Line(y))
				}
			}
		}
	}
}

func TestFramerNoChangeNoFrame(t *testing.T) {
	em := charmvt.New(10, 3)
	defer em.Close()
	f := newFramer("p", em)
	if fr, ok := f.next(); !ok || !fr.Full {
		t.Fatal("first frame must be full")
	}
	if _, ok := f.next(); ok {
		t.Error("second frame with no change should be skipped")
	}
	_, _ = em.Write([]byte("x"))
	fr, ok := f.next()
	if !ok || fr.Full || len(fr.Changed) != 1 || fr.Changed[0].Y != 0 {
		t.Errorf("frame = %+v ok=%v", fr, ok)
	}
	f.forceFull()
	if fr, ok := f.next(); !ok || !fr.Full || len(fr.Changed) != 3 {
		t.Errorf("forced frame = %+v ok=%v", fr, ok)
	}
}

func TestFramerAltScreenSwitchIsFull(t *testing.T) {
	em := charmvt.New(10, 3)
	defer em.Close()
	f := newFramer("p", em)
	f.next()
	_, _ = em.Write([]byte("main"))
	f.next()
	_, _ = em.Write([]byte("\x1b[?1049h"))
	fr, ok := f.next()
	if !ok || !fr.Full {
		t.Errorf("alt screen switch frame = %+v ok=%v, want full", fr, ok)
	}
	_, _ = em.Write([]byte("\x1b[?1049l"))
	fr, ok = f.next()
	if !ok || !fr.Full || !strings.HasPrefix(fr.Changed[0].Text, "main") {
		t.Errorf("switch back frame = %+v ok=%v", fr, ok)
	}
}

func TestFramerResize(t *testing.T) {
	em := charmvt.New(10, 3)
	defer em.Close()
	f := newFramer("p", em)
	f.next()
	em.Resize(12, 5)
	fr, ok := f.next()
	if !ok || !fr.Full || fr.Cols != 12 || fr.Rows != 5 || len(fr.Changed) != 5 {
		t.Errorf("frame after resize = %+v ok=%v", fr, ok)
	}
}

func BenchmarkFramerScrolling(b *testing.B) {
	em := charmvt.New(120, 40)
	defer em.Close()
	f := newFramer("p", em)
	f.next()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = fmt.Fprintf(em, "\x1b[32mline %d\x1b[0m some log text that is moderately long, with numbers %d and more\r\n", i, i*7)
		f.next()
	}
}
