package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/Mohith1612/qr-dining/internal/config"

	"github.com/gin-gonic/gin"
)

func marshalPayload(v any) (json.RawMessage, error) {
	return json.Marshal(v)
}

// activeFeatureFlags captures the bootstrap-time strict-mode flags so cross-
// handler helpers (like enforceBranchScopeFromBody) can branch on them without
// every handler constructor needing to thread a config struct. Set once via
// SetActiveFeatureFlags during server bootstrap.
var activeFeatureFlags config.FeatureFlags

// SetActiveFeatureFlags wires the strict-mode flag values into shared handler
// helpers. Call once at server bootstrap.
func SetActiveFeatureFlags(f config.FeatureFlags) { activeFeatureFlags = f }

// enforceBranchScopeFromBody validates a client-supplied branch_id against the
// branch derived from the resource being mutated. When STRICT_BRANCH_SCOPED_MUTATIONS
// is on, a mismatch (or any nonzero request body branch that differs from the
// derived value) is rejected so the surface for cross-branch confusion shrinks.
// When the flag is off the legacy submission is recorded via a metric and the
// derived value is used.
//
// Returns false when the request was rejected — callers should bail.
func enforceBranchScopeFromBody(c *gin.Context, requestBranchID, derivedBranchID int64, endpointClass string) bool {
	if requestBranchID == 0 || requestBranchID == derivedBranchID {
		return true
	}
	if activeFeatureFlags.StrictBranchScopedMutations {
		respondError(c, http.StatusBadRequest, CodeValidationError, "branch_id must be omitted or match the target resource branch")
		return false
	}
	if handlerMetrics != nil && handlerMetrics.LegacyIdentityUsageTotal != nil {
		handlerMetrics.LegacyIdentityUsageTotal.WithLabelValues("body_branch_id", endpointClass).Inc()
	}
	return true
}
