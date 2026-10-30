package overview

import (
	"context"
	"strconv"

	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/ui/backend"
	"github.com/alphatechini/multi-plex/internal/ui/kit"
	"github.com/alphatechini/multi-plex/internal/ui/view"
)

// actCmd sends a card action to the daemon and reports the outcome as a
// toast (dismiss is silent unless it fails).
func actCmd(be backend.Backend, p protocol.NoteActParams, number int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
		defer cancel()
		if err := be.ActOnNote(ctx, p); err != nil {
			return view.ToastMsg{Tone: kit.ToneDanger, Title: "Could not " + verb(p.Action), Body: err.Error()}
		}
		switch p.Action {
		case core.NoteAsk:
			return view.ToastMsg{Tone: kit.ToneSuccess, Title: "Question sent", Body: "Card " + strconv.Itoa(number) + " · the main conversation is unchanged"}
		case core.NoteSteer:
			return view.ToastMsg{Tone: kit.ToneSuccess, Title: "Steering sent", Body: "The agent picks it up at its next turn"}
		case core.NoteInterruptSteer:
			return view.ToastMsg{Tone: kit.ToneSuccess, Title: "Interrupted and steered", Body: "Card " + strconv.Itoa(number)}
		}
		return nil
	}
}

// verb is the action as a phrase for error toasts.
func verb(a core.NoteAction) string {
	switch a {
	case core.NoteAsk:
		return "ask"
	case core.NoteSteer:
		return "steer"
	case core.NoteInterruptSteer:
		return "interrupt and steer"
	default:
		return "dismiss the card"
	}
}

// switchCmd asks the shell to switch modes.
func switchCmd(mode view.Mode, path string, line int) tea.Cmd {
	return func() tea.Msg { return view.SwitchModeMsg{Mode: mode, Path: path, Line: line} }
}
