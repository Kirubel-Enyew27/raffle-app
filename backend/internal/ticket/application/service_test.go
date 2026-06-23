package application

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
	ticketdomain "github.com/raffle-app/backend/internal/ticket/domain"
)

var testNow = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

type testDriver struct{}

func (d *testDriver) Open(name string) (driver.Conn, error) {
	return &testConn{}, nil
}

func (d *testDriver) OpenConnector(name string) (driver.Connector, error) {
	return &testConnector{}, nil
}

type testConnector struct{}

func (c *testConnector) Connect(ctx context.Context) (driver.Conn, error) {
	return &testConn{}, nil
}

func (c *testConnector) Driver() driver.Driver {
	return &testDriver{}
}

type testConn struct{}

func (c *testConn) Prepare(query string) (driver.Stmt, error) {
	return &testStmt{}, nil
}

func (c *testConn) Close() error                           { return nil }
func (c *testConn) Begin() (driver.Tx, error)              { return &testTx{}, nil }

type testStmt struct{}

func (s *testStmt) Close() error                                   { return nil }
func (s *testStmt) NumInput() int                                  { return -1 }
func (s *testStmt) Exec(args []driver.Value) (driver.Result, error) { return driver.ResultNoRows, nil }
func (s *testStmt) Query(args []driver.Value) (driver.Rows, error) { return &testRows{}, nil }

type testRows struct{}

func (r *testRows) Columns() []string              { return []string{} }
func (r *testRows) Close() error                  { return nil }
func (r *testRows) Next(dest []driver.Value) error { return errors.New("no rows") }

type testTx struct{}

func (t *testTx) Commit() error   { return nil }
func (t *testTx) Rollback() error { return nil }

func init() {
	sql.Register("testdriver", &testDriver{})
}

type mockIdempotencyStore struct {
	store  map[string]string
	mu     sync.Mutex
}

func newMockIdempotencyStore() *mockIdempotencyStore {
	return &mockIdempotencyStore{store: make(map[string]string)}
}

func (m *mockIdempotencyStore) Get(ctx context.Context, key string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.store[key], nil
}

func (m *mockIdempotencyStore) Set(ctx context.Context, key string, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store[key] = value
	return nil
}

type mockTicketRepo struct {
	tickets   map[string][]*ticketdomain.Ticket
	mu        sync.Mutex
	maxNumber map[string]int
}

func newMockTicketRepo() *mockTicketRepo {
	return &mockTicketRepo{
		tickets:   make(map[string][]*ticketdomain.Ticket),
		maxNumber: make(map[string]int),
	}
}

func (m *mockTicketRepo) Create(ctx context.Context, ticket *ticketdomain.Ticket) error {
	ticket.ID = "ticket-" + ticket.RaffleID
	ticket.CreatedAt = testNow
	ticket.UpdatedAt = testNow
	key := ticket.RaffleID
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tickets[key] = append(m.tickets[key], ticket)
	return nil
}

func (m *mockTicketRepo) CreateBatch(ctx context.Context, tickets []*ticketdomain.Ticket) error {
	for i := range tickets {
		key := tickets[i].RaffleID
		m.mu.Lock()
		ticketNum := len(m.tickets[key]) + 1
		tickets[i].ID = "ticket-" + tickets[i].RaffleID + "-" + string(rune(ticketNum))
		tickets[i].TicketNumber = ticketNum
		tickets[i].CreatedAt = testNow
		tickets[i].UpdatedAt = testNow
		m.tickets[key] = append(m.tickets[key], tickets[i])
		m.mu.Unlock()
	}
	return nil
}

func (m *mockTicketRepo) CreateBatchTx(ctx context.Context, tx *sql.Tx, tickets []*ticketdomain.Ticket) error {
	return m.CreateBatch(ctx, tickets)
}

func (m *mockTicketRepo) CountByRaffleID(ctx context.Context, raffleID string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.tickets[raffleID]), nil
}

func (m *mockTicketRepo) FindByRaffleID(ctx context.Context, raffleID string) ([]*ticketdomain.Ticket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.tickets[raffleID], nil
}

