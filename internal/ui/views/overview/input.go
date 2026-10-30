package overview

import (
	"charm.land/bubbles/v2/textinput"

	"github.com/alphatechini/multi-plex/internal/core"
)

// input is the small text box under a card for the ask, steer and
// interrupt-and-steer actions.
type input struct {
	ti     textinput.Model
	action core.NoteAction
	noteID string
	number int
}
