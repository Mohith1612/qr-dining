package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Mohith1612/qr-dining/internal/crypto"
	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/events"
	"github.com/Mohith1612/qr-dining/internal/observability"
	redisPkg "github.com/Mohith1612/qr-dining/internal/redis"
	"github.com/Mohith1612/qr-dining/internal/repository"
	ws "github.com/Mohith1612/qr-dining/internal/websocket"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/sync/errgroup"
)

type SessionService struct {
	repos     *repository.Repos
	publisher *events.Publisher
	metrics   *observability.Metrics
	presence  *redisPkg.Presence
}

func NewSessionService(repos *repository.Repos, publisher *events.Publisher, metrics *observability.Metrics, presence *redisPkg.Presence) *SessionService {
	return &SessionService{repos: repos, publisher: publisher, metrics: metrics, presence: presence}
}

type CreateSessionResult struct {
	Session     sqlc.Session
	Participant sqlc.SessionParticipant
}

// CreateSession opens a new session for a table.
// It runs a single transaction that:
//  1. Verifies no active session exists for the table.
//  2. Inserts the session (host_participant_id = NULL).
//  3. Inserts the host participant (is_host = true).
//  4. Sets sessions.host_participant_id (DEFERRABLE FK committed at step 5).
//  5. Marks the table as occupied.
func (s *SessionService) CreateSession(ctx context.Context, tableID int64, displayName, deviceFingerprint, phoneE164 string) (CreateSessionResult, error) {
	table, err := s.repos.GetTableByID(ctx, tableID)
	if err != nil {
		return CreateSessionResult{}, err
	}

	phone, err := normalizeOptionalPhone(phoneE164)
	if err != nil {
		return CreateSessionResult{}, err
	}

	token, err := crypto.GenerateToken()
	if err != nil {
		return CreateSessionResult{}, fmt.Errorf("generate session token: %w", err)
	}

	var result CreateSessionResult

	err = s.repos.WithTx(ctx, func(tx *repository.Repos) error {
		// Defer the FK check so we can insert session before participant.
		if err := tx.ExecRaw(ctx, "SET CONSTRAINTS fk_sessions_host_participant DEFERRED"); err != nil {
			return err
		}
		if err := tx.ExecRaw(ctx, "SELECT id FROM tables WHERE id = $1 FOR UPDATE", tableID); err != nil {
			return err
		}

		branch, err := tx.GetBranchByID(ctx, table.BranchID)
		if err != nil {
			return fmt.Errorf("get branch: %w", err)
		}
		localDate, businessDate := branchBusinessDate(branch, time.Now())
		visitNumber, err := tx.NextSessionNumber(ctx, table.BranchID, businessDate)
		if err != nil {
			return fmt.Errorf("next session number: %w", err)
		}
		sessionNumber := sessionReference(branch.BranchCode, localDate, visitNumber)

		sess, err := tx.CreateSession(ctx, sqlc.CreateSessionParams{
			BranchID:            table.BranchID,
			TableID:             tableID,
			SessionToken:        token,
			SessionBusinessDate: businessDate,
			VisitNumber:         visitNumber,
			SessionNumber:       sessionNumber,
		})
		if err != nil {
			return fmt.Errorf("create session: %w", err)
		}

		participant, err := tx.CreateParticipant(ctx, sess.ID, displayName, deviceFingerprint, true, phone)
		if err != nil {
			return fmt.Errorf("create host participant: %w", err)
		}

		if err := tx.SetSessionHost(ctx, sess.ID, participant.ID); err != nil {
			return fmt.Errorf("set session host: %w", err)
		}

		if err := tx.UpdateTableStatus(ctx, tableID, sqlc.TableStatusOccupied); err != nil {
			return fmt.Errorf("mark table occupied: %w", err)
		}

		result = CreateSessionResult{Session: sess, Participant: participant}
		return nil
	})
	if err != nil {
		return CreateSessionResult{}, err
	}

	s.publisher.SessionCreated(ctx, result.Session.ID, result)
	s.repos.LogEvent(ctx, result.Session.ID, result.Session.BranchID, "SESSION_CREATED", "participant", result.Participant.ID, result)
	return result, nil
}

