package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type SupportSearchResult struct {
	Type              string    `json:"type"`
	ID                string    `json:"id"`
	Reference         string    `json:"reference"`
	OrganizationID    int64     `json:"organization_id"`
	OrganizationCode  string    `json:"organization_code"`
	BranchID          int64     `json:"branch_id"`
	BranchCode        string    `json:"branch_code"`
	Label             string    `json:"label"`
	Status            string    `json:"status"`
	RelatedSessionID  string    `json:"related_session_id,omitempty"`
	RelatedSessionRef string    `json:"related_session_reference,omitempty"`
	RelatedOrderID    string    `json:"related_order_id,omitempty"`
	RelatedOrderRef   string    `json:"related_order_reference,omitempty"`
	RelatedPaymentID  string    `json:"related_payment_id,omitempty"`
	RelatedPaymentRef string    `json:"related_payment_reference,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

func (r *Repos) SearchSupportReferences(ctx context.Context, rawQuery string, limit int) ([]SupportSearchResult, error) {
	query := strings.TrimSpace(rawQuery)
	if query == "" {
		return []SupportSearchResult{}, nil
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	like := "%" + query + "%"
	results := make([]SupportSearchResult, 0, limit)

	searches := []func(context.Context, string, string, int) ([]SupportSearchResult, error){
		r.searchOrganizations,
		r.searchBranches,
		r.searchTables,
		r.searchSessions,
		r.searchParticipants,
		r.searchOrders,
		r.searchPayments,
		r.searchAuditEvents,
	}
	for _, search := range searches {
		if len(results) >= limit {
			break
		}
		rows, err := search(ctx, query, like, limit-len(results))
		if err != nil {
			return nil, err
		}
		results = append(results, rows...)
	}
	return results, nil
}

func (r *Repos) searchOrganizations(ctx context.Context, query, like string, limit int) ([]SupportSearchResult, error) {
	rows, err := r.db.Query(ctx, `
SELECT id, code, name, status, created_at
FROM organizations
WHERE code ILIKE $1 OR name ILIKE $1 OR id::text = $2
ORDER BY created_at DESC
LIMIT $3
`, like, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []SupportSearchResult{}
	for rows.Next() {
		var res SupportSearchResult
		res.Type = "organization"
		if err := rows.Scan(&res.OrganizationID, &res.OrganizationCode, &res.Label, &res.Status, &res.CreatedAt); err != nil {
			return nil, err
		}
		res.ID = fmt.Sprintf("%d", res.OrganizationID)
		res.Reference = res.OrganizationCode
		out = append(out, res)
	}
	return out, rows.Err()
}

func (r *Repos) searchBranches(ctx context.Context, query, like string, limit int) ([]SupportSearchResult, error) {
	rows, err := r.db.Query(ctx, `
SELECT b.id, b.branch_code, b.name, b.status, b.organization_id, o.code, b.created_at
FROM branches b
JOIN organizations o ON o.id = b.organization_id
WHERE b.branch_code ILIKE $1 OR b.name ILIKE $1 OR b.id::text = $2
ORDER BY b.created_at DESC
LIMIT $3
`, like, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []SupportSearchResult{}
	for rows.Next() {
		var res SupportSearchResult
		res.Type = "branch"
		if err := rows.Scan(&res.BranchID, &res.BranchCode, &res.Label, &res.Status, &res.OrganizationID, &res.OrganizationCode, &res.CreatedAt); err != nil {
			return nil, err
		}
		res.ID = fmt.Sprintf("%d", res.BranchID)
		res.Reference = res.BranchCode
		out = append(out, res)
	}
	return out, rows.Err()
}

func (r *Repos) searchSessions(ctx context.Context, query, like string, limit int) ([]SupportSearchResult, error) {
	parsedID, hasUUID := parseOptionalUUID(query)
	rows, err := r.db.Query(ctx, `
SELECT s.id, s.session_number, s.status::text, s.created_at, s.branch_id, b.branch_code, b.organization_id, o.code, t.identifier
FROM sessions s
JOIN branches b ON b.id = s.branch_id
JOIN organizations o ON o.id = b.organization_id
JOIN tables t ON t.id = s.table_id
WHERE s.session_number ILIKE $1 OR ($2::uuid IS NOT NULL AND s.id = $2::uuid)
ORDER BY s.created_at DESC
LIMIT $3
`, like, nullableUUIDArg(parsedID, hasUUID), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []SupportSearchResult{}
	for rows.Next() {
		var id uuid.UUID
		var tableLabel string
		var res SupportSearchResult
		res.Type = "session"
		if err := rows.Scan(&id, &res.Reference, &res.Status, &res.CreatedAt, &res.BranchID, &res.BranchCode, &res.OrganizationID, &res.OrganizationCode, &tableLabel); err != nil {
			return nil, err
		}
		res.ID = id.String()
		res.Label = "Table " + tableLabel
		out = append(out, res)
	}
	return out, rows.Err()
}

func (r *Repos) searchOrders(ctx context.Context, query, like string, limit int) ([]SupportSearchResult, error) {
	parsedID, hasUUID := parseOptionalUUID(query)
	rows, err := r.db.Query(ctx, `
SELECT ord.id, ord.order_operational_id, ord.status::text, ord.created_at, ord.branch_id, b.branch_code,
       b.organization_id, org.code, s.id, s.session_number
FROM orders ord
JOIN branches b ON b.id = ord.branch_id
JOIN organizations org ON org.id = b.organization_id
JOIN sessions s ON s.id = ord.session_id
WHERE ord.order_operational_id ILIKE $1
   OR ord.order_number_display ILIKE $1
   OR ($2::uuid IS NOT NULL AND ord.id = $2::uuid)
ORDER BY ord.created_at DESC
LIMIT $3
`, like, nullableUUIDArg(parsedID, hasUUID), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []SupportSearchResult{}
	for rows.Next() {
		var id, sessionID uuid.UUID
		var res SupportSearchResult
		res.Type = "order"
		if err := rows.Scan(&id, &res.Reference, &res.Status, &res.CreatedAt, &res.BranchID, &res.BranchCode, &res.OrganizationID, &res.OrganizationCode, &sessionID, &res.RelatedSessionRef); err != nil {
			return nil, err
		}
		res.ID = id.String()
		res.Label = res.Reference
		res.RelatedSessionID = sessionID.String()
		out = append(out, res)
	}
	return out, rows.Err()
}

func (r *Repos) searchPayments(ctx context.Context, query, like string, limit int) ([]SupportSearchResult, error) {
	rows, err := r.db.Query(ctx, `
SELECT p.id, p.payment_reference, p.status::text, p.initiated_at, p.branch_id, b.branch_code,
       b.organization_id, org.code, s.id, s.session_number, ord.id, COALESCE(ord.order_operational_id, '')
FROM payments p
JOIN branches b ON b.id = p.branch_id
JOIN organizations org ON org.id = b.organization_id
JOIN sessions s ON s.id = p.session_id
LEFT JOIN orders ord ON ord.id = p.order_id
WHERE p.payment_reference ILIKE $1
   OR p.provider_payment_ref ILIKE $1
   OR p.id::text = $2
ORDER BY p.initiated_at DESC
LIMIT $3
`, like, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []SupportSearchResult{}
	for rows.Next() {
		var sessionID uuid.UUID
		var orderID pgtype.UUID
		var res SupportSearchResult
		res.Type = "payment"
		var id int64
		if err := rows.Scan(&id, &res.Reference, &res.Status, &res.CreatedAt, &res.BranchID, &res.BranchCode, &res.OrganizationID, &res.OrganizationCode, &sessionID, &res.RelatedSessionRef, &orderID, &res.RelatedOrderRef); err != nil {
			return nil, err
		}
		res.ID = fmt.Sprintf("%d", id)
		res.Label = res.Reference
		res.RelatedSessionID = sessionID.String()
		if orderID.Valid {
			res.RelatedOrderID = uuid.UUID(orderID.Bytes).String()
		}
		out = append(out, res)
	}
	return out, rows.Err()
}

func (r *Repos) searchAuditEvents(ctx context.Context, query, like string, limit int) ([]SupportSearchResult, error) {
	rows, err := r.db.Query(ctx, `
SELECT al.id, al.event_reference, al.result::text, al.created_at,
       COALESCE(al.branch_id, 0), COALESCE(b.branch_code, ''),
       COALESCE(al.organization_id, 0), COALESCE(org.code, ''), al.action
FROM audit_log al
LEFT JOIN branches b ON b.id = al.branch_id
LEFT JOIN organizations org ON org.id = al.organization_id
WHERE al.event_reference ILIKE $1 OR al.id::text = $2 OR al.correlation_id ILIKE $1 OR al.request_id ILIKE $1
ORDER BY al.created_at DESC
LIMIT $3
`, like, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []SupportSearchResult{}
	for rows.Next() {
		var id int64
		var res SupportSearchResult
		res.Type = "audit_event"
		if err := rows.Scan(&id, &res.Reference, &res.Status, &res.CreatedAt, &res.BranchID, &res.BranchCode, &res.OrganizationID, &res.OrganizationCode, &res.Label); err != nil {
			return nil, err
		}
		res.ID = fmt.Sprintf("%d", id)
		out = append(out, res)
	}
	return out, rows.Err()
}

func (r *Repos) searchTables(ctx context.Context, query, like string, limit int) ([]SupportSearchResult, error) {
	rows, err := r.db.Query(ctx, `
SELECT t.id, t.identifier, t.status::text, t.created_at, t.branch_id, b.branch_code, b.organization_id, o.code
FROM tables t
JOIN branches b ON b.id = t.branch_id
JOIN organizations o ON o.id = b.organization_id
WHERE t.identifier ILIKE $1 OR t.qr_code_token = $2 OR t.id::text = $2
ORDER BY t.created_at DESC
LIMIT $3
`, like, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []SupportSearchResult{}
	for rows.Next() {
		var id int64
		var res SupportSearchResult
		res.Type = "table"
		if err := rows.Scan(&id, &res.Reference, &res.Status, &res.CreatedAt, &res.BranchID, &res.BranchCode, &res.OrganizationID, &res.OrganizationCode); err != nil {
			return nil, err
		}
		res.ID = fmt.Sprintf("%d", id)
		res.Label = "Table " + res.Reference
		out = append(out, res)
	}
	return out, rows.Err()
}

// searchParticipants matches by display name or phone (PII — callers audit the
// search). Returns the participant's session as the related entity so an operator
// can jump straight to the session inspection view.
func (r *Repos) searchParticipants(ctx context.Context, query, like string, limit int) ([]SupportSearchResult, error) {
	rows, err := r.db.Query(ctx, `
SELECT sp.id, sp.display_name, s.status::text, sp.joined_at, s.branch_id, b.branch_code,
       b.organization_id, o.code, s.id, s.session_number
FROM session_participants sp
JOIN sessions s ON s.id = sp.session_id
JOIN branches b ON b.id = s.branch_id
JOIN organizations o ON o.id = b.organization_id
WHERE sp.display_name ILIKE $1 OR sp.phone_e164 ILIKE $1 OR sp.id::text = $2
ORDER BY sp.joined_at DESC
LIMIT $3
`, like, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []SupportSearchResult{}
	for rows.Next() {
		var id int64
		var sessionID uuid.UUID
		var res SupportSearchResult
		res.Type = "participant"
		if err := rows.Scan(&id, &res.Label, &res.Status, &res.CreatedAt, &res.BranchID, &res.BranchCode, &res.OrganizationID, &res.OrganizationCode, &sessionID, &res.RelatedSessionRef); err != nil {
			return nil, err
		}
		res.ID = fmt.Sprintf("%d", id)
		res.Reference = res.Label
		res.RelatedSessionID = sessionID.String()
		out = append(out, res)
	}
	return out, rows.Err()
}

func parseOptionalUUID(raw string) (uuid.UUID, bool) {
	id, err := uuid.Parse(raw)
	return id, err == nil
}

func nullableUUIDArg(id uuid.UUID, ok bool) any {
	if !ok {
		return nil
	}
	return id
}
