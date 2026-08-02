// Package audit provides immutable, actor-aware audit logging for security-sensitive events.
//
// Action naming convention: resource.verb or resource.sub.verb (e.g. staff.login, menu.item.update).
// Never SCREAMING_SNAKE. Never mixed styles.
package audit

import (
	"encoding/json"

	"github.com/google/uuid"
)

type ActorType string

const (
	ActorTypePlatformUser     ActorType = "platform_user"
	ActorTypeOrganizationUser ActorType = "organization_user"
	ActorTypeStaff            ActorType = "staff"
	ActorTypeGuest            ActorType = "guest"
	ActorTypeSystem           ActorType = "system"
	ActorTypeWebhook          ActorType = "webhook"
)

type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

type ResultType string

const (
	ResultSuccess ResultType = "success"
	ResultFailure ResultType = "failure"
	ResultDenied  ResultType = "denied"
)

type SourceType string

const (
	SourceWeb     SourceType = "web"
	SourceMobile  SourceType = "mobile"
	SourcePWA     SourceType = "pwa"
	SourceAPI     SourceType = "api"
	SourceWebhook SourceType = "webhook"
	SourceSystem  SourceType = "system"
)

// Resource type constants.
const (
	ResourceStaff                  = "staff"
	ResourceOrganization           = "organization"
	ResourceBranch                 = "branch"
	ResourceSession                = "session"
	ResourceMenuItem               = "menu.item"
	ResourceMenuCategory           = "menu.category"
	ResourceMenuModifier           = "menu.modifier"
	ResourcePayment                = "payment"
	ResourceCustomer               = "customer"
	ResourcePromo                  = "promo"
	ResourceQRToken                = "qr_token"
	ResourceTable                  = "table"
	ResourceAuditLog               = "audit_log"
	ResourcePlatformSupportSession = "platform.support_session"
)

// Action constants — resource.verb or resource.sub.verb.
const (
	ActionStaffLogin               = "staff.login"
	ActionStaffLoginFailed         = "staff.login.failed"
	ActionStaffCreate              = "staff.create"
	ActionStaffDeactivate          = "staff.deactivate"
	ActionStaffPINReset            = "staff.pin.reset"
	ActionOrgMemberInvite          = "org.member.invite"
	ActionOrgMemberRoleChange      = "org.member.role_change"
	ActionOrgMemberRemove          = "org.member.remove"
	ActionBranchSettingsUpdate     = "branch.settings.update"
	ActionSessionCreate            = "session.create"
	ActionSessionClose             = "session.close"
	ActionSessionAbandon           = "session.abandon"
	ActionQRTokenRotate            = "qr_token.rotate"
	ActionTableUpdate              = "table.update"
	ActionTableDelete              = "table.delete"
	ActionMenuItemCreate           = "menu.item.create"
	ActionMenuItemUpdate           = "menu.item.update"
	ActionMenuItemDelete           = "menu.item.delete"
	ActionMenuCategoryCreate       = "menu.category.create"
	ActionMenuCategoryUpdate       = "menu.category.update"
	ActionMenuCategoryDelete       = "menu.category.delete"
	ActionPaymentInitiate          = "payment.initiate"
	ActionPaymentSettle            = "payment.settle"
	ActionPaymentWebhookProcess    = "payment.webhook.process"
	ActionPaymentRefund            = "payment.refund"
	ActionPaymentSettlementStalled = "payment.settlement.stalled"
	ActionPromoRedeem              = "promo.redeem"
	ActionCustomerOptIn            = "customer.opt_in"
	ActionCustomerDelete           = "customer.delete"
	ActionCustomerExport           = "customer.export"
	ActionAuthzDenied              = "authz.denied"
	ActionAuditRead                = "audit.read"
	ActionPlatformSupportAccess    = "platform.support_session.create"
)

// AuditEvent carries all fields for a single audit record.
// ResourceType and Action are required. All other fields are optional but recommended.
type AuditEvent struct {
	// Scope
	OrganizationID int64
	BranchID       int64
	RestaurantID   int64
	SessionID      uuid.UUID
	TableID        int64

	// What
	ResourceType string
	ResourceID   string
	Action       string
	Result       ResultType

	// Who
	ActorType    ActorType
	ActorID      string
	ActorDisplay string
	ActorScope   map[string]any

	// Request context — populated by audit.Middleware if not set by caller
	RequestID      string
	CorrelationID  string
	IdempotencyKey string
	IP             string
	UserAgent      string
	Source         SourceType

	// Change data — redacted by Writer.Record before insertion
	Before json.RawMessage
	After  json.RawMessage

	// Extra
	Metadata  map[string]any
	RiskLevel RiskLevel
}
