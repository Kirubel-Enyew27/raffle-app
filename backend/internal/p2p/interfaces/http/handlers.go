package http

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/raffle-app/backend/internal/p2p/application"
	"github.com/raffle-app/backend/internal/p2p/domain"
)

type P2PHandler struct {
	svc *application.P2PService
}

func NewP2PHandler(svc *application.P2PService) *P2PHandler {
	return &P2PHandler{svc: svc}
}

func (h *P2PHandler) ListGames(c *gin.Context) {
	games, err := h.svc.ListGames(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "SERVER_ERROR", "message": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"code": "SERVER_ERROR", "message": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"code": "SERVER_ERROR", "message": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"code": "SERVER_ERROR", "message": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"code": "SERVER_ERROR", "message": err.Error()})
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

	view, err := h.svc.SubmitMove(c.Request.Context(), userID, matchID, action)
	if err != nil {
		handleError(c, err)
		return
	}
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
	case errors.Is(err, domain.ErrNotParticipant):
		c.JSON(http.StatusForbidden, gin.H{"code": "FORBIDDEN", "message": err.Error()})
	case errors.Is(err, domain.ErrMatchNotFound) || errors.Is(err, domain.ErrGameNotFound):
		c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": err.Error()})
	case errors.Is(err, domain.ErrInsufficientStake):
		c.JSON(http.StatusPaymentRequired, gin.H{"code": "INSUFFICIENT_FUNDS", "message": "Insufficient available wallet balance for match stake"})
	case errors.Is(err, domain.ErrInvalidStake) || errors.Is(err, domain.ErrInvalidMove) || errors.Is(err, domain.ErrUnsupportedGame):
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ACTION", "message": err.Error()})
	case errors.Is(err, domain.ErrNotYourTurn) || errors.Is(err, domain.ErrTurnTimeout) || errors.Is(err, domain.ErrStaleMove) || errors.Is(err, domain.ErrMatchNotInProgress) || errors.Is(err, domain.ErrMatchNotFinished) || errors.Is(err, domain.ErrMatchFull) || errors.Is(err, domain.ErrCannotJoinOwnMatch) || errors.Is(err, domain.ErrMatchNotJoinable):
		c.JSON(http.StatusConflict, gin.H{"code": "MATCH_CONFLICT", "message": err.Error()})
	default:
		// Never echo raw driver/query errors to clients.
		c.JSON(http.StatusInternalServerError, gin.H{"code": "SERVER_ERROR", "message": "internal server error"})
	}
}
