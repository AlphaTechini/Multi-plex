package charmvt

import (
	"bytes"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/alphatechini/multi-plex/internal/vt"
)

func write(t testing.TB, em vt.Emulator, s string) {
	t.Helper()
	if _, err := em.Write([]byte(s)); err != nil {
		t.Fatalf("Write(%q): %v", s, err)
	}
}

// writeFuzz writes hostile input. x/vt v0.1.0 can panic on some sequences
// (for example a scroll region taller than the screen); Write recovers and
// reports an error, and the emulator must stay usable afterwards.
func writeFuzz(t testing.TB, em vt.Emulator, s string) {
	t.Helper()
	if _, err := em.Write([]byte(s)); err != nil && !strings.Contains(err.Error(), "emulator panic") {
		t.Fatalf("Write(%q): %v", s, err)
	}
}

func plain(em vt.Emulator, y int) string { return ansi.Strip(em.Line(y)) }

// safeBuf is a goroutine-safe bytes.Buffer.
type safeBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *safeBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *safeBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func TestLineColoursAndAttributes(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string // exact Line(0) minus the padding, which is checked separately
	}{
		{"plain", "hi", "hi"},
		{"basic fg", "\x1b[31mred\x1b[0m", "\x1b[0;31mred\x1b[m"},
		{"bright fg", "\x1b[91mred", "\x1b[0;91mred\x1b[m"},
		{"basic bg", "\x1b[44mx", "\x1b[0;44mx\x1b[m"},
		{"bright bg", "\x1b[104mx", "\x1b[0;104mx\x1b[m"},
		{"256 fg", "\x1b[38;5;200mx", "\x1b[0;38;5;200mx\x1b[m"},
		{"256 bg", "\x1b[48;5;20mx", "\x1b[0;48;5;20mx\x1b[m"},
		{"truecolor fg", "\x1b[38;2;255;128;0mx", "\x1b[0;38;2;255;128;0mx\x1b[m"},
		{"truecolor bg", "\x1b[48;2;1;2;3mx", "\x1b[0;48;2;1;2;3mx\x1b[m"},
		{"bold", "\x1b[1mx", "\x1b[0;1mx\x1b[m"},
		{"faint", "\x1b[2mx", "\x1b[0;2mx\x1b[m"},
		{"italic", "\x1b[3mx", "\x1b[0;3mx\x1b[m"},
		{"underline", "\x1b[4mx", "\x1b[0;4mx\x1b[m"},
		{"curly underline", "\x1b[4:3mx", "\x1b[0;4:3mx\x1b[m"},
		{"blink", "\x1b[5mx", "\x1b[0;5mx\x1b[m"},
		{"reverse", "\x1b[7mx", "\x1b[0;7mx\x1b[m"},
		{"strikethrough", "\x1b[9mx", "\x1b[0;9mx\x1b[m"},
		{"combined", "\x1b[1;3;32mx", "\x1b[0;1;3;32mx\x1b[m"},
		{"style change", "\x1b[31ma\x1b[32mb", "\x1b[0;31ma\x1b[0;32mb\x1b[m"},
		{"same style cached", "\x1b[31mab", "\x1b[0;31mab\x1b[m"},
		{"reset mid line", "\x1b[31ma\x1b[0mb", "\x1b[0;31ma\x1b[mb"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const cols = 12
			em := New(cols, 2)
			defer em.Close()
			write(t, em, tt.in)
			got := em.Line(0)
			if !strings.HasPrefix(got, tt.want) {
				t.Fatalf("Line(0) = %q, want prefix %q", got, tt.want)
			}
			if w := ansi.StringWidth(got); w != cols {
				t.Errorf("width = %d, want %d (%q)", w, cols, got)
			}
		})
	}
}

func TestLineEndsWithReset(t *testing.T) {
	em := New(5, 1)
	defer em.Close()
	write(t, em, "\x1b[41m")
	write(t, em, "ab")
	got := em.Line(0)
	// Cells after "ab" use the default pen: the line must not leave the style on.
	if strings.Count(got, "\x1b[m") < 1 || !strings.HasSuffix(got, "\x1b[m") && !strings.HasSuffix(got, " ") {
		t.Errorf("unexpected end of line: %q", got)
	}
	// A line that ends in a styled cell must end with a reset.
	em2 := New(3, 1)
	defer em2.Close()
	write(t, em2, "\x1b[41mabc")
	if got := em2.Line(0); !strings.HasSuffix(got, "\x1b[m") {
		t.Errorf("Line = %q, want trailing reset", got)
	}
}

