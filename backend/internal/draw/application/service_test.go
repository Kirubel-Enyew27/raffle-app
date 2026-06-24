package application

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/raffle-app/backend/internal/draw/domain"
	ticketdomain "github.com/raffle-app/backend/internal/ticket/domain"
	winnerdomain "github.com/raffle-app/backend/internal/winner/domain"
	"github.com/raffle-app/backend/pkg/crypto"
)

const testServerSeed = "mockseed123"
const testCommitHash = "e148ed4f45a2b7501a40a01d1b22ecb7540044a334f5d03a840eff73ded6a14b"

type mockDrawRepo struct {
	results     map[string]*domain.DrawResult
	commitments map[string]*domain.DrawCommitment
}

func newMockDrawRepo() *mockDrawRepo {
	return &mockDrawRepo{
		results:     make(map[string]*domain.DrawResult),
		commitments: make(map[string]*domain.DrawCommitment),
	}
}

func (m *mockDrawRepo) Create(ctx context.Context, result *domain.DrawResult) error {
	result.ID = "draw-" + result.RaffleID
	result.CreatedAt = time.Now()
	m.results[result.RaffleID] = result
	return nil
}

func (m *mockDrawRepo) FindByRaffleID(ctx context.Context, raffleID string) (*domain.DrawResult, error) {
	r, ok := m.results[raffleID]
	if !ok {
		return nil, errors.New("draw not found")
	}
	copy := *r
	return &copy, nil
}

func (m *mockDrawRepo) ExistsForRaffle(ctx context.Context, raffleID string) (bool, error) {
	_, ok := m.results[raffleID]
	return ok, nil
}

func (m *mockDrawRepo) CommitSeed(ctx context.Context, commitment *domain.DrawCommitment) error {
	if _, exists := m.commitments[commitment.RaffleID]; exists {
		return errors.New("commitment already exists")
	}
	commitment.ID = "commit-" + commitment.RaffleID
	commitment.CreatedAt = time.Now()
	m.commitments[commitment.RaffleID] = commitment
	return nil
}

func (m *mockDrawRepo) GetCommitment(ctx context.Context, raffleID string) (*domain.DrawCommitment, error) {
	c, ok := m.commitments[raffleID]
	if !ok {
		return nil, errors.New("commitment not found")
	}
	copy := *c
	return &copy, nil
}

type mockSeedService struct{}

func (m *mockSeedService) GenerateSeed() (string, string, error) {
	return testServerSeed, testCommitHash, nil
}

func (m *mockSeedService) CommitSeed(seed string) (string, error) {
	return crypto.SHA256(seed), nil
}

func (m *mockSeedService) VerifyCommit(commit, seed string) bool {
	return crypto.SHA256(seed) == commit
}

type mockRandomService struct{}

func (m *mockRandomService) GenerateRandom(serverSeed, clientSeed string, nonce, maxTickets int) int {
	return 0
}

type mockRaffleRepo struct {
	raffles map[string]*ticketdomain.RaffleEntity
}

func newMockRaffleRepo() *mockRaffleRepo {
	return &mockRaffleRepo{raffles: make(map[string]*ticketdomain.RaffleEntity)}
}

func (m *mockRaffleRepo) FindByID(ctx context.Context, id string) (*ticketdomain.RaffleEntity, error) {
	r, ok := m.raffles[id]
	if !ok {
		return nil, errors.New("raffle not found")
	}
	copy := *r
	return &copy, nil
}

func (m *mockRaffleRepo) UpdateStatus(ctx context.Context, id string, status string) error {
	r, ok := m.raffles[id]
	if !ok {
		return errors.New("raffle not found")
	}
	r.Status = status
	return nil
}

func (m *mockRaffleRepo) FindTicketsByRaffleID(ctx context.Context, raffleID string) ([]ticketdomain.Ticket, error) {
	return []ticketdomain.Ticket{
		{ID: "ticket-1", RaffleID: raffleID, TicketNumber: 1},
	}, nil
}

