package websocket

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10 // must be less than pongWait
	maxMessageSize = 4096                  // bytes; prevents memory exhaustion
	sendChanBuffer = 256
)

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
}

func newClient(sessionID uuid.UUID, participantID int64, conn *websocket.Conn, hub *Hub, logger zerolog.Logger) *Client {
	return &Client{
		id:            uuid.NewString(),
		sessionID:     sessionID,
		participantID: participantID,
		conn:          conn,
		send:          make(chan []byte, sendChanBuffer),
		hub:           hub,
		logger:        logger.With().Str("client_id", uuid.NewString()).Logger(),
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
		// Handle client-originated PING — server echoes PONG.
		var env Envelope
		if err := json.Unmarshal(message, &env); err == nil && env.Event == EventPing {
			pong, _ := json.Marshal(Envelope{Event: EventPong, SessionID: c.sessionID, Timestamp: time.Now().UTC()})
			select {
			case c.send <- pong:
			default:
			}
		}
		// Server is push-only for MVP; other client messages are ignored.
	}
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
