package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Mohith1612/qr-dining/internal/audit"
	"github.com/Mohith1612/qr-dining/internal/auth"
	"github.com/Mohith1612/qr-dining/internal/authz"
	"github.com/Mohith1612/qr-dining/internal/config"
	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/middleware"
	"github.com/Mohith1612/qr-dining/internal/observability"
	redisPkg "github.com/Mohith1612/qr-dining/internal/redis"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type SessionHandler struct {
	svc         *services.SessionService
	repos       *repository.Repos
	metrics     *observability.Metrics
	guestTokens *auth.GuestTokenService
	tickets     *redisPkg.WSTicketStore
	flags       config.FeatureFlags
	authz       *authz.Authorizer
	audit       *audit.Writer
}

func NewSessionHandler(svc *services.SessionService, repos *repository.Repos, metrics *observability.Metrics, guestTokens *auth.GuestTokenService, tickets *redisPkg.WSTicketStore, flags config.FeatureFlags, authorizer *authz.Authorizer, auditWriter *audit.Writer) *SessionHandler {
	return &SessionHandler{svc: svc, repos: repos, metrics: metrics, guestTokens: guestTokens, tickets: tickets, flags: flags, authz: authorizer, audit: auditWriter}
}

type createSessionRequest struct {
	TableID           int64  `json:"table_id" binding:"required"`
	DisplayName       string `json:"display_name" binding:"required,min=1,max=50"`
	DeviceFingerprint string `json:"device_fingerprint"`
	PhoneE164         string `json:"phone_e164"`
}

func (h *SessionHandler) Create(c *gin.Context) {
	var req createSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}

	result, err := h.svc.CreateSession(c.Request.Context(), req.TableID, req.DisplayName, req.DeviceFingerprint, req.PhoneE164)
	if err != nil {
		sessionError(c, err)
		return
	}

	token, err := h.issueGuestToken(c, result.Session, result.Participant)
	if err != nil {
		respondInternalError(c)
		return
	}

	h.audit.Record(c.Request.Context(), audit.AuditEvent{
		BranchID:     result.Session.BranchID,
		SessionID:    result.Session.ID,
		TableID:      result.Session.TableID,
		ResourceType: audit.ResourceSession,
		ResourceID:   result.Session.ID.String(),
		Action:       audit.ActionSessionCreate,
		Result:       audit.ResultSuccess,
		ActorType:    audit.ActorTypeGuest,
		ActorID:      audit.IDStr(result.Participant.ID),
		RiskLevel:    audit.RiskLow,
	})
	c.JSON(http.StatusCreated, gin.H{
		"session":            guestSafeSession(result.Session),
		"participant":        result.Participant,
		"guest_access_token": token,
	})
}

func (h *SessionHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondValidationError(c, "invalid session id")
		return
	}
	if !requireGuestSession(c, h.guestTokens, h.repos, id, h.flags.AuthGuestCredentialsRequired) {
		return
	}

	sess, err := h.svc.GetSession(c.Request.Context(), id)
	if err != nil {
		sessionError(c, err)
		return
	}

	c.JSON(http.StatusOK, guestSafeSession(sess))
}

func (h *SessionHandler) Close(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondValidationError(c, "invalid session id")
		return
	}

	participantID, hasLegacyParticipant := participantIDFromHeader(c)
	if !hasLegacyParticipant && guestTokenFromHeader(c) == "" {
		respondValidationError(c, "X-Participant-ID header required")
		return
	}
	if hasLegacyParticipant {
		recordLegacyIdentityUsage(h.metrics, legacyMechanismHeaderParticipantID, legacyEndpointSession)
	}
	participantID, ok := guestParticipantID(c, h.guestTokens, h.repos, id, participantID, h.flags.AuthGuestCredentialsRequired)
	if !ok {
		return
	}

	if err := h.svc.CloseSession(c.Request.Context(), id, &participantID); err != nil {
		sessionError(c, err)
		return
	}

	h.audit.Record(c.Request.Context(), audit.AuditEvent{
		SessionID:    id,
		ResourceType: audit.ResourceSession,
		ResourceID:   id.String(),
		Action:       audit.ActionSessionClose,
		Result:       audit.ResultSuccess,
		ActorType:    audit.ActorTypeGuest,
		ActorID:      audit.IDStr(participantID),
		RiskLevel:    audit.RiskLow,
	})
	c.Status(http.StatusNoContent)
}

// maxRecoveryReasonLen bounds the free-text reason on the staff recovery routes.
// The reason is stored in the audit trail, so it is capped but not parsed.
const maxRecoveryReasonLen = 500

type forceCloseSessionRequest struct {
	Reason string `json:"reason"`
}

