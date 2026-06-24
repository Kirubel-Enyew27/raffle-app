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

// ProcessDrawResult orchestrates: determine winner, prevent duplicates, create winner record, and audit.
func (s *WinnerService) ProcessDrawResult(ctx context.Context, input domain.ProcessDrawInput) (*domain.Winner, error) {
	if input.PrizeAmount <= 0 {
		return nil, errors.New("prize amount must be positive")
	}
	if input.WinningTicketID == "" || input.WinningUserID == "" {
		return nil, errors.New("winning ticket and user are required")
	}

	exists, err := s.winnerRepo.ExistsByDrawIDAndTicketID(ctx, input.DrawID, input.WinningTicketID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing winner: %w", err)
	}
	if exists {
		return nil, errors.New("winner already exists for this draw and ticket")
	}

	now := time.Now()
	winner := &domain.Winner{
		RaffleID:    input.RaffleID,
		DrawID:      input.DrawID,
		TicketID:    input.WinningTicketID,
		UserID:      input.WinningUserID,
		PrizeAmount: input.PrizeAmount,
		PrizePaid:   false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.winnerRepo.Create(ctx, winner); err != nil {
		return nil, fmt.Errorf("failed to create winner: %w", err)
	}

	if s.auditService != nil {
		actorID := appcontext.GetUserID(ctx)
		actorType := appcontext.GetUserRole(ctx)
		if actorType == "" {
			actorType = "system"
		}
		var actorIDPtr *string
		if actorID != "" {
			actorIDPtr = &actorID
		}
		newVal := fmt.Sprintf("draw %s completed for raffle %s; winner ticket %s user %s prize %.2f",
			input.DrawID, input.RaffleID, input.WinningTicketID, input.WinningUserID, input.PrizeAmount)
		_ = s.auditService.Record(ctx, actorIDPtr, actorType, "winner_created", "winner", &winner.ID, "", nil, &newVal)
	}

	return winner, nil
}

// CreateWinner creates a winner record with duplicate prevention (called from draw service).
func (s *WinnerService) CreateWinner(ctx context.Context, raffleID, drawID, ticketID, userID string, prizeAmount float64) (*domain.Winner, error) {
	return s.ProcessDrawResult(ctx, domain.ProcessDrawInput{
		RaffleID:        raffleID,
		DrawID:          drawID,
		WinningTicketID: ticketID,
		WinningUserID:   userID,
		PrizeAmount:     prizeAmount,
	})
}

func (s *WinnerService) ListAll(ctx context.Context, limit, offset int, paidOnly *bool) ([]domain.WinnerDetail, int, error) {
	winners, total, err := s.winnerRepo.FindAll(ctx, limit, offset, paidOnly)
	if err != nil {
		return nil, 0, err
	}
	details := make([]domain.WinnerDetail, 0, len(winners))
	for _, w := range winners {
		details = append(details, s.enrich(ctx, w))
	}
	return details, total, nil
}

func (s *WinnerService) GetWinnersByRaffle(ctx context.Context, raffleID string) ([]domain.WinnerDetail, error) {
	winners, err := s.winnerRepo.FindByRaffleID(ctx, raffleID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch winners: %w", err)
	}
	details := make([]domain.WinnerDetail, 0, len(winners))
	for _, w := range winners {
		details = append(details, s.enrich(ctx, w))
	}
	return details, nil
}

func (s *WinnerService) GetWinnerByID(ctx context.Context, id string) (*domain.WinnerDetail, error) {
	w, err := s.winnerRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("winner not found: %w", err)
	}
	detail := s.enrich(ctx, *w)
	return &detail, nil
}

// GetWinningTicket returns the winning ticket details for a given winner.
func (s *WinnerService) GetWinningTicket(ctx context.Context, winnerID string) (*domain.WinningTicket, error) {
	w, err := s.winnerRepo.FindByID(ctx, winnerID)
	if err != nil {
		return nil, fmt.Errorf("winner not found: %w", err)
	}

	ticket, err := s.ticketRepo.FindByID(ctx, w.TicketID)
	if err != nil {
		return nil, fmt.Errorf("ticket not found: %w", err)
	}

	wt := &domain.WinningTicket{
		TicketID:     ticket.ID,
		TicketNumber: ticket.TicketNumber,
		RaffleID:     ticket.RaffleID,
		UserID:       ticket.UserID,
	}

	if user, err := s.userRepo.FindByID(ctx, ticket.UserID); err == nil {
		wt.UserEmail = user.Email
	}

	if draw, err := s.drawRepo.FindByRaffleID(ctx, w.RaffleID); err == nil {
		wt.DrawTimestamp = draw.DrawTimestamp
	}

	return wt, nil
}

// GetDrawVerification returns all data needed to independently verify the draw for a winner.
func (s *WinnerService) GetDrawVerification(ctx context.Context, winnerID string) (*domain.DrawVerification, error) {
	w, err := s.winnerRepo.FindByID(ctx, winnerID)
	if err != nil {
		return nil, fmt.Errorf("winner not found: %w", err)
	}

	draw, err := s.drawRepo.FindByRaffleID(ctx, w.RaffleID)
	if err != nil {
		return nil, fmt.Errorf("draw not found: %w", err)
	}

	proof, err := s.drawRepo.GetProofByRaffleID(ctx, w.RaffleID)
	if err != nil {
		return nil, fmt.Errorf("draw proof not found: %w", err)
	}

	return &domain.DrawVerification{
		DrawID:          draw.ID,
		RaffleID:        w.RaffleID,
		DrawTimestamp:   draw.DrawTimestamp,
		CommitHash:      proof.CommitHash,
		ServerSeedHash:  proof.ServerSeedHash,
		RevealedSeed:    proof.RevealedSeed,
		CombinedHash:    proof.CombinedHash,
		WinningNumber:   proof.WinningNumber,
		VerificationURL: proof.VerificationURL,
		WinnerID:        w.ID,
		WinningTicketID: w.TicketID,
	}, nil
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

// enrich populates WinnerDetail with related entity data.
func (s *WinnerService) enrich(ctx context.Context, w domain.Winner) domain.WinnerDetail {
	detail := domain.WinnerDetail{Winner: w}
	if user, err := s.userRepo.FindByID(ctx, w.UserID); err == nil {
		detail.UserEmail = user.Email
	}
	if ticket, err := s.ticketRepo.FindByID(ctx, w.TicketID); err == nil {
		detail.TicketNumber = ticket.TicketNumber
	}
	if raffle, err := s.raffleRepo.FindByID(ctx, w.RaffleID); err == nil {
		detail.RaffleTitle = raffle.Title
	}
	if draw, err := s.drawRepo.FindByRaffleID(ctx, w.RaffleID); err == nil {
		detail.DrawTimestamp = draw.DrawTimestamp
	}
	if proof, err := s.drawRepo.GetProofByRaffleID(ctx, w.RaffleID); err == nil {
		detail.DrawProof = *proof
	}
	return detail
}