func TestBlankLineIsShared(t *testing.T) {
	em := New(10, 3)
	defer em.Close()
	if got := em.Line(1); got != strings.Repeat(" ", 10) {
		t.Errorf("blank line = %q", got)
	}
	if em.Line(-1) != "" || em.Line(3) != "" {
		t.Error("out of range rows must return an empty string")
	}
}

func TestCursorMovement(t *testing.T) {
	em := New(20, 5)
	defer em.Close()
	write(t, em, "\x1b[3;5Hx")
	if got := plain(em, 2); !strings.HasPrefix(got, "    x") {
		t.Errorf("row 2 = %q", got)
	}
	c := em.Cursor()
	if c.X != 5 || c.Y != 2 || !c.Visible {
		t.Errorf("cursor = %+v, want {5 2 true}", c)
	}
	write(t, em, "\x1b[2A\x1b[3D")
	if c := em.Cursor(); c.X != 2 || c.Y != 0 {
		t.Errorf("after CUU/CUB cursor = %+v, want X=2 Y=0", c)
	}
	write(t, em, "\x1b[?25l")
	if em.Cursor().Visible {
		t.Error("cursor should be hidden after ?25l")
	}
	write(t, em, "\x1b[?25h")
	if !em.Cursor().Visible {
		t.Error("cursor should be visible after ?25h")
	}
	write(t, em, "\r\nab\bX")
	if got := plain(em, 1); !strings.HasPrefix(got, "aX") {
		t.Errorf("row 1 = %q", got)
	}
}

func TestWideCharacters(t *testing.T) {
	tests := []struct {
		name string
		in   string
		text string // expected plain prefix
	}{
		{"cjk", "世界", "世界"},
		{"cjk then ascii", "世a界", "世a界"},
		{"emoji", "😀x", "😀x"},
		{"mixed", "a世b😀c", "a世b😀c"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const cols = 10
			em := New(cols, 2)
			defer em.Close()
			write(t, em, tt.in)
			line := em.Line(0)
			if w := ansi.StringWidth(line); w != cols {
				t.Fatalf("width = %d, want %d: %q", w, cols, line)
			}
			if got := ansi.Strip(line); !strings.HasPrefix(got, tt.text) {
				t.Errorf("text = %q, want prefix %q", got, tt.text)
			}
			if c := em.Cursor(); c.X != ansi.StringWidth(tt.in) {
				t.Errorf("cursor x = %d, want %d", c.X, ansi.StringWidth(tt.in))
			}
		})
	}
}

// TestWideCharacterAtRightEdge only checks the width contract. x/vt v0.1.0
// mishandles wide characters at the right margin (a wide character that ends
// exactly in the last column does not set the pending-wrap state, so the next
// character overwrites it; one that does not fit is dropped instead of
// wrapped). That is a library bug tracked for CC-02, not something Line can
// repair, but Line must stay exactly cols wide whatever the library leaves.
func TestWideCharacterAtRightEdge(t *testing.T) {
	for _, in := range []string{"abcd世界", "abc世界", "abcde世界", "世世世", "abcd😀a"} {
		em := New(5, 3)
		write(t, em, in)
		for y := range 3 {
			if w := ansi.StringWidth(em.Line(y)); w != 5 {
				t.Errorf("%q: row %d width = %d: %q", in, y, w, em.Line(y))
			}
		}
		em.Close()
	}
}

func TestClearScreen(t *testing.T) {
	em := New(10, 3)
	defer em.Close()
	write(t, em, "one\r\ntwo\r\nthree")
	write(t, em, "\x1b[2J\x1b[H")
	for y := range 3 {
		if got := em.Line(y); strings.TrimSpace(got) != "" {
			t.Errorf("row %d not cleared: %q", y, got)
		}
	}
	if c := em.Cursor(); c.X != 0 || c.Y != 0 {
		t.Errorf("cursor = %+v after home", c)
	}
}

func TestScrollingKeepsLastRows(t *testing.T) {
	em := New(10, 3)
	defer em.Close()
	write(t, em, "1\r\n2\r\n3\r\n4\r\n5")
	want := []string{"3", "4", "5"}
	for y, w := range want {
		if got := strings.TrimSpace(plain(em, y)); got != w {
			t.Errorf("row %d = %q, want %q", y, got, w)
		}
	}
}

