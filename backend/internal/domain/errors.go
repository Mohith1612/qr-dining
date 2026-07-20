package domain

import "errors"

// Session lifecycle
var (
	ErrSessionNotFound             = errors.New("session not found")
	ErrSessionClosed               = errors.New("session is closed or abandoned")
	ErrSessionAlreadyActive        = errors.New("table already has an active session")
	ErrSessionNotActive            = errors.New("session is not in an active state")
	ErrPaymentInProgress           = errors.New("payment in progress; cart and order changes are frozen")
	ErrSessionTerminalReadExpired  = errors.New("session has ended and read window has expired")
	ErrGuestCredentialRevoked      = errors.New("guest credential has been revoked")
)

// Participants
var (
	ErrParticipantNotFound     = errors.New("participant not found")
	ErrParticipantUnauthorized = errors.New("participant not authorized for this action")
	ErrNotSessionHost          = errors.New("only the session host can perform this action")
	ErrParticipantNotInSession = errors.New("participant does not belong to this session")
	// ErrHostTransferDuringPayment guards a manual host handoff while the host
	// owns an in-flight bill (session in payment_pending).
	ErrHostTransferDuringPayment = errors.New("cannot transfer host while a payment is pending")
)

// Orders
var (
	ErrOrderNotFound          = errors.New("order not found")
	ErrDuplicateOrder         = errors.New("order with this idempotency key already exists")
	ErrIdempotencyConflict    = errors.New("idempotency key already used with different request")
	ErrIdempotencyInProgress  = errors.New("idempotency key is currently being processed")
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
	ErrAssistanceNotFound          = errors.New("assistance request not found")
	ErrAssistanceAlreadyResolved   = errors.New("assistance request already resolved")
	ErrInvalidAssistanceTransition = errors.New("invalid assistance status transition")
)

// Payments
var (
	ErrPaymentNotFound           = errors.New("payment not found")
	ErrPaymentAlreadyProcessed   = errors.New("payment already processed")
	ErrInvalidPaymentTransition  = errors.New("invalid payment status transition")
	ErrPaymentVerificationFailed = errors.New("payment verification failed")
	ErrBillSnapshotStale         = errors.New("bill snapshot no longer covers all orders")
)

// Webhooks
var (
	ErrDuplicateWebhookEvent   = errors.New("webhook event already processed")
	ErrInvalidWebhookSignature = errors.New("invalid webhook signature")
)

// Tables
var (
	ErrTableNotFound            = errors.New("table not found")
	ErrTableOccupied            = errors.New("table already has an active session")
	ErrDuplicateTableIdentifier = errors.New("table identifier already exists for this branch")
)

// Menu categories
var (
	ErrCategoryNotFound = errors.New("category not found")
	ErrCategoryNotEmpty = errors.New("category has items and cannot be deleted")
)

// Auth
var (
	ErrUnauthorized     = errors.New("unauthorized")
	ErrForbidden        = errors.New("insufficient role for this operation")
	ErrAuthLockedOut    = errors.New("authentication temporarily locked due to repeated failures")
	ErrMFARequired      = errors.New("multi-factor authentication required")
	ErrMFAInvalidCode   = errors.New("invalid multi-factor authentication code")
	ErrMFANotConfigured = errors.New("mfa encryption key is not configured")
)

// Tenant
var (
	ErrTenantNotFound = errors.New("tenant not found")
	ErrTenantMismatch = errors.New("resource does not belong to the request tenant")
)

// Subscriptions / Plans
var (
	ErrPlanNotFound        = errors.New("subscription plan not found")
	ErrAnalyticsGated      = errors.New("analytics not available on current plan")
	ErrOrgPlanNotAssigned  = errors.New("organization has no plan assignment")
	ErrEntitlementNotFound = errors.New("entitlement not found in catalog")
)

// Billing (org-level subscription lifecycle, invoices)
var (
	ErrSubscriptionNotFound        = errors.New("organization subscription not found")
	ErrInvalidSubscriptionTransition = errors.New("invalid subscription status transition")
	ErrInvoiceNotFound             = errors.New("invoice not found")
	ErrInvalidInvoiceTransition    = errors.New("invalid invoice status transition")
)

// Platform feature flags / theme
var (
	ErrFlagNotFound           = errors.New("feature flag not found")
	ErrThemePresetNotFound    = errors.New("theme preset not found")
	ErrCustomThemeNotEntitled = errors.New("custom theme tokens require the custom.theme entitlement")
	ErrInvalidThemeToken      = errors.New("invalid theme token")
)

// QR collateral
var (
	ErrInvalidCollateralConfig = errors.New("invalid collateral config")
)

// Staff performance analytics / loyalty (entitlement + platform-flag gated)
var (
	ErrStaffAnalyticsDisabled    = errors.New("staff performance analytics is not enabled for this organization")
	ErrLoyaltyDisabled           = errors.New("loyalty is not enabled for this organization")
	ErrLoyaltyProgramInactive    = errors.New("loyalty program is not active for this organization")
	ErrLoyaltyInsufficientPoints = errors.New("insufficient loyalty points")
	ErrLoyaltyAccountNotFound    = errors.New("loyalty account not found")
)

// Customers
var (
	ErrInvalidPhone     = errors.New("invalid phone number")
	ErrCustomerNotFound = errors.New("customer not found")
	ErrFeatureDisabled  = errors.New("feature disabled for this restaurant")
)

// Promos
var (
	ErrPromoNotFound    = errors.New("promo code not found or not active")
	ErrMinOrderNotMet   = errors.New("order total does not meet promo minimum")
	ErrPromoExhausted   = errors.New("promo has reached its maximum redemption limit")
	ErrPromoAlreadyUsed = errors.New("promo already used by this customer")
	// ErrPromoPhoneRequired is returned when a promo has a per-phone usage limit
	// but no phone was supplied, so the limit cannot be enforced.
	ErrPromoPhoneRequired = errors.New("phone number required to use this promo")
)

// Generic
var (
	ErrInternalError = errors.New("internal server error")
)
