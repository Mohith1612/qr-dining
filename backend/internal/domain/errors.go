package domain

import "errors"

// Session lifecycle
var (
	ErrSessionNotFound      = errors.New("session not found")
	ErrSessionClosed        = errors.New("session is closed or abandoned")
	ErrSessionAlreadyActive = errors.New("table already has an active session")
)

// Participants
var (
	ErrParticipantNotFound     = errors.New("participant not found")
	ErrParticipantUnauthorized = errors.New("participant not authorized for this action")
	ErrNotSessionHost          = errors.New("only the session host can perform this action")
	ErrParticipantNotInSession = errors.New("participant does not belong to this session")
)

// Orders
var (
	ErrOrderNotFound          = errors.New("order not found")
	ErrDuplicateOrder         = errors.New("order with this idempotency key already exists")
	ErrInvalidOrderTransition = errors.New("invalid order status transition")
	ErrOrderNotEditable       = errors.New("order cannot be modified in current status")
)

// Cart
var (
	ErrCartItemNotFound    = errors.New("cart item not found")
	ErrMenuItemUnavailable = errors.New("menu item is not available")
	ErrMenuItemNotFound    = errors.New("menu item not found")
	ErrModifierNotFound    = errors.New("modifier not found for menu item")
)

// Assistance
var (
	ErrAssistanceNotFound      = errors.New("assistance request not found")
	ErrAssistanceAlreadyResolved = errors.New("assistance request already resolved")
	ErrInvalidAssistanceTransition = errors.New("invalid assistance status transition")
)

// Payments
var (
	ErrPaymentNotFound          = errors.New("payment not found")
	ErrPaymentAlreadyProcessed  = errors.New("payment already processed")
	ErrInvalidPaymentTransition = errors.New("invalid payment status transition")
)

// Webhooks
var (
	ErrDuplicateWebhookEvent = errors.New("webhook event already processed")
)

// Tables
var (
	ErrTableNotFound              = errors.New("table not found")
	ErrTableOccupied              = errors.New("table already has an active session")
	ErrDuplicateTableIdentifier   = errors.New("table identifier already exists for this branch")
)

// Menu categories
var (
	ErrCategoryNotFound = errors.New("category not found")
	ErrCategoryNotEmpty = errors.New("category has items and cannot be deleted")
)

// Auth
var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("insufficient role for this operation")
)

// Tenant
var (
	ErrTenantNotFound = errors.New("tenant not found")
	ErrTenantMismatch = errors.New("resource does not belong to the request tenant")
)

// Subscriptions / Plans
var (
	ErrPlanNotFound = errors.New("subscription plan not found")
	ErrAnalyticsGated = errors.New("analytics not available on current plan")
)

// Generic
var (
	ErrInternalError = errors.New("internal server error")
)
