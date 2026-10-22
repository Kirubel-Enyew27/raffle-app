package application

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"time"

	auditapp "github.com/raffle-app/backend/internal/audit/application"
	"github.com/raffle-app/backend/internal/p2p/domain"
	"github.com/raffle-app/backend/internal/realtime"
	"github.com/raffle-app/backend/pkg/money"
)

type MatchView struct {
	ID                string                 `json:"id"`
	GameSlug          string                 `json:"game_slug"`
	Status            string                 `json:"status"`
	MatchType         string                 `json:"match_type"`
	InviteCode        string                 `json:"invite_code,omitempty"`
	StakeAmount       float64                `json:"stake_amount"`
	PrizePool         float64                `json:"prize_pool"`
	ServiceFeePercent float64                `json:"service_fee_percent"`
	TotalFee          float64                `json:"total_fee"`
	WinnerUserID      *string                `json:"winner_user_id,omitempty"`
	CurrentTurnUserID *string                `json:"current_turn_user_id,omitempty"`
	TurnSecondsLeft   int                    `json:"turn_seconds_left"`
	CurrentRound      int                    `json:"current_round"`
	MaxRounds         int                    `json:"max_rounds"`
	Scores            map[string]int         `json:"scores"`
	BoardState        map[string]interface{} `json:"board_state"`
	CreatedBy         string                 `json:"created_by"`
	// Rematch state, so the result screen knows whether the opponent already
	// asked for a return game.
	RematchRequestedBy *string               `json:"rematch_requested_by,omitempty"`
	RematchMatchID     *string               `json:"rematch_match_id,omitempty"`
	MyPlayerIndex      *int                  `json:"my_player_index,omitempty"`
	Players            []*domain.MatchPlayer `json:"players"`
	StartedAt          *time.Time            `json:"started_at,omitempty"`
	CompletedAt        *time.Time            `json:"completed_at,omitempty"`
	CreatedAt          time.Time             `json:"created_at"`
}

// Rematch outcomes returned by RequestRematch.
const (
	RematchRequested = "requested"
	RematchStarted   = "started"
)

type RematchResult struct {
	Status string     `json:"status"`
	Match  *MatchView `json:"match"`
}

type CreateMatchRequest struct {
	GameSlug    string  `json:"game_slug" binding:"required"`
	MatchType   string  `json:"match_type"` // public_queue | private_room
	StakeAmount float64 `json:"stake_amount"`
	// AutoMatch turns a public-queue create into matchmaking: join a waiting
	// room for the same game and stake if one exists, otherwise open a new one.
	AutoMatch bool `json:"auto_match"`
}

type P2PService struct {
	gameRepo           domain.GameRepository
	matchRepo          domain.MatchRepository
	playerRepo         domain.PlayerRepository
	moveRepo           domain.MoveRepository
	settingsRepo       domain.SettingsRepository
	escrowRepo         domain.EscrowRepository
	ruleEngines        map[string]domain.RuleEngine
	realtime           *realtime.Service
	auditSvc           *auditapp.AuditService
	paidMatchesEnabled bool
}

func NewP2PService(
	gameRepo domain.GameRepository,
	matchRepo domain.MatchRepository,
	playerRepo domain.PlayerRepository,
	moveRepo domain.MoveRepository,
	settingsRepo domain.SettingsRepository,
	escrowRepo domain.EscrowRepository,
	realtime *realtime.Service,
	auditSvc *auditapp.AuditService,
) *P2PService {
	engines := map[string]domain.RuleEngine{
		"tic-tac-toe":         domain.NewTicTacToeEngine(),
		"connect-four":        domain.NewConnectFourEngine(),
		"rock-paper-scissors": domain.NewRockPaperScissorsEngine(),
	}

	return &P2PService{
		gameRepo:           gameRepo,
		matchRepo:          matchRepo,
		playerRepo:         playerRepo,
		moveRepo:           moveRepo,
		settingsRepo:       settingsRepo,
		escrowRepo:         escrowRepo,
		ruleEngines:        engines,
		realtime:           realtime,
		auditSvc:           auditSvc,
		paidMatchesEnabled: false, // Strict default: paid matches disabled until regulatory clearance
	}
}

func (s *P2PService) SetPaidMatchesEnabled(enabled bool) {
	s.paidMatchesEnabled = enabled
}

func (s *P2PService) ListGames(ctx context.Context) ([]*domain.Game, error) {
	return s.gameRepo.ListActive(ctx)
}

