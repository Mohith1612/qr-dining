package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// APIError is the standard error envelope for all non-2xx responses.
// The code is a stable machine-readable string; message is human-readable.
// Frontend clients MUST key on code, never on message (messages may change).
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	// Reason is an additive, optional discriminator for cases where the stable
	// Code is too coarse for a client to act on. It is omitted unless set, so it
	// never changes an existing response. Today it exists for one reason: every
	// guest-credential failure answers 401 UNAUTHORIZED, but only some of them
	// are terminal. A client cannot key off Message (messages may change), so
	// without this it can only blind-retry a credential that will never work.
	Reason string `json:"reason,omitempty"`
}

// Stable APIError.Reason values.
const (
	// ReasonCredentialRevoked marks a 401 the client can never recover from by
	// retrying: the guest credential was revoked, or its credential_version was
	// rotated out from under it (which is what closing a session does to every
	// participant). The correct client response is the session-ended screen and
	// clearing stored credentials — not a reconnect loop.
	ReasonCredentialRevoked = "credential_revoked"
)

// Stable API error codes. These are part of the public API contract and must
// not be renamed or removed without a versioned API change.
const (
	CodeSessionNotFound          = "SESSION_NOT_FOUND"
	CodeSessionClosed            = "SESSION_CLOSED"
	CodeSessionAlreadyActive     = "SESSION_ALREADY_ACTIVE"
	CodeSessionEnded             = "SESSION_ENDED"
	CodePaymentInProgress        = "PAYMENT_IN_PROGRESS"
	CodePaymentAmountInvalid     = "PAYMENT_AMOUNT_INVALID"
	CodeCredentialRevoked        = "CREDENTIAL_REVOKED"
	CodeRateLimiterUnavailable   = "RATE_LIMITER_UNAVAILABLE"
	CodeAuthLockedOut            = "AUTH_LOCKED_OUT"
	CodeMFARequired              = "MFA_REQUIRED"
	CodeMFAInvalidCode           = "MFA_INVALID_CODE"
	CodeMFANotConfigured         = "MFA_NOT_CONFIGURED"
	CodeNotSessionHost           = "NOT_SESSION_HOST"
	CodeHostTransferLocked       = "HOST_TRANSFER_LOCKED"
	CodeParticipantNotFound      = "PARTICIPANT_NOT_FOUND"
	CodeMenuItemNotFound         = "MENU_ITEM_NOT_FOUND"
	CodeMenuItemUnavailable      = "MENU_ITEM_UNAVAILABLE"
	CodeModifierConflict         = "MODIFIER_CONFLICT"
	CodeCartItemNotFound         = "CART_ITEM_NOT_FOUND"
	CodeOrderNotFound            = "ORDER_NOT_FOUND"
	CodeInvalidOrderTransition   = "INVALID_ORDER_TRANSITION"
	CodeAssistanceNotFound       = "ASSISTANCE_NOT_FOUND"
	CodeInvalidAssistTransition  = "INVALID_ASSISTANCE_TRANSITION"
	CodePaymentNotFound          = "PAYMENT_NOT_FOUND"
	CodeInvalidPaymentTransition = "INVALID_PAYMENT_TRANSITION"
	CodeTableNotFound            = "TABLE_NOT_FOUND"
	CodeTableOccupied            = "TABLE_OCCUPIED"
	CodeDuplicateTableIdentifier = "DUPLICATE_TABLE_IDENTIFIER"
	CodeDuplicateStaffCode       = "DUPLICATE_STAFF_CODE"
	CodeCategoryNotFound         = "CATEGORY_NOT_FOUND"
	CodeCategoryNotEmpty         = "CATEGORY_NOT_EMPTY"
	CodeUnauthorized             = "UNAUTHORIZED"
	CodeForbidden                = "FORBIDDEN"
	CodeRateLimited              = "RATE_LIMITED"
	CodeValidationError          = "VALIDATION_ERROR"
	CodeInternalError            = "INTERNAL_ERROR"
	CodeTenantNotFound           = "TENANT_NOT_FOUND"
	// CodeOrganizationSuspended / CodeBranchSuspended are guest-facing: a QR
	// scan, session create or join against a tenant whose lifecycle status is
	// not "active". Both answer 403 — the request is well-formed and understood,
	// the tenant is deliberately not permitted to serve. Deliberately NOT 503:
	// this is a durable policy decision, not a transient outage, and must not
	// read as an availability incident to clients or to alerting.
	CodeOrganizationSuspended = "ORGANIZATION_SUSPENDED"
	CodeBranchSuspended       = "BRANCH_SUSPENDED"
	CodePlanNotFound          = "PLAN_NOT_FOUND"
	CodeAnalyticsGated        = "ANALYTICS_GATED"
	CodeInvalidPhone          = "INVALID_PHONE"
	CodeCustomerNotFound      = "CUSTOMER_NOT_FOUND"
	CodeFeatureDisabled       = "FEATURE_DISABLED"
	CodePromoNotFound         = "PROMO_NOT_FOUND"
	CodeMinOrderNotMet        = "MIN_ORDER_NOT_MET"
	CodePromoExhausted        = "PROMO_EXHAUSTED"
	CodePromoAlreadyUsed      = "PROMO_ALREADY_USED"
	CodePromoPhoneRequired    = "PROMO_PHONE_REQUIRED"

	CodeStaffAnalyticsDisabled    = "STAFF_ANALYTICS_DISABLED"
	CodeLoyaltyDisabled           = "LOYALTY_DISABLED"
	CodeLoyaltyInsufficientPoints = "LOYALTY_INSUFFICIENT_POINTS"
	CodeLoyaltyAccountNotFound    = "LOYALTY_ACCOUNT_NOT_FOUND"
)

// respondError writes a structured API error response.
func respondError(c *gin.Context, status int, code, message string) {
	c.JSON(status, APIError{Code: code, Message: message})
}

// respondErrorWithReason writes an error response carrying an additional
// machine-readable reason. Use it only where the stable code cannot express a
// distinction the client must act on; see APIError.Reason.
func respondErrorWithReason(c *gin.Context, status int, code, reason, message string) {
	c.JSON(status, APIError{Code: code, Message: message, Reason: reason})
}

// respondValidationError writes a VALIDATION_ERROR for malformed request bodies.
func respondValidationError(c *gin.Context, msg string) {
	respondError(c, http.StatusBadRequest, CodeValidationError, msg)
}

// respondInternalError writes a generic INTERNAL_ERROR response.
func respondInternalError(c *gin.Context) {
	respondError(c, http.StatusInternalServerError, CodeInternalError, "internal server error")
}
