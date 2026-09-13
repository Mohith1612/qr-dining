package services

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// SupportService assembles READ-ONLY platform-operator views of tenant operational
// data from existing repository reads. It performs NO mutations and never mints or
// validates guest tokens — it is a separate, observability-only read path. All
// money fields are stringified and guest credentials (session_token,
// device_fingerprint) are deliberately omitted from every DTO.
type SupportService struct {
	repos *repository.Repos
}

func NewSupportService(repos *repository.Repos) *SupportService {
	return &SupportService{repos: repos}
}

// ── DTOs (sanitized) ─────────────────────────────────────────────────────────

type SupportOrgRef struct {
	ID   int64  `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

type SupportBranchRef struct {
	ID         int64  `json:"id"`
	BranchCode string `json:"branch_code"`
	Name       string `json:"name"`
	Timezone   string `json:"timezone"`
}

type SupportTableRef struct {
	ID         int64  `json:"id"`
	Identifier string `json:"identifier"`
	Status     string `json:"status"`
}

type SupportSession struct {
	ID                     string     `json:"id"`
	SessionNumber          string     `json:"session_number"`
	Status                 string     `json:"status"`
	HostParticipantID      *int64     `json:"host_participant_id"`
	VisitNumber            int32      `json:"visit_number"`
	CreatedAt              time.Time  `json:"created_at"`
	ClosedAt               *time.Time `json:"closed_at"`
	AwaitingReactivationAt *time.Time `json:"awaiting_reactivation_at"`
}

type SupportParticipant struct {
	ID            int64      `json:"id"`
	DisplayName   string     `json:"display_name"`
	Phone         string     `json:"phone_e164,omitempty"`
	IsHost        bool       `json:"is_host"`
	JoinedAt      time.Time  `json:"joined_at"`
	LastSeenAt    time.Time  `json:"last_seen_at"`
	RevokedAt     *time.Time `json:"revoked_at"`
	RevokedReason string     `json:"revoked_reason,omitempty"`
}

type SupportOrderItem struct {
	ID         int64           `json:"id"`
	MenuItemID int64           `json:"menu_item_id"`
	Quantity   int16           `json:"quantity"`
	UnitPrice  string          `json:"unit_price"`
	Modifiers  json.RawMessage `json:"selected_modifiers_json,omitempty"`
	Note       string          `json:"note,omitempty"`
}

type SupportOrder struct {
	ID                 string             `json:"id"`
	OrderOperationalID string             `json:"order_operational_id"`
	OrderNumberDisplay string             `json:"order_number_display"`
	Status             string             `json:"status"`
	TotalAmount        string             `json:"total_amount"`
	DiscountAmount     string             `json:"discount_amount"`
	CreatedAt          time.Time          `json:"created_at"`
	UpdatedAt          time.Time          `json:"updated_at"`
	Items              []SupportOrderItem `json:"items,omitempty"`
}

type SupportPayment struct {
	ID                 int64      `json:"id"`
	PaymentReference   string     `json:"payment_reference"`
	Status             string     `json:"status"`
	Amount             string     `json:"amount"`
	Currency           string     `json:"currency"`
	Provider           string     `json:"provider,omitempty"`
	ProviderPaymentRef string     `json:"provider_payment_ref,omitempty"`
	ProviderOrderRef   string     `json:"provider_order_ref,omitempty"`
	BillSnapshotID     *int64     `json:"bill_snapshot_id"`
	SettledByStaffID   *int64     `json:"settled_by_staff_id"`
	SettledAt          *time.Time `json:"settled_at"`
	InitiatedAt        time.Time  `json:"initiated_at"`
	CompletedAt        *time.Time `json:"completed_at"`
}

type SupportAssistance struct {
	ID         int64      `json:"id"`
	Type       string     `json:"type"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	ResolvedAt *time.Time `json:"resolved_at"`
}

