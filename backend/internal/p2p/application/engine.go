package application

import (
	"context"
	"database/sql"
	"time"

	"github.com/raffle-app/backend/internal/p2p/domain"
	"github.com/raffle-app/backend/internal/realtime"
	"go.uber.org/zap"
)

type P2PEngine struct {
	matchRepo  domain.MatchRepository
	playerRepo domain.PlayerRepository
	escrowRepo domain.EscrowRepository
	realtime   *realtime.Service
	log        *zap.Logger
}

func NewP2PEngine(
	matchRepo domain.MatchRepository,
	playerRepo domain.PlayerRepository,
	escrowRepo domain.EscrowRepository,
	realtime *realtime.Service,
	log *zap.Logger,
) *P2PEngine {
	return &P2PEngine{
		matchRepo:  matchRepo,
		playerRepo: playerRepo,
		escrowRepo: escrowRepo,
		realtime:   realtime,
		log:        log,
	}
}

// idleRoomTTL is how long an unjoined room may sit before it is cancelled.
func idleRoomTTL(matchType string) time.Duration {
	if matchType == domain.TypePublicQueue {
		return 15 * time.Minute
	}
	return 5 * time.Minute
}

// Run starts the background workers for turn timeouts, disconnect timeouts, and
// idle room sweeps.
func (e *P2PEngine) Run(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	cleanupTicker := time.NewTicker(30 * time.Second)
	defer cleanupTicker.Stop()

	e.log.Info("P2P background engine started")

	for {
		select {
		case <-ctx.Done():
			e.log.Info("P2P background engine stopping")
			return
		case <-ticker.C:
			e.sweepTurnTimeouts(ctx)
			e.sweepDisconnectTimeouts(ctx)
		case <-cleanupTicker.C:
			e.sweepIdleRooms(ctx)
		}
	}
}

func (e *P2PEngine) sweepTurnTimeouts(ctx context.Context) {
	now := time.Now()
	expired, err := e.matchRepo.FindExpiredTurns(ctx, now)
	if err != nil {
		e.log.Error("failed to query expired P2P turns", zap.Error(err))
		return
	}

	for _, m := range expired {
		if err := e.handleTurnTimeout(ctx, m.ID); err != nil {
			e.log.Error("failed to handle P2P turn timeout", zap.String("match_id", m.ID), zap.Error(err))
		}
	}
}

// sweepDisconnectTimeouts forfeits any in-progress match whose disconnect
// deadline has elapsed without the disconnected player reconnecting.
func (e *P2PEngine) sweepDisconnectTimeouts(ctx context.Context) {
	now := time.Now()
	expired, err := e.matchRepo.FindExpiredDisconnects(ctx, now)
	if err != nil {
		e.log.Error("failed to query expired P2P disconnect deadlines", zap.Error(err))
		return
	}

	for _, m := range expired {
		if err := e.handleDisconnectTimeout(ctx, m.ID); err != nil {
			e.log.Error("failed to handle P2P disconnect timeout",
				zap.String("match_id", m.ID), zap.Error(err))
		}
	}
}

// escrowIncomplete reports whether the match is missing stake reservations for
// one or more of its players. A prize pool above zero means both players were
// expected to have reserved their stake when the match started, so a missing
// ledger row can never be settled: the escrow layer refuses to move money
// without the matching reservation, and the sweep would otherwise fail and
// retry every second for ever while both players look at a board that never
// finishes. Free matches (prize pool 0) reserve nothing and are never
// incomplete.
func (e *P2PEngine) escrowIncomplete(ctx context.Context, tx *sql.Tx, match *domain.Match, players []*domain.MatchPlayer) (bool, error) {
	if match.PrizePoolCents <= 0 {
		return false, nil
	}

	userIDs := make([]string, 0, len(players))
	for _, p := range players {
		userIDs = append(userIDs, p.UserID)
	}

	missing, err := e.escrowRepo.PlayersWithoutReservedStake(ctx, tx, match.ID, userIDs)
	if err != nil {
		return false, err
	}
	if len(missing) == 0 {
		return false, nil
	}

	e.log.Error("P2P match has incomplete stake reservations; finishing it without settlement",
		zap.String("match_id", match.ID),
		zap.Strings("user_ids", missing),
		zap.Int64("prize_pool_cents", int64(match.PrizePoolCents)))
	return true, nil
}

