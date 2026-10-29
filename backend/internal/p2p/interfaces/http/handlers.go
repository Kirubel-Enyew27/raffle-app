package http

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/raffle-app/backend/internal/p2p/application"
	"github.com/raffle-app/backend/internal/p2p/domain"
	"go.uber.org/zap"
)

type P2PHandler struct {
	svc *application.P2PService
	log *zap.Logger
}

// NewP2PHandler creates a handler with a no-op logger.
// Prefer NewP2PHandlerWithLogger in production.
func NewP2PHandler(svc *application.P2PService) *P2PHandler {
	return &P2PHandler{svc: svc, log: zap.NewNop()}
}

// NewP2PHandlerWithLogger creates a handler with structured logging.
func NewP2PHandlerWithLogger(svc *application.P2PService, log *zap.Logger) *P2PHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &P2PHandler{svc: svc, log: log}
}

func (h *P2PHandler) ListGames(c *gin.Context) {
	games, err := h.svc.ListGames(c.Request.Context())
	if err != nil {
		h.log.Error("ListGames failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"code": "SERVER_ERROR", "message": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "success", "data": games})
}

func (h *P2PHandler) GetGame(c *gin.Context) {
	slug := c.Param("slug")
	game, err := h.svc.GetGame(c.Request.Context(), slug)
	if err != nil {
		if err == domain.ErrGameNotFound {
			c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "Game not found"})
			return
		}
		h.log.Error("GetGame failed", zap.String("slug", slug), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"code": "SERVER_ERROR", "message": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "success", "data": game})
}

func (h *P2PHandler) GetLeaderboard(c *gin.Context) {
	periodStr := c.DefaultQuery("period", "all_time")
	var period domain.LeaderboardPeriod
	switch domain.LeaderboardPeriod(periodStr) {
	case domain.PeriodAllTime, domain.PeriodDaily, domain.PeriodWeekly, domain.PeriodMonthly:
		period = domain.LeaderboardPeriod(periodStr)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "message": "invalid period: must be all_time, daily, weekly, or monthly"})
		return
	}

	sortByStr := c.DefaultQuery("sort_by", "wins")
	var sortBy domain.LeaderboardSortBy
	switch domain.LeaderboardSortBy(sortByStr) {
	case domain.SortByWins, domain.SortByWinStreak, domain.SortByWinRate:
		sortBy = domain.LeaderboardSortBy(sortByStr)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "message": "invalid sort_by: must be wins, win_streak, or win_rate"})
		return
	}

	page := 1
	if pageStr := c.Query("page"); pageStr != "" {
		p, err := strconv.Atoi(pageStr)
		if err != nil || p < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "message": "page must be an integer >= 1"})
			return
		}
		page = p
	}

	limit := 20
	if limitStr := c.Query("limit"); limitStr != "" {
		l, err := strconv.Atoi(limitStr)
		if err != nil || l < 1 || l > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "message": "limit must be between 1 and 100"})
			return
		}
		limit = l
	}

	var gameSlug *string
	if g := c.Query("game"); g != "" {
		gameSlug = &g
	}

	var userID *string
	if u := c.GetString("user_id"); u != "" {
		userID = &u
	}

	filter := domain.LeaderboardFilter{
		Period: period,
		Game:   gameSlug,
		SortBy: sortBy,
		Page:   page,
		Limit:  limit,
		UserID: userID,
	}

	result, err := h.svc.GetLeaderboard(c.Request.Context(), filter)
	if err != nil {
		if errors.Is(err, domain.ErrGameNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "Game not found"})
			return
		}
		h.log.Error("GetLeaderboard failed", zap.Any("filter", filter), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"code": "SERVER_ERROR", "message": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    "success",
		"success": true,
		"data":    result,
	})
}