// normalizeOptionalPhone normalizes a participant phone number when supplied.
// An empty string is valid (the "continue without phone" path) and returns "".
// A non-empty value must be a valid number or ErrInvalidPhone is returned.
func normalizeOptionalPhone(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	return normalizePhone(raw)
}

// GetSession returns the session by ID. Returns ErrSessionNotFound if missing.
func (s *SessionService) GetSession(ctx context.Context, id uuid.UUID) (sqlc.Session, error) {
	return s.repos.GetSessionByID(ctx, id)
}

// SessionWithTable is a session enriched with the human-readable table identifier.
type SessionWithTable struct {
	sqlc.Session
	TableIdentifier string `json:"table_identifier"`
}

func (s *SessionService) ListActiveForBranch(ctx context.Context, branchID int64) ([]SessionWithTable, error) {
	sessions, err := s.repos.ListActiveSessionsForBranch(ctx, branchID)
	if err != nil {
		return nil, err
	}
	result := make([]SessionWithTable, len(sessions))
	for i, sess := range sessions {
		swt := SessionWithTable{Session: sess}
		if t, err := s.repos.GetTableByID(ctx, sess.TableID); err == nil {
			swt.TableIdentifier = t.Identifier
		}
		result[i] = swt
	}
	return result, nil
}

// CloseSession closes an active session. Only the host participant may close it.
// Pass nil for requesterID to indicate a system-initiated close (payment, worker) — host check is skipped.
func (s *SessionService) CloseSession(ctx context.Context, id uuid.UUID, requesterID *int64) error {
	sess, err := s.repos.GetSessionByID(ctx, id)
	if err != nil {
		return err
	}

	if requesterID != nil {
		if !sess.HostParticipantID.Valid || sess.HostParticipantID.Int64 != *requesterID {
			return domain.ErrNotSessionHost
		}
	}

	closed := false
	err = s.repos.WithTx(ctx, func(tx *repository.Repos) error {
		if _, err := tx.CloseSessionIfActive(ctx, id); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil // already closed — idempotent
			}
			return err
		}
		if err := tx.UpdateTableStatus(ctx, sess.TableID, sqlc.TableStatusAvailable); err != nil {
			return err
		}
		// Revoke guest credentials and rotate credential_version atomically with
		// the close so stored tokens (browser history, restored tabs, share
		// links) cannot resurrect a terminal session.
		if err := tx.RevokeAllParticipants(ctx, id, "session_closed"); err != nil {
			return err
		}
		if err := tx.BumpAllParticipantCredentialVersions(ctx, id); err != nil {
			return err
		}
		closed = true
		return nil
	})
	if err != nil {
		return err
	}
	if !closed {
		return nil
	}

	if s.metrics != nil && s.metrics.SessionDuration != nil {
		s.metrics.SessionDuration.Observe(time.Since(sess.CreatedAt).Seconds())
	}
	if s.presence != nil {
		if organization, err := s.repos.GetOrganizationByBranchID(ctx, sess.BranchID); err == nil {
			s.presence.DeleteScoped(ctx, organization.ID, sess.BranchID, id)
		} else {
			s.presence.Delete(ctx, id)
		}
	}

	actorID := int64(0)
	if requesterID != nil {
		actorID = *requesterID
	}
	s.publisher.SessionClosed(ctx, id, map[string]any{"session_id": id})
	s.repos.LogEvent(ctx, id, sess.BranchID, "SESSION_CLOSED", "participant", actorID, map[string]any{"session_id": id})
	return nil
}

