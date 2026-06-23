package repository

import (
	"context"
	"database/sql"
	"time"
	"fmt"

	"github.com/raffle-app/backend/internal/audit/domain"
)

type AuditRepo struct {
	db *sql.DB
}

func NewAuditRepo(db *sql.DB) *AuditRepo {
	return &AuditRepo{db: db}
}

func (r *AuditRepo) Create(ctx context.Context, log *domain.AuditLog) error {
	query := `INSERT INTO audit_logs (actor_id, actor_type, action, resource_type, resource_id, old_value, new_value, ip_address, user_agent, created_at)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING id`
	err := r.db.QueryRowContext(ctx, query,
		log.ActorID, log.ActorType, log.Action, log.ResourceType, log.ResourceID,
		log.OldValue, log.NewValue, log.IPAddress, log.UserAgent, log.CreatedAt,
	).Scan(&log.ID)
	return err
}

func (r *AuditRepo) FindByID(ctx context.Context, id string) (*domain.AuditLog, error) {
	log := &domain.AuditLog{}
	query := `SELECT id, actor_id, actor_type, action, resource_type, resource_id, old_value, new_value, ip_address, user_agent, created_at
	          FROM audit_logs WHERE id = $1`
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&log.ID, &log.ActorID, &log.ActorType, &log.Action, &log.ResourceType, &log.ResourceID,
		&log.OldValue, &log.NewValue, &log.IPAddress, &log.UserAgent, &log.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("audit log not found")
	}
	return log, err
}

// filterClause builds the shared WHERE predicate from a filter, returning the
// clause string and bound args starting at the given argIdx.
func filterClause(filter domain.AuditLogFilter, argIdx int) (string, []interface{}) {
	clause := ""
	args := []interface{}{}
	if filter.ActorID != nil {
		clause += fmt.Sprintf(" AND actor_id = $%d", argIdx)
		args = append(args, *filter.ActorID)
		argIdx++
	}
	if filter.ActorType != nil {
		clause += fmt.Sprintf(" AND actor_type = $%d", argIdx)
		args = append(args, *filter.ActorType)
		argIdx++
	}
	if filter.Action != nil {
		clause += fmt.Sprintf(" AND action = $%d", argIdx)
		args = append(args, *filter.Action)
		argIdx++
	}
	if filter.ResourceType != nil {
		clause += fmt.Sprintf(" AND resource_type = $%d", argIdx)
		args = append(args, *filter.ResourceType)
		argIdx++
	}
	if filter.ResourceID != nil {
		clause += fmt.Sprintf(" AND resource_id = $%d", argIdx)
		args = append(args, *filter.ResourceID)
		argIdx++
	}
	if filter.StartDate != nil {
		clause += fmt.Sprintf(" AND created_at >= $%d", argIdx)
		args = append(args, *filter.StartDate)
		argIdx++
	}
	if filter.EndDate != nil {
		clause += fmt.Sprintf(" AND created_at <= $%d", argIdx)
		args = append(args, *filter.EndDate)
	}
	return clause, args
}

func (r *AuditRepo) FindByFilter(ctx context.Context, filter domain.AuditLogFilter) ([]domain.AuditLog, int, error) {
	where, args := filterClause(filter, 1)

	countQuery := `SELECT COUNT(*) FROM audit_logs WHERE 1=1` + where
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `SELECT id, actor_id, actor_type, action, resource_type, resource_id, old_value, new_value, ip_address, user_agent, created_at
	          FROM audit_logs WHERE 1=1` + where + " ORDER BY created_at DESC"

	nextIdx := len(args) + 1
	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", nextIdx)
		args = append(args, filter.Limit)
		nextIdx++
		if filter.Offset > 0 {
			query += fmt.Sprintf(" OFFSET $%d", nextIdx)
			args = append(args, filter.Offset)
		}
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []domain.AuditLog
	for rows.Next() {
		var l domain.AuditLog
		if err := rows.Scan(
			&l.ID, &l.ActorID, &l.ActorType, &l.Action, &l.ResourceType, &l.ResourceID,
			&l.OldValue, &l.NewValue, &l.IPAddress, &l.UserAgent, &l.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		logs = append(logs, l)
	}
	return logs, total, nil
}

func (r *AuditRepo) Count(ctx context.Context, filter domain.AuditLogFilter) (int, error) {
	where, args := filterClause(filter, 1)
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_logs WHERE 1=1`+where, args...).Scan(&count)
	return count, err
}

func (r *AuditRepo) DeleteOlderThan(ctx context.Context, cutoffDate time.Time) (int64, error) {
	query := `DELETE FROM audit_logs WHERE created_at < $1`
	result, err := r.db.ExecContext(ctx, query, cutoffDate)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
