package application

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	auditapp "github.com/raffle-app/backend/internal/audit/application"
	ticketdomain "github.com/raffle-app/backend/internal/ticket/domain"
	apperrors "github.com/raffle-app/backend/pkg/errors"
	appcontext "github.com/raffle-app/backend/pkg/context"
)

type IdempotencyStore interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string) error
}

type TicketService struct {
	db             *sql.DB
	ticketRepo     ticketdomain.TicketRepository
	raffleRepo     ticketdomain.RaffleRepository
	walletRepo     ticketdomain.WalletRepository
	auditService   *auditapp.AuditService
	idempotencyStore IdempotencyStore
}

func NewTicketService(
	db *sql.DB,
	ticketRepo ticketdomain.TicketRepository,
	raffleRepo ticketdomain.RaffleRepository,
	walletRepo ticketdomain.WalletRepository,
	auditService *auditapp.AuditService,
	idempotencyStore IdempotencyStore,
) *TicketService {
	return &TicketService{
		db:             db,
		ticketRepo:     ticketRepo,
		raffleRepo:     raffleRepo,
		walletRepo:     walletRepo,
		auditService:   auditService,
		idempotencyStore: idempotencyStore,
	}
}

func generateID() string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	return fmt.Sprintf("%x", buf)
}

func (s *TicketService) ListByRaffle(ctx context.Context, raffleID string) ([]*ticketdomain.Ticket, error) {
	return s.ticketRepo.FindByRaffleID(ctx, raffleID)
}

func (s *TicketService) PurchaseTickets(ctx context.Context, input *ticketdomain.PurchaseTicketsInput) (*ticketdomain.PurchaseResult, error) {
	if input.Quantity <= 0 || input.Quantity > 10 {
		return nil, apperrors.ErrValidationFailed
	}

	cacheKey := "ticket_purchase:" + input.IdempotencyKey
	if s.idempotencyStore != nil && input.IdempotencyKey != "" {
		cached, err := s.idempotencyStore.Get(ctx, cacheKey)
		if err != nil {
			return nil, apperrors.WithField("IDEMPOTENCY_ERROR", "failed to check idempotency key", 500, err)
		}
		if cached != "" {
			var result ticketdomain.PurchaseResult
			if err := json.Unmarshal([]byte(cached), &result); err == nil {
				return &result, nil
			}
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperrors.WithField("TX_ERROR", "failed to begin transaction", 500, err)
	}
	defer tx.Rollback()

	raffle, err := s.raffleRepo.FindByIDForUpdate(ctx, tx, input.RaffleID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.ErrNotFound
		}
		return nil, apperrors.WithField("RAFFLE_ERROR", "failed to load raffle", 500, err)
	}
	if !raffle.IsActive() {
		return nil, apperrors.WithField("INVALID_STATUS", "raffle is not active", 400, nil)
	}
	if !raffle.HasRemainingTickets() {
		return nil, apperrors.WithField("SOLD_OUT", "raffle is sold out", 409, nil)
	}
	if raffle.SoldTickets+input.Quantity > raffle.TotalTickets {
		return nil, apperrors.WithField("SOLD_OUT", "not enough tickets remaining", 409, nil)
	}

	wallet, err := s.walletRepo.FindByUserIDForUpdate(ctx, tx, input.UserID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.ErrNotFound
		}
		return nil, apperrors.WithField("WALLET_ERROR", "failed to load wallet", 500, err)
	}

	cost := float64(input.Quantity) * raffle.TicketPrice
	if wallet.Balance < cost {
		return nil, apperrors.ErrInsufficientFunds
	}

	walletTxID := generateID()
	newBalance := wallet.Balance - cost

	walletTx := &ticketdomain.WalletTransaction{
		ID:            walletTxID,
		WalletID:      wallet.ID,
		UserID:        input.UserID,
		Type:          "ticket_purchase",
		Status:        "completed",
		Amount:        cost,
		BalanceBefore: wallet.Balance,
		BalanceAfter:  newBalance,
		Reference:     input.RaffleID,
		Description:   fmt.Sprintf("Purchased %d tickets", input.Quantity),
		Metadata:      map[string]interface{}{"ticket_count": input.Quantity, "ticket_price": raffle.TicketPrice},
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	if err := s.walletRepo.CreateTransactionTx(ctx, tx, walletTx); err != nil {
		return nil, apperrors.WithField("TX_ERROR", "failed to create wallet transaction", 500, err)
	}
	if err := s.walletRepo.UpdateBalanceTx(ctx, tx, wallet.ID, newBalance); err != nil {
		return nil, apperrors.WithField("TX_ERROR", "failed to update wallet balance", 500, err)
	}

	if err := s.raffleRepo.UpdateSoldCountTx(ctx, tx, input.RaffleID, input.Quantity); err != nil {
		return nil, apperrors.WithField("TX_ERROR", "failed to update raffle sold count", 500, err)
	}

	tickets := make([]*ticketdomain.Ticket, input.Quantity)
	for i := 0; i < input.Quantity; i++ {
		tickets[i] = &ticketdomain.Ticket{
			ID:          generateID(),
			RaffleID:    input.RaffleID,
			UserID:      input.UserID,
			WalletTxID:  walletTxID,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
	}
	if err := s.ticketRepo.CreateBatchTx(ctx, tx, tickets); err != nil {
		return nil, apperrors.WithField("TX_ERROR", "failed to create tickets", 500, err)
	}

	if err := tx.Commit(); err != nil {
		return nil, apperrors.WithField("TX_ERROR", "failed to commit transaction", 500, err)
	}

	result := &ticketdomain.PurchaseResult{
		Tickets:    tickets,
		WalletTxID: walletTxID,
		TotalSpent: cost,
	}

	if s.idempotencyStore != nil && input.IdempotencyKey != "" {
		data, _ := json.Marshal(result)
		_ = s.idempotencyStore.Set(ctx, cacheKey, string(data))
	}

	if s.auditService != nil {
		actorID := input.UserID
		if actorID == "" {
			actorID = appcontext.GetUserID(ctx)
		}
		actorType := "user"
		if appcontext.GetUserRole(ctx) != "" {
			actorType = appcontext.GetUserRole(ctx)
		}
		newVal := fmt.Sprintf("purchased %d tickets for raffle %s costing %.2f", input.Quantity, input.RaffleID, cost)
		_ = s.auditService.Record(ctx, &actorID, actorType, "ticket_purchase", "ticket", &tickets[0].ID, "", nil, &newVal)
	}

	return result, nil
}
