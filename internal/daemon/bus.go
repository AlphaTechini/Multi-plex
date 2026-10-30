package daemon

import (
	"encoding/json"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

// subscriberQueue is the capacity of each subscriber's outgoing queue.
const subscriberQueue = 512

// outItem is one queued event. pane is set for pane.frame events so a dropped
// frame can mark its pane for a full resync.
type outItem struct {
	topic string
	data  json.RawMessage
	pane  core.PaneID
}

// subscriber is one connection's view of the event stream.
type subscriber struct {
	topics map[string]struct{} // nil means every topic
	q      chan outItem

	mu       sync.Mutex
	needFull map[core.PaneID]struct{} // panes whose next frame must be a snapshot

	dropped atomic.Uint64
}

func (s *subscriber) wants(topic string) bool {
	if s.topics == nil {
		return true
	}
	_, ok := s.topics[topic]
	return ok
}

// markNeedFull remembers that a frame for pane was lost.
func (s *subscriber) markNeedFull(p core.PaneID) {
	s.mu.Lock()
	if s.needFull == nil {
		s.needFull = map[core.PaneID]struct{}{}
	}
	s.needFull[p] = struct{}{}
	s.mu.Unlock()
}

// takeNeedFull reports and clears the resync mark of pane.
func (s *subscriber) takeNeedFull(p core.PaneID) bool {
	s.mu.Lock()
	_, ok := s.needFull[p]
	if ok {
		delete(s.needFull, p)
	}
	s.mu.Unlock()
	return ok
}

// bus fans events out to subscribers through bounded queues. Publishing never
// blocks: a slow subscriber loses events, not the daemon.
type bus struct {
	log *slog.Logger
	// snapshot returns a full frame of a pane, for subscribers that lost one.
	snapshot func(core.PaneID) (protocol.PaneFrame, bool)

	mu   sync.RWMutex
	subs map[*subscriber]struct{}

	droppedTotal atomic.Uint64
}

func newBus(log *slog.Logger) *bus {
	return &bus{log: log, subs: map[*subscriber]struct{}{}}
}

// subscribe registers a subscriber for topics (all when empty).
func (b *bus) subscribe(topics []string) *subscriber {
	s := &subscriber{q: make(chan outItem, subscriberQueue)}
	if len(topics) > 0 {
		s.topics = make(map[string]struct{}, len(topics))
		for _, t := range topics {
			s.topics[t] = struct{}{}
		}
	}
	b.mu.Lock()
	b.subs[s] = struct{}{}
	b.mu.Unlock()
	return s
}

// unsubscribe removes s. Its queue is not closed: the writer stops through the
// connection's context.
func (b *bus) unsubscribe(s *subscriber) {
	b.mu.Lock()
	delete(b.subs, s)
	b.mu.Unlock()
}

// Publish sends a non-frame event to every interested subscriber. The payload
// is encoded once.
func (b *bus) Publish(topic string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		b.log.Error("encode event", "topic", topic, "err", err)
		return
	}
	it := outItem{topic: topic, data: data}
	b.mu.RLock()
	for s := range b.subs {
		if s.wants(topic) {
			b.enqueue(s, it)
		}
	}
	b.mu.RUnlock()
}

// PublishFrame sends a pane frame. A subscriber whose queue is full loses the
// frame and is sent a full snapshot instead of the next one, so its picture
// of the pane is never built from a partial history.
func (b *bus) PublishFrame(f protocol.PaneFrame) {
	var data json.RawMessage
	b.mu.RLock()
	defer b.mu.RUnlock()
	for s := range b.subs {
		if !s.wants(protocol.TopicPaneFrame) {
			continue
		}
		it := outItem{topic: protocol.TopicPaneFrame, pane: f.PaneID}
		if f.Full {
			s.takeNeedFull(f.PaneID) // a full frame resyncs the subscriber by itself
		} else if s.takeNeedFull(f.PaneID) {
			if snap, ok := b.snapshotOf(f.PaneID); ok {
				d, err := json.Marshal(snap)
				if err != nil {
					continue
				}
				it.data = d
			}
		}
		if it.data == nil {
			if data == nil {
				d, err := json.Marshal(f)
				if err != nil {
					b.log.Error("encode frame", "err", err)
					return
				}
				data = d
			}
			it.data = data
		}
		select {
		case s.q <- it:
		default:
			// Full: drop this frame and force a snapshot next time.
			s.markNeedFull(f.PaneID)
			b.noteDrop(s, protocol.TopicPaneFrame)
		}
	}
}

func (b *bus) snapshotOf(p core.PaneID) (protocol.PaneFrame, bool) {
	if b.snapshot == nil {
		return protocol.PaneFrame{}, false
	}
	return b.snapshot(p)
}

// enqueue adds it to s, dropping the oldest queued event when the queue is
// full. If the dropped event was a pane frame, that pane is marked for resync.
func (b *bus) enqueue(s *subscriber, it outItem) {
	for attempts := 0; attempts < 2; attempts++ {
		select {
		case s.q <- it:
			return
		default:
		}
		select {
		case old := <-s.q:
			if old.pane != "" {
				s.markNeedFull(old.pane)
			}
			b.noteDrop(s, old.topic)
		default:
		}
	}
	b.noteDrop(s, it.topic)
}

// noteDrop counts a lost event and warns on the first loss and every
// thousandth after it.
func (b *bus) noteDrop(s *subscriber, topic string) {
	n := s.dropped.Add(1)
	b.droppedTotal.Add(1)
	if n == 1 || n%1000 == 0 {
		b.log.Warn("slow subscriber: events dropped", "topic", topic, "dropped", n)
	}
}

// Dropped returns how many events were dropped across all subscribers.
func (b *bus) Dropped() uint64 { return b.droppedTotal.Load() }

// replay queues one event for a single subscriber.
func (b *bus) replay(s *subscriber, topic string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	b.enqueue(s, outItem{topic: topic, data: data})
}
