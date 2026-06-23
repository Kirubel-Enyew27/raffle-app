package main

import (
	"fmt"

	"github.com/gin-gonic/gin"
	auditapp "github.com/raffle-app/backend/internal/audit/application"
	audithttp "github.com/raffle-app/backend/internal/audit/interfaces/http"
	auditmw "github.com/raffle-app/backend/internal/audit/interfaces/http/middleware"
	auditrepo "github.com/raffle-app/backend/internal/audit/interfaces/repository"
	drawrepo "github.com/raffle-app/backend/internal/draw/interfaces/repository"
	identityrepo "github.com/raffle-app/backend/internal/identity/interfaces/repository"
	rafflerepo "github.com/raffle-app/backend/internal/raffle/interfaces/repository"
	ticketrepo "github.com/raffle-app/backend/internal/ticket/interfaces/repository"
	reportapp "github.com/raffle-app/backend/internal/reporting/application"
	reporthttp "github.com/raffle-app/backend/internal/reporting/interfaces/http"
	reportrepo "github.com/raffle-app/backend/internal/reporting/interfaces/repository"
	winnerapp "github.com/raffle-app/backend/internal/winner/application"
	winnerhttp "github.com/raffle-app/backend/internal/winner/interfaces/http"
	winnerrepo "github.com/raffle-app/backend/internal/winner/interfaces/repository"
	"github.com/raffle-app/backend/pkg/config"
	"github.com/raffle-app/backend/pkg/database"
	"github.com/raffle-app/backend/pkg/logger"
	"github.com/raffle-app/backend/pkg/middleware"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	if err := logger.Init(cfg.AppEnv, cfg.AppDebug); err != nil {
		panic(err)
	}

	db, err := database.NewPostgres(cfg.Database)
	if err != nil {
		panic(err)
	}
	defer database.Close(db)

	// Audit
	auditSvc := auditapp.NewAuditService(auditrepo.NewAuditRepo(db))

	// Winner
	winnerSvc := winnerapp.NewWinnerService(
		winnerrepo.NewWinnerRepo(db),
		winnerrepo.NewRaffleAdapter(rafflerepo.NewRaffleRepo(db)),
		winnerrepo.NewDrawAdapter(drawrepo.NewDrawRepo(db)),
		winnerrepo.NewUserAdapter(identityrepo.NewUserRepo(db)),
		winnerrepo.NewTicketAdapter(ticketrepo.NewTicketRepo(db)),
		auditSvc,
	)

	r := gin.New()
	r.Use(middleware.RecoveryMiddleware())
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.AuditContextMiddleware())

	api := r.Group("/api/v1")
	api.Use(auditmw.NewAuditMiddleware(auditSvc).Middleware())

	audithttp.RegisterAuditRoutes(api, audithttp.NewAuditHandler(auditSvc))
	reporthttp.RegisterReportRoutes(api, reporthttp.NewReportHandler(
		reportapp.NewReportService(reportrepo.NewReportRepo(db)),
	))
	winnerhttp.RegisterWinnerRoutes(api, winnerhttp.NewWinnerHandler(winnerSvc))

	if err := r.Run(fmt.Sprintf(":%d", cfg.AppPort)); err != nil {
		panic(err)
	}
}
