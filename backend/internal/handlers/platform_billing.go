package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

// Platform subscription & billing management. All routes sit under the /platform
// group (platform auth; staff/guest tokens rejected). Reads allow support/billing/
// auditor; mutations require billing_admin (super_admin bypasses). Every mutation is
// audited via logPlatformAudit.
//
// Resolve-only/shadow: subscription STATUS is recorded + audited but NOT enforced on
// any operational path. Subscription PLAN changes sync organization_plan_assignments
// (entitlement resolver source) inside the service; the resolver is unchanged.

const (
	codeSubscriptionNotFound = "SUBSCRIPTION_NOT_FOUND"
	codeInvoiceNotFound      = "INVOICE_NOT_FOUND"
	codeInvalidTransition    = "INVALID_TRANSITION"
)

// ── request bodies ───────────────────────────────────────────────────────────

type activateSubscriptionRequest struct {
	PlanID    *int64     `json:"plan_id"`
	ExpiresAt *time.Time `json:"expires_at"`
}

type renewSubscriptionRequest struct {
	ExpiresAt *time.Time `json:"expires_at"`
}

type cancelSubscriptionRequest struct {
	Reason string `json:"reason"`
}

type extendTrialRequest struct {
	PlanID      *int64     `json:"plan_id"`
	TrialEndsAt *time.Time `json:"trial_ends_at" binding:"required"`
}

type changePlanRequest struct {
	PlanID int64 `json:"plan_id" binding:"required"`
}

type billingProfileRequest struct {
	BusinessName   string `json:"business_name"`
	GstNumber      string `json:"gst_number"`
	TaxIdentifier  string `json:"tax_identifier"`
	BillingEmail   string `json:"billing_email"`
	BillingContact string `json:"billing_contact"`
	BillingAddress string `json:"billing_address"`
	Currency       string `json:"currency"`
}

type recordPaymentRequest struct {
	Method          string     `json:"method" binding:"required"`
	Amount          string     `json:"amount" binding:"required"`
	Currency        string     `json:"currency"`
	ReferenceNumber string     `json:"reference_number"`
	Notes           string     `json:"notes"`
	ReceivedAt      *time.Time `json:"received_at"`
	InvoiceID       *int64     `json:"invoice_id"`
}

type createInvoiceRequest struct {
	Amount   string     `json:"amount" binding:"required"`
	Currency string     `json:"currency"`
	DueDate  *time.Time `json:"due_date"`
	Notes    string     `json:"notes"`
}

// ── subscription ─────────────────────────────────────────────────────────────