func (h *P2PHandler) ListLobby(c *gin.Context) {
	gameSlug := c.Query("game_slug")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	matches, err := h.svc.ListWaitingMatches(c.Request.Context(), gameSlug, limit)
	if err != nil {
		h.log.Error("ListLobby failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"code": "SERVER_ERROR", "message": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "success", "data": matches})
}

func (h *P2PHandler) ListUserMatches(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "unauthorized"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	matches, total, err := h.svc.ListUserMatches(c.Request.Context(), userID, limit, offset)
	if err != nil {
		h.log.Error("ListUserMatches failed", zap.String("user_id", userID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"code": "SERVER_ERROR", "message": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "success", "data": gin.H{"matches": matches, "total": total}})
}

func (h *P2PHandler) CreateMatch(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "unauthorized"})
		return
	}

	var req application.CreateMatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "message": err.Error()})
		return
	}

	view, err := h.svc.CreateMatch(c.Request.Context(), userID, req)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"code": "success", "data": view})
}

func (h *P2PHandler) JoinMatch(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "unauthorized"})
		return
	}

	matchID := c.Param("id")
	if !isUUID(matchID) {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "message": "invalid match id"})
		return
	}
	view, err := h.svc.JoinMatch(c.Request.Context(), userID, matchID, "")
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "success", "data": view})
}

