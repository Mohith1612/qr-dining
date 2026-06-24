package websocket

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10 // must be less than pongWait
	maxMessageSize = 4096                // bytes; prevents memory exhaustion
	sendChanBuffer = 256

	// Inbound abuse hardening. The server is push-only; a legitimate client
	// sends at most an occasional PING, so these ceilings are far above honest
	// traffic but cap a flooding connection. The token bucket and strike counter
	// are touched only by readPump (the single reader goroutine) — no locking.
	inboundBurst        = 20  // bucket capacity (frames)
	inboundRefillPerSec = 5.0 // sustained inbound frames/sec allowed
	maxInboundStrikes   = 100 // force-close after this many dropped+malformed frames
)

// tokenBucket is a lock-free token bucket. It is safe only when accessed from a
// single goroutine (readPump).
type tokenBucket struct {
	tokens float64
	last   time.Time
}

func (b *tokenBucket) allow(now time.Time) bool {
	if b.last.IsZero() {
		b.last = now
		b.tokens = inboundBurst
	}
	b.tokens += now.Sub(b.last).Seconds() * inboundRefillPerSec
	if b.tokens > inboundBurst {
		b.tokens = inboundBurst
	}
	b.last = now
	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// Client represents a single WebSocket connection.
// readPump and writePump are the ONLY goroutines that touch the websocket.Conn —
// gorilla/websocket.Conn is not safe for concurrent reads or writes.
type Client struct {
	id            string
	sessionID     uuid.UUID
	participantID int64
	conn          *websocket.Conn
	send          chan []byte
	hub           *Hub
	logger        zerolog.Logger
	closeOnce     sync.Once

	inbound tokenBucket // readPump-only
	strikes int         // readPump-only
}

// closeSend closes the send channel exactly once. The hub may close it from
// three separate code paths (removeFromRoom, broadcastToRoom eviction,
// drainAll); sync.Once prevents the double-close panic.
func (c *Client) closeSend() {
	c.closeOnce.Do(func() { close(c.send) })
}

func newClient(sessionID uuid.UUID, participantID int64, conn *websocket.Conn, hub *Hub, logger zerolog.Logger) *Client {
	id := uuid.NewString()
	return &Client{
		id:            id,
		sessionID:     sessionID,
		participantID: participantID,
		conn:          conn,
		send:          make(chan []byte, sendChanBuffer),
		hub:           hub,
		logger:        logger.With().Str("client_id", id).Logger(),
	}
}

// readPump pumps messages from the WebSocket to the hub.
// It is the only goroutine allowed to read from the connection.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.logger.Warn().Err(err).Msg("websocket unexpected close")
			}
			break
		}
		c.hub.metrics.WSInboundMessagesTotal.Inc()

		// Per-connection inbound flood protection. A flooding client burns through
		// its bucket; each over-limit frame is dropped and counted as a strike.
		if !c.inbound.allow(time.Now()) {
			c.hub.metrics.WSInboundDroppedTotal.Inc()
			if c.recordStrike("inbound_rate_exceeded") {
				break
			}
			continue
		}

		// Handle client-originated PING — server echoes PONG.
		var env Envelope
		if err := json.Unmarshal(message, &env); err != nil {
			c.hub.metrics.WSMalformedEventsTotal.Inc()
			if c.recordStrike("malformed_frame") {
				break
			}
			continue
		}
		if env.Event == EventPing {
			pong, _ := json.Marshal(Envelope{Event: EventPong, SessionID: c.sessionID, Timestamp: time.Now().UTC()})
			select {
			case c.send <- pong:
			default:
			}
		}
		// Server is push-only for MVP; other well-formed client messages are ignored.
	}
}

// recordStrike increments the abuse strike counter and reports whether the
// connection has exhausted its budget and must be force-closed. Called only
// from readPump.
func (c *Client) recordStrike(reason string) bool {
	c.strikes++
	if c.strikes < maxInboundStrikes {
		return false
	}
	c.hub.metrics.WSAbusiveClosesTotal.Inc()
	c.logger.Warn().
		Int64("participant_id", c.participantID).
		Str("session_id", c.sessionID.String()).
		Str("reason", reason).
		Int("strikes", c.strikes).
		Msg("force-closing abusive WebSocket connection")
	return true
}

// writePump pumps messages from the hub to the WebSocket connection.
// It is the only goroutine allowed to write to the connection.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
