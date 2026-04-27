package websocket

import (
	"sync"

	"go.uber.org/zap"

	"workflow/domain"
)

type Hub struct {
	mu     sync.RWMutex
	users  map[int64]map[*Connection]struct{}
	teams  map[string]map[*Connection]struct{}
	logger *zap.Logger
}

func NewHub(logger *zap.Logger) *Hub {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Hub{
		users:  map[int64]map[*Connection]struct{}{},
		teams:  map[string]map[*Connection]struct{}{},
		logger: logger,
	}
}

func (h *Hub) Register(c *Connection) {
	if h == nil || c == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.users[c.UserID] == nil {
		h.users[c.UserID] = map[*Connection]struct{}{}
	}
	h.users[c.UserID][c] = struct{}{}
	if c.TeamCode != "" {
		if h.teams[c.TeamCode] == nil {
			h.teams[c.TeamCode] = map[*Connection]struct{}{}
		}
		h.teams[c.TeamCode][c] = struct{}{}
	}
}

func (h *Hub) Unregister(c *Connection) {
	if h == nil || c == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if conns := h.users[c.UserID]; conns != nil {
		delete(conns, c)
		if len(conns) == 0 {
			delete(h.users, c.UserID)
		}
	}
	if c.TeamCode != "" {
		if conns := h.teams[c.TeamCode]; conns != nil {
			delete(conns, c)
			if len(conns) == 0 {
				delete(h.teams, c.TeamCode)
			}
		}
	}
	close(c.send)
}

func (h *Hub) BroadcastToUser(userID int64, event domain.WebSocketEvent) {
	h.broadcast(h.userConnections(userID), event)
}

func (h *Hub) BroadcastToTeam(teamCode string, event domain.WebSocketEvent) {
	h.broadcast(h.teamConnections(teamCode), event)
}

func (h *Hub) BroadcastPoolCountChanged(teamCode string, poolCount int) {
	h.BroadcastToTeam(teamCode, domain.NewWebSocketEvent(domain.WebSocketEventTaskPoolCountChanged, map[string]interface{}{
		"team_code":  teamCode,
		"pool_count": poolCount,
	}))
}

func (h *Hub) userConnections(userID int64) []*Connection {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var out []*Connection
	for c := range h.users[userID] {
		out = append(out, c)
	}
	return out
}

func (h *Hub) teamConnections(teamCode string) []*Connection {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var out []*Connection
	for c := range h.teams[teamCode] {
		out = append(out, c)
	}
	return out
}

func (h *Hub) broadcast(conns []*Connection, event domain.WebSocketEvent) {
	for _, c := range conns {
		select {
		case c.send <- event:
		default:
			h.Unregister(c)
		}
	}
}
