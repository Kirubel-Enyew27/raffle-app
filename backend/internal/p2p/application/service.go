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
	"strings"
	"time"

	auditapp "github.com/raffle-app/backend/internal/audit/application"
	"github.com/raffle-app/backend/internal/p2p/domain"
	"github.com/raffle-app/backend/internal/realtime"
	"github.com/raffle-app/backend/pkg/money"
)

// DisconnectGraceSeconds is the server-authoritative grace period for a
// disconnected player to reconnect before the match is forfeited. The value
// is deliberately centralised so it can be tuned without scattering magic
// numbers across the codebase.
const DisconnectGraceSeconds = 60

type MatchView struct {
	ID                string                 `json:"id"`
	StateVersion      int64                  `json:"state_version"`
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
	RematchRequestedBy    *string `json:"rematch_requested_by,omitempty"`
	RematchMatchID        *string `json:"rematch_match_id,omitempty"`
	DisconnectUserID      *string `json:"disconnect_user_id,omitempty"`
	DisconnectSecondsLeft int     `json:"disconnect_seconds_left"`
	// Absolute deadlines let clients tick their clocks against the server's
	// clock instead of recomputing from a countdown that goes stale the moment
	// it is rendered. ServerTime is the reference point for both.
	TurnDeadlineAt       *time.Time            `json:"turn_deadline_at,omitempty"`
	DisconnectDeadlineAt *time.Time            `json:"disconnect_deadline_at,omitempty"`
	ServerTime           time.Time             `json:"server_time"`
	MyPlayerIndex        *int                  `json:"my_player_index,omitempty"`
	Players              []*domain.MatchPlayer `json:"players"`
	StartedAt            *time.Time            `json:"started_at,omitempty"`
	CompletedAt          *time.Time            `json:"completed_at,omitempty"`
	ExpiresAt            *time.Time            `json:"expires_at,omitempty"`
	CreatedAt            time.Time             `json:"created_at"`
	UpdatedAt            time.Time             `json:"updated_at"`
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

type InviteSummary struct {
	InviteCode         string    `json:"invite_code"`
	MatchID            string    `json:"match_id"`
	GameSlug           string    `json:"game_slug"`
	GameTitle          string    `json:"game_title"`
	GameDescription    string    `json:"game_description"`
	Status             string    `json:"status"`
	IsJoinable         bool      `json:"is_joinable"`
	IsExpired          bool      `json:"is_expired"`
	ExpiresAt          time.Time `json:"expires_at"`
	SecondsLeft        int       `json:"seconds_left"`
	CreatorID          string    `json:"creator_id"`
	CreatorName        string    `json:"creator_name"`
	CreatorAvatarURL   *string   `json:"creator_avatar_url,omitempty"`
	MatchType          string    `json:"match_type"`
	MaxRounds          int       `json:"max_rounds"`
	DefaultTurnSeconds int       `json:"default_turn_seconds"`
	CreatedAt          time.Time `json:"created_at"`
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
		"ethiopian-dama":      domain.NewDamaEngine(),
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
	games, err := s.gameRepo.ListActive(ctx)
	if err != nil {
		return nil, err
	}

	// Only advertise games this build can actually play: a p2p_games row without
	// a compiled rule engine would show in the catalog and then fail on create
	// with ErrUnsupportedGame.
	playable := make([]*domain.Game, 0, len(games))
	for _, g := range games {
		if _, ok := s.ruleEngines[g.Slug]; ok {
			playable = append(playable, g)
		}
	}
	return playable, nil
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

	publishLobbyUpdate(ctx, s.realtime, "created", match)

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
		cleanCode := strings.TrimSpace(strings.ToUpper(inviteCode))
		if cleanCode == "" {
			return nil, domain.ErrMatchNotFound
		}
		m, err := s.matchRepo.FindByInviteCodeForUpdate(ctx, tx, cleanCode)
		if err != nil {
			return nil, err
		}
		match = m
	} else {
		return nil, errors.New("must provide match_id or invite_code")
	}

	existingPlayers, err := s.playerRepo.ListByMatchTx(ctx, tx, match.ID)
	if err != nil {
		return nil, err
	}

	for _, p := range existingPlayers {
		if p.UserID == userID {
			if p.PlayerIndex == 0 {
				return nil, domain.ErrCannotJoinOwnMatch
			}
			// Already joined as Player 1: return existing match view idempotently
			match.Players = existingPlayers
			return s.buildView(match, userID), nil
		}
	}

	if match.Status != domain.StatusWaiting {
		return nil, domain.ErrMatchNotJoinable
	}

	if match.CreatedAt.Before(time.Now().Add(-idleRoomTTL(match.MatchType))) {
		return nil, domain.ErrInvitationExpired
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

	s.publishMatchState(ctx, "p2p.player_joined", match, match.Players, map[string]interface{}{
		"player": p2,
	})
	// The room is no longer joinable, so it must leave every open lobby list.
	publishLobbyUpdate(ctx, s.realtime, "joined", match)

	return s.buildView(match, userID), nil
}

func (s *P2PService) CancelMatch(ctx context.Context, userID, matchID string) (*MatchView, error) {
	tx, err := s.matchRepo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	match, err := s.matchRepo.FindByIDForUpdate(ctx, tx, matchID)
	if err != nil {
		return nil, err
	}

	if match.CreatedBy != userID {
		return nil, domain.ErrNotParticipant
	}

	if match.Status != domain.StatusWaiting {
		return nil, domain.ErrMatchNotJoinable
	}

	now := time.Now()
	if err := s.matchRepo.MarkCancelledTx(ctx, tx, match, now); err != nil {
		return nil, err
	}

	if match.StakeCents > 0 {
		if err := s.escrowRepo.ReleaseReservedFundsTx(ctx, tx, match.CreatedBy, match.StakeCents, match.ID, "room cancelled by host"); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	match.Status = domain.StatusCancelled
	match.CompletedAt = &now

	players, _ := s.playerRepo.ListByMatch(ctx, match.ID)
	match.Players = players

	if s.realtime != nil {
		s.publishMatchState(ctx, "p2p.match_cancelled", match, players, map[string]interface{}{
			"reason": "cancelled by host",
		})
	}
	publishLobbyUpdate(ctx, s.realtime, "cancelled", match)

	return s.buildView(match, userID), nil
}

func (s *P2PService) GetInvite(ctx context.Context, code string) (*InviteSummary, error) {
	cleanCode := strings.TrimSpace(strings.ToUpper(code))
	if cleanCode == "" {
		return nil, domain.ErrMatchNotFound
	}

	match, err := s.matchRepo.FindByInviteCode(ctx, cleanCode)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrMatchNotFound
		}
		return nil, err
	}

	game, err := s.gameRepo.FindBySlug(ctx, match.GameSlug)
	if err != nil {
		return nil, err
	}

	players, err := s.playerRepo.ListByMatch(ctx, match.ID)
	if err != nil {
		return nil, err
	}

	ttl := idleRoomTTL(match.MatchType)
	expiresAt := match.CreatedAt.Add(ttl)
	now := time.Now()
	isExpired := false
	secondsLeft := 0

	status := match.Status
	if match.Status == domain.StatusWaiting {
		if now.After(expiresAt) {
			isExpired = true
			status = "expired"
		} else {
			secondsLeft = int(time.Until(expiresAt).Seconds())
		}
	}

	isJoinable := (match.Status == domain.StatusWaiting && !isExpired && len(players) < 2)

	var creatorName = "Challenger"
	var creatorAvatar *string
	if len(players) > 0 {
		if players[0].DisplayName != "" {
			creatorName = players[0].DisplayName
		}
		if players[0].AvatarURL != "" {
			avatar := players[0].AvatarURL
			creatorAvatar = &avatar
		}
	}

	summary := &InviteSummary{
		InviteCode:         cleanCode,
		MatchID:            match.ID,
		GameSlug:           match.GameSlug,
		GameTitle:          game.Title,
		GameDescription:    game.Description,
		Status:             status,
		IsJoinable:         isJoinable,
		IsExpired:          isExpired,
		ExpiresAt:          expiresAt,
		SecondsLeft:        secondsLeft,
		CreatorID:          match.CreatedBy,
		CreatorName:        creatorName,
		CreatorAvatarURL:   creatorAvatar,
		MatchType:          match.MatchType,
		MaxRounds:          match.MaxRounds,
		DefaultTurnSeconds: game.DefaultTurnSeconds,
		CreatedAt:          match.CreatedAt,
	}

	return summary, nil
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
	readyChanged := membership.Status != domain.PlayerStatusReady
	if readyChanged {
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

		turnSeconds := turnSecondsFor(match.GameSlug)

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
	} else if readyChanged {
		// Readiness is part of the state snapshot before the match starts, so
		// advance the same sequence that is used to detect missed SSE updates.
		if err := s.matchRepo.UpdateStateTx(ctx, tx, match); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	match.Players = players
	view := s.buildView(match, userID)

	s.publishMatchState(ctx, "p2p.ready_status", match, players, map[string]interface{}{
		"user_id": userID,
	})

	if allReady {
		s.publishMatchState(ctx, "p2p.match_started", match, players, nil)
	}

	return view, nil
}

func (s *P2PService) SubmitMove(ctx context.Context, userID, matchID string, action map[string]interface{}) (*MatchView, error) {
	applicationStartedAt := time.Now()
	tx, err := s.matchRepo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	txStartedAt := time.Now()
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
	validationStartedAt := time.Now()
	newBoard, isRoundOver, roundWinnerIdx, _, err := engine.ValidateAndApplyMove(match.BoardState, activePlayer.PlayerIndex, action)
	if err != nil {
		return nil, err
	}
	validationDuration := time.Since(validationStartedAt)

	match.BoardState = newBoard

	// Record the move log. move_number must increase across a whole round, even
	// after the board has round-tripped through JSONB (where numbers arrive as
	// float64). The previous int-only assertion silently recorded every move as
	// number 0 and the unique constraint discarded all but the first.
	moveCount := boardMoveCount(newBoard)
	if moveCount <= 0 {
		moveCount = 1
	}
	if err := s.moveRepo.RecordMoveTx(ctx, tx, &domain.MatchMove{
		MatchID:       match.ID,
		RoundNumber:   match.CurrentRound,
		MoveNumber:    moveCount,
		UserID:        userID,
		Action:        action,
		BoardSnapshot: newBoard,
	}); err != nil {
		return nil, fmt.Errorf("failed to record move: %w", err)
	}

	var isMatchOver bool
	var matchWinnerIdx *int

	// A multi-capture chain keeps the turn with the same player. Ask the engine
	// before switching seats; engines without this capability always switch.
	turnComplete := true
	if continuation, ok := engine.(domain.TurnContinuation); ok {
		turnComplete = continuation.TurnComplete(newBoard)
	}

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
			nextBoard := engine.InitBoard()
			// Carry the finished round's reveal into the fresh board. Without
			// this, simultaneous-move games (RPS) would wipe the players'
			// choices before either client could display them.
			if result, ok := newBoard["last_round_result"]; ok && result != nil {
				nextBoard["last_round_result"] = result
			}

			// A round can end on a winning line (Tic-Tac-Toe). Hold that
			// finished board for a short reveal, with the clock stopped, so both
			// players actually see the winning combination; the background
			// sweeper starts the next round when the reveal deadline passes.
			// Resetting here instead replaced the winning line with an empty
			// board in the very same event that announced it.
			if newBoard["winning_line"] != nil && roundWinnerIdx != nil {
				reveal := make(map[string]interface{}, len(newBoard)+1)
				for k, v := range newBoard {
					reveal[k] = v
				}
				reveal[roundTransitionKey] = map[string]interface{}{
					"next_board":           nextBoard,
					"next_round":           match.CurrentRound + 1,
					"next_starter_user_id": opponentPlayer.UserID,
					"round_winner_index":   *roundWinnerIdx,
				}
				match.BoardState = reveal
				// Nobody may move during the reveal. A nil turn also keeps the
				// turn-timeout path from forfeiting anyone for it.
				match.CurrentTurnUserID = nil
				deadline := now.Add(roundRevealDelay)
				match.TurnDeadline = &deadline
			} else {
				match.CurrentRound++
				match.BoardState = nextBoard
				// Switch starter for next round
				nextTurn := opponentPlayer.UserID
				match.CurrentTurnUserID = &nextTurn
				deadline := now.Add(time.Duration(turnSecondsFor(match.GameSlug)) * time.Second)
				match.TurnDeadline = &deadline
			}
		}
	} else if !turnComplete {
		// Same player must continue capturing. Refresh the clock so a long chain
		// is not punished by the time spent on earlier hops.
		deadline := now.Add(time.Duration(turnSecondsFor(match.GameSlug)) * time.Second)
		match.TurnDeadline = &deadline
	} else {
		// Switch turn
		nextTurn := opponentPlayer.UserID
		match.CurrentTurnUserID = &nextTurn
		deadline := now.Add(time.Duration(turnSecondsFor(match.GameSlug)) * time.Second)
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
		match.CurrentTurnUserID = nil
		match.TurnDeadline = nil
		match.DisconnectUserID = nil
		match.DisconnectDeadline = nil

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

		// Persist the final board (and the series scores that go with it) before
		// closing the match. MarkCompletedTx only writes terminal columns, so
		// without this the completed match is read back as it was *before* the
		// winning move: the final piece and the whole winning_line disappear the
		// moment either client refetches the terminal state, leaving no winning
		// combination to highlight on the board.
		if err := s.matchRepo.UpdateStateTx(ctx, tx, match); err != nil {
			return nil, err
		}

		// UpdateStateTx already advanced this match's version with the final
		// board; terminal bookkeeping belongs to that same committed snapshot.
		if err := s.matchRepo.MarkCompletedTx(ctx, tx, match, winnerUserID, status, settlement.FeeCents, now, false); err != nil {
			return nil, err
		}
	} else {
		if err := s.matchRepo.UpdateStateTx(ctx, tx, match); err != nil {
			return nil, err
		}
	}

	commitStartedAt := time.Now()
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committedAt := time.Now()

	match.Players = players
	view := s.buildView(match, userID)

	s.publishMatchState(ctx, "p2p.move_made", match, players, map[string]interface{}{
		"move": action,
		// Durations are server-local measurements, independent of client clock
		// skew. The browser E2E test measures through opponent DOM application.
		"server_timing_ms": map[string]float64{
			"transaction_work":      float64(commitStartedAt.Sub(txStartedAt).Microseconds()) / 1000,
			"move_validation":       float64(validationDuration.Microseconds()) / 1000,
			"database_commit":       float64(committedAt.Sub(commitStartedAt).Microseconds()) / 1000,
			"application_to_commit": float64(committedAt.Sub(applicationStartedAt).Microseconds()) / 1000,
		},
	})

	if isMatchOver {
		s.publishMatchState(ctx, "p2p.match_completed", match, players, nil)
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

	if err := s.matchRepo.MarkCompletedTx(ctx, tx, match, &winnerUserID, domain.StatusForfeited, settlement.FeeCents, now, true); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	match.Players = players
	view := s.buildView(match, userID)

	s.publishMatchState(ctx, "p2p.match_completed", match, players, nil)

	return view, nil
}

