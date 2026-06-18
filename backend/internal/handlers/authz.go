package handlers

import (
	"fmt"
	"net/http"

	"github.com/Mohith1612/qr-dining/internal/audit"
	"github.com/Mohith1612/qr-dining/internal/authz"
	"github.com/Mohith1612/qr-dining/internal/middleware"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/gin-gonic/gin"
)

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
	if writer != nil {
		writer.Record(c.Request.Context(), audit.AuditEvent{
			OrganizationID: decision.ActorScope.OrganizationID,
			BranchID:       decision.ActorScope.BranchID,
			ResourceType:   string(resource.Type),
			ResourceID:     resource.ID,
			Action:         audit.ActionAuthzDenied,
			Result:         audit.ResultDenied,
			ActorType:      audit.ActorType(actor.Type),
			ActorID:        fmt.Sprintf("%d", actor.ID),
			RiskLevel:      audit.RiskMedium,
			Metadata: map[string]any{
				"denied_action": string(action),
				"reason":        decision.Reason,
				"actor_role":    string(actor.Role),
			},
		})
	}
	respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
	return false
}

func stringValue(v any) string {
	s, _ := v.(string)
	return s
}
