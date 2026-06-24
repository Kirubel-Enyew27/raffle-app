package application

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"time"

	auditapp "github.com/raffle-app/backend/internal/audit/application"
	"github.com/raffle-app/backend/internal/raffle/domain"
	appcontext "github.com/raffle-app/backend/pkg/context"
	apperrors "github.com/raffle-app/backend/pkg/errors"
)

type RaffleService struct {
	repo         domain.RaffleRepository
	auditService *auditapp.AuditService
}

func NewRaffleService(repo domain.RaffleRepository, auditService *auditapp.AuditService) *RaffleService {
	return &RaffleService{
		repo:         repo,
		auditService: auditService,
	}
}

func generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%12x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func (s *RaffleService) CreateRaffle(ctx context.Context, raffle *domain.Raffle) error {
	if raffle.ID == "" {
		raffle.ID = generateUUID()
	}
	if err := s.validateRaffle(raffle); err != nil {
		return err
	}

	raffle.CreatedAt = time.Now()
	raffle.UpdatedAt = time.Now()
	raffle.SoldTickets = 0

	if err := s.repo.Create(ctx, raffle); err != nil {
		return err
	}

	if s.auditService != nil {
		actorID := appcontext.GetUserID(ctx)
		actorType := appcontext.GetUserRole(ctx)
		if actorID == "" {
			actorID = raffle.CreatorID
		}
		if actorType == "" {
			actorType = "admin"
		}
		newVal := fmt.Sprintf("created raffle %s with price %.2f", raffle.Title, raffle.TicketPrice)
		_ = s.auditService.Record(ctx, &actorID, actorType, "raffle_creation", "raffle", &raffle.ID, "", nil, &newVal)
	}
	return nil
}

func (s *RaffleService) GetRaffle(ctx context.Context, id string) (*domain.Raffle, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *RaffleService) ListRaffles(ctx context.Context, limit, offset int) ([]*domain.Raffle, int64, error) {
	raffles, err := s.repo.List(ctx, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	count, err := s.repo.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	return raffles, count, nil
}

func (s *RaffleService) UpdateRaffle(ctx context.Context, raffle *domain.Raffle) error {
	if raffle.ID == "" {
		return apperrors.ErrNotFound
	}

	existing, err := s.repo.FindByID(ctx, raffle.ID)
	if err != nil {
		return err
	}

	if raffle.Title != "" {
		existing.Title = raffle.Title
	}
	if raffle.Description != "" {
		existing.Description = raffle.Description
	}
	if raffle.TicketPrice > 0 {
		existing.TicketPrice = raffle.TicketPrice
	}
	if raffle.TotalTickets > 0 {
		existing.TotalTickets = raffle.TotalTickets
	}
	if !raffle.DrawDate.IsZero() {
		existing.DrawDate = raffle.DrawDate
	}
	if raffle.Status != "" {
		validStatuses := map[string]bool{"draft": true, "active": true, "scheduled": true, "closed": true}
		if !validStatuses[raffle.Status] {
			return apperrors.WithField("INVALID_STATUS", "invalid raffle status", http.StatusBadRequest, fmt.Errorf("status must be one of: draft, active, scheduled, closed"))
		}
		existing.Status = raffle.Status
	}

	existing.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, existing); err != nil {
		return err
	}

	if s.auditService != nil {
		actorID := appcontext.GetUserID(ctx)
		actorType := appcontext.GetUserRole(ctx)
		if actorID == "" {
			actorID = existing.CreatorID
		}
		if actorType == "" {
			actorType = "admin"
		}
		newVal := fmt.Sprintf("updated raffle %s status to %s", existing.Title, existing.Status)
		_ = s.auditService.Record(ctx, &actorID, actorType, "raffle_update", "raffle", &existing.ID, "", nil, &newVal)
	}
	return nil
}

func (s *RaffleService) CloseRaffle(ctx context.Context, id string) error {
	raffle, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if raffle.Status == "closed" {
		return apperrors.WithField("INVALID_STATUS", "raffle is already closed", http.StatusBadRequest, nil)
	}
	raffle.Status = "closed"
	raffle.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, raffle); err != nil {
		return err
	}

	if s.auditService != nil {
		actorID := appcontext.GetUserID(ctx)
		actorType := appcontext.GetUserRole(ctx)
		if actorID == "" {
			actorID = "system"
		}
		if actorType == "" {
			actorType = "system"
		}
		newVal := "status changed to closed"
		_ = s.auditService.Record(ctx, &actorID, actorType, "raffle_update", "raffle", &raffle.ID, "", nil, &newVal)
	}
	return nil
}

func (s *RaffleService) ScheduleDrawDate(ctx context.Context, id string, drawDate time.Time) error {
	raffle, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if raffle.Status == "closed" {
		return apperrors.WithField("INVALID_STATUS", "cannot schedule draw date for closed raffle", http.StatusBadRequest, nil)
	}
	if drawDate.Before(time.Now()) {
		return apperrors.WithField("INVALID_DRAW_DATE", "draw date must be in the future", http.StatusBadRequest, nil)
	}

	raffle.DrawDate = drawDate
	raffle.Status = "scheduled"
	raffle.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, raffle); err != nil {
		return err
	}

	if s.auditService != nil {
		actorID := appcontext.GetUserID(ctx)
		actorType := appcontext.GetUserRole(ctx)
		if actorID == "" {
			actorID = "system"
		}
		if actorType == "" {
			actorType = "system"
		}
		newVal := fmt.Sprintf("scheduled draw date to %s", drawDate.Format(time.RFC3339))
		_ = s.auditService.Record(ctx, &actorID, actorType, "raffle_draw_scheduled", "raffle", &raffle.ID, "", nil, &newVal)
	}
	return nil
}

func (s *RaffleService) validateRaffle(raffle *domain.Raffle) error {
	if raffle.Title == "" {
		return apperrors.ErrValidationFailed
	}
	if raffle.TicketPrice <= 0 {
		return apperrors.WithField("INVALID_PRICE", "ticket price must be greater than zero", http.StatusBadRequest, nil)
	}
	if raffle.TotalTickets <= 0 {
		return apperrors.WithField("INVALID_TICKETS", "total tickets must be greater than zero", http.StatusBadRequest, nil)
	}
	if raffle.DrawDate.IsZero() {
		return apperrors.WithField("MISSING_DRAW_DATE", "draw date is required", http.StatusBadRequest, nil)
	}
	if raffle.CreatorID == "" {
		return apperrors.WithField("MISSING_CREATOR", "creator id is required", http.StatusBadRequest, nil)
	}
	if raffle.Currency == "" {
		raffle.Currency = "USD"
	}
	validStatuses := map[string]bool{"draft": true, "active": true, "scheduled": true, "closed": true}
	if !validStatuses[raffle.Status] {
		return apperrors.WithField("INVALID_STATUS", "invalid raffle status", http.StatusBadRequest, fmt.Errorf("status must be one of: draft, active, scheduled, closed"))
	}
	return nil
}
