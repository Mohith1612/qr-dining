# WebSocket Events Reference

All events are delivered as JSON text frames over the WebSocket connection at `GET /ws`.

## Envelope shape

Every message follows this envelope:

```json
{
  "event": "<event_name>",
  "session_id": "<uuid>",
  "timestamp": "2026-05-17T10:00:00Z",
  "data": { ... }
}
```

Clients should always key on `event` — never on `data` shape alone.

---

## Session Events

### `session.created`

Fired when a new dining session is opened.

**When:** `POST /sessions` succeeds  
**Recipients:** All clients in the session room

```json
{
  "event": "session.created",
  "session_id": "550e8400-e29b-41d4-a716-446655440000",
  "timestamp": "2026-05-17T10:00:00Z",
  "data": {
    "session": { ... },
    "participant": { ... }
  }
}
```

---

### `session.closed`

Fired when the host closes the session.

**When:** `DELETE /sessions/:id` succeeds  
**Recipients:** All clients in the session room

```json
{
  "event": "session.closed",
  "session_id": "...",
  "timestamp": "...",
  "data": { "session_id": "..." }
}
```

---

## Participant Events

### `participant.joined`

Fired when a new participant joins an existing session.

**When:** `POST /sessions/:id/join` succeeds  
**Recipients:** All clients in the session room

```json
{
  "event": "participant.joined",
  "session_id": "...",
  "timestamp": "...",
  "data": {
    "id": 2,
    "session_id": "...",
    "display_name": "Bob",
    "is_host": false,
    "joined_at": "..."
  }
}
```

---

## Cart Events

### `cart.updated`

Fired when any participant adds or removes a cart item.

**When:** `POST /sessions/:id/cart/items` or `DELETE /sessions/:id/cart/items/:item_id`  
**Recipients:** All clients in the session room

```json
{
  "event": "cart.updated",
  "session_id": "...",
  "timestamp": "...",
  "data": { ... }
}
```

---

## Order Events

### `order.placed`

Fired when a new order is successfully placed.

**When:** `POST /sessions/:id/orders` creates a new order (not an idempotency replay)  
**Recipients:** All clients in the session room

```json
{
  "event": "order.placed",
  "session_id": "...",
  "timestamp": "...",
  "data": {
    "order": {
      "id": "...",
      "status": "pending",
      "total_amount": "24.50",
      ...
    }
  }
}
```

---

### `order.confirmed`

Fired when kitchen staff confirms an order.

**When:** `PATCH /orders/:id/status` with `status: "confirmed"`  
**Recipients:** All clients in the session room

```json
{
  "event": "order.confirmed",
  "session_id": "...",
  "timestamp": "...",
  "data": { "order": { "id": "...", "status": "confirmed", ... } }
}
```

---

### `order.preparing`

Fired when kitchen marks the order as being prepared.

**When:** `PATCH /orders/:id/status` with `status: "preparing"`

```json
{
  "event": "order.preparing",
  "session_id": "...",
  "timestamp": "...",
  "data": { "order": { ... } }
}
```

---

### `order.ready`

Fired when the order is ready for service.

**When:** `PATCH /orders/:id/status` with `status: "ready"`

```json
{
  "event": "order.ready",
  "session_id": "...",
  "timestamp": "...",
  "data": { "order": { ... } }
}
```

---

### `order.served`

Fired when the order has been delivered to the table.

**When:** `PATCH /orders/:id/status` with `status: "served"`

```json
{
  "event": "order.served",
  "session_id": "...",
  "timestamp": "...",
  "data": { "order": { ... } }
}
```

---

### `order.cancelled`

Fired when an order is cancelled.

**When:** `PATCH /orders/:id/status` with `status: "cancelled"`

```json
{
  "event": "order.cancelled",
  "session_id": "...",
  "timestamp": "...",
  "data": { "order": { ... } }
}
```

---

## Assistance Events

### `assistance.requested`

Fired when a guest calls for assistance.

**When:** `POST /sessions/:id/assist` succeeds  
**Recipients:** All clients in the session room (including kitchen display if connected)

```json
{
  "event": "assistance.requested",
  "session_id": "...",
  "timestamp": "...",
  "data": {
    "id": 5,
    "type": "bill",
    "status": "pending",
    "table_id": 3,
    ...
  }
}
```

---

### `assistance.acknowledged`

Fired when staff acknowledges an assistance request.

**When:** `PATCH /assist/:id/ack` succeeds

```json
{
  "event": "assistance.acknowledged",
  "session_id": "...",
  "timestamp": "...",
  "data": { "id": 5, "status": "acknowledged", ... }
}
```

---

### `assistance.resolved`

Fired when staff marks the request as resolved.

**When:** `PATCH /assist/:id/resolve` succeeds

```json
{
  "event": "assistance.resolved",
  "session_id": "...",
  "timestamp": "...",
  "data": { "id": 5, "status": "resolved", ... }
}
```

---

## Payment Events

### `payment.initiated`

Fired when a payment is initiated for a session.

**When:** `POST /sessions/:id/payments` succeeds

```json
{
  "event": "payment.initiated",
  "session_id": "...",
  "timestamp": "...",
  "data": {
    "id": 1,
    "amount": "45.00",
    "method": "card",
    "status": "pending",
    ...
  }
}
```

---

### `payment.completed`

Fired when the payment provider webhook confirms completion.

**When:** `POST /webhooks/payments/:provider` processes a successful payment

```json
{
  "event": "payment.completed",
  "session_id": "...",
  "timestamp": "...",
  "data": { "id": 1, "status": "completed", ... }
}
```

---

## System Events

### `ping` / `pong`

Client-initiated keepalive. Client sends `{"event":"ping","session_id":"..."}`;
server echoes `{"event":"pong","session_id":"...","timestamp":"..."}`.

The server also sends periodic WebSocket-level PING frames (gorilla/websocket).
Clients should respond with PONG frames to stay connected.

---

## Error Code Registry (stable)

| Code | HTTP | Triggered by |
|------|------|--------------|
| `SESSION_NOT_FOUND` | 404 | Session UUID does not exist |
| `SESSION_CLOSED` | 409 | Operation on a closed session |
| `SESSION_ALREADY_ACTIVE` | 409 | Table already has an active session |
| `NOT_SESSION_HOST` | 403 | Closing a session without being host |
| `PARTICIPANT_NOT_FOUND` | 404 | Participant not in session |
| `MENU_ITEM_NOT_FOUND` | 404 | Menu item ID does not exist |
| `MENU_ITEM_UNAVAILABLE` | 422 | Item marked is_available=false |
| `ORDER_NOT_FOUND` | 404 | Order UUID does not exist |
| `INVALID_ORDER_TRANSITION` | 422 | Invalid state machine transition |
| `ASSISTANCE_NOT_FOUND` | 404 | Assistance request ID does not exist |
| `INVALID_ASSISTANCE_TRANSITION` | 422 | Invalid assistance state transition |
| `PAYMENT_NOT_FOUND` | 404 | Payment ID does not exist |
| `INVALID_PAYMENT_TRANSITION` | 422 | Invalid payment state transition |
| `UNAUTHORIZED` | 401 | Missing/invalid auth |
| `FORBIDDEN` | 403 | Wrong role for operation |
| `RATE_LIMITED` | 429 | Rate limit exceeded |
| `VALIDATION_ERROR` | 400 | Malformed request |
| `INTERNAL_ERROR` | 500 | Unhandled server error |