// HandlePlayerDisconnect records a temporary disconnection for userID in matchID
// and sets a server-authoritative reconnection deadline. Calling it a second time
// for the same player while a deadline is already active is idempotent (the
// existing deadline is preserved so the opponent cannot shorten the grace period
// with duplicate events). Returns ErrMatchAlreadyEnded if the match is no longer
// in progress.
func (s *P2PService) HandlePlayerDisconnect(ctx context.Context, userID, matchID string) (*MatchView, error) {
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
		return nil, domain.ErrMatchAlreadyEnded
	}

	players, err := s.playerRepo.ListByMatchTx(ctx, tx, match.ID)
	if err != nil {
		return nil, err
	}

	if findPlayer(players, userID) == nil {
		return nil, domain.ErrNotParticipant
	}

	// Idempotent: if any player already has an active deadline running, keep it.
	// This covers two cases:
	//   1. The same player calls disconnect again (true idempotency).
	//   2. A second player calls disconnect while the first deadline is still
	//      active — the first deadline governs; overwriting it would silently
	//      extend the grace period for a different player.
	if match.DisconnectUserID != nil && match.DisconnectDeadline != nil {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		match.Players = players
		return s.buildView(match, userID), nil
	}
	now := time.Now()
	deadline := now.Add(time.Duration(DisconnectGraceSeconds) * time.Second)
	match.DisconnectUserID = &userID
	match.DisconnectDeadline = &deadline

	if err := s.matchRepo.UpdateStateTx(ctx, tx, match); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	match.Players = players
	view := s.buildView(match, userID)

	if s.realtime != nil {
		s.publishMatchState(ctx, "p2p.player_disconnected", match, players, map[string]interface{}{
			"disconnect_user_id":      userID,
			"disconnect_seconds_left": view.DisconnectSecondsLeft,
		})
	}

	return view, nil
}