// AuthorizeHostAction reports whether actingParticipantID may perform a
// host-only action (submit an order, initiate the bill/payment) on the session.
// It is the single authority for host-gated actions, used by the order and
// payment services.
//
// On-demand host reassignment is layered in so a table never deadlocks on a
// dead host device: if the acting participant is not the host but the current
// host has no live presence, the acting (active) participant is promoted to
// host and the action is allowed to proceed.
func (s *SessionService) AuthorizeHostAction(ctx context.Context, sessionID uuid.UUID, actingParticipantID int64) (bool, error) {
	sess, err := s.repos.GetSessionByID(ctx, sessionID)
	if err != nil {
		return false, err
	}
	if sess.HostParticipantID.Valid && sess.HostParticipantID.Int64 == actingParticipantID {
		return true, nil
	}

	// Acting participant is not the host. Promote them on-demand only when the
	// current host has no live presence — this unblocks a stranded table
	// immediately without churning the host during normal browsing.
	if !s.hostAbsentFromPresence(ctx, sess) {
		return false, nil
	}
	acting, err := s.repos.GetParticipantByID(ctx, actingParticipantID)
	if err != nil {
		return false, err
	}
	if acting.SessionID != sessionID || acting.RevokedAt.Valid {
		return false, nil
	}
	if err := s.reassignHost(ctx, sess, acting); err != nil {
		return false, err
	}
	return true, nil
}

// hostAbsentFromPresence reports whether the current host has no live WebSocket
// presence. Heartbeats are written to the org/branch-scoped key (with a legacy
// unscoped fallback), so both are checked. A missing host_participant_id counts
// as absent. When presence cannot be determined (no presence backend), it
// conservatively reports present so we never strip a host on a false signal.
func (s *SessionService) hostAbsentFromPresence(ctx context.Context, sess sqlc.Session) bool {
	if !sess.HostParticipantID.Valid {
		return true
	}
	if s.presence == nil {
		return false
	}
	hostID := sess.HostParticipantID.Int64
	if org, err := s.repos.GetOrganizationByBranchID(ctx, sess.BranchID); err == nil {
		if present, err := s.presence.GetPresentScoped(ctx, org.ID, sess.BranchID, sess.ID); err == nil {
			if _, ok := present[hostID]; ok {
				return false
			}
		}
	}
	if present, err := s.presence.GetPresent(ctx, sess.ID); err == nil {
		if _, ok := present[hostID]; ok {
			return false
		}
	}
	return true
}

// reassignHost promotes newHost to session host (persisting host_participant_id
// and the is_host flags in one transaction) and broadcasts HOST_CHANGED so live
// participants update their host badge and host-only affordances.
func (s *SessionService) reassignHost(ctx context.Context, sess sqlc.Session, newHost sqlc.SessionParticipant) error {
	if err := s.repos.ReassignHost(ctx, sess.ID, newHost.ID); err != nil {
		return err
	}
	newHost.IsHost = true
	s.publisher.HostChanged(ctx, sess.ID, newHost)
	s.repos.LogEvent(ctx, sess.ID, sess.BranchID, "HOST_CHANGED", "system", newHost.ID, newHost)
	return nil
}

// ensureHostBaseline reassigns the host when it is definitively gone — the
// host_participant_id is unset or the host participant has been revoked. It
// promotes the oldest active participant. Evaluated on snapshot (reconnect) so
// any returning participant heals a host-less session. Returns the (possibly
// updated) session and participants list reflecting the new host.
func (s *SessionService) ensureHostBaseline(ctx context.Context, sess sqlc.Session, participants []sqlc.SessionParticipant) (sqlc.Session, []sqlc.SessionParticipant) {
	hostValid := false
	if sess.HostParticipantID.Valid {
		for _, p := range participants {
			if p.ID == sess.HostParticipantID.Int64 {
				hostValid = !p.RevokedAt.Valid
				break
			}
		}
	}
	if hostValid {
		return sess, participants
	}

	// Promote the oldest active participant (list is ordered joined_at ASC).
	var newHost *sqlc.SessionParticipant
	for i := range participants {
		if !participants[i].RevokedAt.Valid {
			newHost = &participants[i]
			break
		}
	}
	if newHost == nil {
		return sess, participants // no active participant to promote
	}
	if sess.HostParticipantID.Valid && sess.HostParticipantID.Int64 == newHost.ID {
		return sess, participants // already the host; nothing to do
	}
	if err := s.reassignHost(ctx, sess, *newHost); err != nil {
		return sess, participants // best-effort; snapshot still returns current state
	}
	sess.HostParticipantID = pgtype.Int8{Int64: newHost.ID, Valid: true}
	for i := range participants {
		participants[i].IsHost = participants[i].ID == newHost.ID
	}
	return sess, participants
}

