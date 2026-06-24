package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/raffle-app/backend/internal/draw/domain"
)

type DrawRepo struct {
	db *sql.DB
}

func NewDrawRepo(db *sql.DB) *DrawRepo {
	return &DrawRepo{db: db}
}

func (r *DrawRepo) Create(ctx context.Context, result *domain.DrawResult) error {
	query := `INSERT INTO raffle_draws 
		(raffle_id, drawn_at, status, winner_ticket_number, winner_ticket_id, server_seed_hash, revealed_seed, combined_hash, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) 
		RETURNING id, created_at`
	err := r.db.QueryRowContext(ctx, query,
		result.RaffleID, result.DrawTimestamp, result.Status,
		result.WinningTicketNumber, result.WinningTicketID,
		result.Proof.CommitHash, result.Proof.RevealedSeed,
		result.Proof.CombinedHash, result.CreatedAt,
	).Scan(&result.ID, &result.CreatedAt)
	if err != nil {
		return err
	}
	return nil
}

func (r *DrawRepo) FindByRaffleID(ctx context.Context, raffleID string) (*domain.DrawResult, error) {
	result := &domain.DrawResult{}
	query := `SELECT id, raffle_id, drawn_at, status, winner_ticket_number, winner_ticket_id, 
		server_seed_hash, revealed_seed, combined_hash, created_at
		FROM raffle_draws WHERE raffle_id = $1 ORDER BY drawn_at DESC LIMIT 1`
	err := r.db.QueryRowContext(ctx, query, raffleID).Scan(
		&result.ID, &result.RaffleID, &result.DrawTimestamp,
		&result.Status, &result.WinningTicketNumber, &result.WinningTicketID,
		&result.Proof.CommitHash, &result.Proof.RevealedSeed,
		&result.Proof.CombinedHash, &result.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("draw not found")
	}
	if err != nil {
		return nil, err
	}

	result.Proof.WinningNumber = result.WinningTicketNumber
	result.Proof.VerificationURL = "/api/v1/draw/verify"

	return result, nil
}

func (r *DrawRepo) ExistsForRaffle(ctx context.Context, raffleID string) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM raffle_draws WHERE raffle_id = $1`
	err := r.db.QueryRowContext(ctx, query, raffleID).Scan(&count)
	return count > 0, err
}

func (r *DrawRepo) CommitSeed(ctx context.Context, commitment *domain.DrawCommitment) error {
	query := `INSERT INTO draw_commitments (raffle_id, server_seed, commit_hash, created_at) VALUES ($1, $2, $3, $4)`
	_, err := r.db.ExecContext(ctx, query, commitment.RaffleID, commitment.ServerSeed, commitment.CommitHash, commitment.CreatedAt)
	return err
}

func (r *DrawRepo) GetCommitment(ctx context.Context, raffleID string) (*domain.DrawCommitment, error) {
	commitment := &domain.DrawCommitment{}
	query := `SELECT id, raffle_id, server_seed, commit_hash, created_at FROM draw_commitments WHERE raffle_id = $1`
	err := r.db.QueryRowContext(ctx, query, raffleID).Scan(
		&commitment.ID, &commitment.RaffleID, &commitment.ServerSeed, &commitment.CommitHash, &commitment.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, errors.New("commitment not found")
	}
	if err != nil {
		return nil, err
	}
	return commitment, nil
}

func (r *DrawRepo) FindByDrawID(ctx context.Context, drawID string) (*domain.DrawResult, error) {
	result := &domain.DrawResult{}
	query := `SELECT id, raffle_id, drawn_at, status, winner_ticket_number, winner_ticket_id,
		server_seed_hash, revealed_seed, combined_hash, created_at
		FROM raffle_draws WHERE id = $1`
	err := r.db.QueryRowContext(ctx, query, drawID).Scan(
		&result.ID, &result.RaffleID, &result.DrawTimestamp,
		&result.Status, &result.WinningTicketNumber, &result.WinningTicketID,
		&result.Proof.CommitHash, &result.Proof.RevealedSeed,
		&result.Proof.CombinedHash, &result.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("draw not found")
	}
	if err != nil {
		return nil, err
	}
	result.Proof.WinningNumber = result.WinningTicketNumber
	result.Proof.VerificationURL = "/api/v1/draw/verify"
	return result, nil
}
