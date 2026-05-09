package api

import "sync"

// Hub broadcasts SSE messages to subscribers grouped by org.
type Hub struct {
	mu   sync.Mutex
	subs map[string][]*hubSub // orgID → subscribers
}

type hubSub struct {
	ch    chan []byte
	envID string // filter: only send events for this env ("" = all)
}

func NewHub() *Hub {
	return &Hub{subs: make(map[string][]*hubSub)}
}

func (h *Hub) subscribe(orgID, envID string) *hubSub {
	s := &hubSub{ch: make(chan []byte, 16), envID: envID}
	h.mu.Lock()
	h.subs[orgID] = append(h.subs[orgID], s)
	h.mu.Unlock()
	return s
}

func (h *Hub) unsubscribe(orgID string, s *hubSub) {
	h.mu.Lock()
	defer h.mu.Unlock()
	list := h.subs[orgID]
	for i, sub := range list {
		if sub == s {
			h.subs[orgID] = append(list[:i], list[i+1:]...)
			close(s.ch)
			return
		}
	}
}

// Broadcast sends data to all org subscribers that match envID.
func (h *Hub) Broadcast(orgID, envID string, data []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, s := range h.subs[orgID] {
		if s.envID != "" && s.envID != envID {
			continue
		}
		select {
		case s.ch <- data:
		default: // drop if subscriber is slow
		}
	}
}