type emptyTicketRepo struct{}

func (m *emptyTicketRepo) Create(ctx context.Context, ticket *ticketdomain.Ticket) error {
	return nil
}

func (m *emptyTicketRepo) CreateBatch(ctx context.Context, tickets []*ticketdomain.Ticket) error {
	return nil
}

func (m *emptyTicketRepo) CreateBatchTx(ctx context.Context, tx *sql.Tx, tickets []*ticketdomain.Ticket) error {
	return nil
}

func (m *emptyTicketRepo) CountByRaffleID(ctx context.Context, raffleID string) (int, error) {
	return 0, nil
}

func (m *emptyTicketRepo) FindByRaffleID(ctx context.Context, raffleID string) ([]*ticketdomain.Ticket, error) {
	return nil, nil
}

func (m *emptyTicketRepo) FindByWalletTxID(ctx context.Context, walletTxID string) ([]*ticketdomain.Ticket, error) {
	return nil, nil
}

type mockTicketRepo struct{}

func (m *mockTicketRepo) Create(ctx context.Context, ticket *ticketdomain.Ticket) error {
	return nil
}

func (m *mockTicketRepo) CreateBatch(ctx context.Context, tickets []*ticketdomain.Ticket) error {
	return nil
}

func (m *mockTicketRepo) CreateBatchTx(ctx context.Context, tx *sql.Tx, tickets []*ticketdomain.Ticket) error {
	return nil
}

func (m *mockTicketRepo) CountByRaffleID(ctx context.Context, raffleID string) (int, error) {
	return 0, nil
}

func (m *mockTicketRepo) FindByRaffleID(ctx context.Context, raffleID string) ([]*ticketdomain.Ticket, error) {
	return []*ticketdomain.Ticket{
		{ID: "ticket-1", RaffleID: raffleID, TicketNumber: 1},
	}, nil
}

func (m *mockTicketRepo) FindByWalletTxID(ctx context.Context, walletTxID string) ([]*ticketdomain.Ticket, error) {
	return nil, nil
}

type mockWinnerService struct {
	winnerCreated bool
}

func (m *mockWinnerService) CreateWinner(ctx context.Context, raffleID, drawID, ticketID, userID string, prizeAmount float64) (*winnerdomain.Winner, error) {
	m.winnerCreated = true
	return &winnerdomain.Winner{
		ID:          "winner-1",
		RaffleID:    raffleID,
		DrawID:      drawID,
		TicketID:    ticketID,
		UserID:      userID,
		PrizeAmount: prizeAmount,
	}, nil
}

func setupDrawService(drawRepo domain.DrawRepository, raffleRepo domain.RaffleRepository, ticketRepo ticketdomain.TicketRepository, seedService domain.SeedService, randomService domain.RandomService) *DrawService {
	return NewDrawService(drawRepo, raffleRepo, ticketRepo, seedService, randomService, nil, &mockWinnerService{})
}

func TestCommitDrawSeed_Success(t *testing.T) {
	drawRepo := newMockDrawRepo()
	raffleRepo := newMockRaffleRepo()
	seedService := &mockSeedService{}

	svc := setupDrawService(drawRepo, raffleRepo, nil, seedService, nil)

	commitment, err := svc.CommitDrawSeed(context.Background(), "raffle-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if commitment.CommitHash != testCommitHash {
		t.Errorf("expected commit hash %s, got %s", testCommitHash, commitment.CommitHash)
	}
	if commitment.RaffleID != "raffle-1" {
		t.Errorf("expected raffle_id raffle-1, got %s", commitment.RaffleID)
	}
	if commitment.ServerSeed == "" {
		t.Error("expected server seed to be stored")
	}
}

func TestCommitDrawSeed_PreventsDoubleCommit(t *testing.T) {
	drawRepo := newMockDrawRepo()
	seedService := &mockSeedService{}

	svc := setupDrawService(drawRepo, nil, nil, seedService, nil)

	_, err := svc.CommitDrawSeed(context.Background(), "raffle-1")
	if err != nil {
		t.Fatalf("unexpected error on first commit: %v", err)
	}

	_, err = svc.CommitDrawSeed(context.Background(), "raffle-1")
	if err == nil {
		t.Error("expected error on double commit")
	}
}

