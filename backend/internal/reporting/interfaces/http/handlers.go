package http

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/raffle-app/backend/internal/reporting/application"
	"github.com/raffle-app/backend/internal/reporting/domain"
)

type ReportHandler struct {
	svc *application.ReportService
}

func NewReportHandler(svc *application.ReportService) *ReportHandler {
	return &ReportHandler{svc: svc}
}

// parseFilter reads from, to, limit, offset from query params.
func parseFilter(c *gin.Context) domain.Filter {
	f := domain.Filter{
		Limit:  intQuery(c, "limit", 30),
		Offset: intQuery(c, "offset", 0),
	}
	if s := c.Query("from"); s != "" {
		f.From, _ = time.Parse(time.DateOnly, s)
	}
	if s := c.Query("to"); s != "" {
		f.To, _ = time.Parse(time.DateOnly, s)
	}
	return f
}

func parsePeriod(c *gin.Context) domain.Period {
	switch c.Query("period") {
	case "weekly":
		return domain.PeriodWeekly
	case "monthly":
		return domain.PeriodMonthly
	default:
		return domain.PeriodDaily
	}
}

func intQuery(c *gin.Context, key string, def int) int {
	v, err := strconv.Atoi(c.Query(key))
	if err != nil || v <= 0 {
		return def
	}
	return v
}

func respond(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{"code": "SUCCESS", "data": data})
}

func respondErr(c *gin.Context, err error) {
	c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "error": gin.H{"message": err.Error()}})
}

func (h *ReportHandler) Revenue(c *gin.Context) {
	page, err := h.svc.Revenue(c.Request.Context(), parsePeriod(c), parseFilter(c))
	if err != nil {
		respondErr(c, err)
		return
	}
	respond(c, page)
}

func (h *ReportHandler) TicketSales(c *gin.Context) {
	page, err := h.svc.TicketSales(c.Request.Context(), parsePeriod(c), parseFilter(c))
	if err != nil {
		respondErr(c, err)
		return
	}
	respond(c, page)
}

func (h *ReportHandler) ActiveUsers(c *gin.Context) {
	page, err := h.svc.ActiveUsers(c.Request.Context(), parsePeriod(c), parseFilter(c))
	if err != nil {
		respondErr(c, err)
		return
	}
	respond(c, page)
}

func (h *ReportHandler) WinnerSummary(c *gin.Context) {
	page, err := h.svc.WinnerSummary(c.Request.Context(), parseFilter(c))
	if err != nil {
		respondErr(c, err)
		return
	}
	respond(c, page)
}

func (h *ReportHandler) ProfitSummary(c *gin.Context) {
	summary, err := h.svc.ProfitSummary(c.Request.Context(), parseFilter(c))
	if err != nil {
		respondErr(c, err)
		return
	}
	respond(c, summary)
}