func (e *P2PEngine) handleDisconnectTimeout(ctx context.Context, matchID string) error {
	tx, err := e.matchRepo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	match, err := e.matchRepo.FindByIDForUpdate(ctx, tx, matchID)
	if err != nil {
		return err
	}

	// Guard against races: another sweep or a reconnect may have already cleared
	// the disconnect record or moved the match to a terminal state.
	if match.Status != domain.StatusInProgress ||
		match.DisconnectDeadline == nil ||
		time.Now().Before(*match.DisconnectDeadline) {
		return nil
	}

	if match.DisconnectUserID == nil {
		return nil
	}

	disconnectedUserID := *match.DisconnectUserID

	players, err := e.playerRepo.ListByMatchTx(ctx, tx, match.ID)
	if err != nil {
		return err
	}

	var winnerUserID string
	for _, p := range players {
		if p.UserID != disconnectedUserID {
			winnerUserID = p.UserID
			break
		}
	}

	if winnerUserID == "" {
		// Both players somehow disconnected: just cancel cleanly.
		now := time.Now()
		if err := e.matchRepo.MarkCancelledTx(ctx, tx, match, now); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		e.log.Warn("P2P match cancelled: both players disconnected",
			zap.String("match_id", match.ID))
		return nil
	}

	now := time.Now()
	// A match missing stake reservations can never be settled — the money is
	// unreachable — and failing here would retry every second for ever while
	// both players stay on a match that never ends. Finish it without touching
	// a wallet instead.
	incomplete, err := e.escrowIncomplete(ctx, tx, match, players)
	if err != nil {
		return err
	}
	if incomplete {
		if err := e.matchRepo.MarkCompletedTx(ctx, tx, match, &winnerUserID, domain.StatusForfeited, 0, now, true); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		e.log.Info("P2P match forfeited without settlement: incomplete stake reservations",
			zap.String("match_id", match.ID),
			zap.String("disconnected_user_id", disconnectedUserID),
			zap.String("winner_user_id", winnerUserID))
		if e.realtime != nil {
			_ = e.realtime.PublishToChannel(ctx, "p2p_match:"+match.ID, "p2p.match_completed",
				map[string]interface{}{
					"match_id":           match.ID,
					"winner_user_id":     winnerUserID,
					"status":             domain.StatusForfeited,
					"disconnect_user_id": disconnectedUserID,
					"reason":             "disconnect_timeout",
				})
		}
		return nil
	}

	settlement := domain.CalculateSettlement(match.PrizePoolCents, match.ServiceFeeBP, false)

	creditTxID, feeTxID, err := e.escrowRepo.SettleWinnerTx(ctx, tx, winnerUserID,
		match.PrizePoolCents, settlement.FeeCents, settlement.NetPayoutCents, match.ID)
	if err != nil {
		return err
	}

	if err := e.escrowRepo.FinalizeLoserTx(ctx, tx, disconnectedUserID, match.StakeCents, match.ID); err != nil {
		return err
	}

	for _, p := range players {
		if p.UserID == winnerUserID {
			_ = e.playerRepo.SetLedgerTxIDs(ctx, tx, p.ID, nil, &creditTxID, &feeTxID)
		}
	}

	if err := e.matchRepo.MarkCompletedTx(ctx, tx, match, &winnerUserID, domain.StatusForfeited,
		settlement.FeeCents, now, true); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	e.log.Info("P2P match forfeited: disconnect grace period expired",
		zap.String("match_id", match.ID),
		zap.String("disconnected_user_id", disconnectedUserID),
		zap.String("winner_user_id", winnerUserID))

	if e.realtime != nil {
		_ = e.realtime.PublishToChannel(ctx, "p2p_match:"+match.ID, "p2p.match_completed",
			map[string]interface{}{
				"match_id":           match.ID,
				"winner_user_id":     winnerUserID,
				"status":             domain.StatusForfeited,
				"disconnect_user_id": disconnectedUserID,
				"reason":             "disconnect_timeout",
			})
	}

	return nil
}