// HandlePlayerReconnect clears the disconnection record for userID and resumes
// the match normally. If the deadline has already expired the match will have
// been forfeited by the background sweep and this call returns ErrMatchAlreadyEnded.
// A reconnect from a user who was not the disconnected player is silently ignored
// (idempotent), so stale reconnect events cannot affect live matches.
func (s *P2PService) HandlePlayerReconnect(ctx context.Context, userID, matchID string) (*MatchView, error) {
	tx, err := s.matchRepo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	match, err := s.matchRepo.FindByIDForUpdate(ctx, tx, matchID)
	if err != nil {
		return nil, err
	}

	// If the match is already over (grace period expired + sweep ran, or any
	// other terminal state), refuse the reconnect so the UI shows the real result.
	if match.Status != domain.StatusInProgress {
		return nil, domain.ErrMatchAlreadyEnded
	}

	players, err := s.playerRepo.ListByMatchTx(ctx, tx, match.ID)
	if err != nil {
		return nil, err
	}

	if findPlayer(players, userID) == nil {
		return nil, domain.ErrNotParticipant
	}

	// Not the disconnected player: no state change needed, just return current state.
	if match.DisconnectUserID == nil || *match.DisconnectUserID != userID {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		match.Players = players
		return s.buildView(match, userID), nil
	}

	// Clear the disconnect fields to resume normal play.
	match.DisconnectUserID = nil
	match.DisconnectDeadline = nil

	// Give the returning player a full turn clock: otherwise they would resume
	// with whatever fragment was left when their transport dropped and lose on
	// a timeout they never had a chance to beat.
	if match.CurrentTurnUserID != nil && *match.CurrentTurnUserID == userID {
		deadline := time.Now().Add(time.Duration(turnSecondsFor(match.GameSlug)) * time.Second)
		match.TurnDeadline = &deadline
	}

	if err := s.matchRepo.UpdateStateTx(ctx, tx, match); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	match.Players = players
	view := s.buildView(match, userID)

	if s.realtime != nil {
		s.publishMatchState(ctx, "p2p.player_reconnected", match, players, map[string]interface{}{
			"user_id": userID,
		})
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
			s.publishMatchState(ctx, "p2p.rematch_requested", match, players, map[string]interface{}{
				"user_id": userID,
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
		s.publishMatchState(ctx, "p2p.rematch_created", match, players, map[string]interface{}{
			"next_match_id": next.ID,
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
	deadline := now.Add(time.Duration(turnSecondsFor(prev.GameSlug)) * time.Second)
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
	// The staged next-round board is server bookkeeping, not something a client
	// renders or should be able to see before the round starts.
	if boardState != nil {
		if _, pending := boardState[roundTransitionKey]; pending {
			stripped := make(map[string]interface{}, len(boardState))
			for k, v := range boardState {
				if k == roundTransitionKey {
					continue
				}
				stripped[k] = v
			}
			boardState = stripped
		}
	}
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

	var expiresAt *time.Time
	if m.Status == domain.StatusWaiting {
		exp := m.CreatedAt.Add(idleRoomTTL(m.MatchType))
		expiresAt = &exp
	}

	disconnectSecondsLeft := 0
	if m.DisconnectDeadline != nil && m.Status == domain.StatusInProgress {
		d := time.Until(*m.DisconnectDeadline)
		if d > 0 {
			disconnectSecondsLeft = int(math.Ceil(d.Seconds()))
		}
	}

	var turnDeadlineAt *time.Time
	if m.TurnDeadline != nil && m.Status == domain.StatusInProgress {
		deadline := *m.TurnDeadline
		turnDeadlineAt = &deadline
	}

	var disconnectDeadlineAt *time.Time
	if m.DisconnectDeadline != nil && m.Status == domain.StatusInProgress {
		deadline := *m.DisconnectDeadline
		disconnectDeadlineAt = &deadline
	}

	return &MatchView{
		ID:                    m.ID,
		StateVersion:          m.StateVersion,
		GameSlug:              m.GameSlug,
		Status:                m.Status,
		MatchType:             m.MatchType,
		InviteCode:            m.InviteCode,
		StakeAmount:           m.StakeAmount,
		PrizePool:             m.PrizePoolCents.Float(),
		ServiceFeePercent:     float64(m.ServiceFeeBP) / 100.0,
		TotalFee:              m.TotalFeeCents.Float(),
		WinnerUserID:          m.WinnerUserID,
		CurrentTurnUserID:     m.CurrentTurnUserID,
		TurnSecondsLeft:       secondsLeft,
		CurrentRound:          m.CurrentRound,
		MaxRounds:             m.MaxRounds,
		Scores:                m.Scores,
		BoardState:            boardState,
		CreatedBy:             m.CreatedBy,
		RematchRequestedBy:    m.RematchRequestedBy,
		RematchMatchID:        m.RematchMatchID,
		DisconnectUserID:      m.DisconnectUserID,
		DisconnectSecondsLeft: disconnectSecondsLeft,
		TurnDeadlineAt:        turnDeadlineAt,
		DisconnectDeadlineAt:  disconnectDeadlineAt,
		ServerTime:            time.Now(),
		MyPlayerIndex:         myIdx,
		Players:               m.Players,
		StartedAt:             m.StartedAt,
		CompletedAt:           m.CompletedAt,
		ExpiresAt:             expiresAt,
		CreatedAt:             m.CreatedAt,
		UpdatedAt:             m.UpdatedAt,
	}
}

func matchChannel(matchID string) string {
	return "p2p_match:" + matchID
}

// publishMatchState delivers a match event to every seated player with a view
// built for that player.
//
// A single channel broadcast cannot carry per-viewer state: it would ship one
// player's my_player_index to both clients and, in Rock-Paper-Scissors, reveal
// the mover's committed choice before both players have picked. Publishing one
// user-targeted event per player keeps each client's payload authoritative for
// that client while still arriving on its match-channel subscription.
func (s *P2PService) publishMatchState(
	ctx context.Context,
	eventType string,
	match *domain.Match,
	players []*domain.MatchPlayer,
	extra map[string]interface{},
) {
	if s.realtime == nil || match == nil {
		return
	}

	match.Players = players

	seen := make(map[string]bool, len(players))
	for _, p := range players {
		if p == nil || p.UserID == "" || seen[p.UserID] {
			continue
		}
		seen[p.UserID] = true

		payload := make(map[string]interface{}, len(extra)+2)
		for key, value := range extra {
			payload[key] = value
		}
		payload["match_id"] = match.ID
		payload["state"] = s.buildView(match, p.UserID)

		_ = s.realtime.Publish(ctx, eventType, p.UserID, "", payload)
	}
}

// lobbyChannel is the shared channel for public matchmaking changes. Lobby
// viewers subscribe to it so an open room appears or disappears immediately
// instead of waiting for the next poll.
const lobbyChannel = "p2p_lobby"

// publishLobbyUpdate announces that the set of joinable rooms changed. Viewers
// refetch the authoritative lobby list on receipt; the event only carries
// enough context to log and filter, never the list itself.
func publishLobbyUpdate(ctx context.Context, rt *realtime.Service, change string, m *domain.Match) {
	// The lobby only lists public rooms, so private-room churn is not news.
	if rt == nil || m == nil || m.MatchType != domain.TypePublicQueue {
		return
	}
	_ = rt.PublishToChannel(ctx, lobbyChannel, "p2p.lobby_updated", map[string]interface{}{
		"change":     change,
		"match_id":   m.ID,
		"game_slug":  m.GameSlug,
		"match_type": m.MatchType,
		"status":     m.Status,
		"stake":      m.StakeAmount,
		"created_by": m.CreatedBy,
	})
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
		case "ethiopian-dama":
			return "LIGHT"
		default:
			return "X"
		}
	}
	switch gameSlug {
	case "connect-four":
		return "YELLOW"
	case "rock-paper-scissors":
		return "P2"
	case "ethiopian-dama":
		return "DARK"
	default:
		return "O"
	}
}

// roundTransitionKey is the board_state entry that stages a between-round
// reveal: the finished board stays visible until the sweeper promotes
// `next_board` and hands the turn to `next_starter_user_id`. It is internal
// state and is stripped from every client view (see buildView).
const roundTransitionKey = "round_transition"

// roundRevealDelay is how long the winning line stays on screen between rounds.
// The next round's turn clock starts only after it, so the reveal costs no
// playing time. A variable so tests can shorten it.
var roundRevealDelay = 2 * time.Second

// turnSecondsFor is the server-enforced move clock for each game. Dama gets a
// slightly longer clock because capture chains require several hops.
func turnSecondsFor(gameSlug string) int {
	switch gameSlug {
	case "tic-tac-toe", "rock-paper-scissors":
		return 15
	case "ethiopian-dama":
		return 30
	default:
		return 20
	}
}

// boardMoveCount reads move_count from a board snapshot. Boards restored from
// JSONB store numbers as float64, while freshly produced boards use int.
func boardMoveCount(boardState map[string]interface{}) int {
	if boardState == nil {
		return 0
	}
	value, ok := boardState["move_count"]
	if !ok {
		return 0
	}
	switch count := value.(type) {
	case int:
		return count
	case int64:
		return int(count)
	case float64:
		return int(count)
	default:
		return 0
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