func (m *mockTicketRepo) FindByWalletTxID(ctx context.Context, walletTxID string) ([]*ticketdomain.Ticket, error) {
	return nil, nil
}

type mockRaffleRepo struct {
	raffles map[string]*ticketdomain.RaffleEntity
	mu      sync.Mutex
}

func newMockRaffleRepo() *mockRaffleRepo {
	return &mockRaffleRepo{raffles: make(map[string]*ticketdomain.RaffleEntity)}
}

func (m *mockRaffleRepo) FindByID(ctx context.Context, id string) (*ticketdomain.RaffleEntity, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.raffles[id]
	if !ok {
		return nil, errors.New("raffle not found")
	}
	copy := *r
	return &copy, nil
}

func (m *mockRaffleRepo) FindByIDForUpdate(ctx context.Context, tx *sql.Tx, id string) (*ticketdomain.RaffleEntity, error) {
	return m.FindByID(ctx, id)
}

func (m *mockRaffleRepo) UpdateSoldCount(ctx context.Context, raffleID string, increment int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.raffles[raffleID]
	if !ok {
		return errors.New("raffle not found")
	}
	r.SoldTickets += increment
	return nil
}

func (m *mockRaffleRepo) UpdateSoldCountTx(ctx context.Context, tx *sql.Tx, raffleID string, increment int) error {
	return m.UpdateSoldCount(ctx, raffleID, increment)
}

type mockWalletRepo struct {
	balance float64
	mu      sync.Mutex
	txs     []*ticketdomain.WalletTransaction
}

func newMockWalletRepo(balance float64) *mockWalletRepo {
	return &mockWalletRepo{balance: balance}
}

func (m *mockWalletRepo) FindByUserID(ctx context.Context, userID string) (*ticketdomain.Wallet, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if userID == "" {
		return nil, sql.ErrNoRows
	}
	return &ticketdomain.Wallet{ID: "wallet-" + userID, Balance: m.balance}, nil
}

func (m *mockWalletRepo) FindByUserIDForUpdate(ctx context.Context, tx *sql.Tx, userID string) (*ticketdomain.Wallet, error) {
	return m.FindByUserID(ctx, userID)
}

func (m *mockWalletRepo) UpdateBalanceTx(ctx context.Context, tx *sql.Tx, walletID string, newBalance float64) error {
	return m.UpdateBalance(ctx, walletID, newBalance)
}

func (m *mockWalletRepo) DebitTx(ctx context.Context, tx *sql.Tx, walletID string, amount float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.balance -= amount
	return nil
}

func (m *mockWalletRepo) CreateTransaction(ctx context.Context, walletTx *ticketdomain.WalletTransaction) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.txs = append(m.txs, walletTx)
	return nil
}

func (m *mockWalletRepo) CreateTransactionTx(ctx context.Context, tx *sql.Tx, walletTx *ticketdomain.WalletTransaction) error {
	return m.CreateTransaction(ctx, walletTx)
}

func (m *mockWalletRepo) UpdateBalance(ctx context.Context, walletID string, newBalance float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.balance = newBalance
	return nil
}

func (m *mockWalletRepo) Debit(ctx context.Context, walletID string, amount float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.balance -= amount
	return nil
}

func validRaffle() *ticketdomain.RaffleEntity {
	return &ticketdomain.RaffleEntity{
		ID: "raffle-1", Status: "active", TotalTickets: 100, SoldTickets: 0, TicketPrice: 10.0,
	}
}

func setupService(ticketRepo ticketdomain.TicketRepository, raffleRepo ticketdomain.RaffleRepository, walletRepo ticketdomain.WalletRepository, idempotencyStore *mockIdempotencyStore) *TicketService {
	var store IdempotencyStore
	if idempotencyStore != nil {
		store = idempotencyStore
	}
	db, _ := sql.Open("testdriver", "")
	return NewTicketService(db, ticketRepo, raffleRepo, walletRepo, nil, store)
}