func TestAltScreen(t *testing.T) {
	em := New(10, 3)
	defer em.Close()
	write(t, em, "main")
	if em.AltScreen() {
		t.Fatal("alt screen active at start")
	}
	write(t, em, "\x1b[?1049h")
	if !em.AltScreen() {
		t.Fatal("alt screen not detected after ?1049h")
	}
	if got := strings.TrimSpace(plain(em, 0)); got != "" {
		t.Errorf("alt screen row 0 = %q, want blank", got)
	}
	write(t, em, "\x1b[Halt")
	write(t, em, "\x1b[?1049l")
	if em.AltScreen() {
		t.Fatal("alt screen still active after ?1049l")
	}
	if got := strings.TrimSpace(plain(em, 0)); got != "main" {
		t.Errorf("main screen row 0 = %q, want main", got)
	}
}

func TestResize(t *testing.T) {
	em := New(10, 3)
	defer em.Close()
	write(t, em, "hello")
	em.Resize(20, 6)
	if c, r := em.Size(); c != 20 || r != 6 {
		t.Fatalf("Size = %d,%d", c, r)
	}
	for y := range 6 {
		if w := ansi.StringWidth(em.Line(y)); w != 20 {
			t.Errorf("row %d width = %d, want 20", y, w)
		}
	}
	if got := strings.TrimSpace(plain(em, 0)); got != "hello" {
		t.Errorf("row 0 = %q", got)
	}
	em.Resize(4, 2)
	for y := range 2 {
		if w := ansi.StringWidth(em.Line(y)); w != 4 {
			t.Errorf("row %d width = %d, want 4", y, w)
		}
	}
	if em.Line(2) != "" {
		t.Error("row 2 should be out of range after shrinking")
	}
	em.Resize(0, 0) // clamped, must not panic
	if c, r := em.Size(); c != 1 || r != 1 {
		t.Errorf("Size after Resize(0,0) = %d,%d, want 1,1", c, r)
	}
	write(t, em, "xyz\r\nabc")
}

// TestPanicRecovery: a scroll region taller than the screen makes x/vt panic
// on the next scroll. Write must report it, repair the emulator, and carry on.
func TestPanicRecovery(t *testing.T) {
	em := New(10, 3)
	defer em.Close()
	if _, err := em.Write([]byte("\x1b[1;5r\n\n\n\n\n")); err == nil {
		t.Skip("x/vt no longer panics here; recovery is untested")
	}
	write(t, em, "\x1b[Habc\r\nd\r\ne\r\nf")
	if got := strings.TrimSpace(plain(em, 2)); got != "f" {
		t.Errorf("row 2 = %q after recovery, want f", got)
	}
	checkInvariants(t, em, "recovery")
}

func TestTitle(t *testing.T) {
	em := New(10, 2)
	defer em.Close()
	if em.Title() != "" {
		t.Errorf("initial title %q", em.Title())
	}
	write(t, em, "\x1b]0;first\x07")
	if em.Title() != "first" {
		t.Errorf("OSC 0 title = %q", em.Title())
	}
	write(t, em, "\x1b]2;second\x1b\\")
	if em.Title() != "second" {
		t.Errorf("OSC 2 title = %q", em.Title())
	}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", what)
}

func TestRepliesReachWriter(t *testing.T) {
	em := New(20, 5)
	defer em.Close()
	var out safeBuf
	em.SetReplyWriter(&out)

	write(t, em, "\x1b[c")
	waitFor(t, "DA1 reply", func() bool { return strings.Contains(out.String(), "\x1b[?62;") })

	write(t, em, "ab\x1b[6n")
	waitFor(t, "CPR reply", func() bool { return strings.Contains(out.String(), "\x1b[1;3R") })

	write(t, em, "\x1b[?u")
	waitFor(t, "kitty query reply", func() bool { return strings.Contains(out.String(), "\x1b[?0u") })
}

// blockedWriter never returns from Write until released.
type blockedWriter struct{ release chan struct{} }

func (b blockedWriter) Write(p []byte) (int, error) { <-b.release; return len(p), nil }

