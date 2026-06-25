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
	enforced := authorizer.Enforce()
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
