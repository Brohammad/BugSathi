package hub

import (
	"sync"

	"github.com/Brohammad/BugSathi/internal/collab/domain"
	"github.com/Brohammad/BugSathi/internal/collab/port"
	"github.com/google/uuid"
)

type subscriber struct {
	ch     chan port.StreamEvent
	userID uuid.UUID
	name   string
}

type MemoryHub struct {
	mu    sync.RWMutex
	rooms map[uuid.UUID]map[chan port.StreamEvent]*subscriber
}

func New() *MemoryHub {
	return &MemoryHub{rooms: make(map[uuid.UUID]map[chan port.StreamEvent]*subscriber)}
}

func (h *MemoryHub) Subscribe(reportID, userID uuid.UUID, name string) (<-chan port.StreamEvent, func()) {
	ch := make(chan port.StreamEvent, 16)
	sub := &subscriber{ch: ch, userID: userID, name: name}

	h.mu.Lock()
	room, ok := h.rooms[reportID]
	if !ok {
		room = make(map[chan port.StreamEvent]*subscriber)
		h.rooms[reportID] = room
	}
	room[ch] = sub
	h.mu.Unlock()

	h.broadcastPresence(reportID)

	unsub := func() {
		h.mu.Lock()
		if room, ok := h.rooms[reportID]; ok {
			if _, exists := room[ch]; exists {
				delete(room, ch)
				close(ch)
				if len(room) == 0 {
					delete(h.rooms, reportID)
				}
			}
		}
		h.mu.Unlock()
		h.broadcastPresence(reportID)
	}
	return ch, unsub
}

func (h *MemoryHub) Publish(reportID uuid.UUID, ev port.StreamEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	room := h.rooms[reportID]
	for ch := range room {
		select {
		case ch <- ev:
		default:
			// slow consumer: drop rather than block writers
		}
	}
}

func (h *MemoryHub) Presence(reportID uuid.UUID) []port.PresenceUser {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return presenceLocked(h.rooms[reportID])
}

func (h *MemoryHub) broadcastPresence(reportID uuid.UUID) {
	h.mu.RLock()
	users := presenceLocked(h.rooms[reportID])
	h.mu.RUnlock()
	h.Publish(reportID, port.StreamEvent{
		Type: domain.EventPresenceUpdated,
		Data: map[string]any{"users": users},
	})
}

func presenceLocked(room map[chan port.StreamEvent]*subscriber) []port.PresenceUser {
	seen := make(map[uuid.UUID]struct{})
	out := make([]port.PresenceUser, 0)
	for _, sub := range room {
		if _, ok := seen[sub.userID]; ok {
			continue
		}
		seen[sub.userID] = struct{}{}
		out = append(out, port.PresenceUser{UserID: sub.userID, Name: sub.name})
	}
	return out
}