// JoinSession adds a participant to an existing active session.
func (s *SessionService) JoinSession(ctx context.Context, sessionID uuid.UUID, displayName, deviceFingerprint, phoneE164 string) (sqlc.SessionParticipant, error) {
	sess, err := s.repos.GetSessionByID(ctx, sessionID)
	if err != nil {
		return sqlc.SessionParticipant{}, err
	}
	if sess.Status != sqlc.SessionStatusActive {
		return sqlc.SessionParticipant{}, domain.ErrSessionClosed
	}

	phone, err := normalizeOptionalPhone(phoneE164)
	if err != nil {
		return sqlc.SessionParticipant{}, err
	}

	participant, err := s.repos.CreateParticipant(ctx, sessionID, displayName, deviceFingerprint, false, phone)
	if err != nil {
		return sqlc.SessionParticipant{}, err
	}

	s.publisher.ParticipantJoined(ctx, sessionID, participant)
	s.repos.LogEvent(ctx, sessionID, sess.BranchID, "PARTICIPANT_JOINED", "participant", participant.ID, participant)
	return participant, nil
}

// Reactivate transitions an awaiting_reactivation session back to active. It is
// the explicit counterpart to the snapshot reconnect side-effect: the guest's
// "still ordering" action calls this so subsequent ordering does not hit a 409.
// Idempotent for already-active sessions; terminal/frozen sessions return
// ErrSessionClosed so the caller surfaces a clean 409.
func (s *SessionService) Reactivate(ctx context.Context, sessionID uuid.UUID) (sqlc.Session, error) {
	sess, err := s.repos.GetSessionByID(ctx, sessionID)
	if err != nil {
		return sqlc.Session{}, err
	}
	switch sess.Status {
	case sqlc.SessionStatusActive:
		return sess, nil
	case sqlc.SessionStatusAwaitingReactivation:
		if _, err := s.repos.ReactivateSession(ctx, sessionID); err != nil {
			// The worker abandoned us between read and update — it's terminal now.
			return sqlc.Session{}, domain.ErrSessionClosed
		}
		sess.Status = sqlc.SessionStatusActive
		sess.AwaitingReactivationAt.Valid = false
		s.publisher.SessionCreated(ctx, sessionID, map[string]string{"reason": "reactivated"})
		s.repos.LogEvent(ctx, sessionID, sess.BranchID, "SESSION_REACTIVATED", "participant", 0, map[string]any{})
		return sess, nil
	default:
		return sqlc.Session{}, domain.ErrSessionClosed
	}
}

// SessionSnapshot is the full authoritative state of a session at a point in time.
// Clients call GET /sessions/:id/snapshot on WebSocket reconnect to reconcile local state.
type SessionSnapshot struct {
	Session         sqlc.Session              `json:"session"`
	TableIdentifier string                    `json:"table_identifier"`
	Participants    []sqlc.SessionParticipant `json:"participants"`
	Orders          []sqlc.Order              `json:"orders"`
	Assistance      []sqlc.AssistanceRequest  `json:"assistance"`
	MissedEvents    []ws.Envelope             `json:"missed_events,omitempty"`
	SnapshotAt      time.Time                 `json:"snapshot_at"`
	SessionEnded    bool                      `json:"session_ended,omitempty"`
	CloseReason     string                    `json:"close_reason,omitempty"`
	// SnapshotAuthoritative tells the client this snapshot IS the full source of
	// truth and must replace local state wholesale — set when there is no
	// incremental basis (last_sequence=0) or the requested gap can't be replayed
	// contiguously (earlier events pruned). Per realtime-reconciliation-invariants.
	SnapshotAuthoritative bool `json:"snapshot_authoritative,omitempty"`
}

// TerminalReadWindow is the grace window during which the snapshot endpoint
// still returns a read-only payload for a terminal session. After this window
// the snapshot returns 410. Per session-lifecycle-state-machine.md §13.
const TerminalReadWindow = 60 * time.Minute

