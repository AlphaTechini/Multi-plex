package keys

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	xvt "github.com/charmbracelet/x/vt"
)

func press(code rune, mod tea.KeyMod, text string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code, Mod: mod, Text: text}
}

const (
	shift = tea.ModShift
	alt   = tea.ModAlt
	ctrl  = tea.ModCtrl
)

var encodeTests = []struct {
	name string
	key  tea.KeyPressMsg
	want string
}{
	// Printable text.
	{"a", press('a', 0, "a"), "a"},
	{"A", press('a', shift, "A"), "A"},
	{"digit", press('1', 0, "1"), "1"},
	{"bang", press('1', shift, "!"), "!"},
	{"space", press(tea.KeySpace, 0, " "), " "},
	{"unicode", press('é', 0, "é"), "é"},
	{"cjk", press('世', 0, "世"), "世"},
	{"emoji text", press(tea.KeyExtended, 0, "😀"), "😀"},
	{"multi rune text", press(tea.KeyExtended, 0, "ab"), "ab"},

	// Basic specials.
	{"enter", press(tea.KeyEnter, 0, ""), "\r"},
	{"tab", press(tea.KeyTab, 0, ""), "\t"},
	{"shift+tab", press(tea.KeyTab, shift, ""), "\x1b[Z"},
	{"backspace", press(tea.KeyBackspace, 0, ""), "\x7f"},
	{"esc", press(tea.KeyEscape, 0, ""), "\x1b"},
	{"shift+backspace", press(tea.KeyBackspace, shift, ""), "\x7f"},
	{"ctrl+backspace", press(tea.KeyBackspace, ctrl, ""), "\x08"},
	{"alt+backspace", press(tea.KeyBackspace, alt, ""), "\x1b\x7f"},
	{"alt+esc", press(tea.KeyEscape, alt, ""), "\x1b\x1b"},
	{"alt+tab", press(tea.KeyTab, alt, ""), "\x1b\t"},
	{"alt+shift+tab", press(tea.KeyTab, alt|shift, ""), "\x1b\x1b[Z"},
	{"ctrl+tab has no encoding", press(tea.KeyTab, ctrl, ""), ""},

	// Enter with modifiers is "newline" for the agent CLIs.
	{"shift+enter", press(tea.KeyEnter, shift, ""), "\x1b\r"},
	{"ctrl+enter", press(tea.KeyEnter, ctrl, ""), "\x1b\r"},
	{"alt+enter", press(tea.KeyEnter, alt, ""), "\x1b\r"},
	{"keypad enter", press(tea.KeyKpEnter, 0, ""), "\r"},

	// Arrows and navigation.
	{"up", press(tea.KeyUp, 0, ""), "\x1b[A"},
	{"down", press(tea.KeyDown, 0, ""), "\x1b[B"},
	{"right", press(tea.KeyRight, 0, ""), "\x1b[C"},
	{"left", press(tea.KeyLeft, 0, ""), "\x1b[D"},
	{"home", press(tea.KeyHome, 0, ""), "\x1b[H"},
	{"end", press(tea.KeyEnd, 0, ""), "\x1b[F"},
	{"pgup", press(tea.KeyPgUp, 0, ""), "\x1b[5~"},
	{"pgdn", press(tea.KeyPgDown, 0, ""), "\x1b[6~"},
	{"insert", press(tea.KeyInsert, 0, ""), "\x1b[2~"},
	{"delete", press(tea.KeyDelete, 0, ""), "\x1b[3~"},
	{"ctrl+up", press(tea.KeyUp, ctrl, ""), "\x1b[1;5A"},
	{"shift+left", press(tea.KeyLeft, shift, ""), "\x1b[1;2D"},
	{"alt+right", press(tea.KeyRight, alt, ""), "\x1b[1;3C"},
	{"ctrl+shift+down", press(tea.KeyDown, ctrl|shift, ""), "\x1b[1;6B"},
	{"ctrl+alt+up", press(tea.KeyUp, ctrl|alt, ""), "\x1b[1;7A"},
	{"alt+home", press(tea.KeyHome, alt, ""), "\x1b[1;3H"},
	{"ctrl+end", press(tea.KeyEnd, ctrl, ""), "\x1b[1;5F"},
	{"ctrl+delete", press(tea.KeyDelete, ctrl, ""), "\x1b[3;5~"},
	{"shift+pgup", press(tea.KeyPgUp, shift, ""), "\x1b[5;2~"},
	{"alt+insert", press(tea.KeyInsert, alt, ""), "\x1b[2;3~"},
	{"meta+up", press(tea.KeyUp, tea.ModMeta, ""), "\x1b[1;9A"},

	// Function keys.
	{"f1", press(tea.KeyF1, 0, ""), "\x1bOP"},
	{"f2", press(tea.KeyF2, 0, ""), "\x1bOQ"},
	{"f3", press(tea.KeyF3, 0, ""), "\x1bOR"},
	{"f4", press(tea.KeyF4, 0, ""), "\x1bOS"},
	{"f5", press(tea.KeyF5, 0, ""), "\x1b[15~"},
	{"f6", press(tea.KeyF6, 0, ""), "\x1b[17~"},
	{"f7", press(tea.KeyF7, 0, ""), "\x1b[18~"},
	{"f8", press(tea.KeyF8, 0, ""), "\x1b[19~"},
	{"f9", press(tea.KeyF9, 0, ""), "\x1b[20~"},
	{"f10", press(tea.KeyF10, 0, ""), "\x1b[21~"},
	{"f11", press(tea.KeyF11, 0, ""), "\x1b[23~"},
	{"f12", press(tea.KeyF12, 0, ""), "\x1b[24~"},
	{"f13", press(tea.KeyF13, 0, ""), "\x1b[25~"},
	{"f20", press(tea.KeyF20, 0, ""), "\x1b[34~"},
	{"f21 unsupported", press(tea.KeyF21, 0, ""), ""},
	{"shift+f1", press(tea.KeyF1, shift, ""), "\x1b[1;2P"},
	{"ctrl+f4", press(tea.KeyF4, ctrl, ""), "\x1b[1;5S"},
	{"ctrl+f5", press(tea.KeyF5, ctrl, ""), "\x1b[15;5~"},
	{"alt+f12", press(tea.KeyF12, alt, ""), "\x1b[24;3~"},

	// Ctrl+letter.
	{"ctrl+a", press('a', ctrl, ""), "\x01"},
	{"ctrl+c", press('c', ctrl, ""), "\x03"},
	{"ctrl+d", press('d', ctrl, ""), "\x04"},
	{"ctrl+l", press('l', ctrl, ""), "\x0c"},
	{"ctrl+z", press('z', ctrl, ""), "\x1a"},
	{"ctrl+shift+a", press('a', ctrl|shift, ""), "\x01"},
	{"ctrl+uppercase A", press('A', ctrl, ""), "\x01"},
	{"ctrl+space", press(tea.KeySpace, ctrl, ""), "\x00"},
	{"ctrl+@", press('@', ctrl, ""), "\x00"},
	{"ctrl+[", press('[', ctrl, ""), "\x1b"},
	{"ctrl+\\", press('\\', ctrl, ""), "\x1c"},
	{"ctrl+]", press(']', ctrl, ""), "\x1d"},
	{"ctrl+^", press('^', ctrl, ""), "\x1e"},
	{"ctrl+_", press('_', ctrl, ""), "\x1f"},
	{"ctrl+/", press('/', ctrl, ""), "\x1f"},
	{"ctrl+?", press('?', ctrl, ""), "\x7f"},
	{"ctrl+unmapped", press('1', ctrl, ""), ""},

	// Alt+key is an ESC prefix.
	{"alt+a (no text)", press('a', alt, ""), "\x1ba"},
	{"alt+a (text)", press('a', alt, "a"), "\x1ba"},
	{"alt+A", press('a', alt|shift, ""), "\x1bA"},
	{"alt+shifted code", tea.KeyPressMsg{Code: '1', Mod: alt | shift, ShiftedCode: '!'}, "\x1b!"},
	{"alt+space", press(tea.KeySpace, alt, ""), "\x1b "},
	{"alt+é", press('é', alt, ""), "\x1bé"},
	{"ctrl+alt+a", press('a', ctrl|alt, ""), "\x1b\x01"},
	{"ctrl+alt+space", press(tea.KeySpace, ctrl|alt, ""), "\x1b\x00"},
	{"alt+.", press('.', alt, ""), "\x1b."},

	// Keypad (numeric mode).
	{"kp5", press(tea.KeyKp5, 0, ""), "5"},
	{"kp plus", press(tea.KeyKpPlus, 0, ""), "+"},
	{"kp decimal", press(tea.KeyKpDecimal, 0, ""), "."},
	{"kp divide", press(tea.KeyKpDivide, 0, ""), "/"},

	// No encoding.
	{"super+c", press('c', tea.ModSuper, ""), ""},
	{"hyper+a", press('a', tea.ModHyper, "a"), ""},
	{"caps lock key", press(tea.KeyCapsLock, 0, ""), ""},
	{"caps lock state ignored", press('a', tea.ModCapsLock, "A"), "A"},
	{"num lock state ignored", press(tea.KeyUp, tea.ModNumLock, ""), "\x1b[A"},
	{"zero key", tea.KeyPressMsg{}, ""},
}