// ForceClose ends any session on the staff member's own branch — for a table
// that left without paying, or where something happened the app never recorded.
// Managers and owners only: this can discard an unpaid bill, so it is a narrower
// permission than cancelling a payment.
//
// It does everything the host close does (close, release the table, revoke
// participants and rotate their credential versions, clear presence, publish
// SESSION_CLOSED) via the same shared implementation, and additionally disposes
// of any payment still in flight. The host check on the guest close path is
// untouched.
func (h *SessionHandler) ForceClose(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondValidationError(c, "invalid session id")
		return
	}
	staffSession, ok := middleware.GetStaffSession(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "staff authentication required")
		return
	}
	var req forceCloseSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		respondValidationError(c, "reason is required")
		return
	}
	if len(reason) > maxRecoveryReasonLen {
		respondValidationError(c, "reason is too long")
		return
	}

	sess, err := h.svc.GetSession(c.Request.Context(), id)
	if err != nil {
		sessionError(c, err)
		return
	}
	actor, ok := staffActorForRequest(c, h.repos, staffSession)
	if !ok {
		return
	}
	// The branch comes from the session row, never from the request.
	orgID, ok := restaurantIDForBranch(c, h.repos, sess.BranchID)
	if !ok {
		return
	}
	resource := authz.SessionResource(id, sess.BranchID, orgID)
	if !requireAuthorized(c, h.repos, h.authz, h.audit, actor, authz.ActionSessionForceClose, resource) {
		return
	}
	if !requireActorBranch(c, staffSession, sess.BranchID) {
		return
	}
	if !staffRoleIn(staffSession.Role, sqlc.StaffRoleOwner, sqlc.StaffRoleManager) {
		respondError(c, http.StatusForbidden, CodeForbidden, "only managers and owners can force-close a session")
		return
	}

	result, err := h.svc.ForceCloseByStaff(c.Request.Context(), id, staffSession.StaffID, staffSession.BranchID, reason)
	if err != nil {
		sessionError(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
	h.audit.Record(c.Request.Context(), audit.AuditEvent{
		OrganizationID: orgID,
		BranchID:       sess.BranchID,
		SessionID:      id,
		TableID:        sess.TableID,
		ResourceType:   audit.ResourceSession,
		ResourceID:     id.String(),
		Action:         audit.ActionSessionForceClose,
		ActorType:      audit.ActorTypeStaff,
		ActorID:        strconv.FormatInt(staffSession.StaffID, 10),
		Result:         audit.ResultSuccess,
		RiskLevel:      audit.RiskHigh,
		Metadata: map[string]any{
			"reason":                reason,
			"previous_status":       string(sess.Status),
			"cancelled_payment_ids": result.CancelledPaymentIDs,
			"stranded_payment_ids":  result.StrandedPaymentIDs,
		},
	})
}

type joinSessionRequest struct {
	DisplayName       string `json:"display_name" binding:"required,min=1,max=50"`
	DeviceFingerprint string `json:"device_fingerprint"`
	PhoneE164         string `json:"phone_e164"`
}

func (h *SessionHandler) Join(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondValidationError(c, "invalid session id")
		return
	}

	var req joinSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}

	participant, err := h.svc.JoinSession(c.Request.Context(), id, req.DisplayName, req.DeviceFingerprint, req.PhoneE164)
	if err != nil {
		sessionError(c, err)
		return
	}

	sess, err := h.svc.GetSession(c.Request.Context(), id)
	if err != nil {
		sessionError(c, err)
		return
	}
	token, err := h.issueGuestToken(c, sess, participant)
	if err != nil {
		respondInternalError(c)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"session":            guestSafeSession(sess),
		"participant":        participant,
		"guest_access_token": token,
	})
}

// Reactivate transitions an awaiting_reactivation session back to active so the
// guest can resume ordering without a full page refresh. Requires the caller to
// already be a credentialed member of the session.
func (h *SessionHandler) Reactivate(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondValidationError(c, "invalid session id")
		return
	}
	if !requireGuestSession(c, h.guestTokens, h.repos, id, h.flags.AuthGuestCredentialsRequired) {
		return
	}

	sess, err := h.svc.Reactivate(c.Request.Context(), id)
	if err != nil {
		sessionError(c, err)
		return
	}
	c.JSON(http.StatusOK, guestSafeSession(sess))
}

type transferHostRequest struct {
	ParticipantID int64 `json:"participant_id" binding:"required"`
}

// TransferHost hands the host role to another active participant in the session.
// Host-only: the acting participant (from the guest token) must be the current
// host. The HOST_CHANGED broadcast (from the service) updates every client.
func (h *SessionHandler) TransferHost(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondValidationError(c, "invalid session id")
		return
	}

	var req transferHostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}

	actingID, ok := guestParticipantID(c, h.guestTokens, h.repos, id, 0, h.flags.AuthGuestCredentialsRequired)
	if !ok {
		return
	}
	if actingID == 0 {
		respondValidationError(c, "guest participant required")
		return
	}

	if err := h.svc.TransferHost(c.Request.Context(), id, actingID, req.ParticipantID); err != nil {
		sessionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "host_participant_id": req.ParticipantID})
}

