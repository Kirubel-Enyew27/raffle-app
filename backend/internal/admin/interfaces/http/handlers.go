package http

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/raffle-app/backend/internal/admin/application"
	"github.com/raffle-app/backend/internal/admin/domain"
	"github.com/raffle-app/backend/pkg/logger"
	"go.uber.org/zap"
)

type AdminHandler struct {
	svc *application.AdminService
}

func NewAdminHandler(svc *application.AdminService) *AdminHandler {
	return &AdminHandler{svc: svc}
}

func respond(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{"code": "success", "data": data})
}

func respondError(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"code": "error", "message": message})
}

func logAdminError(handler string, err error) {
	logger.Error("admin request failed", zap.String("handler", handler), zap.Error(err))
}

func intQuery(c *gin.Context, key string, fallback int) int {
	value, err := strconv.Atoi(c.Query(key))
	if err != nil || value < 0 {
		return fallback
	}
	return value
}

func parseDate(value string) (*time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.DateOnly, value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func parseDateRange(c *gin.Context) (from, to *time.Time, ok bool) {
	var err error
	from, err = parseDate(c.Query("from"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "from must use YYYY-MM-DD")
		return nil, nil, false
	}
	to, err = parseDate(c.Query("to"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "to must use YYYY-MM-DD")
		return nil, nil, false
	}
	if from != nil && to != nil && from.After(*to) {
		respondError(c, http.StatusBadRequest, "from must be before to")
		return nil, nil, false
	}
	return from, to, true
}

func (h *AdminHandler) Overview(c *gin.Context) {
	from, to, ok := parseDateRange(c)
	if !ok {
		return
	}
	var fromValue, toValue time.Time
	if from != nil {
		fromValue = *from
	}
	if to != nil {
		toValue = *to
	}
	overview, err := h.svc.Overview(c.Request.Context(), fromValue, toValue, c.Query("period"))
	if err != nil {
		logAdminError("overview", err)
		respondError(c, http.StatusInternalServerError, "could not load the admin overview")
		return
	}
	respond(c, overview)
}

func (h *AdminHandler) Games(c *gin.Context) {
	from, to, ok := parseDateRange(c)
	if !ok {
		return
	}
	var fromValue, toValue time.Time
	if from != nil {
		fromValue = *from
	}
	if to != nil {
		toValue = *to
	}
	games, err := h.svc.Games(c.Request.Context(), fromValue, toValue, c.Query("period"))
	if err != nil {
		logAdminError("games", err)
		respondError(c, http.StatusInternalServerError, "could not load game analytics")
		return
	}
	respond(c, games)
}

func (h *AdminHandler) Users(c *gin.Context) {
	page, err := h.svc.Users(c.Request.Context(), domain.UserFilter{
		Search: c.Query("search"),
		Status: c.Query("status"),
		Sort:   c.Query("sort"),
		Limit:  intQuery(c, "limit", 25),
		Offset: intQuery(c, "offset", 0),
	})
	if err != nil {
		logAdminError("users", err)
		respondError(c, http.StatusInternalServerError, "could not load users")
		return
	}
	respond(c, page)
}

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

func (h *AdminHandler) UserDetail(c *gin.Context) {
	id := c.Param("id")
	if !isUUID(id) {
		respondError(c, http.StatusBadRequest, "invalid user id")
		return
	}
	detail, err := h.svc.UserDetail(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(c, http.StatusNotFound, "user not found")
			return
		}
		logAdminError("user detail", err)
		respondError(c, http.StatusInternalServerError, "could not load the user")
		return
	}
	respond(c, detail)
}

func (h *AdminHandler) Matches(c *gin.Context) {
	from, to, ok := parseDateRange(c)
	if !ok {
		return
	}
	page, err := h.svc.Matches(c.Request.Context(), domain.MatchFilter{
		Search: c.Query("search"),
		Game:   c.Query("game"),
		Status: c.Query("status"),
		Mode:   c.Query("mode"),
		Sort:   c.Query("sort"),
		From:   from,
		To:     to,
		Limit:  intQuery(c, "limit", 25),
		Offset: intQuery(c, "offset", 0),
	})
	if err != nil {
		logAdminError("matches", err)
		respondError(c, http.StatusInternalServerError, "could not load matches")
		return
	}
	respond(c, page)
}

func (h *AdminHandler) Deposits(c *gin.Context) {
	from, to, ok := parseDateRange(c)
	if !ok {
		return
	}
	page, err := h.svc.Deposits(c.Request.Context(), domain.FinanceFilter{
		Search: c.Query("search"),
		Status: c.Query("status"),
		From:   from,
		To:     to,
		Limit:  intQuery(c, "limit", 25),
		Offset: intQuery(c, "offset", 0),
	})
	if err != nil {
		logAdminError("deposits", err)
		respondError(c, http.StatusInternalServerError, "could not load deposits")
		return
	}
	respond(c, page)
}

func (h *AdminHandler) Withdrawals(c *gin.Context) {
	from, to, ok := parseDateRange(c)
	if !ok {
		return
	}
	page, err := h.svc.Withdrawals(c.Request.Context(), domain.FinanceFilter{
		Search: c.Query("search"),
		Status: c.Query("status"),
		From:   from,
		To:     to,
		Limit:  intQuery(c, "limit", 25),
		Offset: intQuery(c, "offset", 0),
	})
	if err != nil {
		logAdminError("withdrawals", err)
		respondError(c, http.StatusInternalServerError, "could not load withdrawals")
		return
	}
	respond(c, page)
}

func (h *AdminHandler) Live(c *gin.Context) {
	snapshot, err := h.svc.Live(c.Request.Context())
	if err != nil {
		logAdminError("live", err)
		respondError(c, http.StatusInternalServerError, "could not load live activity")
		return
	}
	respond(c, snapshot)
}
