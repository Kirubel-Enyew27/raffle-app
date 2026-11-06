package http

import (
	"github.com/gin-gonic/gin"
	"github.com/raffle-app/backend/pkg/middleware"
)

// RegisterReportRoutes mounts all reporting endpoints under /reports.
// Administrative reporting endpoints enforce both authentication and admin role.
func RegisterReportRoutes(r *gin.RouterGroup, h *ReportHandler) {
	g := r.Group("/reports", middleware.AuthMiddleware(), middleware.AdminMiddleware())
	{
		g.GET("/revenue",       h.Revenue)      // ?period=daily|weekly|monthly&from=&to=&limit=&offset=
		g.GET("/tickets",       h.TicketSales)  // ?period=&from=&to=&limit=&offset=
		g.GET("/active-users",  h.ActiveUsers)  // ?period=&from=&to=&limit=&offset=
		g.GET("/winners",       h.WinnerSummary) // ?from=&to=&limit=&offset=
		g.GET("/profit",        h.ProfitSummary) // ?from=&to=
	}
}
