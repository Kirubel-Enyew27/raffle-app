package application

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/raffle-app/backend/internal/wallet/domain"
	"github.com/raffle-app/backend/internal/wallet/infrastructure/verifier"
)

// ─── In-Memory Mock Implementations for Application Testing ─────────────────

type mockWalletRepo struct {
	mu           sync.Mutex
	wallets      map[string]*domain.Wallet
	userWallets  map[string]*domain.Wallet
	transactions map[string]*domain.WalletTransaction
	references   map[string]bool
	dbErr        error
}

func newMockWalletRepo() *mockWalletRepo {
	return &mockWalletRepo{
		wallets:      make(map[string]*domain.Wallet),
		userWallets:  make(map[string]*domain.Wallet),
		transactions: make(map[string]*domain.WalletTransaction),
		references:   make(map[string]bool),
	}
}

func (m *mockWalletRepo) Create(ctx context.Context, w *domain.Wallet) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.wallets[w.ID] = w
	m.userWallets[w.UserID] = w
	return nil
}

func (m *mockWalletRepo) CreateTx(ctx context.Context, tx *sql.Tx, w *domain.Wallet) error {
	return m.Create(ctx, w)
}

func (m *mockWalletRepo) FindByUserID(ctx context.Context, userID string) (*domain.Wallet, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if w, ok := m.userWallets[userID]; ok {
		return w, nil
	}
	// Auto create
	w := &domain.Wallet{
		ID:        "w-" + userID,
		UserID:    userID,
		Balance:   100.0,
		Currency:  "ETB",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.wallets[w.ID] = w
	m.userWallets[userID] = w
	return w, nil
}

func (m *mockWalletRepo) FindByUserIDWithLock(ctx context.Context, tx *sql.Tx, userID string) (*domain.Wallet, error) {
	return m.FindByUserID(ctx, userID)
}

func (m *mockWalletRepo) FindByID(ctx context.Context, id string) (*domain.Wallet, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if w, ok := m.wallets[id]; ok {
		return w, nil
	}
	return nil, fmt.Errorf("wallet not found")
}

func (m *mockWalletRepo) GetBalance(ctx context.Context, walletID string) (float64, error) {
	w, err := m.FindByID(ctx, walletID)
	if err != nil {
		return 0, err
	}
	return w.Balance, nil
}

func (m *mockWalletRepo) UpdateBalance(ctx context.Context, walletID string, newBalance float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.dbErr != nil {
		return m.dbErr
	}
	if w, ok := m.wallets[walletID]; ok {
		w.Balance = newBalance
		return nil
	}
	return fmt.Errorf("wallet not found")
}

func (m *mockWalletRepo) UpdateBalanceTx(ctx context.Context, tx *sql.Tx, walletID string, newBalance float64) error {
	return m.UpdateBalance(ctx, walletID, newBalance)
}

func (m *mockWalletRepo) Debit(ctx context.Context, walletID string, amount float64) error {
	return nil
}

func (m *mockWalletRepo) CreateTransaction(ctx context.Context, tx *domain.WalletTransaction) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.dbErr != nil {
		return m.dbErr
	}
	if tx.Reference != "" {
		if m.references[tx.Reference] {
			return fmt.Errorf("duplicate reference")
		}
		m.references[tx.Reference] = true
	}
	m.transactions[tx.ID] = tx
	return nil
}

func (m *mockWalletRepo) CreateTransactionTx(ctx context.Context, tx *sql.Tx, walletTx *domain.WalletTransaction) error {
	return m.CreateTransaction(ctx, walletTx)
}

func (m *mockWalletRepo) FindTransactionsByWalletID(ctx context.Context, walletID string, limit, offset int, txType string) ([]*domain.WalletTransaction, error) {
	return nil, nil
}

func (m *mockWalletRepo) CountTransactionsByWalletID(ctx context.Context, walletID string, txType string) (int64, error) {
	return 0, nil
}

func (m *mockWalletRepo) FindTransactionsByStatus(ctx context.Context, txType, status string) ([]*domain.WalletTransaction, error) {
	return nil, nil
}

func (m *mockWalletRepo) FindTransactionByID(ctx context.Context, id string) (*domain.WalletTransaction, error) {
	return nil, nil
}

func (m *mockWalletRepo) UpdateTransactionStatus(ctx context.Context, id, status string) error {
	return nil
}

