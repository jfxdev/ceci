package service

import (
	"sync"

	"github.com/google/uuid"
)

// FlagBroadcaster is an in-process pub/sub used to notify OFREP SSE
// subscribers when a project's flags change. It intentionally has no
// cross-instance fan-out (e.g. via Redis) — leaflag runs as a single binary, so
// in-memory channels are sufficient.
type FlagBroadcaster struct {
	mu   sync.Mutex
	subs map[uuid.UUID]map[chan struct{}]struct{}
}

func NewFlagBroadcaster() *FlagBroadcaster {
	return &FlagBroadcaster{subs: make(map[uuid.UUID]map[chan struct{}]struct{})}
}

// Subscribe registers a new listener for a project's flag changes. The
// returned cancel func must be called when the subscriber disconnects.
func (b *FlagBroadcaster) Subscribe(projectID uuid.UUID) (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)

	b.mu.Lock()
	if b.subs[projectID] == nil {
		b.subs[projectID] = make(map[chan struct{}]struct{})
	}
	b.subs[projectID][ch] = struct{}{}
	b.mu.Unlock()

	cancel := func() {
		b.mu.Lock()
		delete(b.subs[projectID], ch)
		if len(b.subs[projectID]) == 0 {
			delete(b.subs, projectID)
		}
		b.mu.Unlock()
	}
	return ch, cancel
}

// Publish notifies every current subscriber of a project that its flags
// changed. Non-blocking: a subscriber that hasn't drained its previous
// notification yet simply skips this one, since the next read re-evaluates
// the full current state anyway.
func (b *FlagBroadcaster) Publish(projectID uuid.UUID) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs[projectID] {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