func (e *P2PEngine) handleTurnTimeout(ctx context.Context, matchID string) error {
	tx, err := e.matchRepo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	match, err := e.matchRepo.FindByIDForUpdate(ctx, tx, matchID)
	if err != nil {
		return err
	}

	if match.Status != domain.StatusInProgress || match.TurnDeadline == nil || time.Now().Before(*match.TurnDeadline) {
		return nil // state changed or refreshed
	}

	// A between-round reveal has no current turn: once its deadline passes,
	// promote the staged board and start the round the previous winning move
	// was holding back. This is checked before the disconnect guard so a
	// player dropping during the reveal cannot freeze the match until their
	// grace period expires.
	if match.CurrentTurnUserID == nil {
		return e.startNextRound(ctx, tx, match)
	}

	// A player inside the reconnection grace period is judged by that deadline,
	// not by the turn clock. Two competing timeout mechanisms would otherwise
	// forfeit a reconnecting player after seconds, making the documented grace
	// period meaningless (and the loss unfair).
	if match.DisconnectDeadline != nil && time.Now().Before(*match.DisconnectDeadline) {
		return nil
	}

	timedOutUserID := match.CurrentTurnUserID
	if timedOutUserID == nil {
		return nil
	}

	players, err := e.playerRepo.ListByMatchTx(ctx, tx, match.ID)
	if err != nil {
		return err
	}

	var winnerUserID string
	for _, p := range players {
		if p.UserID != *timedOutUserID {
			winnerUserID = p.UserID
			break
		}
	}

	if winnerUserID == "" {
		return nil
	}

	now := time.Now()
	settlement := domain.CalculateSettlement(match.PrizePoolCents, match.ServiceFeeBP, false)

	// See handleDisconnectTimeout: an unsettleable match must still reach a
	// terminal state, or the sweep retries it every second for ever.
	incomplete, err := e.escrowIncomplete(ctx, tx, match, players)
	if err != nil {
		return err
	}
	if incomplete {
		if err := e.matchRepo.MarkCompletedTx(ctx, tx, match, &winnerUserID, domain.StatusForfeited, 0, now, true); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		e.log.Info("P2P match timed out and forfeited without settlement: incomplete stake reservations",
			zap.String("match_id", match.ID),
			zap.String("forfeited_user_id", *timedOutUserID),
			zap.String("winner_user_id", winnerUserID))
		if e.realtime != nil {
			_ = e.realtime.PublishToChannel(ctx, "p2p_match:"+match.ID, "p2p.turn_timeout", map[string]interface{}{
				"match_id":          match.ID,
				"timed_out_user_id": *timedOutUserID,
				"winner_user_id":    winnerUserID,
				"status":            domain.StatusForfeited,
			})
		}
		return nil
	}

	creditTxID, feeTxID, err := e.escrowRepo.SettleWinnerTx(ctx, tx, winnerUserID, match.PrizePoolCents, settlement.FeeCents, settlement.NetPayoutCents, match.ID)
	if err != nil {
		return err
	}

	if err := e.escrowRepo.FinalizeLoserTx(ctx, tx, *timedOutUserID, match.StakeCents, match.ID); err != nil {
		return err
	}

	for _, p := range players {
		if p.UserID == winnerUserID {
			_ = e.playerRepo.SetLedgerTxIDs(ctx, tx, p.ID, nil, &creditTxID, &feeTxID)
		}
	}

	if err := e.matchRepo.MarkCompletedTx(ctx, tx, match, &winnerUserID, domain.StatusForfeited, settlement.FeeCents, now, true); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	e.log.Info("P2P match timed out and forfeited",
		zap.String("match_id", match.ID),
		zap.String("forfeited_user_id", *timedOutUserID),
		zap.String("winner_user_id", winnerUserID))

	if e.realtime != nil {
		_ = e.realtime.PublishToChannel(ctx, "p2p_match:"+match.ID, "p2p.turn_timeout", map[string]interface{}{
			"match_id":          match.ID,
			"timed_out_user_id": *timedOutUserID,
			"winner_user_id":    winnerUserID,
			"status":            domain.StatusForfeited,
		})
	}

	return nil
}