func (h *SessionHandler) IssueWSTicket(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondValidationError(c, "invalid session id")
		return
	}

	token := guestTokenFromHeader(c)
	if token == "" {
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "guest credential required")
		return
	}
	claims, err := h.guestTokens.Validate(token)
	if err != nil {
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "invalid guest credential")
		return
	}
	if claims.SessionID != id {
		respondError(c, http.StatusForbidden, CodeForbidden, "guest credential does not belong to this session")
		return
	}

	sess, err := h.repos.GetSessionByID(c.Request.Context(), id)
	if err != nil {
		sessionError(c, err)
		return
	}
	// payment_pending holds a socket too — see sessionServesRealtime. A guest
	// watching their bill settle is exactly who needs live updates.
	if !sessionServesRealtime(sess.Status) {
		sessionError(c, domain.ErrSessionClosed)
		return
	}

	participant, err := h.repos.GetSessionParticipantByID(c.Request.Context(), claims.ParticipantID)
	if err != nil || participant.SessionID != id {
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "guest credential is no longer valid")
		return
	}
	if participant.CredentialVersion != claims.CredentialVersion {
		// Terminal: the credential was rotated (a session close does this). Tell
		// the client so it stops retrying the ticket endpoint (F-22 / F-07).
		respondErrorWithReason(c, http.StatusUnauthorized, CodeUnauthorized, ReasonCredentialRevoked,
			"guest credential is no longer valid")
		return
	}

	wsTicket, err := h.tickets.Issue(c.Request.Context(), redisPkg.WSTicketClaims{
		SessionID:         id,
		OrganizationID:    claims.OrganizationID,
		BranchID:          claims.BranchID,
		ParticipantID:     claims.ParticipantID,
		CredentialVersion: claims.CredentialVersion,
		JTI:               claims.JTI,
	})
	if err != nil {
		respondInternalError(c)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"ticket":     wsTicket,
		"expires_in": 30,
	})
}

func (h *SessionHandler) ListActiveForBranch(c *gin.Context) {
	branchID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondValidationError(c, "invalid branch id")
		return
	}

	staffSession, ok := middleware.GetStaffSession(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "staff authentication required")
		return
	}
	if staffSession.BranchID != branchID {
		respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
		return
	}

	sessions, err := h.svc.ListActiveForBranch(c.Request.Context(), branchID)
	if err != nil {
		respondInternalError(c)
		return
	}
	c.JSON(http.StatusOK, sessions)
}

func sessionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrSessionNotFound):
		respondError(c, http.StatusNotFound, CodeSessionNotFound, err.Error())
	case errors.Is(err, domain.ErrSessionClosed):
		respondError(c, http.StatusConflict, CodeSessionClosed, err.Error())
	case errors.Is(err, domain.ErrSessionAlreadyActive):
		respondError(c, http.StatusConflict, CodeSessionAlreadyActive, err.Error())
	case errors.Is(err, domain.ErrOrganizationSuspended):
		respondError(c, http.StatusForbidden, CodeOrganizationSuspended, err.Error())
	case errors.Is(err, domain.ErrBranchSuspended):
		respondError(c, http.StatusForbidden, CodeBranchSuspended, err.Error())
	case errors.Is(err, domain.ErrNotSessionHost):
		respondError(c, http.StatusForbidden, CodeNotSessionHost, err.Error())
	case errors.Is(err, domain.ErrHostTransferDuringPayment):
		respondError(c, http.StatusConflict, CodeHostTransferLocked, err.Error())
	case errors.Is(err, domain.ErrParticipantNotInSession):
		respondError(c, http.StatusBadRequest, CodeValidationError, "that guest is not part of this table")
	case errors.Is(err, domain.ErrParticipantUnauthorized):
		respondError(c, http.StatusForbidden, CodeForbidden, "that guest is not currently present at this table")
	case errors.Is(err, domain.ErrParticipantNotFound):
		respondError(c, http.StatusNotFound, CodeParticipantNotFound, err.Error())
	case errors.Is(err, domain.ErrInvalidPhone):
		respondError(c, http.StatusBadRequest, CodeInvalidPhone, "Please enter a valid mobile number, or continue without one.")
	case errors.Is(err, domain.ErrTableNotFound):
		respondError(c, http.StatusNotFound, CodeSessionNotFound, err.Error())
	default:
		respondInternalError(c)
	}
}

// participantIDFromHeader reads X-Participant-ID from the request header.
func participantIDFromHeader(c *gin.Context) (int64, bool) {
	raw := c.GetHeader("X-Participant-ID")
	if raw == "" {
		return 0, false
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	return id, err == nil
}

func (h *SessionHandler) issueGuestToken(c *gin.Context, sess sqlc.Session, participant sqlc.SessionParticipant) (string, error) {
	organization, err := h.repos.GetOrganizationByBranchID(c.Request.Context(), sess.BranchID)
	if err != nil {
		return "", err
	}
	role := "guest"
	if participant.IsHost {
		role = "host"
	}
	return h.guestTokens.Issue(auth.GuestClaims{
		SessionID:         sess.ID,
		BranchID:          sess.BranchID,
		TableID:           sess.TableID,
		OrganizationID:    organization.ID,
		Role:              role,
		ParticipantID:     participant.ID,
		CredentialVersion: participant.CredentialVersion,
	})
}
