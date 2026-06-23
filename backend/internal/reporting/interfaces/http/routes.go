package http

import (
	"github.com/gin-gonic/gin"
	"github.com/raffle-app/backend/pkg/middleware"
)

// RegisterReportRoutes mounts all reporting endpoints under /reports.
// All routes require authentication; admin-only enforcement is handled
// by the caller attaching a role-check middleware before this group.
func RegisterReportRoutes(r *gin.RouterGroup, h *ReportHandler) {
	g := r.Group("/reports", middleware.AuthMiddleware())
	{
		g.GET("/revenue",       h.Revenue)      // ?period=daily|weekly|monthly&from=&to=&limit=&offset=
		g.GET("/tickets",       h.TicketSales)  // ?period=&from=&to=&limit=&offset=
		g.GET("/active-users",  h.ActiveUsers)  // ?period=&from=&to=&limit=&offset=
		g.GET("/winners",       h.WinnerSummary) // ?from=&to=&limit=&offset=
		g.GET("/profit",        h.ProfitSummary) // ?from=&to=
	}
}