func TestEncode(t *testing.T) {
	for _, tt := range encodeTests {
		t.Run(tt.name, func(t *testing.T) {
			got := Encode(tt.key)
			if string(got) != tt.want {
				t.Errorf("Encode(%q) = %q, want %q", tt.key.String(), got, tt.want)
			}
			if tt.want == "" && got != nil {
				t.Errorf("want nil for no encoding, got %q", got)
			}
		})
	}
}

func TestEncodeAppCursor(t *testing.T) {
	tests := []struct {
		name string
		key  tea.KeyPressMsg
		want string
	}{
		{"up", press(tea.KeyUp, 0, ""), "\x1bOA"},
		{"down", press(tea.KeyDown, 0, ""), "\x1bOB"},
		{"right", press(tea.KeyRight, 0, ""), "\x1bOC"},
		{"left", press(tea.KeyLeft, 0, ""), "\x1bOD"},
		{"home", press(tea.KeyHome, 0, ""), "\x1bOH"},
		{"end", press(tea.KeyEnd, 0, ""), "\x1bOF"},
		{"modified arrows stay CSI", press(tea.KeyUp, ctrl, ""), "\x1b[1;5A"},
		{"pgup unaffected", press(tea.KeyPgUp, 0, ""), "\x1b[5~"},
		{"text unaffected", press('x', 0, "x"), "x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EncodeApp(tt.key, true); string(got) != tt.want {
				t.Errorf("EncodeApp(%q, true) = %q, want %q", tt.key.String(), got, tt.want)
			}
		})
	}
}

