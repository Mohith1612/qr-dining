package repository

import (
	"context"
	"encoding/json"

	"github.com/Mohith1612/qr-dining/internal/audit"
	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// LogAuditV2 is a convenience wrapper for callers that hold *Repos but not *audit.Writer.
// Follows the same non-fatal fire-and-forget pattern as LogEvent and LogPlatformAudit.
func (r *Repos) LogAuditV2(ctx context.Context, ev audit.AuditEvent) {
	actorScope, err := json.Marshal(ev.ActorScope)
	if err != nil {
		actorScope = []byte(`{}`)
	}
	metadata, err := json.Marshal(ev.Metadata)
	if err != nil {
		metadata = []byte(`{}`)
	}
	result := ev.Result
	if result == "" {
		result = audit.ResultSuccess
	}
	riskLevel := ev.RiskLevel
	if riskLevel == "" {
		riskLevel = audit.RiskLow
	}
	source := ev.Source
	if source == "" {
		source = audit.SourceSystem
	}

	before, after := audit.Redact(ev.Before, ev.After)

	params := sqlc.InsertAuditLogParams{
		OrganizationID: pgtype.Int8{Int64: ev.OrganizationID, Valid: ev.OrganizationID != 0},
		BranchID:       pgtype.Int8{Int64: ev.BranchID, Valid: ev.BranchID != 0},
		RestaurantID:   pgtype.Int8{Int64: ev.RestaurantID, Valid: ev.RestaurantID != 0},
		SessionID:      pgtype.UUID{Bytes: ev.SessionID, Valid: ev.SessionID != uuid.Nil},
		TableID:        pgtype.Int8{Int64: ev.TableID, Valid: ev.TableID != 0},
		ResourceType:   ev.ResourceType,
		ResourceID:     ev.ResourceID,
		Action:         ev.Action,
		Result:         sqlc.AuditResultType(result),
		ActorType:      sqlc.AuditActorType(ev.ActorType),
		ActorID:        ev.ActorID,
		ActorDisplay:   ev.ActorDisplay,
		ActorScopeJson: json.RawMessage(actorScope),
		RequestID:      ev.RequestID,
		CorrelationID:  ev.CorrelationID,
		IdempotencyKey: ev.IdempotencyKey,
		Ip:             ev.IP,
		UserAgent:      ev.UserAgent,
		Source:         sqlc.AuditSourceType(source),
		BeforeJson:     before,
		AfterJson:      after,
		MetadataJson:   json.RawMessage(metadata),
		RiskLevel:      sqlc.AuditRiskLevel(riskLevel),
	}

	if err := r.q.InsertAuditLog(ctx, params); err != nil {
		r.logger.Error().Err(err).Str("action", ev.Action).Msg("audit_log: insert")
	}
}

func (r *Repos) ListAuditLogForBranch(ctx context.Context, p sqlc.ListAuditLogForBranchParams) ([]sqlc.AuditLog, error) {
	return r.q.ListAuditLogForBranch(ctx, p)
}

func (r *Repos) ListAuditLogForOrganization(ctx context.Context, p sqlc.ListAuditLogForOrganizationParams) ([]sqlc.AuditLog, error) {
	return r.q.ListAuditLogForOrganization(ctx, p)
}

func (r *Repos) ListAuditLogPlatform(ctx context.Context, p sqlc.ListAuditLogPlatformParams) ([]sqlc.AuditLog, error) {
	return r.q.ListAuditLogPlatform(ctx, p)
}