func (m *mockWalletRepo) ExistsByReference(ctx context.Context, reference string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.references[reference], nil
}

// ─── Mock Deposit Repo ───────────────────────────────────────────────────────

type mockDepositRepo struct {
	mu         sync.Mutex
	deposits   map[string]*domain.Deposit
	references map[string]string
}

func newMockDepositRepo() *mockDepositRepo {
	return &mockDepositRepo{
		deposits:   make(map[string]*domain.Deposit),
		references: make(map[string]string),
	}
}

func (r *mockDepositRepo) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return nil, nil
}

func (r *mockDepositRepo) Create(ctx context.Context, d *domain.Deposit) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deposits[d.ID] = d
	return nil
}

func (r *mockDepositRepo) CreateTx(ctx context.Context, tx *sql.Tx, d *domain.Deposit) error {
	return r.Create(ctx, d)
}

func (r *mockDepositRepo) FindByID(ctx context.Context, id string) (*domain.Deposit, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if d, ok := r.deposits[id]; ok {
		// return a clone
		clone := *d
		return &clone, nil
	}
	return nil, fmt.Errorf("deposit not found")
}

func (r *mockDepositRepo) FindByIDWithLock(ctx context.Context, tx *sql.Tx, id string) (*domain.Deposit, error) {
	return r.FindByID(ctx, id)
}

func (r *mockDepositRepo) FindByExternalReference(ctx context.Context, ref string) (*domain.Deposit, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, d := range r.deposits {
		if d.ExternalReference == ref {
			clone := *d
			return &clone, nil
		}
	}
	return nil, fmt.Errorf("not found")
}

func (r *mockDepositRepo) Update(ctx context.Context, d *domain.Deposit) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deposits[d.ID] = d
	return nil
}

func (r *mockDepositRepo) UpdateTx(ctx context.Context, tx *sql.Tx, d *domain.Deposit) error {
	return r.Update(ctx, d)
}

func (r *mockDepositRepo) ListByUserID(ctx context.Context, userID string, limit, offset int) ([]*domain.Deposit, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var list []*domain.Deposit
	for _, d := range r.deposits {
		if d.UserID == userID {
			list = append(list, d)
		}
	}
	return list, int64(len(list)), nil
}

// ─── Tests ──────────────────────────────────────────────────────────────────

func TestWalletService_CreateDepositIntent(t *testing.T) {
	wRepo := newMockWalletRepo()
	dRepo := newMockDepositRepo()
	svc := NewWalletService(wRepo, nil)
	svc.SetDepositRepository(dRepo)

	// Valid intent
	dep, err := svc.CreateDepositIntent(context.Background(), "user-1", 600.0, "cbe")
	if err != nil {
		t.Fatalf("expected intent creation to succeed, got %v", err)
	}
	if dep.Status != domain.DepositStatusPending {
		t.Errorf("expected pending status, got %s", dep.Status)
	}
	if dep.Amount != 600.0 {
		t.Errorf("expected 600.0 amount, got %.2f", dep.Amount)
	}
	if dep.ExpiresAt.Before(time.Now()) {
		t.Errorf("expected future expiration, got %v", dep.ExpiresAt)
	}

	// Invalid amount
	_, err = svc.CreateDepositIntent(context.Background(), "user-1", -50.0, "cbe")
	if err == nil {
		t.Fatal("expected error for negative amount, got nil")
	}

	// Unsupported method
	_, err = svc.CreateDepositIntent(context.Background(), "user-1", 100.0, "bitcoin")
	if err == nil {
		t.Fatal("expected error for unsupported method, got nil")
	}
}