// TestMatchesVTSendKey checks every unmodified (or Ctrl/Alt) case against the
// x/vt emulator's own SendKey, which is what a hosted program would get if we
// went through the emulator. The cases x/vt leaves out are covered by the
// table above.
func TestMatchesVTSendKey(t *testing.T) {
	compared := 0
	defer func() {
		if compared < 40 {
			t.Errorf("only %d keys were cross-checked against x/vt", compared)
		}
	}()
	for _, tt := range encodeTests {
		k := tt.key.Key()
		mods := k.Mod &^ (tea.ModCapsLock | tea.ModNumLock | tea.ModScrollLock)
		// SendKey writes nothing for modified specials and Enter, and has a
		// few quirks (documented in docs api-notes); compare only where it speaks.
		if mods&^(ctrl|alt) != 0 || tt.want == "" {
			continue
		}
		if k.Code == tea.KeyEnter || k.Code == tea.KeyBackspace || k.Code == tea.KeyEscape || k.Code == tea.KeyTab {
			if mods != 0 {
				continue
			}
		}
		if mods != 0 && k.Code >= tea.KeyExtended {
			continue
		}
		if k.Code == tea.KeyKp5 || k.Code == tea.KeyKpPlus || k.Code == tea.KeyKpDecimal || k.Code == tea.KeyKpDivide ||
			k.Code == tea.KeyF13 || k.Code == tea.KeyF20 || k.Code == tea.KeyKpEnter {
			continue // not in SendKey's table or differs by design
		}
		t.Run(tt.name, func(t *testing.T) {
			got := vtSend(t, uv.KeyPressEvent(tt.key))
			if got == "" {
				return // SendKey does not encode this key
			}
			compared++
			if got != tt.want {
				t.Errorf("x/vt SendKey = %q, Encode = %q", got, tt.want)
			}
		})
	}
}

