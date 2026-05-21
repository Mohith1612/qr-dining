package websocket

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/Mohith1612/qr-dining/internal/observability"
	redisPkg "github.com/Mohith1612/qr-dining/internal/redis"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

// Hub is the central WebSocket connection registry.
// A single goroutine (Run) owns all mutations to the rooms map — no mutex needed.
// The broadcast channel receives messages from the Redis subscriber goroutine
// and delivers them to the correct room without blocking the subscriber.
type Hub struct {
	rooms      map[string]map[string]*Client // session_id → client_id → *Client
	register   chan *Client
	unregister chan *Client
	broadcast  chan redisPkg.Message
	pubsub     *redisPkg.PubSub
	metrics    *observability.Metrics
	logger     zerolog.Logger
	upgrader   websocket.Upgrader // per-Hub so CheckOrigin captures the origin set by value
}

func NewHub(pubsub *redisPkg.PubSub, metrics *observability.Metrics, logger zerolog.Logger, allowedOrigins []string) *Hub {
	originSet := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		originSet[o] = struct{}{}
	}

	h := &Hub{
		rooms:      make(map[string]map[string]*Client),
		register:   make(chan *Client, 32),
		unregister: make(chan *Client, 32),
		broadcast:  make(chan redisPkg.Message, 512),
		pubsub:     pubsub,
		metrics:    metrics,
		logger:     logger,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				if len(allowedOrigins) == 0 {
					return true // dev mode: allow all
				}
				_, ok := originSet[origin]
				return ok
			},
		},
	}

	if len(allowedOrigins) == 0 {
		logger.Warn().Msg("WebSocket origin validation disabled — set CORS_ALLOWED_ORIGINS in production")
	}

	return h
}

// Run processes all registry and broadcast events sequentially.
// Must be started in a goroutine before the HTTP server accepts traffic.
func (h *Hub) Run(ctx context.Context) {
	go h.runSubscriber(ctx)

	for {
		select {
		case <-ctx.Done():
			h.drainAll()
			return
		case client := <-h.register:
			h.addToRoom(client)
		case client := <-h.unregister:
			h.removeFromRoom(client)
		case msg := <-h.broadcast:
			h.broadcastToRoom(msg)
		}
	}
}

// Upgrade upgrades an HTTP request to a WebSocket connection and registers
// the client with the hub. The caller provides session and participant IDs.
// Sets X-Reconnect-Endpoint so clients know where to reconcile state on reconnect.
func (h *Hub) Upgrade(w http.ResponseWriter, r *http.Request, sessionID uuid.UUID, participantID int64) error {
	// Advertise the reconciliation endpoint before upgrading — clients use this on reconnect.
	w.Header().Set("X-Reconnect-Endpoint", "/sessions/"+sessionID.String()+"/snapshot")

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}
	client := newClient(sessionID, participantID, conn, h, h.logger)
	h.register <- client
	go client.writePump()
	go client.readPump()
	return nil
}

func (h *Hub) addToRoom(c *Client) {
	sid := c.sessionID.String()
	if _, ok := h.rooms[sid]; !ok {
		h.rooms[sid] = make(map[string]*Client)
	}
	// Detect reconnect: participant already has an active client in this room.
	for _, existing := range h.rooms[sid] {
		if existing.participantID == c.participantID {
			h.metrics.WSReconnectsTotal.Inc()
			break
		}
	}
	h.rooms[sid][c.id] = c
	h.metrics.WSConnectionsActive.Inc()
}

func (h *Hub) removeFromRoom(c *Client) {
	sid := c.sessionID.String()
	if room, ok := h.rooms[sid]; ok {
		delete(room, c.id)
		if len(room) == 0 {
			delete(h.rooms, sid)
		}
	}
	c.closeSend()
	h.metrics.WSConnectionsActive.Dec()
}

func (h *Hub) broadcastToRoom(msg redisPkg.Message) {
	sid := msg.SessionID.String()
	room, ok := h.rooms[sid]
	if !ok {
		return
	}

	// Parse event type for metrics — best effort.
	var env struct {
		Event string `json:"event"`
	}
	if err := json.Unmarshal(msg.Data, &env); err == nil && env.Event != "" {
		h.metrics.WSMessagesSentTotal.WithLabelValues(env.Event).Add(float64(len(room)))
	}

	for _, client := range room {
		select {
		case client.send <- msg.Data:
		default:
			// Slow consumer: evict immediately so one stalled client cannot block the room.
			delete(room, client.id)
			client.closeSend()
			h.metrics.WSConnectionsActive.Dec()
			h.metrics.WSClientEvictions.Inc()
			h.logger.Warn().Str("client_id", client.id).Str("session_id", sid).Msg("evicted slow WebSocket client")
		}
	}
}

func (h *Hub) drainAll() {
	for sid, room := range h.rooms {
		for _, client := range room {
			client.closeSend()
		}
		delete(h.rooms, sid)
	}
}

// runSubscriber keeps the Redis PubSub subscription alive with exponential backoff.
// It restarts automatically when the channel closes unexpectedly (Redis restart, network blip).
// Returns only when ctx is cancelled.
func (h *Hub) runSubscriber(ctx context.Context) {
	const maxBackoff = 30 * time.Second
	backoff := time.Second
	for {
		if err := h.pubsub.Subscribe(ctx, h.broadcast); err != nil {
			h.logger.Error().Err(err).Msg("pubsub subscriber exited with error")
		}
		if ctx.Err() != nil {
			return
		}
		h.metrics.RedisReconnectsTotal.Inc()
		h.logger.Warn().Dur("backoff", backoff).Msg("pubsub subscriber exited unexpectedly; reconnecting")
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < maxBackoff {
			backoff *= 2
		}
	}
}
