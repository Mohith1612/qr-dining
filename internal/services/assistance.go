package services

import (
	"context"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/events"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/google/uuid"
)

type AssistanceService struct {
	repos     *repository.Repos
	publisher *events.Publisher
}

func NewAssistanceService(repos *repository.Repos, publisher *events.Publisher) *AssistanceService {
	return &AssistanceService{repos: repos, publisher: publisher}
}

func (s *AssistanceService) Request(ctx context.Context, sessionID uuid.UUID, tableID, participantID int64, reqType sqlc.AssistanceType) (sqlc.AssistanceRequest, error) {
	ar, err := s.repos.CreateAssistanceRequest(ctx, sessionID, tableID, participantID, reqType)
	if err != nil {
		return sqlc.AssistanceRequest{}, err
	}
	s.publisher.AssistanceRequested(ctx, sessionID, ar)
	s.repos.LogEvent(ctx, sessionID, 0, "ASSISTANCE_REQUESTED", "participant", participantID, ar)
	return ar, nil
}

func (s *AssistanceService) Acknowledge(ctx context.Context, id, staffID int64) (sqlc.AssistanceRequest, error) {
	ar, err := s.repos.GetAssistanceRequestByID(ctx, id)
	if err != nil {
		return sqlc.AssistanceRequest{}, err
	}
	if err := domain.ValidateAssistanceTransition(
		domain.AssistanceStatus(ar.Status),
		domain.AssistanceStatusAcknowledged,
	); err != nil {
		return sqlc.AssistanceRequest{}, err
	}

	updated, err := s.repos.UpdateAssistanceStatus(ctx, id, sqlc.AssistanceStatusAcknowledged)
	if err != nil {
		return sqlc.AssistanceRequest{}, err
	}
	s.publisher.AssistanceAcknowledged(ctx, ar.SessionID, updated)
	s.repos.LogEvent(ctx, ar.SessionID, 0, "ASSISTANCE_ACKNOWLEDGED", "staff", staffID, updated)
	return updated, nil
}

func (s *AssistanceService) Resolve(ctx context.Context, id, staffID int64) (sqlc.AssistanceRequest, error) {
	ar, err := s.repos.GetAssistanceRequestByID(ctx, id)
	if err != nil {
		return sqlc.AssistanceRequest{}, err
	}
	if err := domain.ValidateAssistanceTransition(
		domain.AssistanceStatus(ar.Status),
		domain.AssistanceStatusResolved,
	); err != nil {
		return sqlc.AssistanceRequest{}, err
	}

	updated, err := s.repos.UpdateAssistanceStatus(ctx, id, sqlc.AssistanceStatusResolved)
	if err != nil {
		return sqlc.AssistanceRequest{}, err
	}
	s.publisher.AssistanceResolved(ctx, ar.SessionID, updated)
	s.repos.LogEvent(ctx, ar.SessionID, 0, "ASSISTANCE_RESOLVED", "staff", staffID, updated)
	return updated, nil
}

func (s *AssistanceService) ListForSession(ctx context.Context, sessionID uuid.UUID) ([]sqlc.AssistanceRequest, error) {
	return s.repos.ListAssistanceForSession(ctx, sessionID)
}

func (s *AssistanceService) ListActiveForBranch(ctx context.Context, branchID int64) ([]sqlc.AssistanceRequest, error) {
	return s.repos.ListActiveAssistanceForBranch(ctx, branchID)
}