func vtSend(t *testing.T, ev uv.KeyPressEvent) string {
	t.Helper()
	em := xvt.NewEmulator(10, 3)
	defer em.Close()
	out := make(chan string, 1)
	go func() {
		buf := make([]byte, 64)
		n, _ := em.Read(buf)
		out <- string(buf[:n])
	}()
	em.SendKey(ev)
	select {
	case s := <-out:
		return s
	case <-time.After(100 * time.Millisecond):
		return "" // nothing written
	}
}

// TestRoundTrip feeds the encoded bytes back through the decoder Bubble Tea
// uses and requires the same key (code and modifiers) to come out.
func TestRoundTrip(t *testing.T) {
	keys := []tea.KeyPressMsg{
		press('a', 0, "a"), press(tea.KeyEnter, 0, ""), press(tea.KeyTab, 0, ""), press(tea.KeyTab, shift, ""),
		press(tea.KeyBackspace, 0, ""), press(tea.KeyEscape, 0, ""),
		press(tea.KeyUp, 0, ""), press(tea.KeyDown, ctrl, ""), press(tea.KeyLeft, shift, ""), press(tea.KeyRight, alt, ""),
		press(tea.KeyHome, 0, ""), press(tea.KeyEnd, ctrl, ""), press(tea.KeyPgUp, 0, ""), press(tea.KeyPgDown, shift, ""),
		press(tea.KeyInsert, 0, ""), press(tea.KeyDelete, ctrl, ""),
		press(tea.KeyF1, 0, ""), press(tea.KeyF4, shift, ""), press(tea.KeyF5, 0, ""), press(tea.KeyF12, ctrl, ""),
		press('a', ctrl, ""), press('z', ctrl, ""), press('a', alt, ""), press('x', ctrl|alt, ""),
		press(tea.KeySpace, ctrl, ""),
	}
	for _, k := range keys {
		var d uv.EventDecoder
		b := Encode(k)
		n, ev := d.Decode(b)
		if n != len(b) {
			t.Errorf("%q: decoder consumed %d of %d bytes (%q)", k.String(), n, len(b), b)
			continue
		}
		kp, ok := ev.(uv.KeyPressEvent)
		if !ok {
			t.Errorf("%q: decoded as %T (%q)", k.String(), ev, b)
			continue
		}
		want := k.Key()
		if kp.Code != want.Code || kp.Mod&^ignoredMods != want.Mod {
			t.Errorf("%q -> %q -> code %q mod %v, want code %q mod %v", k.String(), b, kp.Code, kp.Mod, want.Code, want.Mod)
		}
	}
}

func TestEncodePaste(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		bracketed bool
		want      string
	}{
		{"bracketed", "hello", true, "\x1b[200~hello\x1b[201~"},
		{"bracketed multiline", "a\nb\r\nc", true, "\x1b[200~a\nb\r\nc\x1b[201~"},
		{"bracketed empty", "", true, "\x1b[200~\x1b[201~"},
		{"bracketed strips end marker", "x\x1b[201~rm -rf /\n", true, "\x1b[200~xrm -rf /\n\x1b[201~"},
		{"bracketed strips nested marker", "\x1b[20\x1b[201~1~y", true, "\x1b[200~y\x1b[201~"},
		{"plain", "hello", false, "hello"},
		{"plain newlines", "a\nb\r\nc\rd", false, "a\rb\rc\rd"},
		{"plain keeps tab and unicode", "a\t世", false, "a\t世"},
		{"plain drops controls", "a\x1b[31m\x00\x07b\x7f", false, "a[31mb"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EncodePaste(tt.text, tt.bracketed); string(got) != tt.want {
				t.Errorf("EncodePaste(%q, %v) = %q, want %q", tt.text, tt.bracketed, got, tt.want)
			}
		})
	}
}

func BenchmarkEncode(b *testing.B) {
	k := press('a', 0, "a")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = Encode(k)
	}
}
