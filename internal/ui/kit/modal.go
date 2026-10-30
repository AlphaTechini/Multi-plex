package kit

import (
	"image/color"
	"strings"

	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// ModalOptions describes a Modal.
type ModalOptions struct {
	// Width is the box width in cells, frame included. It shrinks to fit the
	// screen with a two-cell margin.
	Width int
	// Title is the dialog title (strong, bold). Empty omits the title row.
	Title string
	// Meta is right-aligned plain text on the title row (muted).
	Meta string
	// Tone colours the frame: ToneAccent (default) for ordinary dialogs,
	// ToneWarning for approvals.
	Tone Tone
	// Body is pre-rendered content, fitted to the inner width (Width - 6).
	Body string
	// Buttons are shown on the last row.
	Buttons []ButtonOptions
	// Hints are shown right-aligned on the button row (or alone).
	Hints []Hint
	// Screen is the background to draw over. Empty returns the box alone.
	Screen string
	// ScreenWidth and ScreenHeight are Screen's size in cells.
	ScreenWidth, ScreenHeight int
	// Top is the box's row on the screen; a negative value centres it
	// optically (a little above the middle).
	Top int
	// Flush drops the inner padding and the blank rows, for bodies that draw
	// edge to edge (the command palette list).
	Flush bool
	// Scrim is how much the screen is dimmed toward the sunken surface
	// (0..1). Zero means the default of 0.7.
	Scrim float64
}

// ModalInnerWidth returns the content width of a modal: what to render Body
// at.
func ModalInnerWidth(o ModalOptions) int {
	w := o.Width
	if o.ScreenWidth > 0 {
		w = min(w, o.ScreenWidth-4)
	}
	if o.Flush {
		return max(w-2, 0)
	}
	return max(w-6, 0)
}

// Modal renders a dialog box on the overlay surface (rounded frame, two cells
// of padding, title row, body, buttons) and, when Screen is given, composites
// it over the dimmed screen and returns the whole screen.
func Modal(t *theme.Theme, o ModalOptions) string {
	box := modalBox(t, o)
	if o.Screen == "" {
		return box
	}
	scrim := o.Scrim
	if scrim == 0 {
		scrim = 0.7
	}
	screen := Dim(t, o.Screen, t.Surface.Sunken, scrim)
	bw, bh := Width(box), len(Lines(box))
	x := (o.ScreenWidth - bw) / 2
	y := o.Top
	if y < 0 {
		y = max((o.ScreenHeight-bh)*2/5, 0)
	}
	return Composite(screen, box, x, y)
}

func modalBox(t *theme.Theme, o ModalOptions) string {
	w := o.Width
	if o.ScreenWidth > 0 {
		w = min(w, o.ScreenWidth-4)
	}
	if w < 6 {
		return Backfill(Fit("", max(w, 0)), t.Surface.Overlay)
	}
	var border color.Color = t.Border.Focus
	if o.Tone != ToneDefault {
		border = ToneColor(t, o.Tone)
	}
	if o.Flush {
		body := Lines(o.Body)
		return Panel(t, PanelOptions{
			Width: w, Height: len(body) + 2, Content: o.Body,
			Bg: t.Surface.Overlay, Flush: true, BorderColor: border,
		})
	}
	iw := w - 6
	var body []string
	body = append(body, "")
	if o.Title != "" {
		title := ink{fg: t.Text.Strong, bold: true}.paint(o.Title)
		body = append(body, spread(title, fg(t.Text.Muted).paint(o.Meta), iw), "")
	}
	if o.Body != "" {
		body = append(body, Lines(o.Body)...)
	}
	if len(o.Buttons) > 0 || len(o.Hints) > 0 {
		buttons := ButtonRow(t, o.Buttons, iw)
		hints := KeyHints(t, o.Hints, 0)
		body = append(body, "", spread(buttons, hints, iw))
	}
	body = append(body, "")
	return Panel(t, PanelOptions{
		Width:       w,
		Height:      len(body) + 2,
		Content:     "  " + strings.Join(body, "\n  "),
		Bg:          t.Surface.Overlay,
		Flush:       true,
		BorderColor: border,
	})
}
