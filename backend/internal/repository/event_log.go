package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Mohith1612/qr-dining/internal/authz"
	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// GetEventsBySession returns up to 200 events for a session in chronological order.
func (r *Repos) GetEventsBySession(ctx context.Context, sessionID uuid.UUID) ([]sqlc.EventLog, error) {
	return r.q.GetEventsBySession(ctx, pgtype.UUID{Bytes: sessionID, Valid: true})
}

// GetRecentEventsByBranch returns the 100 most recent events for a branch (newest first).
func (r *Repos) GetRecentEventsByBranch(ctx context.Context, branchID int64) ([]sqlc.EventLog, error) {
	return r.q.GetRecentEventsByBranch(ctx, pgtype.Int8{Int64: branchID, Valid: true})
}

// LogEvent persists an operational event to the audit log.
// Errors are logged and swallowed — event logging must never block a service call.
// Call this after the primary DB operation succeeds, never inside a WithTx callback.
func (r *Repos) LogEvent(ctx context.Context, sessionID uuid.UUID, branchID int64, eventType, actorType string, actorID int64, payload any) {
	raw, err := json.Marshal(payload)
	if err != nil {
		r.logger.Error().Err(err).Str("event_type", eventType).Msg("event_log: marshal payload")
		return
	}

	params := sqlc.InsertEventLogParams{
		SessionID: pgtype.UUID{Bytes: sessionID, Valid: sessionID != uuid.Nil},
		BranchID:  pgtype.Int8{Int64: branchID, Valid: branchID != 0},
		EventType: eventType,
		ActorType: actorType,
		ActorID:   pgtype.Text{String: fmt.Sprintf("%d", actorID), Valid: actorID != 0},
		Payload:   json.RawMessage(raw),
	}

	if err := r.q.InsertEventLog(ctx, params); err != nil {
		r.logger.Error().Err(err).Str("event_type", eventType).Msg("event_log: insert")
	}
}

func (r *Repos) LogAuthzDenied(ctx context.Context, actor authz.Actor, action authz.Action, resource authz.Resource, decision authz.Decision, requestID string) {
	r.logger.Warn().
		Str("request_id", requestID).
		Str("action", string(action)).
		Str("actor_type", string(actor.Type)).
		Int64("actor_id", actor.ID).
		Str("actor_role", string(actor.Role)).
		Int64("actor_branch_id", decision.ActorScope.BranchID).
		Int64("actor_organization_id", decision.ActorScope.OrganizationID).
		Str("resource_type", string(resource.Type)).
		Str("resource_id", resource.ID).
		Int64("resource_branch_id", decision.ResourceScope.BranchID).
		Int64("resource_organization_id", decision.ResourceScope.OrganizationID).
		Str("reason", decision.Reason).
		Msg("authorization denied")

	if decision.ResourceScope.BranchID == 0 && decision.ResourceScope.SessionID == uuid.Nil {
		return
	}
	r.LogEvent(ctx, decision.ResourceScope.SessionID, decision.ResourceScope.BranchID, "AUTHZ_DENIED", string(actor.Type), actor.ID, map[string]any{
		"request_id":               requestID,
		"action":                   action,
		"resource_type":            resource.Type,
		"resource_id":              resource.ID,
		"reason":                   decision.Reason,
		"actor_branch_id":          decision.ActorScope.BranchID,
		"actor_organization_id":    decision.ActorScope.OrganizationID,
		"resource_branch_id":       decision.ResourceScope.BranchID,
		"resource_organization_id": decision.ResourceScope.OrganizationID,
	})
}