func (h *P2PHandler) JoinMatchByCode(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "unauthorized"})
		return
	}

	var req struct {
		InviteCode string `json:"invite_code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "message": err.Error()})
		return
	}

	view, err := h.svc.JoinMatch(c.Request.Context(), userID, "", req.InviteCode)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "success", "data": view})
}

func (h *P2PHandler) CancelMatch(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "unauthorized"})
		return
	}

	matchID := c.Param("id")
	if !isUUID(matchID) {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "message": "invalid match id"})
		return
	}
	view, err := h.svc.CancelMatch(c.Request.Context(), userID, matchID)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "success", "data": view})
}

func (h *P2PHandler) GetInvite(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "message": "invite code is required"})
		return
	}

	summary, err := h.svc.GetInvite(c.Request.Context(), code)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "success", "data": summary})
}

func (h *P2PHandler) ReadyUp(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "unauthorized"})
		return
	}

	matchID := c.Param("id")
	if !isUUID(matchID) {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "message": "invalid match id"})
		return
	}
	view, err := h.svc.ReadyUp(c.Request.Context(), userID, matchID)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "success", "data": view})
}

func (h *P2PHandler) SubmitMove(c *gin.Context) {
	start := time.Now()
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "unauthorized"})
		return
	}

	matchID := c.Param("id")
	if !isUUID(matchID) {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "message": "invalid match id"})
		return
	}
	var action map[string]interface{}
	if err := c.ShouldBindJSON(&action); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "message": err.Error()})
		return
	}

	// Collect action keys only — never log values (may contain game secrets).
	actionKeys := make([]string, 0, len(action))
	for k := range action {
		actionKeys = append(actionKeys, k)
	}

	view, err := h.svc.SubmitMove(c.Request.Context(), userID, matchID, action)
	latency := time.Since(start)

	if err != nil {
		h.log.Info("submit_move",
			zap.String("user_id", userID),
			zap.String("match_id", matchID),
			zap.Strings("action_keys", actionKeys),
			zap.String("result", "rejected"),
			zap.Duration("latency", latency),
			zap.Error(err),
		)
		handleError(c, err)
		return
	}

	h.log.Info("submit_move",
		zap.String("user_id", userID),
		zap.String("match_id", matchID),
		zap.Strings("action_keys", actionKeys),
		zap.String("result", "accepted"),
		zap.Duration("latency", latency),
	)
	c.JSON(http.StatusOK, gin.H{"code": "success", "data": view})
}

func (h *P2PHandler) ResignMatch(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "unauthorized"})
		return
	}

	matchID := c.Param("id")
	if !isUUID(matchID) {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "message": "invalid match id"})
		return
	}
	view, err := h.svc.ResignMatch(c.Request.Context(), userID, matchID)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "success", "data": view})
}

// PlayerDisconnect notifies the server that the caller has lost connection.
// The server starts a grace-period countdown; if the player does not reconnect
// within the configured deadline, the opponent is awarded the match.
func (h *P2PHandler) PlayerDisconnect(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "unauthorized"})
		return
	}

	matchID := c.Param("id")
	if !isUUID(matchID) {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "message": "invalid match id"})
		return
	}

	view, err := h.svc.HandlePlayerDisconnect(c.Request.Context(), userID, matchID)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "success", "data": view})
}

// PlayerReconnect notifies the server that the caller has restored their
// connection and wishes to resume the match. If the grace period has already
// expired the match will be in a terminal state and ErrMatchAlreadyEnded is
// returned so the client can show the final result.
func (h *P2PHandler) PlayerReconnect(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "unauthorized"})
		return
	}

	matchID := c.Param("id")
	if !isUUID(matchID) {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "message": "invalid match id"})
		return
	}

	view, err := h.svc.HandlePlayerReconnect(c.Request.Context(), userID, matchID)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "success", "data": view})
}

func (h *P2PHandler) Rematch(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "unauthorized"})
		return
	}

	matchID := c.Param("id")
	if !isUUID(matchID) {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "message": "invalid match id"})
		return
	}

	result, err := h.svc.RequestRematch(c.Request.Context(), userID, matchID)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "success", "data": result})
}

func (h *P2PHandler) GetMatchState(c *gin.Context) {
	userID := c.GetString("user_id")
	matchID := c.Param("id")
	if !isUUID(matchID) {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "message": "invalid match id"})
		return
	}
	view, err := h.svc.GetMatchState(c.Request.Context(), userID, matchID)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "success", "data": view})
}

// isUUID validates the canonical 8-4-4-4-12 hex form. Route parameters are
// passed to UUID columns, so rejecting malformed ids up front avoids a database
// cast error being surfaced as a 500.
func isUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i, c := range value {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
	}
	return true
}

func handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrPaidMatchesDisabled):
		c.JSON(http.StatusForbidden, gin.H{"code": "PAID_MATCHES_DISABLED", "message": err.Error()})
	case errors.Is(err, domain.ErrInvitationExpired):
		c.JSON(http.StatusGone, gin.H{"code": "INVITATION_EXPIRED", "message": "This invitation has expired"})
	case errors.Is(err, domain.ErrNotParticipant):
		c.JSON(http.StatusForbidden, gin.H{"code": "FORBIDDEN", "message": err.Error()})
	case errors.Is(err, domain.ErrMatchAlreadyEnded):
		c.JSON(http.StatusGone, gin.H{"code": "MATCH_ALREADY_ENDED", "message": err.Error()})
	case errors.Is(err, domain.ErrMatchNotFound) || errors.Is(err, domain.ErrGameNotFound):
		c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": err.Error()})
	case errors.Is(err, domain.ErrInsufficientStake):
		c.JSON(http.StatusPaymentRequired, gin.H{"code": "INSUFFICIENT_FUNDS", "message": "Insufficient available wallet balance for match stake"})
	case errors.Is(err, domain.ErrInvalidStake) || errors.Is(err, domain.ErrInvalidMove) || errors.Is(err, domain.ErrUnsupportedGame):
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ACTION", "message": err.Error()})
	case errors.Is(err, domain.ErrCannotJoinOwnMatch):
		c.JSON(http.StatusConflict, gin.H{"code": "CANNOT_JOIN_OWN_MATCH", "message": "cannot join your own match"})
	case errors.Is(err, domain.ErrMatchFull):
		c.JSON(http.StatusConflict, gin.H{"code": "MATCH_FULL", "message": "match is already full"})
	case errors.Is(err, domain.ErrMatchNotJoinable):
		c.JSON(http.StatusConflict, gin.H{"code": "MATCH_NOT_JOINABLE", "message": "match is not joinable"})
	case errors.Is(err, domain.ErrNotYourTurn) || errors.Is(err, domain.ErrTurnTimeout) || errors.Is(err, domain.ErrStaleMove) || errors.Is(err, domain.ErrMatchNotInProgress) || errors.Is(err, domain.ErrMatchNotFinished):
		c.JSON(http.StatusConflict, gin.H{"code": "MATCH_CONFLICT", "message": err.Error()})
	default:
		// Never echo raw driver/query errors to clients.
		c.JSON(http.StatusInternalServerError, gin.H{"code": "SERVER_ERROR", "message": "internal server error"})
	}
}