func TestWalletService_VerifyDeposit_Success(t *testing.T) {
	wRepo := newMockWalletRepo()
	dRepo := newMockDepositRepo()
	svc := NewWalletService(wRepo, nil)
	svc.SetDepositRepository(dRepo)

	mockCBE := &appMockVerifier{
		result: &domain.VerificationResult{
			Provider:        "cbe",
			TransactionID:   "FT26248LDXLB",
			Status:          "COMPLETED",
			AmountCredited:  600.0,
			AmountDebited:   600.61,
			FeeAmount:       0.61,
			Currency:        "ETB",
			ReceiverName:    "Kirubel Enyew",
			ReceiverAccount: "1********7383",
		},
	}
	vm := verifier.NewVerificationManager(mockCBE, nil, nil, "Kirubel Enyew", "1000249277383", "", "", true)
	svc.SetVerificationManager(vm)

	// Create deposit intent for 600 ETB
	dep, err := svc.CreateDepositIntent(context.Background(), "user-1", 600.0, "cbe")
	if err != nil {
		t.Fatalf("intent creation failed: %v", err)
	}

	// Verify deposit
	updatedDep, tx, err := svc.VerifyDeposit(context.Background(), dep.ID, "user-1", "v2-token123", "", nil, "")
	if err != nil {
		t.Fatalf("verification failed: %v", err)
	}

	if updatedDep.Status != domain.DepositStatusCredited {
		t.Errorf("expected credited status, got %s", updatedDep.Status)
	}
	if updatedDep.ActualAmountCredited != 600.0 {
		t.Errorf("expected 600.0 credited, got %.2f", updatedDep.ActualAmountCredited)
	}
	if tx.Amount != 600.0 {
		t.Errorf("expected 600.0 tx amount, got %.2f", tx.Amount)
	}

	// Wallet balance was 100, now should be 700
	w, _ := wRepo.FindByUserID(context.Background(), "user-1")
	if w.Balance != 700.0 {
		t.Errorf("expected wallet balance 700.0, got %.2f", w.Balance)
	}
}

func TestWalletService_VerifyDeposit_DuplicateProtection(t *testing.T) {
	wRepo := newMockWalletRepo()
	dRepo := newMockDepositRepo()
	svc := NewWalletService(wRepo, nil)
	svc.SetDepositRepository(dRepo)

	mockCBE := &appMockVerifier{
		result: &domain.VerificationResult{
			Provider:       "cbe",
			TransactionID:  "FT_DUP_123",
			Status:         "COMPLETED",
			AmountCredited: 600.0,
			Currency:       "ETB",
			ReceiverName:   "Kirubel Enyew",
		},
	}
	vm := verifier.NewVerificationManager(mockCBE, nil, nil, "Kirubel Enyew", "1000", "", "", true)
	svc.SetVerificationManager(vm)

	// 1. User 1 creates intent and deposits
	dep1, _ := svc.CreateDepositIntent(context.Background(), "user-1", 600.0, "cbe")
	_, _, err := svc.VerifyDeposit(context.Background(), dep1.ID, "user-1", "v2-dup", "", nil, "")
	if err != nil {
		t.Fatalf("first verification should succeed: %v", err)
	}

	// 2. User 1 submits again on same intent
	_, _, err = svc.VerifyDeposit(context.Background(), dep1.ID, "user-1", "v2-dup", "", nil, "")
	if err == nil {
		t.Fatal("expected error on re-submitting credited deposit, got nil")
	}

	// 3. User 2 tries to submit the same transaction reference on a new intent
	dep2, _ := svc.CreateDepositIntent(context.Background(), "user-2", 600.0, "cbe")
	_, _, err = svc.VerifyDeposit(context.Background(), dep2.ID, "user-2", "v2-dup", "", nil, "")
	if err == nil {
		t.Fatal("expected error on submitting duplicate transaction reference by different user, got nil")
	}

	// Balance of user 2 must remain unchanged
	w2, _ := wRepo.FindByUserID(context.Background(), "user-2")
	if w2.Balance != 100.0 {
		t.Errorf("user 2 wallet balance should remain 100.0, got %.2f", w2.Balance)
	}
}