type SupportEvent struct {
	ID        int64           `json:"id"`
	EventType string          `json:"event_type"`
	ActorType string          `json:"actor_type"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

type SupportBillSnapshot struct {
	ID             int64           `json:"id"`
	Subtotal       string          `json:"subtotal"`
	DiscountAmount string          `json:"discount_amount"`
	TaxAmount      string          `json:"tax_amount"`
	ServiceCharge  string          `json:"service_charge"`
	TipAmount      string          `json:"tip_amount"`
	Total          string          `json:"total"`
	Currency       string          `json:"currency"`
	SourceOrderIDs json.RawMessage `json:"source_order_ids,omitempty"`
	CreatedByActor string          `json:"created_by_actor"`
	CreatedAt      time.Time       `json:"created_at"`
}

type SupportWebhookEvent struct {
	ID              int64      `json:"id"`
	ExternalEventID string     `json:"external_event_id"`
	Provider        string     `json:"provider"`
	EventType       string     `json:"event_type"`
	Processed       bool       `json:"processed"`
	ProcessedAt     *time.Time `json:"processed_at"`
	ErrorMessage    string     `json:"error_message,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

type SupportSessionDetail struct {
	Session      SupportSession       `json:"session"`
	Organization SupportOrgRef        `json:"organization"`
	Branch       SupportBranchRef     `json:"branch"`
	Table        SupportTableRef      `json:"table"`
	Participants []SupportParticipant `json:"participants"`
	Orders       []SupportOrder       `json:"orders"`
	Payments     []SupportPayment     `json:"payments"`
	Assistance   []SupportAssistance  `json:"assistance"`
	Timeline     []SupportEvent       `json:"timeline"`
}

type SupportOrderDetail struct {
	Order        SupportOrder     `json:"order"`
	Organization SupportOrgRef    `json:"organization"`
	Branch       SupportBranchRef `json:"branch"`
	SessionID    string           `json:"session_id"`
	SessionRef   string           `json:"session_number"`
}

type SupportPaymentDetail struct {
	Payment      SupportPayment        `json:"payment"`
	Organization SupportOrgRef         `json:"organization"`
	Branch       SupportBranchRef      `json:"branch"`
	SessionID    string                `json:"session_id"`
	SessionRef   string                `json:"session_number"`
	BillSnapshot *SupportBillSnapshot  `json:"bill_snapshot"`
	Webhooks     []SupportWebhookEvent `json:"webhooks"`
}

// ── Aggregation (read-only) ──────────────────────────────────────────────────

func (s *SupportService) GetSessionDetail(ctx context.Context, sessionID uuid.UUID) (SupportSessionDetail, error) {
	var out SupportSessionDetail
	session, err := s.repos.GetSessionByID(ctx, sessionID)
	if err != nil {
		return out, err
	}
	org, branch, err := s.orgBranch(ctx, session.BranchID)
	if err != nil {
		return out, err
	}
	out.Organization, out.Branch = org, branch
	out.Session = toSupportSession(session)
	if table, terr := s.repos.GetTableByID(ctx, session.TableID); terr == nil {
		out.Table = SupportTableRef{ID: table.ID, Identifier: table.Identifier, Status: string(table.Status)}
	}

	parts, err := s.repos.ListParticipantsBySession(ctx, sessionID)
	if err != nil {
		return out, err
	}
	out.Participants = make([]SupportParticipant, 0, len(parts))
	for _, p := range parts {
		out.Participants = append(out.Participants, toSupportParticipant(p))
	}

	orders, err := s.repos.ListOrdersForSession(ctx, sessionID)
	if err != nil {
		return out, err
	}
	out.Orders = make([]SupportOrder, 0, len(orders))
	for _, o := range orders {
		so := toSupportOrder(o)
		items, ierr := s.repos.ListOrderItems(ctx, o.ID)
		if ierr != nil {
			return out, ierr
		}
		so.Items = toSupportOrderItems(items)
		out.Orders = append(out.Orders, so)
	}

	payments, err := s.repos.ListPaymentsForSession(ctx, sessionID)
	if err != nil {
		return out, err
	}
	out.Payments = make([]SupportPayment, 0, len(payments))
	for _, p := range payments {
		out.Payments = append(out.Payments, toSupportPayment(p))
	}

	assist, err := s.repos.ListAssistanceForSession(ctx, sessionID)
	if err != nil {
		return out, err
	}
	out.Assistance = make([]SupportAssistance, 0, len(assist))
	for _, a := range assist {
		out.Assistance = append(out.Assistance, SupportAssistance{
			ID: a.ID, Type: string(a.Type), Status: string(a.Status),
			CreatedAt: a.CreatedAt, ResolvedAt: optTime(a.ResolvedAt),
		})
	}

	events, err := s.repos.GetEventsBySession(ctx, sessionID)
	if err != nil {
		return out, err
	}
	out.Timeline = make([]SupportEvent, 0, len(events))
	for _, e := range events {
		out.Timeline = append(out.Timeline, SupportEvent{
			ID: e.ID, EventType: e.EventType, ActorType: e.ActorType,
			Payload: e.Payload, CreatedAt: e.CreatedAt,
		})
	}
	return out, nil
}

func (s *SupportService) GetOrderDetail(ctx context.Context, orderID uuid.UUID) (SupportOrderDetail, error) {
	var out SupportOrderDetail
	order, err := s.repos.GetOrderByID(ctx, orderID)
	if err != nil {
		return out, err
	}
	org, branch, err := s.orgBranch(ctx, order.BranchID)
	if err != nil {
		return out, err
	}
	out.Organization, out.Branch = org, branch
	so := toSupportOrder(order)
	items, err := s.repos.ListOrderItems(ctx, orderID)
	if err != nil {
		return out, err
	}
	so.Items = toSupportOrderItems(items)
	out.Order = so
	out.SessionID = order.SessionID.String()
	if session, serr := s.repos.GetSessionByID(ctx, order.SessionID); serr == nil {
		out.SessionRef = session.SessionNumber
	}
	return out, nil
}

func (s *SupportService) GetPaymentDetail(ctx context.Context, paymentID int64) (SupportPaymentDetail, error) {
	var out SupportPaymentDetail
	payment, err := s.repos.GetPaymentByID(ctx, paymentID)
	if err != nil {
		return out, err
	}
	org, branch, err := s.orgBranch(ctx, payment.BranchID)
	if err != nil {
		return out, err
	}
	out.Organization, out.Branch = org, branch
	out.Payment = toSupportPayment(payment)
	out.SessionID = payment.SessionID.String()
	if session, serr := s.repos.GetSessionByID(ctx, payment.SessionID); serr == nil {
		out.SessionRef = session.SessionNumber
	}
	if payment.BillSnapshotID.Valid {
		if bs, berr := s.repos.GetBillSnapshotByID(ctx, payment.BillSnapshotID.Int64); berr == nil {
			snap := toSupportBillSnapshot(bs)
			out.BillSnapshot = &snap
		}
	}
	hooks, err := s.repos.ListWebhookEventsByPayment(ctx, paymentID)
	if err != nil {
		return out, err
	}
	out.Webhooks = make([]SupportWebhookEvent, 0, len(hooks))
	for _, h := range hooks {
		out.Webhooks = append(out.Webhooks, SupportWebhookEvent{
			ID: h.ID, ExternalEventID: h.ExternalEventID, Provider: h.Provider,
			EventType: h.EventType, Processed: h.Processed, ProcessedAt: optTime(h.ProcessedAt),
			ErrorMessage: h.ErrorMessage.String, CreatedAt: h.CreatedAt,
		})
	}
	return out, nil
}

func (s *SupportService) orgBranch(ctx context.Context, branchID int64) (SupportOrgRef, SupportBranchRef, error) {
	branch, err := s.repos.GetBranchByID(ctx, branchID)
	if err != nil {
		return SupportOrgRef{}, SupportBranchRef{}, err
	}
	org, err := s.repos.GetOrganizationByBranchID(ctx, branchID)
	if err != nil {
		return SupportOrgRef{}, SupportBranchRef{}, err
	}
	return SupportOrgRef{ID: org.ID, Code: org.Code, Name: org.Name},
		SupportBranchRef{ID: branch.ID, BranchCode: branch.BranchCode, Name: branch.Name, Timezone: branch.Timezone},
		nil
}

// ── Converters (sanitize + stringify money; omit credentials) ────────────────

func toSupportSession(s sqlc.Session) SupportSession {
	return SupportSession{
		ID:                     s.ID.String(),
		SessionNumber:          s.SessionNumber,
		Status:                 string(s.Status),
		HostParticipantID:      optInt64(s.HostParticipantID),
		VisitNumber:            s.VisitNumber,
		CreatedAt:              s.CreatedAt,
		ClosedAt:               optTime(s.ClosedAt),
		AwaitingReactivationAt: optTime(s.AwaitingReactivationAt),
		// session_token deliberately omitted — it is a guest credential.
	}
}

func toSupportParticipant(p sqlc.SessionParticipant) SupportParticipant {
	return SupportParticipant{
		ID:            p.ID,
		DisplayName:   p.DisplayName,
		Phone:         p.PhoneE164.String,
		IsHost:        p.IsHost,
		JoinedAt:      p.JoinedAt,
		LastSeenAt:    p.LastSeenAt,
		RevokedAt:     optTime(p.RevokedAt),
		RevokedReason: p.RevokedReason.String,
		// device_fingerprint deliberately omitted.
	}
}

func toSupportOrder(o sqlc.Order) SupportOrder {
	return SupportOrder{
		ID:                 o.ID.String(),
		OrderOperationalID: o.OrderOperationalID,
		OrderNumberDisplay: o.OrderNumberDisplay,
		Status:             string(o.Status),
		TotalAmount:        numericString(o.TotalAmount),
		DiscountAmount:     numericString(o.DiscountAmount),
		CreatedAt:          o.CreatedAt,
		UpdatedAt:          o.UpdatedAt,
	}
}

func toSupportOrderItems(items []sqlc.OrderItem) []SupportOrderItem {
	out := make([]SupportOrderItem, 0, len(items))
	for _, it := range items {
		out = append(out, SupportOrderItem{
			ID: it.ID, MenuItemID: it.MenuItemID, Quantity: it.Quantity,
			UnitPrice: numericString(it.UnitPrice), Modifiers: it.SelectedModifiersJson, Note: it.Note,
		})
	}
	return out
}

func toSupportPayment(p sqlc.Payment) SupportPayment {
	return SupportPayment{
		ID:                 p.ID,
		PaymentReference:   p.PaymentReference,
		Status:             string(p.Status),
		Amount:             numericString(p.Amount),
		Currency:           p.Currency,
		Provider:           p.Provider.String,
		ProviderPaymentRef: p.ProviderPaymentRef.String,
		ProviderOrderRef:   p.ProviderOrderRef.String,
		BillSnapshotID:     optInt64(p.BillSnapshotID),
		SettledByStaffID:   optInt64(p.SettledByStaffID),
		SettledAt:          optTime(p.SettledAt),
		InitiatedAt:        p.InitiatedAt,
		CompletedAt:        optTime(p.CompletedAt),
	}
}

func toSupportBillSnapshot(b sqlc.BillSnapshot) SupportBillSnapshot {
	return SupportBillSnapshot{
		ID: b.ID, Subtotal: numericString(b.Subtotal), DiscountAmount: numericString(b.DiscountAmount),
		TaxAmount: numericString(b.TaxAmount), ServiceCharge: numericString(b.ServiceCharge),
		TipAmount: numericString(b.TipAmount), Total: numericString(b.Total), Currency: b.Currency,
		SourceOrderIDs: b.SourceOrderIds, CreatedByActor: b.CreatedByActor, CreatedAt: b.CreatedAt,
	}
}

func optInt64(v pgtype.Int8) *int64 {
	if !v.Valid {
		return nil
	}
	x := v.Int64
	return &x
}

func optTime(v pgtype.Timestamptz) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time
	return &t
}

func numericString(n pgtype.Numeric) string {
	v, err := n.Value()
	if err != nil || v == nil {
		return "0"
	}
	if s, ok := v.(string); ok {
		return s
	}
	return "0"
}
