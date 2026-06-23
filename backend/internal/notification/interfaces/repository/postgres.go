package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/raffle-app/backend/internal/notification/domain"
)

type NotificationRepo struct{ db *sql.DB }

func NewNotificationRepo(db *sql.DB) *NotificationRepo { return &NotificationRepo{db: db} }

func (r *NotificationRepo) Create(ctx context.Context, n *domain.Notification) error {
	q := `INSERT INTO notifications
	        (user_id, channel, event, subject, body, status, retries, created_at, updated_at)
	      VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id`
	return r.db.QueryRowContext(ctx, q,
		n.UserID, n.Channel, n.Event, n.Subject, n.Body, n.Status, n.Retries, n.CreatedAt, n.UpdatedAt,
	).Scan(&n.ID)
}

func (r *NotificationRepo) UpdateStatus(ctx context.Context, id string, status domain.Status, errMsg string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE notifications SET status=$1, error=$2, updated_at=$3 WHERE id=$4`,
		status, errMsg, time.Now(), id,
	)
	return err
}

func (r *NotificationRepo) IncrRetries(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE notifications SET retries = retries + 1, updated_at=$1 WHERE id=$2`,
		time.Now(), id,
	)
	return err
}

func (r *NotificationRepo) FindByUserID(ctx context.Context, userID string, limit, offset int) ([]domain.Notification, int, error) {
	var total int
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM notifications WHERE user_id=$1 AND channel='in_app'`, userID,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, channel, event, subject, body, status, retries, error, read_at, created_at, updated_at
		 FROM notifications WHERE user_id=$1 AND channel='in_app'
		 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var ns []domain.Notification
	for rows.Next() {
		var n domain.Notification
		var errStr sql.NullString
		if err := rows.Scan(
			&n.ID, &n.UserID, &n.Channel, &n.Event, &n.Subject, &n.Body,
			&n.Status, &n.Retries, &errStr, &n.ReadAt, &n.CreatedAt, &n.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		if errStr.Valid {
			n.Error = errStr.String
		}
		ns = append(ns, n)
	}
	return ns, total, nil
}

func (r *NotificationRepo) FindByID(ctx context.Context, id string) (*domain.Notification, error) {
	n := &domain.Notification{}
	var errStr sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, channel, event, subject, body, status, retries, error, read_at, created_at, updated_at
		 FROM notifications WHERE id=$1`, id,
	).Scan(
		&n.ID, &n.UserID, &n.Channel, &n.Event, &n.Subject, &n.Body,
		&n.Status, &n.Retries, &errStr, &n.ReadAt, &n.CreatedAt, &n.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("notification not found")
	}
	if err != nil {
		return nil, err
	}
	if errStr.Valid {
		n.Error = errStr.String
	}
	return n, nil
}

func (r *NotificationRepo) MarkRead(ctx context.Context, id, userID string) error {
	now := time.Now()
	res, err := r.db.ExecContext(ctx,
		`UPDATE notifications SET read_at=$1, updated_at=$2 WHERE id=$3 AND user_id=$4 AND channel='in_app' AND read_at IS NULL`,
		now, now, id, userID,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("notification not found or already read")
	}
	return nil
}

func (r *NotificationRepo) CountUnread(ctx context.Context, userID string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM notifications WHERE user_id=$1 AND channel='in_app' AND read_at IS NULL`,
		userID,
	).Scan(&count)
	return count, err
}
