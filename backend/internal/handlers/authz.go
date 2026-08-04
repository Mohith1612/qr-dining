package handlers

import (
	"fmt"
	"net/http"

	"github.com/Mohith1612/qr-dining/internal/audit"
	"github.com/Mohith1612/qr-dining/internal/authz"
	"github.com/Mohith1612/qr-dining/internal/middleware"
	"github.com/Mohith1612/qr-dining/internal/observability"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/gin-gonic/gin"
)

// handlerMetrics is set once at server bootstrap so cross-handler helpers like
// requireAuthorized can record observability data without threading the
// metrics pointer through every handler constructor. nil-safe.
var handlerMetrics *observability.Metrics

// SetHandlerMetrics wires the metrics registry into shared handler helpers.
// Call this once during server bootstrap before serving requests.
func SetHandlerMetrics(m *observability.Metrics) { handlerMetrics = m }

func staffActorForRequest(c *gin.Context, repos *repository.Repos, sess services.StaffSession) (authz.Actor, bool) {
	organizationID := sess.OrganizationID
	if organizationID == 0 {
		organization, err := repos.GetOrganizationByBranchID(c.Request.Context(), sess.BranchID)
		if err != nil {
			respondInternalError(c)
			return authz.Actor{}, false
		}
		organizationID = organization.ID
	}
	return authz.StaffActor(sess.StaffID, sess.Role, sess.BranchID, organizationID, sess.SessionID.String()), true
}

func restaurantIDForBranch(c *gin.Context, repos *repository.Repos, branchID int64) (int64, bool) {
	organization, err := repos.GetOrganizationByBranchID(c.Request.Context(), branchID)
	if err != nil {
		respondInternalError(c)
		return 0, false
	}
	return organization.ID, true
}

func requireAuthorized(c *gin.Context, repos *repository.Repos, authorizer *authz.Authorizer, writer *audit.Writer, actor authz.Actor, action authz.Action, resource authz.Resource) bool {
	decision := authorizer.Authorize(actor, action, resource)
	if decision.Allowed {
		return true
	}
	requestID, _ := c.Get(middleware.RequestIDKey)
	repos.LogAuthzDenied(c.Request.Context(), actor, action, resource, decision, stringValue(requestID))
	// Tenant scope violations are enforced regardless of AUTHZ_CENTRAL_POLICY_ENFORCE.
	// A staff actor acting on another branch's or organization's resource is never
	// legitimate traffic, so it has nothing to learn from a shadow week. Role-policy
	// denials stay flag-gated: those are what the R3 rollout gate actually measures.
	enforced := authorizer.Enforce() || decision.ScopeViolation
	if writer != nil {
		result := audit.ResultDenied
		if !enforced {
			// Shadow mode: record the would-have-denied event but allow the
			// request. Operators read this to gauge cutover risk.
			result = audit.ResultSuccess
		}
		writer.Record(c.Request.Context(), audit.AuditEvent{
			OrganizationID: decision.ActorScope.OrganizationID,
			BranchID:       decision.ActorScope.BranchID,
			ResourceType:   string(resource.Type),
			ResourceID:     resource.ID,
			Action:         audit.ActionAuthzDenied,
			Result:         result,
			ActorType:      audit.ActorType(actor.Type),
			ActorID:        fmt.Sprintf("%d", actor.ID),
			RiskLevel:      audit.RiskMedium,
			Metadata: map[string]any{
				"denied_action": string(action),
				"reason":        decision.Reason,
				"actor_role":    string(actor.Role),
				"enforced":      enforced,
			},
		})
	}
	reason := authzReasonSlug(decision.Reason)
	if !enforced {
		if handlerMetrics != nil {
			if handlerMetrics.LegacyAuthzBypassTotal != nil {
				handlerMetrics.LegacyAuthzBypassTotal.WithLabelValues(string(action), string(actor.Role)).Inc()
			}
			// Pre-flip would-break signal: this request is allowed now, but flipping
			// AUTHZ_CENTRAL_POLICY_ENFORCE would deny it. Route pinpoints the handler.
			if handlerMetrics.PolicyShadowMismatchTotal != nil {
				handlerMetrics.PolicyShadowMismatchTotal.WithLabelValues(routeLabel(c), reason).Inc()
			}
		}
		return true
	}
	if handlerMetrics != nil && handlerMetrics.AuthzDeniedTotal != nil {
		handlerMetrics.AuthzDeniedTotal.WithLabelValues(reason).Inc()
	}
	respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
	return false
}

// requireActorBranch asserts that the authenticated staff member's own branch owns the
// resource being acted on. It is defense in depth behind requireAuthorized on the
// item-scoped routes that carry no branch path parameter (and therefore no
// BranchTenantGuard): it holds even if the central policy is misconfigured, or if a
// future action is added without being listed in authz.requiresSameBranch().
//
// Call it *after* requireAuthorized so the central path still records the AUTHZ_DENIED
// audit row and its metrics; this is the backstop for the case where that path wrongly
// allows the request.
//
// Returns false when the request was rejected — callers should bail.
func requireActorBranch(c *gin.Context, sess services.StaffSession, resourceBranchID int64) bool {
	if sess.BranchID == 0 || sess.BranchID != resourceBranchID {
		respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
		return false
	}
	return true
}

// authzReasonSlug maps the human-readable policy reason to a bounded metric label.
func authzReasonSlug(reason string) string {
	switch reason {
	case "unsupported actor type":
		return "unsupported_actor_type"
	case "actor branch does not match resource branch":
		return "branch_mismatch"
	case "actor organization does not match resource organization":
		return "org_mismatch"
	case "role is not allowed for action":
		return "role_not_allowed"
	default:
		return "other"
	}
}

// routeLabel returns the gin route pattern (bounded cardinality) or "unknown".
func routeLabel(c *gin.Context) string {
	if fp := c.FullPath(); fp != "" {
		return fp
	}
	return "unknown"
}

func stringValue(v any) string {
	s, _ := v.(string)
	return s
}
