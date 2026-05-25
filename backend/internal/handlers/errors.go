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
}

// Stable API error codes. These are part of the public API contract and must
// not be renamed or removed without a versioned API change.
const (
	CodeSessionNotFound          = "SESSION_NOT_FOUND"
	CodeSessionClosed            = "SESSION_CLOSED"
	CodeSessionAlreadyActive     = "SESSION_ALREADY_ACTIVE"
	CodeNotSessionHost           = "NOT_SESSION_HOST"
	CodeParticipantNotFound      = "PARTICIPANT_NOT_FOUND"
	CodeMenuItemNotFound         = "MENU_ITEM_NOT_FOUND"
	CodeMenuItemUnavailable      = "MENU_ITEM_UNAVAILABLE"
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
	CodeUnauthorized             = "UNAUTHORIZED"
	CodeForbidden                = "FORBIDDEN"
	CodeRateLimited              = "RATE_LIMITED"
	CodeValidationError          = "VALIDATION_ERROR"
	CodeInternalError            = "INTERNAL_ERROR"
	CodeTenantNotFound           = "TENANT_NOT_FOUND"
	CodePlanNotFound             = "PLAN_NOT_FOUND"
	CodeAnalyticsGated           = "ANALYTICS_GATED"
)

// respondError writes a structured API error response.
func respondError(c *gin.Context, status int, code, message string) {
	c.JSON(status, APIError{Code: code, Message: message})
}

// respondValidationError writes a VALIDATION_ERROR for malformed request bodies.
func respondValidationError(c *gin.Context, msg string) {
	respondError(c, http.StatusBadRequest, CodeValidationError, msg)
}

// respondInternalError writes a generic INTERNAL_ERROR response.
func respondInternalError(c *gin.Context) {
	respondError(c, http.StatusInternalServerError, CodeInternalError, "internal server error")
}