// startNextRound completes a staged between-round reveal: the finished board
// (with its winning line) has been on screen for roundRevealDelay, so swap in
// the prepared board and hand the move clock to the next round's starter.
func (e *P2PEngine) startNextRound(ctx context.Context, tx *sql.Tx, match *domain.Match) error {
	transition, ok := match.BoardState[roundTransitionKey].(map[string]interface{})
	if !ok {
		return nil // not a reveal; nothing to advance
	}

	nextBoard, ok := transition["next_board"].(map[string]interface{})
	starter, _ := transition["next_starter_user_id"].(string)
	if !ok || starter == "" {
		e.log.Error("P2P round transition is malformed; leaving the match unchanged",
			zap.String("match_id", match.ID))
		return nil
	}

	nextRound := intFromBoardValue(transition["next_round"])
	if nextRound <= match.CurrentRound {
		nextRound = match.CurrentRound + 1
	}

	now := time.Now()
	match.BoardState = nextBoard
	match.CurrentRound = nextRound
	match.CurrentTurnUserID = &starter
	deadline := now.Add(time.Duration(turnSecondsFor(match.GameSlug)) * time.Second)
	match.TurnDeadline = &deadline

	if err := e.matchRepo.UpdateStateTx(ctx, tx, match); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	e.log.Info("P2P next round started after the winning-line reveal",
		zap.String("match_id", match.ID),
		zap.Int("round", match.CurrentRound),
		zap.String("starter_user_id", starter))

	if e.realtime != nil {
		_ = e.realtime.PublishToChannel(ctx, "p2p_match:"+match.ID, "p2p.round_started", map[string]interface{}{
			"match_id":             match.ID,
			"round":                match.CurrentRound,
			"current_turn_user_id": starter,
		})
	}

	return nil
}

// intFromBoardValue reads an integer that may arrive as int (freshly built) or
// float64 (after a JSONB round trip).
func intFromBoardValue(value interface{}) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

func (e *P2PEngine) sweepIdleRooms(ctx context.Context) {
	now := time.Now()
	// Private rooms are only useful while the host is sharing the code; public
	// rooms live longer because an opponent may still match into them.
	idle, err := e.matchRepo.FindIdleWaitingRooms(ctx, now.Add(-idleRoomTTL(domain.TypePublicQueue)))
	if err != nil {
		e.log.Error("failed to query idle waiting rooms", zap.Error(err))
		return
	}

	for _, candidate := range idle {
		tx, err := e.matchRepo.BeginTx(ctx)
		if err != nil {
			continue
		}

		// Re-read under a row lock. Between the sweep query and this point the
		// room may have been joined, so cancelling (and refunding) must be based
		// on the current locked state, never on the earlier snapshot.
		r, err := e.matchRepo.FindByIDForUpdate(ctx, tx, candidate.ID)
		if err != nil {
			_ = tx.Rollback()
			continue
		}
		if r.Status != domain.StatusWaiting || !r.CreatedAt.Before(now.Add(-idleRoomTTL(r.MatchType))) {
			_ = tx.Rollback()
			continue
		}

		if err := e.matchRepo.MarkCancelledTx(ctx, tx, r, now); err != nil {
			_ = tx.Rollback()
			continue
		}

		if r.StakeCents > 0 {
			if err := e.escrowRepo.ReleaseReservedFundsTx(ctx, tx, r.CreatedBy, r.StakeCents, r.ID, "idle room auto-cancelled"); err != nil {
				_ = tx.Rollback()
				e.log.Error("failed to refund idle room stake", zap.String("match_id", r.ID), zap.Error(err))
				continue
			}
		}

		if err := tx.Commit(); err == nil {
			e.log.Info("idle match room auto-cancelled and refunded", zap.String("match_id", r.ID), zap.String("match_type", r.MatchType))
			if e.realtime != nil {
				_ = e.realtime.PublishToChannel(ctx, "p2p_match:"+r.ID, "p2p.match_cancelled", map[string]interface{}{
					"match_id": r.ID,
					"reason":   "idle timeout expired",
				})
			}
			// Cancelled rooms must disappear from open lobby lists right away.
			publishLobbyUpdate(ctx, e.realtime, "cancelled", r)
		}
	}
}