func TestStuckReplyWriterDoesNotBlockWrite(t *testing.T) {
	em := New(20, 5)
	bw := blockedWriter{release: make(chan struct{})}
	em.SetReplyWriter(bw)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 200 {
			if _, err := em.Write([]byte("\x1b[c\x1b[6n\x1b[?2026$p text\r\n")); err != nil {
				t.Errorf("Write: %v", err)
				return
			}
		}
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Write blocked behind a stuck reply writer")
	}
	close(bw.release)
	if err := em.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

func TestNoReplyWriterDoesNotBlock(t *testing.T) {
	em := New(20, 5)
	defer em.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		write(t, em, "\x1b[c\x1b[6n")
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Write blocked with no reply writer")
	}
}

func TestCloseIdempotentAndWriteAfterClose(t *testing.T) {
	em := New(10, 2)
	if err := em.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := em.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if _, err := em.Write([]byte("x")); err == nil {
		t.Error("Write after Close should fail")
	}
	if em.Line(0) != "" {
		t.Error("Line after Close should be empty")
	}
}

func TestTakeDirty(t *testing.T) {
	em := New(10, 5).(*emulator)
	defer em.Close()
	em.TakeDirty(nil) // clear whatever initialisation touched
	if got := em.TakeDirty(nil); len(got) != 0 {
		t.Fatalf("expected no dirty rows, got %v", got)
	}
	write(t, em, "\x1b[2;1Hx\x1b[4;1Hy")
	got := em.TakeDirty(nil)
	if len(got) != 2 || got[0] != 1 || got[1] != 3 {
		t.Errorf("dirty = %v, want [1 3]", got)
	}
	if got := em.TakeDirty(nil); len(got) != 0 {
		t.Errorf("marks not cleared: %v", got)
	}
}

// TestTakeDirtyCoversChanges checks that every row whose rendering changed is
// reported dirty, which is what lets a host skip untouched rows.
func TestTakeDirtyCoversChanges(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	em := New(30, 10).(*emulator)
	defer em.Close()
	prev := make([]string, 10)
	for i := range prev {
		prev[i] = em.Line(i)
	}
	em.TakeDirty(nil)
	for iter := range 3000 {
		writeFuzz(t, em, randomInput(rng, 1+rng.Intn(40)))
		if iter%500 == 499 {
			em.Resize(20+rng.Intn(20), 8+rng.Intn(5))
			prev = prev[:0]
			_, r := em.Size()
			for i := 0; i < r; i++ {
				prev = append(prev, em.Line(i))
			}
			em.TakeDirty(nil)
			continue
		}
		alt := em.AltScreen()
		dirty := map[int]bool{}
		for _, y := range em.TakeDirty(nil) {
			dirty[y] = true
		}
		for i := range prev {
			now := em.Line(i)
			if now != prev[i] && !dirty[i] && !alt {
				t.Fatalf("iter %d: row %d changed but was not reported dirty:\n was %q\n now %q", iter, i, prev[i], now)
			}
			prev[i] = now
		}
		if alt {
			writeFuzz(t, em, "\x1b[?1049l")
			em.TakeDirty(nil)
			for i := range prev {
				prev[i] = em.Line(i)
			}
		}
	}
}

func TestConcurrentWriteAndLine(t *testing.T) {
	em := New(40, 10)
	defer em.Close()
	var wg sync.WaitGroup
	stop := make(chan struct{})
	wg.Add(2)
	go func() {
		defer wg.Done()
		rng := rand.New(rand.NewSource(1))
		for i := 0; i < 2000; i++ {
			_, _ = em.Write([]byte(randomInput(rng, 30)))
		}
		close(stop)
	}()
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			for y := 0; y < 10; y++ {
				_ = em.Line(y)
			}
			_ = em.Cursor()
			_ = em.Title()
			_ = em.AltScreen()
		}
	}()
	wg.Wait()
}

