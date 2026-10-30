package clientbackend

import (
	"encoding/json"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/ui/backend"
)

// eventBuffer is the capacity of the converted event channel.
const eventBuffer = 512

// pump converts client events to backend events. Events other than pane
// frames are never dropped: the pump blocks on them (the client buffers
// behind it). Pane frames are coalesced instead of dropped when the UI is
// behind: pending frames for a pane are merged (changed rows replace older
// ones, a full frame replaces everything) and delivered when there is room,
// so a slow UI sees a correct screen, just fewer intermediate states.
func (b *Backend) pump() {
	defer close(b.events)
	in := b.c.Events()
	pending := map[core.PaneID]*protocol.PaneFrame{}
	var order []core.PaneID

	for in != nil || len(pending) > 0 {
		var out chan<- backend.Event
		var next backend.Event
		if len(pending) > 0 {
			id := order[0]
			if ev, ok := encodeFrame(pending[id]); ok {
				out, next = b.events, ev
			} else {
				delete(pending, id)
				order = order[1:]
				continue
			}
		}
		select {
		case ev, ok := <-in:
			if !ok {
				in = nil
				continue
			}
			b.handle(ev.Topic, ev.Data, pending, &order)
		case out <- next:
			id := order[0]
			delete(pending, id)
			order = order[1:]
		}
	}
}

// handle forwards one client event, or merges a frame into pending when the
// output is congested or older frames are already waiting.
func (b *Backend) handle(topic string, data json.RawMessage, pending map[core.PaneID]*protocol.PaneFrame, order *[]core.PaneID) {
	ev := backend.Event{Topic: topic, Data: data}
	if topic != protocol.TopicPaneFrame {
		b.events <- ev
		return
	}
	if len(pending) == 0 {
		select {
		case b.events <- ev:
			return
		default:
		}
	}
	var f protocol.PaneFrame
	if json.Unmarshal(data, &f) != nil {
		return
	}
	old, ok := pending[f.PaneID]
	if !ok {
		pending[f.PaneID] = &f
		*order = append(*order, f.PaneID)
		return
	}
	mergeFrame(old, f)
}

// mergeFrame folds a newer frame into an older pending one.
func mergeFrame(dst *protocol.PaneFrame, f protocol.PaneFrame) {
	if f.Full || f.Cols != dst.Cols || f.Rows != dst.Rows {
		*dst = f
		return
	}
	idx := make(map[int]int, len(dst.Changed))
	for i, r := range dst.Changed {
		idx[r.Y] = i
	}
	for _, r := range f.Changed {
		if i, ok := idx[r.Y]; ok {
			dst.Changed[i] = r
		} else {
			idx[r.Y] = len(dst.Changed)
			dst.Changed = append(dst.Changed, r)
		}
	}
	dst.CursorX, dst.CursorY, dst.CursorVisible = f.CursorX, f.CursorY, f.CursorVisible
	if f.Title != "" {
		dst.Title = f.Title
	}
}

func encodeFrame(f *protocol.PaneFrame) (backend.Event, bool) {
	data, err := json.Marshal(f)
	if err != nil {
		return backend.Event{}, false
	}
	return backend.Event{Topic: protocol.TopicPaneFrame, Data: data}, true
}