func TestWalletService_VerifyDeposit_ConcurrentRequests(t *testing.T) {
	wRepo := newMockWalletRepo()
	dRepo := newMockDepositRepo()
	svc := NewWalletService(wRepo, nil)
	svc.SetDepositRepository(dRepo)

	mockCBE := &appMockVerifier{
		result: &domain.VerificationResult{
			Provider:       "cbe",
			TransactionID:  "FT_CONCURRENT",
			Status:         "COMPLETED",
			AmountCredited: 500.0,
			Currency:       "ETB",
			ReceiverName:   "Kirubel Enyew",
		},
	}
	vm := verifier.NewVerificationManager(mockCBE, nil, nil, "Kirubel Enyew", "1000", "", "", true)
	svc.SetVerificationManager(vm)

	const concurrency = 10
	var wg sync.WaitGroup
	wg.Add(concurrency)

	successCount := 0
	var mu sync.Mutex

	for i := 0; i < concurrency; i++ {
		userID := fmt.Sprintf("user-%d", i)
		go func(uid string) {
			defer wg.Done()
			dep, _ := svc.CreateDepositIntent(context.Background(), uid, 500.0, "cbe")
			_, _, err := svc.VerifyDeposit(context.Background(), dep.ID, uid, "v2-concurrent", "", nil, "")
			if err == nil {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(userID)
	}

	wg.Wait()

	if successCount != 1 {
		t.Errorf("expected exactly 1 verification to succeed under concurrent calls, got %d", successCount)
	}
}

func TestWalletService_VerifyDeposit_ProviderFailure(t *testing.T) {
	wRepo := newMockWalletRepo()
	dRepo := newMockDepositRepo()
	svc := NewWalletService(wRepo, nil)
	svc.SetDepositRepository(dRepo)

	mockFailing := &appMockVerifier{
		err: fmt.Errorf("connection timeout to bank"),
	}
	vm := verifier.NewVerificationManager(mockFailing, nil, nil, "Kirubel", "1000", "", "", true)
	svc.SetVerificationManager(vm)

	dep, _ := svc.CreateDepositIntent(context.Background(), "user-1", 600.0, "cbe")
	_, _, err := svc.VerifyDeposit(context.Background(), dep.ID, "user-1", "v2-timeout", "", nil, "")
	if err == nil {
		t.Fatal("expected error on provider timeout, got nil")
	}

	// Deposit status should be failed
	storedDep, _ := dRepo.FindByID(context.Background(), dep.ID)
	if storedDep.Status != domain.DepositStatusFailed {
		t.Errorf("expected failed status, got %s", storedDep.Status)
	}

	// Balance must NOT have increased
	w, _ := wRepo.FindByUserID(context.Background(), "user-1")
	if w.Balance != 100.0 {
		t.Errorf("wallet balance should be unchanged (100.0), got %.2f", w.Balance)
	}

	// Now retry with working verifier
	mockWorking := &appMockVerifier{
		result: &domain.VerificationResult{
			Provider:       "cbe",
			TransactionID:  "FT_RETRY_OK",
			Status:         "COMPLETED",
			AmountCredited: 600.0,
			Currency:       "ETB",
			ReceiverName:   "Kirubel",
		},
	}
	svc.SetVerificationManager(verifier.NewVerificationManager(mockWorking, nil, nil, "Kirubel", "1000", "", "", true))

	updatedDep, tx, err := svc.VerifyDeposit(context.Background(), dep.ID, "user-1", "v2-retry", "", nil, "")
	if err != nil {
		t.Fatalf("retry should succeed, got error: %v", err)
	}
	if updatedDep.Status != domain.DepositStatusCredited || tx == nil {
		t.Errorf("expected credited status on retry, got %s", updatedDep.Status)
	}
}

func TestWalletService_VerifyDeposit_DatabaseErrorRollback(t *testing.T) {
	wRepo := newMockWalletRepo()
	dRepo := newMockDepositRepo()
	svc := NewWalletService(wRepo, nil)
	svc.SetDepositRepository(dRepo)

	// Simulate database failure during transaction insert
	wRepo.dbErr = fmt.Errorf("disk full or connection dropped")

	mockCBE := &appMockVerifier{
		result: &domain.VerificationResult{
			Provider:       "cbe",
			TransactionID:  "FT_DB_ERR",
			Status:         "COMPLETED",
			AmountCredited: 600.0,
			Currency:       "ETB",
			ReceiverName:   "Kirubel",
		},
	}
	vm := verifier.NewVerificationManager(mockCBE, nil, nil, "Kirubel", "1000", "", "", true)
	svc.SetVerificationManager(vm)

	dep, _ := svc.CreateDepositIntent(context.Background(), "user-1", 600.0, "cbe")
	_, _, err := svc.VerifyDeposit(context.Background(), dep.ID, "user-1", "v2-dberr", "", nil, "")
	if err == nil {
		t.Fatal("expected error on DB failure, got nil")
	}

	// Balance must remain unchanged
	w, _ := wRepo.FindByUserID(context.Background(), "user-1")
	if w.Balance != 100.0 {
		t.Errorf("balance should be rolled back to 100.0, got %.2f", w.Balance)
	}
}

type appMockVerifier struct {
	result *domain.VerificationResult
	err    error
}

func (m *appMockVerifier) Verify(ctx context.Context, req domain.VerificationRequest) (*domain.VerificationResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}