func TestExecuteDraw_Success(t *testing.T) {
	drawRepo := newMockDrawRepo()
	raffleRepo := newMockRaffleRepo()
	ticketRepo := &mockTicketRepo{}
	seedService := &mockSeedService{}
	randomService := &mockRandomService{}

	raffleRepo.raffles["raffle-1"] = &ticketdomain.RaffleEntity{
		ID: "raffle-1", Status: "active", TotalTickets: 100, SoldTickets: 1, TicketPrice: 10.0,
	}

	svc := setupDrawService(drawRepo, raffleRepo, ticketRepo, seedService, randomService)

	_, err := svc.CommitDrawSeed(context.Background(), "raffle-1")
	if err != nil {
		t.Fatalf("unexpected error committing seed: %v", err)
	}

	result, err := svc.ExecuteDraw(context.Background(), "raffle-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RaffleID != "raffle-1" {
		t.Errorf("expected raffle_id raffle-1, got %s", result.RaffleID)
	}
	if result.Proof.WinningNumber != 1 {
		t.Errorf("expected winning number 1, got %d", result.Proof.WinningNumber)
	}
	if result.Proof.RevealedSeed != testServerSeed {
		t.Errorf("expected revealed seed %s, got %s", testServerSeed, result.Proof.RevealedSeed)
	}
	if result.Proof.CommitHash != testCommitHash {
		t.Errorf("expected commit hash %s, got %s", testCommitHash, result.Proof.CommitHash)
	}
}

func TestExecuteDraw_RaffleNotFound(t *testing.T) {
	drawRepo := newMockDrawRepo()
	raffleRepo := newMockRaffleRepo()
	svc := setupDrawService(drawRepo, raffleRepo, nil, nil, nil)

	_, err := svc.ExecuteDraw(context.Background(), "non-existent")
	if err == nil {
		t.Fatal("expected error for non-existent raffle")
	}
}

func TestExecuteDraw_RaffleNotActive(t *testing.T) {
	drawRepo := newMockDrawRepo()
	raffleRepo := newMockRaffleRepo()
	seedService := &mockSeedService{}

	raffleRepo.raffles["raffle-1"] = &ticketdomain.RaffleEntity{
		ID: "raffle-1", Status: "scheduled", TotalTickets: 100, SoldTickets: 0, TicketPrice: 10.0,
	}

	svc := setupDrawService(drawRepo, raffleRepo, nil, seedService, nil)

	_, err := svc.ExecuteDraw(context.Background(), "raffle-1")
	if err == nil {
		t.Fatal("expected error for non-active raffle")
	}
}

func TestExecuteDraw_NoTickets(t *testing.T) {
	drawRepo := newMockDrawRepo()
	raffleRepo := newMockRaffleRepo()
	seedService := &mockSeedService{}

	raffleRepo.raffles["raffle-1"] = &ticketdomain.RaffleEntity{
		ID: "raffle-1", Status: "active", TotalTickets: 100, SoldTickets: 0, TicketPrice: 10.0,
	}

	svc := setupDrawService(drawRepo, raffleRepo, &emptyTicketRepo{}, seedService, nil)

	_, err := svc.CommitDrawSeed(context.Background(), "raffle-1")
	if err != nil {
		t.Fatalf("unexpected error committing seed: %v", err)
	}

	_, err = svc.ExecuteDraw(context.Background(), "raffle-1")
	if err == nil {
		t.Fatal("expected error for no tickets")
	}
}