// GetOrganizationSubscription returns the org's subscription (or null if none set).
// GET /platform/organizations/:org_id/subscription
func (h *PlatformHandler) GetOrganizationSubscription(c *gin.Context) {
	session, orgID, ok := h.billingReadContext(c)
	if !ok {
		return
	}
	sub, err := h.billing.GetSubscription(c.Request.Context(), orgID)
	if errors.Is(err, domain.ErrSubscriptionNotFound) {
		h.logPlatformAudit(c, session.PlatformUserID, "platform.subscription.read", "organization", strconv.FormatInt(orgID, 10), orgID, 0, 0, gin.H{})
		c.JSON(http.StatusOK, gin.H{"subscription": nil})
		return
	}
	if err != nil {
		respondInternalError(c)
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.subscription.read", "organization", strconv.FormatInt(orgID, 10), orgID, 0, 0, gin.H{})
	c.JSON(http.StatusOK, gin.H{"subscription": h.subscriptionResponse(c, sub)})
}

// ActivateSubscription creates or activates the org's subscription.
// POST /platform/organizations/:org_id/subscription/activate — billing_admin.
func (h *PlatformHandler) ActivateSubscription(c *gin.Context) {
	session, orgID, ok := h.billingWriteContext(c)
	if !ok {
		return
	}
	var req activateSubscriptionRequest
	if err := bindOptionalJSON(c, &req); err != nil {
		respondValidationError(c, err.Error())
		return
	}
	if req.PlanID != nil && !h.planExists(c, *req.PlanID) {
		return
	}
	sub, err := h.billing.Activate(c.Request.Context(), orgID, req.PlanID, req.ExpiresAt, session.PlatformUserID)
	if !h.handleSubscriptionResult(c, err) {
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.subscription.activate", "organization", strconv.FormatInt(orgID, 10), orgID, 0, 0, gin.H{"plan_id": sub.PlanID})
	c.JSON(http.StatusOK, gin.H{"subscription": h.subscriptionResponse(c, sub)})
}

// SuspendSubscription suspends the org's subscription.
// POST /platform/organizations/:org_id/subscription/suspend — billing_admin.
func (h *PlatformHandler) SuspendSubscription(c *gin.Context) {
	session, orgID, ok := h.billingWriteContext(c)
	if !ok {
		return
	}
	sub, err := h.billing.Suspend(c.Request.Context(), orgID, session.PlatformUserID)
	if !h.handleSubscriptionResult(c, err) {
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.subscription.suspend", "organization", strconv.FormatInt(orgID, 10), orgID, 0, 0, gin.H{})
	c.JSON(http.StatusOK, gin.H{"subscription": h.subscriptionResponse(c, sub)})
}

// RenewSubscription renews the org's subscription and extends expiry.
// POST /platform/organizations/:org_id/subscription/renew — billing_admin.
func (h *PlatformHandler) RenewSubscription(c *gin.Context) {
	session, orgID, ok := h.billingWriteContext(c)
	if !ok {
		return
	}
	var req renewSubscriptionRequest
	if err := bindOptionalJSON(c, &req); err != nil {
		respondValidationError(c, err.Error())
		return
	}
	sub, err := h.billing.Renew(c.Request.Context(), orgID, req.ExpiresAt, session.PlatformUserID)
	if !h.handleSubscriptionResult(c, err) {
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.subscription.renew", "organization", strconv.FormatInt(orgID, 10), orgID, 0, 0, gin.H{})
	c.JSON(http.StatusOK, gin.H{"subscription": h.subscriptionResponse(c, sub)})
}

// CancelSubscription cancels the org's subscription.
// POST /platform/organizations/:org_id/subscription/cancel — billing_admin.
func (h *PlatformHandler) CancelSubscription(c *gin.Context) {
	session, orgID, ok := h.billingWriteContext(c)
	if !ok {
		return
	}
	var req cancelSubscriptionRequest
	if err := bindOptionalJSON(c, &req); err != nil {
		respondValidationError(c, err.Error())
		return
	}
	sub, err := h.billing.Cancel(c.Request.Context(), orgID, req.Reason, session.PlatformUserID)
	if !h.handleSubscriptionResult(c, err) {
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.subscription.cancel", "organization", strconv.FormatInt(orgID, 10), orgID, 0, 0, gin.H{})
	c.JSON(http.StatusOK, gin.H{"subscription": h.subscriptionResponse(c, sub)})
}

// ExtendTrialSubscription creates or extends a trial subscription.
// POST /platform/organizations/:org_id/subscription/extend-trial — billing_admin.
func (h *PlatformHandler) ExtendTrialSubscription(c *gin.Context) {
	session, orgID, ok := h.billingWriteContext(c)
	if !ok {
		return
	}
	var req extendTrialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}
	if req.PlanID != nil && !h.planExists(c, *req.PlanID) {
		return
	}
	sub, err := h.billing.ExtendTrial(c.Request.Context(), orgID, req.PlanID, *req.TrialEndsAt, session.PlatformUserID)
	if !h.handleSubscriptionResult(c, err) {
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.subscription.extend_trial", "organization", strconv.FormatInt(orgID, 10), orgID, 0, 0, gin.H{"trial_ends_at": req.TrialEndsAt})
	c.JSON(http.StatusOK, gin.H{"subscription": h.subscriptionResponse(c, sub)})
}

// ChangeSubscriptionPlan upgrades/downgrades the plan (syncs the resolver's assignment).
// POST /platform/organizations/:org_id/subscription/plan — billing_admin.
func (h *PlatformHandler) ChangeSubscriptionPlan(c *gin.Context) {
	session, orgID, ok := h.billingWriteContext(c)
	if !ok {
		return
	}
	var req changePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}
	if !h.planExists(c, req.PlanID) {
		return
	}
	sub, err := h.billing.ChangePlan(c.Request.Context(), orgID, req.PlanID, session.PlatformUserID)
	if !h.handleSubscriptionResult(c, err) {
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.subscription.plan_change", "organization", strconv.FormatInt(orgID, 10), orgID, 0, 0, gin.H{"plan_id": req.PlanID})
	c.JSON(http.StatusOK, gin.H{"subscription": h.subscriptionResponse(c, sub)})
}

// ── billing profile ──────────────────────────────────────────────────────────

// GetBillingProfile returns the org's billing metadata (empty defaults if unset).
// GET /platform/organizations/:org_id/billing-profile
func (h *PlatformHandler) GetBillingProfile(c *gin.Context) {
	session, orgID, ok := h.billingReadContext(c)
	if !ok {
		return
	}
	profile, _, err := h.billing.GetBillingProfile(c.Request.Context(), orgID)
	if err != nil {
		respondInternalError(c)
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.billing_profile.read", "organization", strconv.FormatInt(orgID, 10), orgID, 0, 0, gin.H{})
	c.JSON(http.StatusOK, gin.H{"billing_profile": billingProfileResponse(profile)})
}

// UpdateBillingProfile upserts the org's billing metadata.
// PUT /platform/organizations/:org_id/billing-profile — billing_admin.
func (h *PlatformHandler) UpdateBillingProfile(c *gin.Context) {
	session, orgID, ok := h.billingWriteContext(c)
	if !ok {
		return
	}
	var req billingProfileRequest
	if err := bindOptionalJSON(c, &req); err != nil {
		respondValidationError(c, err.Error())
		return
	}
	profile, err := h.billing.UpsertBillingProfile(c.Request.Context(), repository.BillingProfileParams{
		OrganizationID: orgID,
		BusinessName:   req.BusinessName,
		GstNumber:      req.GstNumber,
		TaxIdentifier:  req.TaxIdentifier,
		BillingEmail:   req.BillingEmail,
		BillingContact: req.BillingContact,
		BillingAddress: req.BillingAddress,
		Currency:       req.Currency,
	})
	if err != nil {
		respondInternalError(c)
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.billing_profile.update", "organization", strconv.FormatInt(orgID, 10), orgID, 0, 0, gin.H{})
	c.JSON(http.StatusOK, gin.H{"billing_profile": billingProfileResponse(profile)})
}

// ── manual payments ──────────────────────────────────────────────────────────

// ListOrganizationPayments returns the org's manual payment records.
// GET /platform/organizations/:org_id/payments
func (h *PlatformHandler) ListOrganizationPayments(c *gin.Context) {
	session, orgID, ok := h.billingReadContext(c)
	if !ok {
		return
	}
	payments, err := h.billing.ListPayments(c.Request.Context(), orgID)
	if err != nil {
		respondInternalError(c)
		return
	}
	out := make([]gin.H, len(payments))
	for i, p := range payments {
		out[i] = paymentResponse(p)
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.payment.list", "organization", strconv.FormatInt(orgID, 10), orgID, 0, 0, gin.H{})
	c.JSON(http.StatusOK, gin.H{"payments": out})
}

// RecordOrganizationPayment records a manual payment for bookkeeping.
// POST /platform/organizations/:org_id/payments — billing_admin.
func (h *PlatformHandler) RecordOrganizationPayment(c *gin.Context) {
	session, orgID, ok := h.billingWriteContext(c)
	if !ok {
		return
	}
	var req recordPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}
	if !validNumeric(req.Amount) {
		respondValidationError(c, "amount must be a numeric value")
		return
	}
	payment, err := h.billing.RecordPayment(c.Request.Context(), orgID, req.Method, req.Amount, req.Currency, req.ReferenceNumber, req.Notes, req.ReceivedAt, req.InvoiceID, session.PlatformUserID)
	if err != nil {
		switch {
		case services.IsInvalidPaymentMethod(err):
			respondValidationError(c, "method must be one of: upi, bank_transfer, cash, cheque, other")
		case errors.Is(err, domain.ErrInvoiceNotFound):
			respondError(c, http.StatusNotFound, codeInvoiceNotFound, "invoice not found")
		default:
			respondInternalError(c)
		}
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.payment.record", "organization", strconv.FormatInt(orgID, 10), orgID, 0, 0, gin.H{"payment_id": payment.ID, "method": payment.Method})
	c.JSON(http.StatusCreated, gin.H{"payment": paymentResponse(payment)})
}

// ── invoices ─────────────────────────────────────────────────────────────────

// ListOrganizationInvoices returns the org's invoices.
// GET /platform/organizations/:org_id/invoices
func (h *PlatformHandler) ListOrganizationInvoices(c *gin.Context) {
	session, orgID, ok := h.billingReadContext(c)
	if !ok {
		return
	}
	invoices, err := h.billing.ListInvoices(c.Request.Context(), orgID)
	if err != nil {
		respondInternalError(c)
		return
	}
	out := make([]gin.H, len(invoices))
	for i, inv := range invoices {
		out[i] = invoiceResponse(inv)
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.invoice.list", "organization", strconv.FormatInt(orgID, 10), orgID, 0, 0, gin.H{})
	c.JSON(http.StatusOK, gin.H{"invoices": out})
}

// CreateOrganizationInvoice creates a draft invoice.
// POST /platform/organizations/:org_id/invoices — billing_admin.
func (h *PlatformHandler) CreateOrganizationInvoice(c *gin.Context) {
	session, orgID, ok := h.billingWriteContext(c)
	if !ok {
		return
	}
	var req createInvoiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}
	if !validNumeric(req.Amount) {
		respondValidationError(c, "amount must be a numeric value")
		return
	}
	invoice, err := h.billing.CreateInvoice(c.Request.Context(), orgID, req.Amount, req.Currency, req.DueDate, req.Notes, session.PlatformUserID)
	if err != nil {
		respondInternalError(c)
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.invoice.create", "invoice", strconv.FormatInt(invoice.ID, 10), orgID, 0, 0, gin.H{"invoice_number": invoice.InvoiceNumber})
	c.JSON(http.StatusCreated, gin.H{"invoice": invoiceResponse(invoice)})
}

// GetOrganizationInvoice returns an invoice's details.
// GET /platform/organizations/:org_id/invoices/:invoice_id
func (h *PlatformHandler) GetOrganizationInvoice(c *gin.Context) {
	session, orgID, ok := h.billingReadContext(c)
	if !ok {
		return
	}
	invoiceID, ok := parseInt64Param(c, "invoice_id", "invalid invoice id")
	if !ok {
		return
	}
	invoice, err := h.billing.GetInvoice(c.Request.Context(), orgID, invoiceID)
	if errors.Is(err, domain.ErrInvoiceNotFound) {
		respondError(c, http.StatusNotFound, codeInvoiceNotFound, "invoice not found")
		return
	}
	if err != nil {
		respondInternalError(c)
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.invoice.read", "invoice", strconv.FormatInt(invoiceID, 10), orgID, 0, 0, gin.H{})
	c.JSON(http.StatusOK, gin.H{"invoice": invoiceResponse(invoice)})
}

// IssueOrganizationInvoice / MarkOrganizationInvoicePaid / CancelOrganizationInvoice
// drive the invoice lifecycle (draft -> issued -> paid; draft|issued -> cancelled).
func (h *PlatformHandler) IssueOrganizationInvoice(c *gin.Context) {
	h.transitionInvoice(c, "issue", "platform.invoice.issue")
}

func (h *PlatformHandler) MarkOrganizationInvoicePaid(c *gin.Context) {
	h.transitionInvoice(c, "mark_paid", "platform.invoice.mark_paid")
}

func (h *PlatformHandler) CancelOrganizationInvoice(c *gin.Context) {
	h.transitionInvoice(c, "cancel", "platform.invoice.cancel")
}

func (h *PlatformHandler) transitionInvoice(c *gin.Context, action, auditAction string) {
	session, orgID, ok := h.billingWriteContext(c)
	if !ok {
		return
	}
	invoiceID, ok := parseInt64Param(c, "invoice_id", "invalid invoice id")
	if !ok {
		return
	}
	invoice, err := h.billing.TransitionInvoice(c.Request.Context(), orgID, invoiceID, action)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvoiceNotFound):
			respondError(c, http.StatusNotFound, codeInvoiceNotFound, "invoice not found")
		case errors.Is(err, domain.ErrInvalidInvoiceTransition):
			respondError(c, http.StatusConflict, codeInvalidTransition, "invalid invoice status transition")
		default:
			respondInternalError(c)
		}
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, auditAction, "invoice", strconv.FormatInt(invoiceID, 10), orgID, 0, 0, gin.H{"status": invoice.Status})
	c.JSON(http.StatusOK, gin.H{"invoice": invoiceResponse(invoice)})
}

// ── shared context + result helpers ──────────────────────────────────────────

// billingReadContext authorizes a read role and resolves a valid existing org.
func (h *PlatformHandler) billingReadContext(c *gin.Context) (services.PlatformSession, int64, bool) {
	session, ok := h.requireAnyPlatformRole(c, services.PlatformRoleSupportAdmin, services.PlatformRoleBillingAdmin, services.PlatformRoleReadOnlyAuditor)
	if !ok {
		return services.PlatformSession{}, 0, false
	}
	return h.resolveBillingOrg(c, session)
}

// billingWriteContext authorizes billing_admin (super bypass) and resolves the org.
func (h *PlatformHandler) billingWriteContext(c *gin.Context) (services.PlatformSession, int64, bool) {
	session, ok := h.requireAnyPlatformRole(c, services.PlatformRoleBillingAdmin)
	if !ok {
		return services.PlatformSession{}, 0, false
	}
	return h.resolveBillingOrg(c, session)
}

func (h *PlatformHandler) resolveBillingOrg(c *gin.Context, session services.PlatformSession) (services.PlatformSession, int64, bool) {
	orgID, ok := parseInt64Param(c, "org_id", "invalid organization id")
	if !ok {
		return services.PlatformSession{}, 0, false
	}
	if _, err := h.repos.GetOrganizationByID(c.Request.Context(), orgID); err != nil {
		respondError(c, http.StatusNotFound, CodeTenantNotFound, "organization not found")
		return services.PlatformSession{}, 0, false
	}
	return session, orgID, true
}

// planExists validates a plan id, responding 404 if missing.
func (h *PlatformHandler) planExists(c *gin.Context, planID int64) bool {
	if _, err := h.repos.GetPlanByID(c.Request.Context(), planID); err != nil {
		respondError(c, http.StatusNotFound, CodePlanNotFound, "plan not found")
		return false
	}
	return true
}

// handleSubscriptionResult maps subscription service errors to HTTP responses.
// Returns true when the result is ok to continue.
func (h *PlatformHandler) handleSubscriptionResult(c *gin.Context, err error) bool {
	switch {
	case err == nil:
		return true
	case errors.Is(err, domain.ErrSubscriptionNotFound):
		respondError(c, http.StatusNotFound, codeSubscriptionNotFound, "no subscription exists for this organization")
	case errors.Is(err, domain.ErrInvalidSubscriptionTransition):
		respondError(c, http.StatusConflict, codeInvalidTransition, "invalid subscription status transition")
	case services.IsPlanRequired(err):
		respondValidationError(c, "plan_id is required to create a subscription")
	default:
		respondInternalError(c)
	}
	return false
}

// bindOptionalJSON binds a JSON body that may be empty (all-optional requests).
func bindOptionalJSON(c *gin.Context, obj any) error {
	if c.Request == nil || c.Request.ContentLength == 0 {
		return nil
	}
	return c.ShouldBindJSON(obj)
}

// ── response builders (clean JSON; money stringified, nullables as pointers) ──

func (h *PlatformHandler) subscriptionResponse(c *gin.Context, sub sqlc.OrganizationSubscription) gin.H {
	out := gin.H{
		"id":                       sub.ID,
		"organization_id":          sub.OrganizationID,
		"plan_id":                  sub.PlanID,
		"status":                   sub.Status,
		"provider_type":            sub.ProviderType,
		"provider_subscription_id": sub.ProviderSubscriptionID,
		"provider_customer_id":     sub.ProviderCustomerID,
		"started_at":               tsPtr(sub.StartedAt),
		"trial_ends_at":            tsPtr(sub.TrialEndsAt),
		"expires_at":               tsPtr(sub.ExpiresAt),
		"renewed_at":               tsPtr(sub.RenewedAt),
		"cancelled_at":             tsPtr(sub.CancelledAt),
		"suspended_at":             tsPtr(sub.SuspendedAt),
		"cancellation_reason":      sub.CancellationReason,
		"created_at":               sub.CreatedAt,
		"updated_at":               sub.UpdatedAt,
	}
	if plan, err := h.repos.GetPlanByID(c.Request.Context(), sub.PlanID); err == nil {
		out["plan_name"] = plan.Name
		out["plan_tier"] = plan.Tier
		out["plan_price_monthly"] = numericToString(plan.PriceMonthly)
	}
	return out
}

func billingProfileResponse(p sqlc.OrganizationBillingProfile) gin.H {
	return gin.H{
		"organization_id": p.OrganizationID,
		"business_name":   p.BusinessName,
		"gst_number":      p.GstNumber,
		"tax_identifier":  p.TaxIdentifier,
		"billing_email":   p.BillingEmail,
		"billing_contact": p.BillingContact,
		"billing_address": p.BillingAddress,
		"currency":        p.Currency,
	}
}

func invoiceResponse(inv sqlc.SubscriptionInvoice) gin.H {
	return gin.H{
		"id":              inv.ID,
		"organization_id": inv.OrganizationID,
		"subscription_id": entInt64Ptr(inv.SubscriptionID),
		"invoice_number":  inv.InvoiceNumber,
		"status":          inv.Status,
		"amount":          numericToString(inv.Amount),
		"currency":        inv.Currency,
		"issue_date":      tsPtr(inv.IssueDate),
		"due_date":        tsPtr(inv.DueDate),
		"paid_at":         tsPtr(inv.PaidAt),
		"notes":           inv.Notes,
		"created_at":      inv.CreatedAt,
		"updated_at":      inv.UpdatedAt,
	}
}

func paymentResponse(p sqlc.SubscriptionPayment) gin.H {
	return gin.H{
		"id":               p.ID,
		"organization_id":  p.OrganizationID,
		"subscription_id":  entInt64Ptr(p.SubscriptionID),
		"invoice_id":       entInt64Ptr(p.InvoiceID),
		"provider_type":    p.ProviderType,
		"method":           p.Method,
		"amount":           numericToString(p.Amount),
		"currency":         p.Currency,
		"reference_number": p.ReferenceNumber,
		"notes":            p.Notes,
		"received_at":      p.ReceivedAt,
		"created_at":       p.CreatedAt,
	}
}

// tsPtr converts a nullable pgtype.Timestamptz to *time.Time for JSON (null when unset).
func tsPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}