// GetSnapshot assembles the full current state of a session in parallel.
// Used by clients to reconcile state after a WebSocket reconnect.
//
// For terminal sessions (closed/abandoned/expired), the snapshot is still
// returned during a 60-minute read window so dispute handling and the
// guest's own receipt screen continue to work. After the window the caller
// receives ErrSessionTerminalReadExpired and must surface a 410.
func (s *SessionService) GetSnapshot(ctx context.Context, sessionID uuid.UUID, lastSequence int64) (SessionSnapshot, error) {
	sess, err := s.repos.GetSessionByID(ctx, sessionID)
	if err != nil {
		return SessionSnapshot{}, err
	}
	// Reconnect path: if the worker put us into awaiting_reactivation but the
	// guest is back within the window, transition back to active. The window
	// itself is enforced by the worker (it only enters awaiting_reactivation
	// after presence_grace and abandons after reactivation_window), so the
	// snapshot endpoint just trusts the state column here.
	if sess.Status == sqlc.SessionStatusAwaitingReactivation {
		if _, err := s.repos.ReactivateSession(ctx, sessionID); err == nil {
			sess.Status = sqlc.SessionStatusActive
			sess.AwaitingReactivationAt.Valid = false
			s.publisher.SessionCreated(ctx, sessionID, map[string]string{"reason": "reactivated"})
			s.repos.LogEvent(ctx, sessionID, sess.BranchID, "SESSION_REACTIVATED", "participant", 0, map[string]any{})
		}
		// If ReactivateSession returned pgx.ErrNoRows it means the worker
		// abandoned us between the read and the update — fall through to the
		// terminal-window path below.
	}
	terminal := domain.IsSessionTerminal(domain.SessionStatus(sess.Status))
	if terminal && sess.ClosedAt.Valid && time.Since(sess.ClosedAt.Time) > TerminalReadWindow {
		return SessionSnapshot{}, domain.ErrSessionTerminalReadExpired
	}

	var (
		participants    []sqlc.SessionParticipant
		orders          []sqlc.Order
		assistance      []sqlc.AssistanceRequest
		tableIdentifier string
		missedEvents    []ws.Envelope
	)

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		var err error
		participants, err = s.repos.ListParticipantsBySession(gctx, sessionID)
		return err
	})
	g.Go(func() error {
		var err error
		orders, err = s.repos.ListOrdersForSession(gctx, sessionID)
		return err
	})
	g.Go(func() error {
		var err error
		assistance, err = s.repos.ListAssistanceForSession(gctx, sessionID)
		return err
	})
	g.Go(func() error {
		if t, err := s.repos.GetTableByID(gctx, sess.TableID); err == nil {
			tableIdentifier = t.Identifier
		}
		return nil // non-fatal: fall back to numeric table_id on frontend
	})
	if lastSequence > 0 {
		g.Go(func() error {
			var err error
			missedEvents, err = s.repos.ListSessionEventsAfter(gctx, sessionID, lastSequence)
			return err
		})
	}

	if err := g.Wait(); err != nil {
		return SessionSnapshot{}, err
	}

	// Heal a host-less session on reconnect: if the host is gone for good
	// (unset or revoked), promote the oldest active participant. This is the
	// baseline reassignment path; transient host disconnects are handled
	// on-demand at action time (AuthorizeHostAction).
	if !terminal {
		sess, participants = s.ensureHostBaseline(ctx, sess, participants)
	}

	snap := SessionSnapshot{
		Session:         sess,
		TableIdentifier: tableIdentifier,
		Participants:    participants,
		Orders:          orders,
		Assistance:      assistance,
		MissedEvents:    missedEvents,
		SnapshotAt:      time.Now().UTC(),
	}
	// Authoritative when the client has no incremental basis, or when the replay
	// has a hole (the earliest available event is past last_sequence+1, meaning
	// older events were pruned). In both cases the client must adopt this full
	// snapshot rather than rely on missed_events.
	if lastSequence == 0 || (len(missedEvents) > 0 && missedEvents[0].Sequence > lastSequence+1) {
		snap.SnapshotAuthoritative = true
	}
	if terminal {
		snap.SessionEnded = true
		snap.CloseReason = string(sess.Status)
	}
	return snap, nil
}
