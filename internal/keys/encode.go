// Package keys converts Bubble Tea v2 key presses into the bytes a terminal
// program expects on its input, for forwarding keystrokes into a pane.
//
// It produces the legacy xterm encodings, which every hosted CLI understands;
// the VT emulator offers no Kitty keyboard protocol to the program
// (docs/02-tech-stack.md). Where the legacy encoding cannot express a
// modified key (Shift+Enter, Ctrl+Enter) it uses the sequence the agent CLIs
// bind to "newline": ESC followed by CR.
package keys

import (
	"strconv"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

// ignoredMods are lock states that never change the encoding.
const ignoredMods = tea.ModCapsLock | tea.ModNumLock | tea.ModScrollLock

// Encode returns the bytes to send to a program for key press k, assuming it
// has not enabled application cursor keys. It returns nil for keys with no
// encoding (bare modifiers, Super/Hyper combinations, Ctrl+Tab, ...).
func Encode(k tea.KeyPressMsg) []byte { return EncodeApp(k, false) }

// EncodeApp is Encode with the program's DECCKM state: in application cursor
// mode the unmodified arrows, Home and End use the SS3 form (ESC O A).
func EncodeApp(k tea.KeyPressMsg, appCursor bool) []byte {
	return encode(k.Key(), appCursor)
}

func encode(k tea.Key, appCursor bool) []byte {
	mods := k.Mod &^ ignoredMods
	if mods&(tea.ModSuper|tea.ModHyper) != 0 {
		return nil // no terminal encoding; do not turn Cmd+C into "c"
	}
	// Alt (and Meta) are sent as an ESC prefix for everything that has no
	// modified CSI form of its own.
	prefix := mods&(tea.ModAlt|tea.ModMeta) != 0
	shift := mods&tea.ModShift != 0
	ctrl := mods&tea.ModCtrl != 0

	if b, ok := encodeSpecial(k.Code, mods, appCursor); ok {
		return b
	}

	var base []byte
	switch k.Code {
	case tea.KeyEnter, tea.KeyKpEnter:
		if mods != 0 {
			return []byte("\x1b\r") // Shift/Ctrl/Alt+Enter: newline
		}
		return []byte("\r")
	case tea.KeyTab:
		switch {
		case shift && !ctrl:
			base = []byte("\x1b[Z")
		case ctrl:
			return nil
		default:
			base = []byte("\t")
		}
	case tea.KeyBackspace:
		if ctrl {
			base = []byte("\x08")
		} else {
			base = []byte("\x7f")
		}
	case tea.KeyEscape:
		base = []byte("\x1b")
	default:
		base = encodeText(k, shift, ctrl)
	}
	if base == nil {
		return nil
	}
	if prefix {
		return append([]byte{0x1b}, base...)
	}
	return base
}

// encodeText handles printable keys and Ctrl+key combinations.
func encodeText(k tea.Key, shift, ctrl bool) []byte {
	if ctrl {
		if c, ok := ctrlByte(k.Code); ok {
			return []byte{c}
		}
		return nil
	}
	if k.Text != "" {
		return []byte(k.Text)
	}
	if k.Code == tea.KeySpace {
		return []byte{' '}
	}
	// Alt+key arrives without Text: rebuild the character from the key code.
	if k.Code < 0x20 || k.Code == 0x7f || k.Code >= tea.KeyExtended || !utf8.ValidRune(k.Code) {
		return nil
	}
	r := k.Code
	if shift {
		if k.ShiftedCode != 0 {
			r = k.ShiftedCode
		} else {
			r = unicode.ToUpper(r)
		}
	}
	return utf8.AppendRune(nil, r)
}

// ctrlByte maps a key code to its control character (Ctrl+A is 0x01, ...).
func ctrlByte(code rune) (byte, bool) {
	switch {
	case code >= 'a' && code <= 'z':
		return byte(code-'a') + 1, true
	case code >= 'A' && code <= 'Z':
		return byte(code-'A') + 1, true
	}
	switch code {
	case tea.KeySpace, '@', '2':
		return 0x00, true
	case '[', '3':
		return 0x1b, true
	case '\\', '4':
		return 0x1c, true
	case ']', '5':
		return 0x1d, true
	case '^', '6':
		return 0x1e, true
	case '_', '/', '7':
		return 0x1f, true
	case '?', '8':
		return 0x7f, true
	}
	return 0, false
}

// xtermMod is the modifier parameter of CSI sequences: 1 plus shift 1, alt 2,
// ctrl 4, meta 8.
func xtermMod(m tea.KeyMod) int {
	p := 1
	if m&tea.ModShift != 0 {
		p += 1
	}
	if m&tea.ModAlt != 0 {
		p += 2
	}
	if m&tea.ModCtrl != 0 {
		p += 4
	}
	if m&tea.ModMeta != 0 {
		p += 8
	}
	return p
}

// letterKeys are the keys encoded as CSI <letter> (SS3 <letter> for F1-F4 and,
// in application cursor mode, the arrows, Home and End).
var letterKeys = map[rune]byte{
	tea.KeyUp: 'A', tea.KeyDown: 'B', tea.KeyRight: 'C', tea.KeyLeft: 'D',
	tea.KeyHome: 'H', tea.KeyEnd: 'F', tea.KeyBegin: 'E',
	tea.KeyF1: 'P', tea.KeyF2: 'Q', tea.KeyF3: 'R', tea.KeyF4: 'S',
	tea.KeyKpUp: 'A', tea.KeyKpDown: 'B', tea.KeyKpRight: 'C', tea.KeyKpLeft: 'D',
	tea.KeyKpHome: 'H', tea.KeyKpEnd: 'F', tea.KeyKpBegin: 'E',
}

// tildeKeys are the keys encoded as CSI <n> ~.
var tildeKeys = map[rune]int{
	tea.KeyInsert: 2, tea.KeyDelete: 3, tea.KeyPgUp: 5, tea.KeyPgDown: 6,
	tea.KeyKpInsert: 2, tea.KeyKpDelete: 3, tea.KeyKpPgUp: 5, tea.KeyKpPgDown: 6,
	tea.KeyF5: 15, tea.KeyF6: 17, tea.KeyF7: 18, tea.KeyF8: 19, tea.KeyF9: 20,
	tea.KeyF10: 21, tea.KeyF11: 23, tea.KeyF12: 24,
	tea.KeyF13: 25, tea.KeyF14: 26, tea.KeyF15: 28, tea.KeyF16: 29,
	tea.KeyF17: 31, tea.KeyF18: 32, tea.KeyF19: 33, tea.KeyF20: 34,
}

// keypadKeys are the keypad keys as the characters they type with the keypad
// in numeric mode.
var keypadKeys = map[rune]byte{
	tea.KeyKp0: '0', tea.KeyKp1: '1', tea.KeyKp2: '2', tea.KeyKp3: '3', tea.KeyKp4: '4',
	tea.KeyKp5: '5', tea.KeyKp6: '6', tea.KeyKp7: '7', tea.KeyKp8: '8', tea.KeyKp9: '9',
	tea.KeyKpDecimal: '.', tea.KeyKpDivide: '/', tea.KeyKpMultiply: '*',
	tea.KeyKpMinus: '-', tea.KeyKpPlus: '+', tea.KeyKpEqual: '=', tea.KeyKpComma: ',',
}

// encodeSpecial encodes navigation, function and keypad keys.
func encodeSpecial(code rune, mods tea.KeyMod, appCursor bool) ([]byte, bool) {
	mod := xtermMod(mods)
	if l, ok := letterKeys[code]; ok {
		if mod != 1 {
			return []byte("\x1b[1;" + strconv.Itoa(mod) + string(l)), true
		}
		ss3 := code >= tea.KeyF1 && code <= tea.KeyF4
		if appCursor && (l == 'A' || l == 'B' || l == 'C' || l == 'D' || l == 'H' || l == 'F') {
			ss3 = true
		}
		if ss3 {
			return []byte{0x1b, 'O', l}, true
		}
		return []byte{0x1b, '[', l}, true
	}
	if n, ok := tildeKeys[code]; ok {
		if mod != 1 {
			return []byte("\x1b[" + strconv.Itoa(n) + ";" + strconv.Itoa(mod) + "~"), true
		}
		return []byte("\x1b[" + strconv.Itoa(n) + "~"), true
	}
	if c, ok := keypadKeys[code]; ok {
		if mods&(tea.ModAlt|tea.ModMeta) != 0 {
			return []byte{0x1b, c}, true
		}
		return []byte{c}, true
	}
	return nil, false
}
