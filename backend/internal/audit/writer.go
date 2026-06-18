package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
)

// Writer records audit events to the audit_log table.
// All writes are non-fatal: errors are logged and counted but never crash the request.
// When enabled=false (AUDIT_LOG_V2_ENABLED=false), Record is a no-op.
type Writer struct {
	q              sqlc.Querier
	enabled        bool
	logger         zerolog.Logger
	failureCounter *prometheus.CounterVec
}

// NewWriter constructs a Writer backed by a pool-level Querier (never tx-backed).
// failureCounter is the AuditWriteFailuresTotal counter vec with labels {action, error_class}.
func NewWriter(q sqlc.Querier, enabled bool, logger zerolog.Logger, failureCounter *prometheus.CounterVec) *Writer {
	return &Writer{q: q, enabled: enabled, logger: logger, failureCounter: failureCounter}
}

// Record persists an audit event. Safe to call after any mutation; never blocks the caller.
// Request context fields (IP, UserAgent, RequestID, CorrelationID, Source) are read from
// the context if the audit middleware populated them and the caller did not set them explicitly.
func (w *Writer) Record(ctx context.Context, ev AuditEvent) {
	if !w.enabled {
		return
	}

	reqCtx := FromContext(ctx)
	if ev.RequestID == "" {
		ev.RequestID = reqCtx.RequestID
	}
	if ev.CorrelationID == "" {
		ev.CorrelationID = reqCtx.CorrelationID
	}
	if ev.IP == "" {
		ev.IP = reqCtx.IP
	}
	if ev.UserAgent == "" {
		ev.UserAgent = reqCtx.UserAgent
	}
	if ev.Source == "" {
		if reqCtx.Source != "" {
			ev.Source = reqCtx.Source
		} else {
			ev.Source = SourceAPI
		}
	}
	if ev.Result == "" {
		ev.Result = ResultSuccess
	}
	if ev.RiskLevel == "" {
		ev.RiskLevel = RiskLow
	}

	ev.Before, ev.After = Redact(ev.Before, ev.After)

	actorScope := toJSON(ev.ActorScope)
	metadata := toJSON(ev.Metadata)

	params := sqlc.InsertAuditLogParams{
		OrganizationID: optInt8(ev.OrganizationID),
		BranchID:       optInt8(ev.BranchID),
		RestaurantID:   optInt8(ev.RestaurantID),
		SessionID:      optUUID(ev.SessionID),
		TableID:        optInt8(ev.TableID),
		ResourceType:   ev.ResourceType,
		ResourceID:     ev.ResourceID,
		Action:         ev.Action,
		Result:         sqlc.AuditResultType(ev.Result),
		ActorType:      sqlc.AuditActorType(ev.ActorType),
		ActorID:        ev.ActorID,
		ActorDisplay:   ev.ActorDisplay,
		ActorScopeJson: actorScope,
		RequestID:      ev.RequestID,
		CorrelationID:  ev.CorrelationID,
		IdempotencyKey: ev.IdempotencyKey,
		Ip:             ev.IP,
		UserAgent:      ev.UserAgent,
		Source:         sqlc.AuditSourceType(ev.Source),
		BeforeJson:     ev.Before,
		AfterJson:      ev.After,
		MetadataJson:   metadata,
		RiskLevel:      sqlc.AuditRiskLevel(ev.RiskLevel),
	}

	if err := w.q.InsertAuditLog(ctx, params); err != nil {
		errClass := errorClass(err)
		w.logger.Error().
			Err(err).
			Str("audit_action", ev.Action).
			Str("error_class", errClass).
			Str("resource_type", ev.ResourceType).
			Msg("audit_log: write failed")
		if w.failureCounter != nil {
			w.failureCounter.WithLabelValues(ev.Action, errClass).Inc()
		}
	}
}

func toJSON(v any) json.RawMessage {
	if v == nil {
		return json.RawMessage(`{}`)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return b
}

func optInt8(v int64) pgtype.Int8 {
	return pgtype.Int8{Int64: v, Valid: v != 0}
}

func optUUID(v uuid.UUID) pgtype.UUID {
	if v == uuid.Nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: v, Valid: true}
}

func errorClass(err error) string {
	msg := err.Error()
	switch {
	case contains(msg, "timeout", "deadline"):
		return "timeout"
	case contains(msg, "connection"):
		return "connection"
	case contains(msg, "immutable"):
		return "immutable"
	default:
		return "db_error"
	}
}

func contains(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}

// MustJSON marshals v to RawMessage. Returns nil on nil input or marshal error.
// Use for Before/After fields in AuditEvent where nil means "no change data".
func MustJSON(v any) json.RawMessage {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
}

// IDStr formats an int64 as a string for ResourceID/ActorID fields.
func IDStr(id int64) string {
	return fmt.Sprintf("%d", id)
}