// randomInput builds a hostile-ish terminal stream from fragments.
func randomInput(rng *rand.Rand, n int) string {
	frags := []string{
		"a", "Z", " ", "hello", "世", "界", "😀", "é", "é", "👨‍👩‍👧", "🇯🇵", "‍", "️",
		"\r", "\n", "\t", "\b", "\x07", "\x1b[m", "\x1b[0m", "\x1b[1m", "\x1b[3m", "\x1b[4m", "\x1b[7m", "\x1b[9m",
		"\x1b[31m", "\x1b[42m", "\x1b[91m", "\x1b[38;5;123m", "\x1b[48;5;7m", "\x1b[38;2;10;20;30m", "\x1b[48;2;200;100;50m",
		"\x1b[2J", "\x1b[H", "\x1b[K", "\x1b[1K", "\x1b[2K", "\x1b[3;7H", "\x1b[10;40H", "\x1b[A", "\x1b[B", "\x1b[C", "\x1b[D",
		"\x1b[5@", "\x1b[3P", "\x1b[2L", "\x1b[2M", "\x1b[3X", "\x1b[2S", "\x1b[2T", "\x1b[?1049h", "\x1b[?1049l", "\x1b[?25l",
		"\x1b[?25h", "\x1b[1;5r", "\x1b[r", "\x1bM", "\x1bD", "\x1bE", "\x1b7", "\x1b8", "\x1b]0;title\x07", "\x1b]2;t\x1b\\",
		"\x1b(0lqk\x1b(B", "\x1b[c", "\x1b[6n", "\x1b[?2004h", "\x1b[4h", "\x1b[4l", "\x1b[20h", "\x1b[?7l", "\x1b[?7h",
		"\x1b[4:3m", "\x1b[58;5;9m", "\x1b]8;;http://x\x07link\x1b]8;;\x07", "\x1bPq#0;2;0;0;0\x1b\\", "\x00", "\x7f", "\x9b31m", "\xff\xfe",
	}
	var sb strings.Builder
	for i := 0; i < n; i++ {
		sb.WriteString(frags[rng.Intn(len(frags))])
	}
	return sb.String()
}

// checkInvariants verifies the Line contract on every row.
func checkInvariants(t testing.TB, em vt.Emulator, input string) {
	t.Helper()
	cols, rows := em.Size()
	for y := 0; y < rows; y++ {
		line := em.Line(y)
		if w := ansi.StringWidth(line); w != cols {
			t.Fatalf("row %d width %d != cols %d after %q: %q", y, w, cols, input, line)
		}
		if strings.ContainsAny(ansi.Strip(line), "\x1b\x07\r\n\x00") {
			t.Fatalf("row %d contains control characters after %q: %q", y, input, line)
		}
	}
}

func TestLineWidthInvariantRandom(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	for round := 0; round < 40; round++ {
		cols, rows := 1+rng.Intn(40), 1+rng.Intn(12)
		em := New(cols, rows)
		for i := 0; i < 60; i++ {
			in := randomInput(rng, 1+rng.Intn(30))
			writeFuzz(t, em, in)
			checkInvariants(t, em, in)
			if rng.Intn(15) == 0 {
				em.Resize(1+rng.Intn(40), 1+rng.Intn(12))
				checkInvariants(t, em, "resize")
			}
		}
		em.Close()
	}
}

func FuzzLineWidth(f *testing.F) {
	for _, s := range []string{
		"", "plain text", "\x1b[31mred\x1b[0m", "世界😀", "a世b😀c", "\x1b[2J\x1b[H", "\x1b[?1049hhi\x1b[?1049l",
		"\x1b[38;2;1;2;3mx", "é👨‍👩‍👧🇯🇵", "\x1b[5@\x1b[3P\x1b[2L", "abcdefghijklmnopqrstuvwxyz0123456789",
		"\x1b[1;3r\n\n\n\n\x1bM\x1bM", "\x1b]0;t\x07", "\xff\xfe\x9b",
	} {
		f.Add(s, uint8(10), uint8(4))
	}
	f.Fuzz(func(t *testing.T, input string, c, r uint8) {
		cols, rows := 1+int(c)%40, 1+int(r)%12
		em := New(cols, rows)
		defer em.Close()
		if _, err := em.Write([]byte(input)); err != nil {
			t.Skip() // an emulator panic is reported as an error
		}
		checkInvariants(t, em, input)
		em.Resize(rows+3, cols+1)
		checkInvariants(t, em, input)
	})
}

func BenchmarkLine(b *testing.B) {
	em := New(120, 40)
	defer em.Close()
	for y := 0; y < 40; y++ {
		write(b, em, "\x1b[32mfunc\x1b[0m main() { fmt.Println(\x1b[33m\"hello world\"\x1b[0m) } // some comment 世界\r\n")
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for y := 0; y < 40; y++ {
			_ = em.Line(y)
		}
	}
}