func TestPurchaseTickets_Success(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	raffleRepo := newMockRaffleRepo()
	walletRepo := newMockWalletRepo(100.0)
	idempotencyStore := newMockIdempotencyStore()

	raffleRepo.raffles["raffle-1"] = validRaffle()

	svc := setupService(ticketRepo, raffleRepo, walletRepo, idempotencyStore)

	result, err := svc.PurchaseTickets(context.Background(), &ticketdomain.PurchaseTicketsInput{
		RaffleID: "raffle-1", UserID: "user-1", Quantity: 3, IdempotencyKey: "key-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Tickets) != 3 {
		t.Errorf("expected 3 tickets, got %d", len(result.Tickets))
	}
	if raffleRepo.raffles["raffle-1"].SoldTickets != 3 {
		t.Errorf("expected sold_tickets 3, got %d", raffleRepo.raffles["raffle-1"].SoldTickets)
	}
	if walletRepo.balance != 70.0 {
		t.Errorf("expected balance 70.0, got %.2f", walletRepo.balance)
	}
	if len(walletRepo.txs) != 1 {
		t.Errorf("expected 1 wallet transaction, got %d", len(walletRepo.txs))
	}

	// Verify idempotency key is stored
	if idempotencyStore.store["ticket_purchase:key-1"] == "" {
		t.Error("expected idempotency key to be stored")
	}
}

func TestPurchaseTickets_Idempotency(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	raffleRepo := newMockRaffleRepo()
	walletRepo := newMockWalletRepo(100.0)
	idempotencyStore := newMockIdempotencyStore()

	raffleRepo.raffles["raffle-1"] = validRaffle()

	svc := setupService(ticketRepo, raffleRepo, walletRepo, idempotencyStore)

	_, err := svc.PurchaseTickets(context.Background(), &ticketdomain.PurchaseTicketsInput{
		RaffleID: "raffle-1", UserID: "user-1", Quantity: 3, IdempotencyKey: "key-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Second call with same idempotency key should return cached result without double-charging
	walletRepo.balance = 100.0
	result, err := svc.PurchaseTickets(context.Background(), &ticketdomain.PurchaseTicketsInput{
		RaffleID: "raffle-1", UserID: "user-1", Quantity: 3, IdempotencyKey: "key-1",
	})
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
	if len(result.Tickets) != 3 {
		t.Errorf("expected 3 tickets, got %d", len(result.Tickets))
	}
	if walletRepo.balance != 100.0 {
		t.Errorf("expected balance unchanged (100.0), got %.2f", walletRepo.balance)
	}
	if len(walletRepo.txs) != 1 {
		t.Errorf("expected 1 wallet transaction total, got %d", len(walletRepo.txs))
	}
}

func TestPurchaseTickets_IdempotencyCacheCorrupted(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	raffleRepo := newMockRaffleRepo()
	walletRepo := newMockWalletRepo(100.0)
	idempotencyStore := newMockIdempotencyStore()

	raffleRepo.raffles["raffle-1"] = validRaffle()

	svc := setupService(ticketRepo, raffleRepo, walletRepo, idempotencyStore)

	_, err := svc.PurchaseTickets(context.Background(), &ticketdomain.PurchaseTicketsInput{
		RaffleID: "raffle-1", UserID: "user-1", Quantity: 3, IdempotencyKey: "key-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	idempotencyStore.store["ticket_purchase:key-1"] = "corrupted"

	// Should fall back to processing
	_, err = svc.PurchaseTickets(context.Background(), &ticketdomain.PurchaseTicketsInput{
		RaffleID: "raffle-1", UserID: "user-1", Quantity: 1, IdempotencyKey: "key-1",
	})
	if err != nil {
		t.Fatalf("expected fallback to success on corrupted cache, got error: %v", err)
	}
	if raffleRepo.raffles["raffle-1"].SoldTickets != 4 {
		t.Errorf("expected 4 tickets sold after fallback, got %d", raffleRepo.raffles["raffle-1"].SoldTickets)
	}
}

func TestPurchaseTickets_EmptyBody(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	raffleRepo := newMockRaffleRepo()
	walletRepo := newMockWalletRepo(100.0)

	svc := setupService(ticketRepo, raffleRepo, walletRepo, nil)

	_, err := svc.PurchaseTickets(context.Background(), &ticketdomain.PurchaseTicketsInput{})
	if err == nil {
		t.Fatal("expected error for empty body")
	}
}

func TestPurchaseTickets_ZeroQuantity(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	raffleRepo := newMockRaffleRepo()
	walletRepo := newMockWalletRepo(100.0)

	svc := setupService(ticketRepo, raffleRepo, walletRepo, nil)

	_, err := svc.PurchaseTickets(context.Background(), &ticketdomain.PurchaseTicketsInput{
		RaffleID: "raffle-1", UserID: "user-1", Quantity: 0,
	})
	if err == nil {
		t.Fatal("expected error for zero quantity")
	}
}

func TestPurchaseTickets_TooManyQuantity(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	raffleRepo := newMockRaffleRepo()
	walletRepo := newMockWalletRepo(100.0)

	svc := setupService(ticketRepo, raffleRepo, walletRepo, nil)

	_, err := svc.PurchaseTickets(context.Background(), &ticketdomain.PurchaseTicketsInput{
		RaffleID: "raffle-1", UserID: "user-1", Quantity: 11,
	})
	if err == nil {
		t.Fatal("expected error for quantity > 10")
	}
}

func TestPurchaseTickets_RaffleNotFound(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	raffleRepo := newMockRaffleRepo()
	walletRepo := newMockWalletRepo(100.0)

	svc := setupService(ticketRepo, raffleRepo, walletRepo, nil)

	_, err := svc.PurchaseTickets(context.Background(), &ticketdomain.PurchaseTicketsInput{
		RaffleID: "nonexistent", UserID: "user-1", Quantity: 1,
	})
	if err == nil {
		t.Fatal("expected error for nonexistent raffle")
	}
}

func TestPurchaseTickets_RaffleNotActive(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	raffleRepo := newMockRaffleRepo()
	walletRepo := newMockWalletRepo(100.0)

	raffleRepo.raffles["raffle-1"] = &ticketdomain.RaffleEntity{
		ID: "raffle-1", Status: "scheduled", TotalTickets: 100, SoldTickets: 0, TicketPrice: 10.0,
	}

	svc := setupService(ticketRepo, raffleRepo, walletRepo, nil)

	_, err := svc.PurchaseTickets(context.Background(), &ticketdomain.PurchaseTicketsInput{
		RaffleID: "raffle-1", UserID: "user-1", Quantity: 1,
	})
	if err == nil {
		t.Fatal("expected error for non-active raffle")
	}
}

func TestPurchaseTickets_SoldOut(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	raffleRepo := newMockRaffleRepo()
	walletRepo := newMockWalletRepo(100.0)

	raffleRepo.raffles["raffle-1"] = &ticketdomain.RaffleEntity{
		ID: "raffle-1", Status: "active", TotalTickets: 2, SoldTickets: 2, TicketPrice: 10.0,
	}

	svc := setupService(ticketRepo, raffleRepo, walletRepo, nil)

	_, err := svc.PurchaseTickets(context.Background(), &ticketdomain.PurchaseTicketsInput{
		RaffleID: "raffle-1", UserID: "user-1", Quantity: 1,
	})
	if err == nil {
		t.Fatal("expected error for sold out raffle")
	}
}

func TestPurchaseTickets_ExceedsRemaining(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	raffleRepo := newMockRaffleRepo()
	walletRepo := newMockWalletRepo(100.0)

	raffleRepo.raffles["raffle-1"] = &ticketdomain.RaffleEntity{
		ID: "raffle-1", Status: "active", TotalTickets: 5, SoldTickets: 3, TicketPrice: 10.0,
	}

	svc := setupService(ticketRepo, raffleRepo, walletRepo, nil)

	_, err := svc.PurchaseTickets(context.Background(), &ticketdomain.PurchaseTicketsInput{
		RaffleID: "raffle-1", UserID: "user-1", Quantity: 5,
	})
	if err == nil {
		t.Fatal("expected error for exceeding remaining tickets")
	}
}

func TestPurchaseTickets_InsufficientBalance(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	raffleRepo := newMockRaffleRepo()
	walletRepo := newMockWalletRepo(5.0)

	raffleRepo.raffles["raffle-1"] = validRaffle()

	svc := setupService(ticketRepo, raffleRepo, walletRepo, nil)

	_, err := svc.PurchaseTickets(context.Background(), &ticketdomain.PurchaseTicketsInput{
		RaffleID: "raffle-1", UserID: "user-1", Quantity: 1,
	})
	if err == nil {
		t.Fatal("expected error for insufficient balance")
	}
}

func TestPurchaseTickets_WalletNotFound(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	raffleRepo := newMockRaffleRepo()
	walletRepo := newMockWalletRepo(100.0)

	raffleRepo.raffles["raffle-1"] = validRaffle()

	svc := setupService(ticketRepo, raffleRepo, walletRepo, nil)

	_, err := svc.PurchaseTickets(context.Background(), &ticketdomain.PurchaseTicketsInput{
		RaffleID: "raffle-1", UserID: "", Quantity: 1,
	})
	if err == nil {
		t.Fatal("expected error for wallet not found")
	}
}

func TestPurchaseTickets_GeneratesSequentialTicketNumbers(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	raffleRepo := newMockRaffleRepo()
	walletRepo := newMockWalletRepo(100.0)

	raffle := validRaffle()
	raffleRepo.raffles["raffle-1"] = raffle

	svc := setupService(ticketRepo, raffleRepo, walletRepo, nil)

	result, err := svc.PurchaseTickets(context.Background(), &ticketdomain.PurchaseTicketsInput{
		RaffleID: "raffle-1", UserID: "user-1", Quantity: 3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i, ticket := range result.Tickets {
		if ticket.TicketNumber != i+1 {
			t.Errorf("expected ticket number %d, got %d", i+1, ticket.TicketNumber)
		}
	}
}

func TestPurchaseTickets_SequentialNumbersAfterExistingTickets(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	raffleRepo := newMockRaffleRepo()
	walletRepo := newMockWalletRepo(100.0)

	raffle := validRaffle()
	raffle.SoldTickets = 5
	raffleRepo.raffles["raffle-1"] = raffle

	for i := 1; i <= 5; i++ {
		ticketRepo.tickets["raffle-1"] = append(ticketRepo.tickets["raffle-1"], &ticketdomain.Ticket{
			ID: "existing-" + string(rune('0'+i)), RaffleID: "raffle-1", TicketNumber: i,
		})
	}

	svc := setupService(ticketRepo, raffleRepo, walletRepo, nil)

	result, err := svc.PurchaseTickets(context.Background(), &ticketdomain.PurchaseTicketsInput{
		RaffleID: "raffle-1", UserID: "user-1", Quantity: 2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Tickets[0].TicketNumber != 6 {
		t.Errorf("expected first ticket number 6, got %d", result.Tickets[0].TicketNumber)
	}
	if result.Tickets[1].TicketNumber != 7 {
		t.Errorf("expected second ticket number 7, got %d", result.Tickets[1].TicketNumber)
	}
}

func TestPurchaseTickets_ConcurrencySafety(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	raffleRepo := newMockRaffleRepo()
	walletRepo := newMockWalletRepo(100.0)

	raffle := validRaffle()
	raffle.TotalTickets = 5
	raffle.SoldTickets = 0
	raffleRepo.raffles["raffle-1"] = raffle

	idempotencyStore := newMockIdempotencyStore()
	svc := setupService(ticketRepo, raffleRepo, walletRepo, idempotencyStore)

	var wg sync.WaitGroup
	errCount := 0
	var mu sync.Mutex

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := svc.PurchaseTickets(context.Background(), &ticketdomain.PurchaseTicketsInput{
				RaffleID: "raffle-1", UserID: "user-" + string(rune('1'+idx)), Quantity: 1, IdempotencyKey: "key-" + string(rune('1'+idx)),
			})
			if err != nil {
				mu.Lock()
				errCount++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	if raffleRepo.raffles["raffle-1"].SoldTickets != 5 {
		t.Errorf("expected 5 tickets sold, got %d", raffleRepo.raffles["raffle-1"].SoldTickets)
	}
	if len(ticketRepo.tickets["raffle-1"]) != 5 {
		t.Errorf("expected 5 tickets created, got %d", len(ticketRepo.tickets["raffle-1"]))
	}
}

func TestPurchaseTickets_CreatesWalletTransactionWithCorrectDetails(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	raffleRepo := newMockRaffleRepo()
	walletRepo := newMockWalletRepo(100.0)

	raffleRepo.raffles["raffle-1"] = validRaffle()

	svc := setupService(ticketRepo, raffleRepo, walletRepo, nil)

	_, err := svc.PurchaseTickets(context.Background(), &ticketdomain.PurchaseTicketsInput{
		RaffleID: "raffle-1", UserID: "user-1", Quantity: 2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(walletRepo.txs) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(walletRepo.txs))
	}
	tx := walletRepo.txs[0]
	if tx.Type != "ticket_purchase" {
		t.Errorf("expected type ticket_purchase, got %s", tx.Type)
	}
	if tx.Amount != 20.0 {
		t.Errorf("expected amount 20.0, got %.2f", tx.Amount)
	}
	if tx.BalanceBefore != 100.0 {
		t.Errorf("expected balance_before 100.0, got %.2f", tx.BalanceBefore)
	}
	if tx.BalanceAfter != 80.0 {
		t.Errorf("expected balance_after 80.0, got %.2f", tx.BalanceAfter)
	}
}

func TestPurchaseTickets_ReturnsPurchaseResult(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	raffleRepo := newMockRaffleRepo()
	walletRepo := newMockWalletRepo(100.0)

	raffleRepo.raffles["raffle-1"] = validRaffle()

	svc := setupService(ticketRepo, raffleRepo, walletRepo, nil)

	result, err := svc.PurchaseTickets(context.Background(), &ticketdomain.PurchaseTicketsInput{
		RaffleID: "raffle-1", UserID: "user-1", Quantity: 2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.WalletTxID == "" {
		t.Error("expected wallet transaction ID in result")
	}
	if result.TotalSpent != 20.0 {
		t.Errorf("expected total spent 20.0, got %.2f", result.TotalSpent)
	}
	if len(result.Tickets) != 2 {
		t.Errorf("expected 2 tickets, got %d", len(result.Tickets))
	}
}

func TestPurchaseTickets_WithoutIdempotencyKey(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	raffleRepo := newMockRaffleRepo()
	walletRepo := newMockWalletRepo(100.0)

	raffleRepo.raffles["raffle-1"] = validRaffle()

	svc := setupService(ticketRepo, raffleRepo, walletRepo, newMockIdempotencyStore())

	result, err := svc.PurchaseTickets(context.Background(), &ticketdomain.PurchaseTicketsInput{
		RaffleID: "raffle-1", UserID: "user-1", Quantity: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Tickets) != 1 {
		t.Errorf("expected 1 ticket, got %d", len(result.Tickets))
	}
}

func TestPurchaseTickets_IdempotencyStorageRoundTrip(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	raffleRepo := newMockRaffleRepo()
	walletRepo := newMockWalletRepo(100.0)
	idempotencyStore := newMockIdempotencyStore()

	raffleRepo.raffles["raffle-1"] = validRaffle()

	svc := setupService(ticketRepo, raffleRepo, walletRepo, idempotencyStore)

	result, err := svc.PurchaseTickets(context.Background(), &ticketdomain.PurchaseTicketsInput{
		RaffleID: "raffle-1", UserID: "user-1", Quantity: 1, IdempotencyKey: "key-roundtrip",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify JSON round-trip
	stored := idempotencyStore.store["ticket_purchase:key-roundtrip"]
	var decoded ticketdomain.PurchaseResult
	if err := json.Unmarshal([]byte(stored), &decoded); err != nil {
		t.Fatalf("failed to unmarshal stored result: %v", err)
	}
	if len(decoded.Tickets) != len(result.Tickets) {
		t.Errorf("expected %d tickets in cache, got %d", len(result.Tickets), len(decoded.Tickets))
	}
	if decoded.WalletTxID != result.WalletTxID {
		t.Errorf("expected wallet tx ID %s, got %s", result.WalletTxID, decoded.WalletTxID)
	}
}
