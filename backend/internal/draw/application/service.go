package application

import (
	"context"
	"fmt"
	"time"

	auditapp "github.com/raffle-app/backend/internal/audit/application"
	"github.com/raffle-app/backend/internal/draw/domain"
	ticketdomain "github.com/raffle-app/backend/internal/ticket/domain"
	appcontext "github.com/raffle-app/backend/pkg/context"
	apperrors "github.com/raffle-app/backend/pkg/errors"
	"github.com/raffle-app/backend/pkg/crypto"
)

type DrawService struct {
	drawRepo      domain.DrawRepository
	raffleRepo    domain.RaffleRepository
	ticketRepo    ticketdomain.TicketRepository
	seedService   domain.SeedService
	randomService domain.RandomService
	auditService  *auditapp.AuditService
}

func NewDrawService(
	drawRepo domain.DrawRepository,
	raffleRepo domain.RaffleRepository,
	ticketRepo ticketdomain.TicketRepository,
	seedService domain.SeedService,
	randomService domain.RandomService,
	auditService *auditapp.AuditService,
) *DrawService {
	return &DrawService{
		drawRepo:      drawRepo,
		raffleRepo:    raffleRepo,
		ticketRepo:    ticketRepo,
		seedService:   seedService,
		randomService: randomService,
		auditService:  auditService,
	}
}

func (s *DrawService) CommitDrawSeed(ctx context.Context, raffleID string) (*domain.DrawCommitment, error) {
	serverSeed, commitHash, err := s.seedService.GenerateSeed()
	if err != nil {
		return nil, apperrors.WithField("SEED_ERROR", "failed to generate server seed", 500, err)
	}

	commitment := &domain.DrawCommitment{
		RaffleID:   raffleID,
		ServerSeed: serverSeed,
		CommitHash: commitHash,
		CreatedAt:  time.Now(),
	}

	if err := s.drawRepo.CommitSeed(ctx, commitment); err != nil {
		return nil, apperrors.WithField("DB_ERROR", "failed to save seed commitment", 500, err)
	}

	if s.auditService != nil {
		actorID := appcontext.GetUserID(ctx)
		actorType := appcontext.GetUserRole(ctx)
		if actorType == "" {
			actorType = "system"
		}
		newVal := fmt.Sprintf("committed draw seed for raffle %s with hash %s", raffleID, commitHash)
		_ = s.auditService.Record(ctx, &actorID, actorType, "draw_commit", "draw", &commitment.ID, "", nil, &newVal)
	}

	return commitment, nil
}

func (s *DrawService) ExecuteDraw(ctx context.Context, raffleID string) (*domain.DrawResult, error) {
	raffle, err := s.raffleRepo.FindByID(ctx, raffleID)
	if err != nil {
		return nil, apperrors.ErrNotFound
	}
	if raffle.Status != "active" {
		return nil, apperrors.WithField("INVALID_STATUS", "raffle is not active", 400, nil)
	}

	tickets, err := s.ticketRepo.FindByRaffleID(ctx, raffleID)
	if err != nil {
		return nil, apperrors.WithField("DB_ERROR", "failed to fetch tickets", 500, err)
	}
	if len(tickets) == 0 {
		return nil, apperrors.WithField("NO_TICKETS", "no tickets sold", 400, nil)
	}

	commitment, err := s.drawRepo.GetCommitment(ctx, raffleID)
	if err != nil {
		return nil, apperrors.WithField("NO_COMMITMENT", "no seed committed for this raffle", 400, err)
	}

	serverSeed := commitment.ServerSeed
	drawTimestamp := time.Now()
	clientSeed := crypto.GenerateClientSeed(raffleID, drawTimestamp.UnixNano())
	nonce := 1

	winningIndex := s.randomService.GenerateRandom(serverSeed, clientSeed, nonce, len(tickets))
	winningTicket := tickets[winningIndex]

	combinedInput := fmt.Sprintf("%s:%s:%d", serverSeed, clientSeed, nonce)
	combinedHash := crypto.SHA256(combinedInput)

	proof := domain.DrawProof{
		CommitHash:      commitment.CommitHash,
		RevealedSeed:    serverSeed,
		CombinedHash:    combinedHash,
		WinningNumber:   winningTicket.TicketNumber,
		VerificationURL: "/api/v1/draw/verify",
	}

	result := &domain.DrawResult{
		ID:                fmt.Sprintf("draw-%d", drawTimestamp.UnixNano()),
		RaffleID:          raffleID,
		DrawTimestamp:     drawTimestamp,
		Status:            "completed",
		WinningTicketID:   winningTicket.ID,
		WinningTicketNumber: winningTicket.TicketNumber,
		Proof:             proof,
		CreatedAt:         drawTimestamp,
	}

	if err := s.drawRepo.Create(ctx, result); err != nil {
		return nil, apperrors.WithField("DB_ERROR", "failed to save draw result", 500, err)
	}

	if err := s.raffleRepo.UpdateStatus(ctx, raffleID, "completed"); err != nil {
		return nil, apperrors.WithField("DB_ERROR", "failed to update raffle status", 500, err)
	}

	if s.auditService != nil {
		actorID := appcontext.GetUserID(ctx)
		actorType := appcontext.GetUserRole(ctx)
		if actorType == "" {
			actorType = "system"
		}
		newVal := fmt.Sprintf("executed draw for raffle %s winning ticket number %d", raffleID, winningTicket.TicketNumber)
		_ = s.auditService.Record(ctx, &actorID, actorType, "draw_execution", "draw", &result.ID, "", nil, &newVal)
	}

	return result, nil
}

func (s *DrawService) GetDrawResult(ctx context.Context, raffleID string) (*domain.DrawResult, error) {
	return s.drawRepo.FindByRaffleID(ctx, raffleID)
}

func (s *DrawService) VerifyDraw(ctx context.Context, raffleID string) (*domain.VerificationResult, error) {
	result, err := s.drawRepo.FindByRaffleID(ctx, raffleID)
	if err != nil {
		return nil, apperrors.ErrNotFound
	}

	commit, err := s.drawRepo.GetCommitment(ctx, raffleID)
	if err != nil {
		return nil, apperrors.WithField("NO_COMMITMENT", "no commitment found for raffle", 404, err)
	}

	seedMatches := crypto.VerifyCommit(commit.CommitHash, result.Proof.RevealedSeed)

	clientSeed := crypto.GenerateClientSeed(raffleID, result.DrawTimestamp.UnixNano())
	expectedCombinedHash := crypto.GenerateCombinedHash(result.Proof.RevealedSeed, clientSeed, 1)
	hashMatches := expectedCombinedHash == result.Proof.CombinedHash

	expectedIndex := crypto.IndexFromHash(result.Proof.CombinedHash, 0)
	_ = expectedIndex

	verified := seedMatches && hashMatches

	return &domain.VerificationResult{
		Verified:         verified,
		SeedMatches:      seedMatches,
		HashMatches:      hashMatches,
		CommitHash:       commit.CommitHash,
		RevealedSeed:     result.Proof.RevealedSeed,
		CombinedHash:     result.Proof.CombinedHash,
		WinningNumber:    result.Proof.WinningNumber,
		VerificationURL:  result.Proof.VerificationURL,
	}, nil
}
