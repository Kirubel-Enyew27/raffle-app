package repository

import (
	"context"
	"database/sql"

	"github.com/raffle-app/backend/internal/sms/domain"
)

type SMSLogRepo struct {
	db *sql.DB
}

func NewSMSLogRepo(db *sql.DB) *SMSLogRepo {
	return &SMSLogRepo{db: db}
}

func (r *SMSLogRepo) Create(ctx context.Context, entry *domain.SMSLogEntry) error {
	query := `INSERT INTO sms_logs
		(id, sender, raw_message, parsed_amount, parsed_sender_name, parsed_sender_phone,
		 parsed_transaction_id, parsed_timestamp, credited, credited_amount,
		 credited_user_id, credited_wallet_id,
		 receipt_verified, receipt_amount, receipt_payer_name, receipt_status,
		 error_message, ip_address, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, NOW())`
	_, err := r.db.ExecContext(ctx, query,
		entry.ID, entry.Sender, entry.RawMessage,
		entry.ParsedAmount, entry.ParsedSenderName, entry.ParsedSenderPhone,
		entry.ParsedTransactionID, entry.ParsedTimestamp,
		entry.Credited, entry.CreditedAmount,
		entry.CreditedUserID, entry.CreditedWalletID,
		entry.ReceiptVerified, entry.ReceiptAmount,
		entry.ReceiptPayerName, entry.ReceiptStatus,
		entry.ErrorMessage, entry.IPAddress,
	)
	return err
}

func (r *SMSLogRepo) ExistsByTransactionID(ctx context.Context, transactionID string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM sms_logs WHERE parsed_transaction_id = $1 AND credited = TRUE)`,
		transactionID,
	).Scan(&exists)
	return exists, err
}