func TestExecuteDraw_NoCommitment(t *testing.T) {
	drawRepo := newMockDrawRepo()
	raffleRepo := newMockRaffleRepo()
	ticketRepo := &mockTicketRepo{}

	raffleRepo.raffles["raffle-1"] = &ticketdomain.RaffleEntity{
		ID: "raffle-1", Status: "active", TotalTickets: 100, SoldTickets: 1, TicketPrice: 10.0,
	}

	svc := setupDrawService(drawRepo, raffleRepo, ticketRepo, nil, nil)

	_, err := svc.ExecuteDraw(context.Background(), "raffle-1")
	if err == nil {
		t.Fatal("expected error when no seed is committed")
	}
}

func TestVerifyDraw_Success(t *testing.T) {
	drawRepo := newMockDrawRepo()
	raffleRepo := newMockRaffleRepo()
	ticketRepo := &mockTicketRepo{}
	seedService := &mockSeedService{}

	raffleRepo.raffles["raffle-1"] = &ticketdomain.RaffleEntity{
		ID: "raffle-1", Status: "active", TotalTickets: 100, SoldTickets: 1, TicketPrice: 10.0,
	}

	svc := setupDrawService(drawRepo, raffleRepo, ticketRepo, seedService, &mockRandomService{})

	_, err := svc.CommitDrawSeed(context.Background(), "raffle-1")
	if err != nil {
		t.Fatalf("unexpected error committing seed: %v", err)
	}

	_, err = svc.ExecuteDraw(context.Background(), "raffle-1")
	if err != nil {
		t.Fatalf("unexpected error executing draw: %v", err)
	}

	result, err := svc.VerifyDraw(context.Background(), "raffle-1")
	if err != nil {
		t.Fatalf("unexpected error verifying draw: %v", err)
	}
	if !result.Verified {
		t.Error("expected verification to pass")
	}
	if !result.SeedMatches {
		t.Error("expected seed to match commitment")
	}
	if !result.HashMatches {
		t.Error("expected combined hash to match")
	}
}

func TestVerifyDraw_NoDrawResult(t *testing.T) {
	drawRepo := newMockDrawRepo()
	svc := setupDrawService(drawRepo, nil, nil, nil, nil)

	_, err := svc.VerifyDraw(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent draw")
	}
}

func TestVerifyDraw_NoCommitment(t *testing.T) {
	drawRepo := newMockDrawRepo()
	raffleRepo := newMockRaffleRepo()
	ticketRepo := &mockTicketRepo{}
	seedService := &mockSeedService{}

	raffleRepo.raffles["raffle-1"] = &ticketdomain.RaffleEntity{
		ID: "raffle-1", Status: "active", TotalTickets: 100, SoldTickets: 1, TicketPrice: 10.0,
	}

	svc := setupDrawService(drawRepo, raffleRepo, ticketRepo, seedService, &mockRandomService{})

	_, err := svc.CommitDrawSeed(context.Background(), "raffle-1")
	if err != nil {
		t.Fatalf("unexpected error committing seed: %v", err)
	}

	_, err = svc.ExecuteDraw(context.Background(), "raffle-1")
	if err != nil {
		t.Fatalf("unexpected error executing draw: %v", err)
	}

	delete(drawRepo.commitments, "raffle-1")

	_, err = svc.VerifyDraw(context.Background(), "raffle-1")
	if err == nil {
		t.Fatal("expected error when no commitment exists")
	}
}

func TestGetDrawResult_Success(t *testing.T) {
	drawRepo := newMockDrawRepo()

	drawRepo.results["raffle-1"] = &domain.DrawResult{
		RaffleID: "raffle-1",
		Status:   "completed",
	}

	svc := setupDrawService(drawRepo, nil, nil, nil, nil)

	result, err := svc.GetDrawResult(context.Background(), "raffle-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RaffleID != "raffle-1" {
		t.Errorf("expected raffle_id raffle-1, got %s", result.RaffleID)
	}
}

func TestGetDrawResult_NotFound(t *testing.T) {
	drawRepo := newMockDrawRepo()
	svc := setupDrawService(drawRepo, nil, nil, nil, nil)

	_, err := svc.GetDrawResult(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent draw")
	}
}