func (s *P2PService) GetGame(ctx context.Context, slug string) (*domain.Game, error) {
	return s.gameRepo.FindBySlug(ctx, slug)
}

func (s *P2PService) GetLeaderboard(ctx context.Context, filter domain.LeaderboardFilter) (*domain.LeaderboardResult, error) {
	if filter.Game != nil && *filter.Game != "" {
		if _, err := s.gameRepo.FindBySlug(ctx, *filter.Game); err != nil {
			return nil, err
		}
	}
	return s.matchRepo.GetLeaderboard(ctx, filter)
}

func (s *P2PService) ListWaitingMatches(ctx context.Context, gameSlug string, limit int) ([]*MatchView, error) {
	matches, err := s.matchRepo.ListWaiting(ctx, gameSlug, limit)
	if err != nil {
		return nil, err
	}

	views := make([]*MatchView, len(matches))
	for i, m := range matches {
		players, _ := s.playerRepo.ListByMatch(ctx, m.ID)
		m.Players = players
		views[i] = s.buildView(m, "")
	}
	return views, nil
}

func (s *P2PService) ListUserMatches(ctx context.Context, userID string, limit, offset int) ([]*MatchView, int, error) {
	matches, total, err := s.matchRepo.ListUserMatches(ctx, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	views := make([]*MatchView, len(matches))
	for i, m := range matches {
		players, _ := s.playerRepo.ListByMatch(ctx, m.ID)
		m.Players = players
		views[i] = s.buildView(m, userID)
	}
	return views, total, nil
}

func (s *P2PService) CreateMatch(ctx context.Context, userID string, req CreateMatchRequest) (*MatchView, error) {
	game, err := s.gameRepo.FindBySlug(ctx, req.GameSlug)
	if err != nil {
		return nil, err
	}
	if !game.IsActive {
		return nil, domain.ErrGameNotFound
	}

	// Resolve the rule engine up front: a game row without a compiled engine must
	// fail as a client error rather than panicking on InitBoard().
	engine, ok := s.ruleEngines[game.Slug]
	if !ok {
		return nil, domain.ErrUnsupportedGame
	}

	stakeCents := money.FromFloat(req.StakeAmount)
	if stakeCents > 0 {
		if !s.paidMatchesEnabled {
			return nil, domain.ErrPaidMatchesDisabled
		}
		if stakeCents < game.MinStakeCents || stakeCents > game.MaxStakeCents {
			return nil, fmt.Errorf("%w: stake must be between %.2f and %.2f ETB",
				domain.ErrInvalidStake, game.MinStakeCents.Float(), game.MaxStakeCents.Float())
		}
	}

	matchType := domain.TypePublicQueue
	if req.MatchType == domain.TypePrivateRoom {
		matchType = domain.TypePrivateRoom
	}

	// Quick Play: pair with a waiting opponent instead of opening a second
	// room. Both players used to sit in their own "waiting" room forever.
	if matchType == domain.TypePublicQueue && req.AutoMatch {
		if view := s.tryAutoMatch(ctx, userID, game.Slug, stakeCents); view != nil {
			return view, nil
		}
	}

	var inviteCode string
	if matchType == domain.TypePrivateRoom {
		inviteCode = generateInviteCode()
	}

	symbol := symbolFor(game.Slug, 0)

	maxRounds := 1
	if game.Slug == "tic-tac-toe" || game.Slug == "rock-paper-scissors" {
		maxRounds = 3
	}

	tx, err := s.matchRepo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	match := &domain.Match{
		GameSlug:       game.Slug,
		Status:         domain.StatusWaiting,
		MatchType:      matchType,
		InviteCode:     inviteCode,
		StakeCents:     stakeCents,
		StakeAmount:    stakeCents.Float(),
		PrizePoolCents: stakeCents,
		ServiceFeeBP:   game.DefaultFeeBP,
		CurrentRound:   1,
		MaxRounds:      maxRounds,
		Scores:         map[string]int{"0": 0, "1": 0},
		BoardState:     engine.InitBoard(),
		CreatedBy:      userID,
	}

	// 1. Create the match row first so the generated ID is available as the
	// ledger reference for the stake reservation. Reserving before the insert
	// produced rows referenced only by "p2p_stake:", which could never be
	// settled or refunded.
	if err := s.matchRepo.CreateTx(ctx, tx, match); err != nil {
		return nil, err
	}

	// 2. Reserve creator funds in escrow
	var debitTxID string
	if stakeCents > 0 {
		txID, err := s.escrowRepo.ReserveFundsTx(ctx, tx, userID, stakeCents, match.ID)
		if err != nil {
			return nil, err
		}
		debitTxID = txID
	}

	// 3. Add creator as Player 0
	var debitPtr *string
	if debitTxID != "" {
		debitPtr = &debitTxID
	}

	player := &domain.MatchPlayer{
		MatchID:     match.ID,
		UserID:      userID,
		PlayerIndex: 0,
		Symbol:      symbol,
		Status:      domain.PlayerStatusReserved,
		Score:       0,
		DebitTxID:   debitPtr,
	}

	if err := s.playerRepo.AddPlayerTx(ctx, tx, player); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	match.Players = []*domain.MatchPlayer{player}

	if s.realtime != nil {
		_ = s.realtime.Publish(ctx, "p2p.match_created", "", "", map[string]interface{}{
			"match_id":   match.ID,
			"game_slug":  match.GameSlug,
			"match_type": match.MatchType,
			"stake":      match.StakeAmount,
			"created_by": userID,
		})
	}

	return s.buildView(match, userID), nil
}

// tryAutoMatch joins the newest waiting public room for the same game and stake
// that belongs to somebody else. Failing that it re-opens the caller's own
// waiting room, so a double click does not queue them twice. Returns nil when
// there is nothing to join and the caller should open a fresh room.
func (s *P2PService) tryAutoMatch(ctx context.Context, userID, gameSlug string, stakeCents money.Cents) *MatchView {
	rooms, err := s.matchRepo.ListWaiting(ctx, gameSlug, 20)
	if err != nil {
		return nil
	}

	// ListWaiting returns newest first: prefer the room whose host just opened
	// it, so an opponent who is still on the page is picked over an old room.
	for _, room := range rooms {
		if room.CreatedBy == userID || room.StakeCents != stakeCents {
			continue
		}
		// JoinMatch re-checks the room under a row lock, so a room filled or
		// cancelled between the query above and this call simply fails and we
		// move on to the next candidate.
		if view, err := s.JoinMatch(ctx, userID, room.ID, ""); err == nil {
			return view
		}
	}

	for _, room := range rooms {
		if room.CreatedBy != userID || room.StakeCents != stakeCents {
			continue
		}
		players, err := s.playerRepo.ListByMatch(ctx, room.ID)
		if err != nil {
			return nil
		}
		if findPlayer(players, userID) == nil {
			continue
		}
		room.Players = players
		return s.buildView(room, userID)
	}

	return nil
}

func (s *P2PService) JoinMatch(ctx context.Context, userID, matchID, inviteCode string) (*MatchView, error) {
	tx, err := s.matchRepo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var match *domain.Match
	if matchID != "" {
		m, err := s.matchRepo.FindByIDForUpdate(ctx, tx, matchID)
		if err != nil {
			return nil, err
		}
		match = m
	} else if inviteCode != "" {
		m, err := s.matchRepo.FindByInviteCode(ctx, inviteCode)
		if err != nil {
			return nil, err
		}
		// Re-lock
		locked, err := s.matchRepo.FindByIDForUpdate(ctx, tx, m.ID)
		if err != nil {
			return nil, err
		}
		match = locked
	} else {
		return nil, errors.New("must provide match_id or invite_code")
	}

	if match.Status != domain.StatusWaiting {
		return nil, domain.ErrMatchNotJoinable
	}

	existingPlayers, err := s.playerRepo.ListByMatchTx(ctx, tx, match.ID)
	if err != nil {
		return nil, err
	}

	for _, p := range existingPlayers {
		if p.UserID == userID {
			return nil, domain.ErrCannotJoinOwnMatch
		}
	}

	if len(existingPlayers) >= 2 {
		return nil, domain.ErrMatchFull
	}

	// Reserve joiner's stake in escrow
	var debitTxID string
	if match.StakeCents > 0 {
		txID, err := s.escrowRepo.ReserveFundsTx(ctx, tx, userID, match.StakeCents, match.ID)
		if err != nil {
			return nil, err
		}
		debitTxID = txID
	}

	symbol := symbolFor(match.GameSlug, 1)

	var debitPtr *string
	if debitTxID != "" {
		debitPtr = &debitTxID
	}

	p2 := &domain.MatchPlayer{
		MatchID:     match.ID,
		UserID:      userID,
		PlayerIndex: 1,
		Symbol:      symbol,
		Status:      domain.PlayerStatusReserved,
		Score:       0,
		DebitTxID:   debitPtr,
	}

	if err := s.playerRepo.AddPlayerTx(ctx, tx, p2); err != nil {
		return nil, err
	}

	// Update match to ready & pool to 2x stake
	match.Status = domain.StatusReady
	match.PrizePoolCents = match.StakeCents * 2
	if err := s.matchRepo.UpdateStateTx(ctx, tx, match); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	match.Players = append(existingPlayers, p2)

	if s.realtime != nil {
		_ = s.realtime.PublishToChannel(ctx, matchChannel(match.ID), "p2p.player_joined", map[string]interface{}{
			"match_id": match.ID,
			"player":   p2,
		})
	}

	return s.buildView(match, userID), nil
}

func (s *P2PService) ReadyUp(ctx context.Context, userID, matchID string) (*MatchView, error) {
	tx, err := s.matchRepo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	match, err := s.matchRepo.FindByIDForUpdate(ctx, tx, matchID)
	if err != nil {
		return nil, err
	}

	if match.Status != domain.StatusReady && match.Status != domain.StatusWaiting {
		return nil, fmt.Errorf("%w: cannot ready up in status %s", domain.ErrMatchNotJoinable, match.Status)
	}

	players, err := s.playerRepo.ListByMatchTx(ctx, tx, match.ID)
	if err != nil {
		return nil, err
	}

	membership := findPlayer(players, userID)
	if membership == nil {
		return nil, domain.ErrNotParticipant
	}

	now := time.Now()
	// Ready-up is idempotent: an already-ready player must not error out on a
	// retried request.
	if membership.Status != domain.PlayerStatusReady {
		if err := s.playerRepo.UpdateStatusTx(ctx, tx, match.ID, userID, domain.PlayerStatusReady, &now); err != nil {
			return nil, err
		}
		membership.Status = domain.PlayerStatusReady
		membership.ReadyAt = &now
	}

	// Check if all players (both 2) are ready
	allReady := len(players) == 2
	for _, p := range players {
		if p.Status != domain.PlayerStatusReady {
			allReady = false
		}
	}

	if allReady {
		// Game begins!
		engine, ok := s.ruleEngines[match.GameSlug]
		if !ok {
			return nil, errors.New("unsupported game rule engine")
		}

		turnSeconds := 20
		if match.GameSlug == "tic-tac-toe" || match.GameSlug == "rock-paper-scissors" {
			turnSeconds = 15
		}

		deadline := now.Add(time.Duration(turnSeconds) * time.Second)
		p0ID := players[0].UserID

		match.Status = domain.StatusInProgress
		match.CurrentTurnUserID = &p0ID
		match.TurnDeadline = &deadline
		match.StartedAt = &now
		match.BoardState = engine.InitBoard()

		if err := s.matchRepo.UpdateStateTx(ctx, tx, match); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	match.Players = players
	view := s.buildView(match, userID)

	if s.realtime != nil {
		_ = s.realtime.PublishToChannel(ctx, matchChannel(match.ID), "p2p.ready_status", map[string]interface{}{
			"match_id": match.ID,
			"user_id":  userID,
		})

		if allReady {
			_ = s.realtime.PublishToChannel(ctx, matchChannel(match.ID), "p2p.match_started", view)
		}
	}

	return view, nil
}

func (s *P2PService) SubmitMove(ctx context.Context, userID, matchID string, action map[string]interface{}) (*MatchView, error) {
	tx, err := s.matchRepo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	match, err := s.matchRepo.FindByIDForUpdate(ctx, tx, matchID)
	if err != nil {
		return nil, err
	}

	if match.Status != domain.StatusInProgress {
		return nil, domain.ErrMatchNotInProgress
	}

	if match.CurrentTurnUserID == nil || *match.CurrentTurnUserID != userID {
		return nil, domain.ErrNotYourTurn
	}

	now := time.Now()
	// Allow 1.5s network buffer beyond deadline
	if match.TurnDeadline != nil && now.After(match.TurnDeadline.Add(1500*time.Millisecond)) {
		return nil, domain.ErrTurnTimeout
	}

	players, err := s.playerRepo.ListByMatchTx(ctx, tx, match.ID)
	if err != nil {
		return nil, err
	}

	var activePlayer *domain.MatchPlayer
	var opponentPlayer *domain.MatchPlayer
	for _, p := range players {
		if p.UserID == userID {
			activePlayer = p
		} else {
			opponentPlayer = p
		}
	}

	if activePlayer == nil || opponentPlayer == nil {
		return nil, errors.New("players not found")
	}

	engine, ok := s.ruleEngines[match.GameSlug]
	if !ok {
		return nil, errors.New("rule engine missing")
	}

	// Validate & apply move
	newBoard, isRoundOver, roundWinnerIdx, _, err := engine.ValidateAndApplyMove(match.BoardState, activePlayer.PlayerIndex, action)
	if err != nil {
		return nil, err
	}

	match.BoardState = newBoard

	// Record move log
	moveCount := 0
	if mc, ok := newBoard["move_count"].(int); ok {
		moveCount = mc
	}
	_ = s.moveRepo.RecordMoveTx(ctx, tx, &domain.MatchMove{
		MatchID:       match.ID,
		RoundNumber:   match.CurrentRound,
		MoveNumber:    moveCount,
		UserID:        userID,
		Action:        action,
		BoardSnapshot: newBoard,
	})

	var isMatchOver bool
	var matchWinnerIdx *int

	if isRoundOver {
		if roundWinnerIdx != nil {
			if *roundWinnerIdx == 0 {
				match.Scores["0"]++
				_ = s.playerRepo.UpdateScoreTx(ctx, tx, match.ID, players[0].UserID, match.Scores["0"])
			} else {
				match.Scores["1"]++
				_ = s.playerRepo.UpdateScoreTx(ctx, tx, match.ID, players[1].UserID, match.Scores["1"])
			}
		}

		// Check if match over
		isMatchOver, matchWinnerIdx = engine.CheckMatchOver(match.Scores, match.CurrentRound, match.MaxRounds)

		if !isMatchOver {
			// Advance to next round, reset board
			match.CurrentRound++
			match.BoardState = engine.InitBoard()
			// Carry the finished round's reveal into the fresh board. Without
			// this, simultaneous-move games (RPS) would wipe the players'
			// choices before either client could display them.
			if result, ok := newBoard["last_round_result"]; ok && result != nil {
				match.BoardState["last_round_result"] = result
			}
			// Switch starter for next round
			nextTurn := opponentPlayer.UserID
			match.CurrentTurnUserID = &nextTurn
			turnDuration := 20
			if match.GameSlug == "tic-tac-toe" || match.GameSlug == "rock-paper-scissors" {
				turnDuration = 15
			}
			deadline := now.Add(time.Duration(turnDuration) * time.Second)
			match.TurnDeadline = &deadline
		}
	} else {
		// Switch turn
		nextTurn := opponentPlayer.UserID
		match.CurrentTurnUserID = &nextTurn
		turnDuration := 20
		if match.GameSlug == "tic-tac-toe" || match.GameSlug == "rock-paper-scissors" {
			turnDuration = 15
		}
		deadline := now.Add(time.Duration(turnDuration) * time.Second)
		match.TurnDeadline = &deadline
	}

	if isMatchOver {
		var winnerUserID *string
		var status = domain.StatusCompleted
		isTie := (matchWinnerIdx == nil)

		if !isTie {
			wID := players[*matchWinnerIdx].UserID
			winnerUserID = &wID
		} else {
			status = domain.StatusTied
		}

		// Settle funds
		settlement := domain.CalculateSettlement(match.PrizePoolCents, match.ServiceFeeBP, isTie)
		match.TotalFeeCents = settlement.FeeCents
		match.WinnerUserID = winnerUserID
		match.Status = status
		match.CompletedAt = &now

		if isTie {
			// 100% refund to both players
			for _, p := range players {
				if err := s.escrowRepo.ReleaseReservedFundsTx(ctx, tx, p.UserID, match.StakeCents, match.ID, "match tied"); err != nil {
					return nil, fmt.Errorf("escrow refund error: %w", err)
				}
			}
		} else {
			// Winner gets net payout, loser finalized
			loserIdx := 1 - *matchWinnerIdx
			loserID := players[loserIdx].UserID

			creditTxID, feeTxID, err := s.escrowRepo.SettleWinnerTx(ctx, tx, *winnerUserID, match.PrizePoolCents, settlement.FeeCents, settlement.NetPayoutCents, match.ID)
			if err != nil {
				return nil, fmt.Errorf("escrow settlement error: %w", err)
			}
			if err := s.escrowRepo.FinalizeLoserTx(ctx, tx, loserID, match.StakeCents, match.ID); err != nil {
				return nil, fmt.Errorf("escrow loser finalization error: %w", err)
			}

			_ = s.playerRepo.SetLedgerTxIDs(ctx, tx, players[*matchWinnerIdx].ID, nil, &creditTxID, &feeTxID)
		}

		if err := s.matchRepo.MarkCompletedTx(ctx, tx, match.ID, winnerUserID, status, settlement.FeeCents, now); err != nil {
			return nil, err
		}
	} else {
		if err := s.matchRepo.UpdateStateTx(ctx, tx, match); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	match.Players = players
	view := s.buildView(match, userID)

	if s.realtime != nil {
		_ = s.realtime.PublishToChannel(ctx, matchChannel(match.ID), "p2p.move_made", map[string]interface{}{
			"match_id": match.ID,
			"move":     action,
			"state":    view,
		})

		if isMatchOver {
			_ = s.realtime.PublishToChannel(ctx, matchChannel(match.ID), "p2p.match_completed", view)
		}
	}

	return view, nil
}

func (s *P2PService) ResignMatch(ctx context.Context, userID, matchID string) (*MatchView, error) {
	tx, err := s.matchRepo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	match, err := s.matchRepo.FindByIDForUpdate(ctx, tx, matchID)
	if err != nil {
		return nil, err
	}

	if match.Status != domain.StatusInProgress && match.Status != domain.StatusReady {
		return nil, fmt.Errorf("%w: cannot resign match in status %s", domain.ErrMatchNotInProgress, match.Status)
	}

	players, err := s.playerRepo.ListByMatchTx(ctx, tx, match.ID)
	if err != nil {
		return nil, err
	}

	// Only a seated player may forfeit. Without this check any authenticated
	// user could resign someone else's match and trigger its settlement.
	if findPlayer(players, userID) == nil {
		return nil, domain.ErrNotParticipant
	}

	var winnerUserID string
	for _, p := range players {
		if p.UserID != userID {
			winnerUserID = p.UserID
			break
		}
	}

	if winnerUserID == "" {
		return nil, errors.New("opponent not found")
	}

	now := time.Now()
	settlement := domain.CalculateSettlement(match.PrizePoolCents, match.ServiceFeeBP, false)
	match.TotalFeeCents = settlement.FeeCents
	match.WinnerUserID = &winnerUserID
	match.Status = domain.StatusForfeited
	match.CompletedAt = &now

	creditTxID, feeTxID, err := s.escrowRepo.SettleWinnerTx(ctx, tx, winnerUserID, match.PrizePoolCents, settlement.FeeCents, settlement.NetPayoutCents, match.ID)
	if err != nil {
		return nil, err
	}
	if err := s.escrowRepo.FinalizeLoserTx(ctx, tx, userID, match.StakeCents, match.ID); err != nil {
		return nil, err
	}

	for _, p := range players {
		if p.UserID == winnerUserID {
			_ = s.playerRepo.SetLedgerTxIDs(ctx, tx, p.ID, nil, &creditTxID, &feeTxID)
		}
	}

	if err := s.matchRepo.MarkCompletedTx(ctx, tx, match.ID, &winnerUserID, domain.StatusForfeited, settlement.FeeCents, now); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	match.Players = players
	view := s.buildView(match, userID)

	if s.realtime != nil {
		_ = s.realtime.PublishToChannel(ctx, matchChannel(match.ID), "p2p.match_completed", view)
	}

	return view, nil
}

// RequestRematch asks for a return game against the same opponent after a
// match has finished. The first request only records the intent: the opponent
// sees it on the result screen and accepts from there. When the second player
// accepts, a fresh match with both players already seated is created and both
// clients are told where to go. Returns RematchStarted with that match.
func (s *P2PService) RequestRematch(ctx context.Context, userID, matchID string) (*RematchResult, error) {
	tx, err := s.matchRepo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// The row lock serialises the two acceptances: whoever commits second sees
	// the first request (or the match it created) and never opens a duplicate.
	match, err := s.matchRepo.FindByIDForUpdate(ctx, tx, matchID)
	if err != nil {
		return nil, err
	}

	players, err := s.playerRepo.ListByMatchTx(ctx, tx, match.ID)
	if err != nil {
		return nil, err
	}
	if findPlayer(players, userID) == nil {
		return nil, domain.ErrNotParticipant
	}
	if !match.Finished() {
		return nil, fmt.Errorf("%w: cannot rematch a match in status %s", domain.ErrMatchNotFinished, match.Status)
	}
	if len(players) != 2 {
		return nil, fmt.Errorf("%w: no opponent to rematch against", domain.ErrMatchNotFinished)
	}

	// Both players already agreed earlier: send the caller into the match that
	// was created then instead of opening a second one.
	if match.RematchMatchID != nil {
		next, err := s.matchRepo.FindByIDForUpdate(ctx, tx, *match.RematchMatchID)
		if err != nil {
			return nil, err
		}
		nextPlayers, err := s.playerRepo.ListByMatchTx(ctx, tx, next.ID)
		if err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		next.Players = nextPlayers
		return &RematchResult{Status: RematchStarted, Match: s.buildView(next, userID)}, nil
	}

	// Idempotent: a retry or a double click stays on "waiting for opponent".
	if match.RematchRequestedBy != nil && *match.RematchRequestedBy == userID {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		match.Players = players
		return &RematchResult{Status: RematchRequested, Match: s.buildView(match, userID)}, nil
	}

	if match.RematchRequestedBy == nil {
		match.RematchRequestedBy = &userID
		if err := s.matchRepo.UpdateStateTx(ctx, tx, match); err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		match.Players = players

		if s.realtime != nil {
			_ = s.realtime.PublishToChannel(ctx, matchChannel(match.ID), "p2p.rematch_requested", map[string]interface{}{
				"match_id": match.ID,
				"user_id":  userID,
			})
		}
		return &RematchResult{Status: RematchRequested, Match: s.buildView(match, userID)}, nil
	}

	// Second acceptance: the return match starts right away, since both players
	// explicitly asked for it.
	next, nextPlayers, err := s.createRematchTx(ctx, tx, match, players, userID)
	if err != nil {
		return nil, err
	}

	match.RematchMatchID = &next.ID
	if err := s.matchRepo.UpdateStateTx(ctx, tx, match); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	next.Players = nextPlayers

	if s.realtime != nil {
		_ = s.realtime.PublishToChannel(ctx, matchChannel(match.ID), "p2p.rematch_created", map[string]interface{}{
			"match_id":      match.ID,
			"next_match_id": next.ID,
			"state":         s.buildView(next, ""),
		})
	}

	return &RematchResult{Status: RematchStarted, Match: s.buildView(next, userID)}, nil
}

// createRematchTx inserts the return match and seats both players in it. The
// previous second mover opens the new game, so the first move alternates.
func (s *P2PService) createRematchTx(ctx context.Context, tx *sql.Tx, prev *domain.Match, prevPlayers []*domain.MatchPlayer, creatorID string) (*domain.Match, []*domain.MatchPlayer, error) {
	engine, ok := s.ruleEngines[prev.GameSlug]
	if !ok {
		return nil, nil, domain.ErrUnsupportedGame
	}

	// A staked rematch reserves both stakes again, exactly like a first match
	// would. Paid play stays gated behind the same compliance switch.
	if prev.StakeCents > 0 && !s.paidMatchesEnabled {
		return nil, nil, domain.ErrPaidMatchesDisabled
	}

	now := time.Now()
	turnSeconds := 20
	if prev.GameSlug == "tic-tac-toe" || prev.GameSlug == "rock-paper-scissors" {
		turnSeconds = 15
	}
	deadline := now.Add(time.Duration(turnSeconds) * time.Second)
	firstUserID := prevPlayers[1].UserID

	next := &domain.Match{
		GameSlug:          prev.GameSlug,
		Status:            domain.StatusInProgress,
		MatchType:         prev.MatchType,
		StakeCents:        prev.StakeCents,
		StakeAmount:       prev.StakeAmount,
		PrizePoolCents:    prev.StakeCents * 2,
		ServiceFeeBP:      prev.ServiceFeeBP,
		CurrentTurnUserID: &firstUserID,
		TurnDeadline:      &deadline,
		CurrentRound:      1,
		MaxRounds:         prev.MaxRounds,
		Scores:            map[string]int{"0": 0, "1": 0},
		BoardState:        engine.InitBoard(),
		CreatedBy:         creatorID,
		StartedAt:         &now,
	}

	if err := s.matchRepo.CreateTx(ctx, tx, next); err != nil {
		return nil, nil, err
	}

	seated := []*domain.MatchPlayer{prevPlayers[1], prevPlayers[0]}
	players := make([]*domain.MatchPlayer, 0, len(seated))
	for i, previous := range seated {
		var debitTxID string
		if next.StakeCents > 0 {
			id, err := s.escrowRepo.ReserveFundsTx(ctx, tx, previous.UserID, next.StakeCents, next.ID)
			if err != nil {
				return nil, nil, err
			}
			debitTxID = id
		}
		var debitPtr *string
		if debitTxID != "" {
			debitPtr = &debitTxID
		}

		p := &domain.MatchPlayer{
			MatchID:     next.ID,
			UserID:      previous.UserID,
			PlayerIndex: i,
			Symbol:      symbolFor(next.GameSlug, i),
			Status:      domain.PlayerStatusActive,
			Score:       0,
			DebitTxID:   debitPtr,
		}
		if err := s.playerRepo.AddPlayerTx(ctx, tx, p); err != nil {
			return nil, nil, err
		}
		players = append(players, p)
	}

	return next, players, nil
}

func (s *P2PService) GetMatchState(ctx context.Context, userID, matchID string) (*MatchView, error) {
	match, err := s.matchRepo.FindByID(ctx, matchID)
	if err != nil {
		return nil, err
	}

	players, err := s.playerRepo.ListByMatch(ctx, match.ID)
	if err != nil {
		return nil, err
	}
	match.Players = players

	// Match state contains both players' ids and the live board; it is only
	// exposed to participants.
	if userID == "" || findPlayer(players, userID) == nil {
		return nil, domain.ErrNotParticipant
	}

	return s.buildView(match, userID), nil
}

// findPlayer returns the caller's row in the match, or nil when they are not a
// participant. Centralised so every state-mutating operation enforces the same
// membership rule.
func findPlayer(players []*domain.MatchPlayer, userID string) *domain.MatchPlayer {
	if userID == "" {
		return nil
	}
	for _, p := range players {
		if p.UserID == userID {
			return p
		}
	}
	return nil
}

func (s *P2PService) buildView(m *domain.Match, userID string) *MatchView {
	secondsLeft := 0
	if m.TurnDeadline != nil && m.Status == domain.StatusInProgress {
		d := time.Until(*m.TurnDeadline)
		if d > 0 {
			// Round up so the UI never shows 0s while the move is still legal.
			secondsLeft = int(math.Ceil(d.Seconds()))
		}
	}

	var myIdx *int
	if userID != "" && m.Players != nil {
		for _, p := range m.Players {
			if p.UserID == userID {
				idx := p.PlayerIndex
				myIdx = &idx
				break
			}
		}
	}

	boardState := m.BoardState
	if m.GameSlug == "rock-paper-scissors" && m.Status == domain.StatusInProgress && boardState != nil {
		if rawChoices, ok := boardState["choices"].(map[string]interface{}); ok && len(rawChoices) < 2 {
			clonedBoard := make(map[string]interface{})
			for k, v := range boardState {
				clonedBoard[k] = v
			}
			masked := make(map[string]interface{})
			for k, v := range rawChoices {
				if myIdx != nil && k == strconv.Itoa(*myIdx) {
					masked[k] = v
				} else {
					masked[k] = "hidden"
				}
			}
			clonedBoard["choices"] = masked
			boardState = clonedBoard
		}
	}

	return &MatchView{
		ID:                 m.ID,
		GameSlug:           m.GameSlug,
		Status:             m.Status,
		MatchType:          m.MatchType,
		InviteCode:         m.InviteCode,
		StakeAmount:        m.StakeAmount,
		PrizePool:          m.PrizePoolCents.Float(),
		ServiceFeePercent:  float64(m.ServiceFeeBP) / 100.0,
		TotalFee:           m.TotalFeeCents.Float(),
		WinnerUserID:       m.WinnerUserID,
		CurrentTurnUserID:  m.CurrentTurnUserID,
		TurnSecondsLeft:    secondsLeft,
		CurrentRound:       m.CurrentRound,
		MaxRounds:          m.MaxRounds,
		Scores:             m.Scores,
		BoardState:         boardState,
		CreatedBy:          m.CreatedBy,
		RematchRequestedBy: m.RematchRequestedBy,
		RematchMatchID:     m.RematchMatchID,
		MyPlayerIndex:      myIdx,
		Players:            m.Players,
		StartedAt:          m.StartedAt,
		CompletedAt:        m.CompletedAt,
		CreatedAt:          m.CreatedAt,
	}
}

func matchChannel(matchID string) string {
	return "p2p_match:" + matchID
}

// symbolFor is the single source of truth for the seat markers shown in the
// board UI. Index 0 is the room creator (and the opening mover).
func symbolFor(gameSlug string, playerIndex int) string {
	if playerIndex == 0 {
		switch gameSlug {
		case "connect-four":
			return "RED"
		case "rock-paper-scissors":
			return "P1"
		default:
			return "X"
		}
	}
	switch gameSlug {
	case "connect-four":
		return "YELLOW"
	case "rock-paper-scissors":
		return "P2"
	default:
		return "O"
	}
}

func generateInviteCode() string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 6)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[n.Int64()]
	}
	return string(b)
}
