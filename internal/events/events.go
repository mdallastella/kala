// Package events fans out per-user server-sent events.
package events

import (
	"sync"
)

// Event is a message pushed to a user's open clients.
type Event struct {
	// Type is the SSE event name: changed, reminder, briefing or wrapup.
	Type string `json:"type"`
	Data any    `json:"data,omitempty"`
}

// Changed builds a "changed" event listing which resources changed.
func Changed(kinds ...string) Event {
	return Event{Type: "changed", Data: map[string][]string{"kinds": kinds}}
}

// Hub keeps the subscribers of every user.
type Hub struct {
	mu   sync.Mutex
	subs map[int64]map[chan Event]struct{}
	// onSubscribe, if set, is called (outside the lock) for every new
	// subscription, so the scheduler can push catch-up events.
	onSubscribe func(userID int64, ch chan<- Event)
}

// NewHub returns an empty hub.
func NewHub() *Hub {
	return &Hub{subs: map[int64]map[chan Event]struct{}{}}
}

// OnSubscribe registers a callback run for each new subscription.
func (h *Hub) OnSubscribe(f func(userID int64, ch chan<- Event)) {
	h.mu.Lock()
	h.onSubscribe = f
	h.mu.Unlock()
}

// Subscribe registers a new client of userID. The returned cancel function
// must be called when the client goes away.
func (h *Hub) Subscribe(userID int64) (<-chan Event, func()) {
	ch := make(chan Event, 64)
	h.mu.Lock()
	if h.subs[userID] == nil {
		h.subs[userID] = map[chan Event]struct{}{}
	}
	h.subs[userID][ch] = struct{}{}
	cb := h.onSubscribe
	h.mu.Unlock()
	if cb != nil {
		cb(userID, ch)
	}
	return ch, func() {
		h.mu.Lock()
		delete(h.subs[userID], ch)
		if len(h.subs[userID]) == 0 {
			delete(h.subs, userID)
		}
		h.mu.Unlock()
	}
}

// Publish sends e to every client of userID. Slow clients drop events rather
// than block publishers; the UI refetches on reconnect anyway.
func (h *Hub) Publish(userID int64, e Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs[userID] {
		select {
		case ch <- e:
		default:
		}
	}
}

// Users returns the users with at least one connected client.
func (h *Hub) Users() []int64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]int64, 0, len(h.subs))
	for id := range h.subs {
		out = append(out, id)
	}
	return out
}
