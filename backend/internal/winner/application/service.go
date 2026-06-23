package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	auditapp "github.com/raffle-app/backend/internal/audit/application"
	"github.com/raffle-app/backend/internal/winner/domain"
	appcontext "github.com/raffle-app/backend/pkg/context"
)

type WinnerService struct {
	winnerRepo   domain.WinnerRepository
	raffleRepo   domain.RaffleRepository
	drawRepo     domain.DrawRepository
	userRepo     domain.UserRepository
	ticketRepo   domain.TicketRepository
	auditService *auditapp.AuditService
}

func NewWinnerService(
	winnerRepo domain.WinnerRepository,
	raffleRepo domain.RaffleRepository,
	drawRepo domain.DrawRepository,
	userRepo domain.UserRepository,
	ticketRepo domain.TicketRepository,
	auditService *auditapp.AuditService,
) *WinnerService {
	return &WinnerService{
		winnerRepo:   winnerRepo,
		raffleRepo:   raffleRepo,
		drawRepo:     drawRepo,
		userRepo:     userRepo,
		ticketRepo:   ticketRepo,
		auditService: auditService,
	}
}

func (s *WinnerService) CreateWinner(ctx context.Context, raffleID, drawID, ticketID, userID string, prizeAmount float64) (*domain.Winner, error) {
	if prizeAmount <= 0 {
		return nil, errors.New("prize amount must be positive")
	}

	exists, err := s.winnerRepo.ExistsByDrawIDAndTicketID(ctx, drawID, ticketID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing winner: %w", err)
	}
	if exists {
		return nil, errors.New("winner already exists for this draw and ticket")
	}

	now := time.Now()
	winner := &domain.Winner{
		RaffleID:     raffleID,
		DrawID:       drawID,
		TicketID:     ticketID,
		UserID:       userID,
		PrizeAmount:  prizeAmount,
		PrizePaid:    false,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.winnerRepo.Create(ctx, winner); err != nil {
		return nil, fmt.Errorf("failed to create winner: %w", err)
	}

	return winner, nil
}

func (s *WinnerService) GetWinnersByRaffle(ctx context.Context, raffleID string) ([]domain.WinnerDetail, error) {
	winners, err := s.winnerRepo.FindByRaffleID(ctx, raffleID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch winners: %w", err)
	}

	details := make([]domain.WinnerDetail, 0, len(winners))
	for _, w := range winners {
		detail := domain.WinnerDetail{
			Winner: w,
		}

		user, err := s.userRepo.FindByID(ctx, w.UserID)
		if err == nil {
			detail.UserEmail = user.Email
		}

		ticket, err := s.ticketRepo.FindByID(ctx, w.TicketID)
		if err == nil {
			detail.TicketNumber = ticket.TicketNumber
		}

		raffle, err := s.raffleRepo.FindByID(ctx, w.RaffleID)
		if err == nil {
			detail.RaffleTitle = raffle.Title
		}

		draw, err := s.drawRepo.FindByRaffleID(ctx, w.RaffleID)
		if err == nil {
			detail.DrawTimestamp = draw.DrawTimestamp
			detail.DrawProof.WinningNumber = detail.TicketNumber
			detail.DrawProof.VerificationURL = "/api/v1/draw/verify"
		}

		details = append(details, detail)
	}

	return details, nil
}

func (s *WinnerService) GetWinnerByID(ctx context.Context, id string) (*domain.WinnerDetail, error) {
	w, err := s.winnerRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("winner not found: %w", err)
	}

	detail := &domain.WinnerDetail{
		Winner: *w,
	}

	user, err := s.userRepo.FindByID(ctx, w.UserID)
	if err == nil {
		detail.UserEmail = user.Email
	}

	ticket, err := s.ticketRepo.FindByID(ctx, w.TicketID)
	if err == nil {
		detail.TicketNumber = ticket.TicketNumber
	}

	raffle, err := s.raffleRepo.FindByID(ctx, w.RaffleID)
	if err == nil {
		detail.RaffleTitle = raffle.Title
	}

	draw, err := s.drawRepo.FindByRaffleID(ctx, w.RaffleID)
	if err == nil {
		detail.DrawTimestamp = draw.DrawTimestamp
		detail.DrawProof.WinningNumber = detail.TicketNumber
		detail.DrawProof.VerificationURL = "/api/v1/draw/verify"
	}

	return detail, nil
}

func (s *WinnerService) MarkPrizePaid(ctx context.Context, id string, paymentReference string) (*domain.Winner, error) {
	w, err := s.winnerRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("winner not found: %w", err)
	}

	if w.PrizePaid {
		return nil, errors.New("prize already paid")
	}

	now := time.Now()
	if err := s.winnerRepo.MarkPrizePaid(ctx, id, now, paymentReference); err != nil {
		return nil, fmt.Errorf("failed to mark prize as paid: %w", err)
	}

	w.PrizePaid = true
	w.PaymentDate = &now
	w.PaymentReference = paymentReference
	w.UpdatedAt = now

	// Record winner payment audit log
	if s.auditService != nil {
		actorID := appcontext.GetUserID(ctx)
		actorType := appcontext.GetUserRole(ctx)
		var actorIDPtr *string
		if actorID != "" {
			actorIDPtr = &actorID
		}
		if actorType == "" {
			actorType = "admin"
		}
		newVal := fmt.Sprintf("paid prize of %.2f with ref %s", w.PrizeAmount, paymentReference)
		_ = s.auditService.Record(ctx, actorIDPtr, actorType, "winner_payment", "winner", &w.ID, "", nil, &newVal)
	}

	return w, nil
}
