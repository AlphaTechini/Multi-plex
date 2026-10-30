package daemon

import (
	"encoding/json"
	"io"
	"log/slog"
	"testing"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

func testBus() *bus {
	return newBus(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func frameOf(id core.PaneID, full bool, rows ...protocol.Row) protocol.PaneFrame {
	return protocol.PaneFrame{PaneID: id, Cols: 10, Rows: 3, Full: full, Changed: rows}
}

// TestBusSlowSubscriberDoesNotBlock: publishing never blocks; a full queue
// drops frames and marks the pane, and the next frame that fits is a snapshot.
func TestBusSlowSubscriberDoesNotBlock(t *testing.T) {
	b := testBus()
	b.snapshot = func(id core.PaneID) (protocol.PaneFrame, bool) {
		return frameOf(id, true, protocol.Row{Y: 0, Text: "SNAPSHOT"}), true
	}
	slow := b.subscribe(nil)
	fast := b.subscribe([]string{protocol.TopicNote})

	for i := 0; i < subscriberQueue+50; i++ {
		b.PublishFrame(frameOf("p1", false, protocol.Row{Y: 0, Text: "delta"}))
	}
	if slow.dropped.Load() != 50 {
		t.Errorf("dropped = %d, want 50", slow.dropped.Load())
	}
	if len(fast.q) != 0 {
		t.Error("a subscriber that did not ask for frames got some")
	}

	// Drain a few: now there is room, and the next frame becomes a snapshot.
	for i := 0; i < 10; i++ {
		<-slow.q
	}
	b.PublishFrame(frameOf("p1", false, protocol.Row{Y: 1, Text: "delta2"}))
	var last outItem
	for len(slow.q) > 0 {
		last = <-slow.q
	}
	var f protocol.PaneFrame
	if err := json.Unmarshal(last.data, &f); err != nil {
		t.Fatal(err)
	}
	if !f.Full || f.Changed[0].Text != "SNAPSHOT" {
		t.Errorf("frame after the gap = %+v, want the snapshot", f)
	}
	// And the one after is a delta again.
	b.PublishFrame(frameOf("p1", false, protocol.Row{Y: 2, Text: "delta3"}))
	last = <-slow.q
	json.Unmarshal(last.data, &f)
	if f.Full {
		t.Error("only one snapshot should follow a gap")
	}
}

// TestBusDropsOldestForEvents: other events drop the oldest, never block.
func TestBusDropsOldestForEvents(t *testing.T) {
	b := testBus()
	s := b.subscribe(nil)
	for i := 0; i < subscriberQueue+10; i++ {
		b.Publish(protocol.TopicToast, protocol.Toast{Level: "info", Text: "x"})
	}
	if len(s.q) != subscriberQueue {
		t.Errorf("queue = %d, want %d", len(s.q), subscriberQueue)
	}
	if s.dropped.Load() != 10 || b.Dropped() != 10 {
		t.Errorf("dropped = %d / %d", s.dropped.Load(), b.Dropped())
	}
}

// TestBusDroppedFrameOldestMarksPane: if making room drops a queued frame,
// that pane is resynced with a snapshot.
func TestBusDroppedFrameOldestMarksPane(t *testing.T) {
	b := testBus()
	b.snapshot = func(id core.PaneID) (protocol.PaneFrame, bool) { return frameOf(id, true), true }
	s := b.subscribe(nil)
	b.PublishFrame(frameOf("p9", false, protocol.Row{Y: 0, Text: "a"}))
	for i := 0; i < subscriberQueue; i++ {
		b.Publish(protocol.TopicToast, protocol.Toast{Text: "x"})
	}
	// The frame was the oldest and has been pushed out.
	if !s.takeNeedFull("p9") {
		t.Fatal("pane p9 was not marked for resync after its frame was dropped")
	}
}

func TestBusUnsubscribe(t *testing.T) {
	b := testBus()
	s := b.subscribe(nil)
	b.unsubscribe(s)
	b.Publish(protocol.TopicToast, protocol.Toast{})
	if len(s.q) != 0 {
		t.Error("event delivered after unsubscribe")
	}
}
