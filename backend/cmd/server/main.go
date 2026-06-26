package main

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	auditapp "github.com/raffle-app/backend/internal/audit/application"
	audithttp "github.com/raffle-app/backend/internal/audit/interfaces/http"
	auditmw "github.com/raffle-app/backend/internal/audit/interfaces/http/middleware"
	auditrepo "github.com/raffle-app/backend/internal/audit/interfaces/repository"
	drawapp "github.com/raffle-app/backend/internal/draw/application"
	drawhttp "github.com/raffle-app/backend/internal/draw/interfaces/http"
	drawrepo "github.com/raffle-app/backend/internal/draw/interfaces/repository"
	drawinfra "github.com/raffle-app/backend/internal/draw/infrastructure"
	identityapp "github.com/raffle-app/backend/internal/identity/application"
	identityhttp "github.com/raffle-app/backend/internal/identity/interfaces/http"
	identityrepo "github.com/raffle-app/backend/internal/identity/interfaces/repository"
	notificationapp "github.com/raffle-app/backend/internal/notification/application"
	notificationhttp "github.com/raffle-app/backend/internal/notification/interfaces/http"
	notificationqueue "github.com/raffle-app/backend/internal/notification/interfaces/queue"
	notificationrepo "github.com/raffle-app/backend/internal/notification/interfaces/repository"
	raffleapp "github.com/raffle-app/backend/internal/raffle/application"
	rafflehttp "github.com/raffle-app/backend/internal/raffle/interfaces/http"
	rafflerepo "github.com/raffle-app/backend/internal/raffle/interfaces/repository"
	reportapp "github.com/raffle-app/backend/internal/reporting/application"
	reporthttp "github.com/raffle-app/backend/internal/reporting/interfaces/http"
	reportrepo "github.com/raffle-app/backend/internal/reporting/interfaces/repository"
	ticketapp "github.com/raffle-app/backend/internal/ticket/application"
	tickethttp "github.com/raffle-app/backend/internal/ticket/interfaces/http"
	ticketrepo "github.com/raffle-app/backend/internal/ticket/interfaces/repository"
	walletapp "github.com/raffle-app/backend/internal/wallet/application"
	wallethttp "github.com/raffle-app/backend/internal/wallet/interfaces/http"
	walletrepo "github.com/raffle-app/backend/internal/wallet/interfaces/repository"
	winnerapp "github.com/raffle-app/backend/internal/winner/application"
	winnerhttp "github.com/raffle-app/backend/internal/winner/interfaces/http"
	winnerrepo "github.com/raffle-app/backend/internal/winner/interfaces/repository"
	smsapp "github.com/raffle-app/backend/internal/sms/application"
	smshttp "github.com/raffle-app/backend/internal/sms/interfaces/http"
	smsinfra "github.com/raffle-app/backend/internal/sms/infrastructure"
	smsrepo "github.com/raffle-app/backend/internal/sms/interfaces/repository"
	"github.com/raffle-app/backend/pkg/config"
	"github.com/raffle-app/backend/pkg/database"
	"github.com/raffle-app/backend/pkg/idempotency"
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

	rdb, err := database.NewRedis(cfg.Redis)
	if err != nil {
		panic(err)
	}
	defer database.CloseRedis(rdb)

	// Run database migrations (safe to run every startup — tracks applied in schema_migrations)
	if err := database.RunMigrations(db, "migrations"); err != nil {
		panic(fmt.Errorf("failed to run migrations: %w", err))
	}

	// Services
	auditSvc := auditapp.NewAuditService(auditrepo.NewAuditRepo(db))

	identitySvc := identityapp.NewIdentityService(
		identityrepo.NewUserRepo(db),
		auditSvc,
		[]byte(cfg.JWT.Secret),
		cfg.JWT.AccessExpiry,
	)

	walletSvc := walletapp.NewWalletService(walletrepo.NewWalletRepo(db), auditSvc)

	raffleSvc := raffleapp.NewRaffleService(rafflerepo.NewRaffleRepo(db), auditSvc)

	idempotencyStore := idempotency.NewStore(rdb, 24*time.Hour)
	ticketSvc := ticketapp.NewTicketService(
		db,
		ticketrepo.NewTicketRepo(db),
		ticketrepo.NewTicketRaffleRepo(db),
		ticketrepo.NewTicketWalletRepo(db),
		auditSvc,
		idempotencyStore,
	)

	winnerSvc := winnerapp.NewWinnerService(
		winnerrepo.NewWinnerRepo(db),
		winnerrepo.NewRaffleAdapter(rafflerepo.NewRaffleRepo(db)),
		winnerrepo.NewDrawAdapter(drawrepo.NewDrawRepo(db)),
		winnerrepo.NewUserAdapter(identityrepo.NewUserRepo(db)),
		winnerrepo.NewTicketAdapter(ticketrepo.NewTicketRepo(db)),
		auditSvc,
	)

	drawSvc := drawapp.NewDrawService(
		drawrepo.NewDrawRepo(db),
		drawrepo.NewDrawRaffleAdapter(rafflerepo.NewRaffleRepo(db), ticketrepo.NewTicketRepo(db)),
		ticketrepo.NewTicketRepo(db),
		drawinfra.NewCryptoSeedService(),
		drawinfra.NewCryptoRandomService(),
		auditSvc,
		winnerSvc,
	)

	notificationSvc := notificationapp.NewNotificationService(
		notificationrepo.NewNotificationRepo(db),
		notificationqueue.NewRedisQueue(rdb),
	)

	reportSvc := reportapp.NewReportService(reportrepo.NewReportRepo(db))

	userRepo := identityrepo.NewUserRepo(db)
	smsSvc := smsapp.NewSMSService(
		db,
		userRepo,
		walletrepo.NewWalletRepo(db),
		smsrepo.NewSMSLogRepo(db),
		smsinfra.NewReceiptFetcher(30*time.Second),
		cfg.SMSAPIKey,
	)

	// Router
	r := gin.New()
	r.Use(middleware.RecoveryMiddleware())
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.AuditContextMiddleware())

	// Serve uploaded files (avatars, etc.)
	r.Static("/uploads", "./uploads")

	api := r.Group("/api/v1")
	api.Use(auditmw.NewAuditMiddleware(auditSvc).Middleware())

	identityhttp.RegisterIdentityRoutes(api, identityhttp.NewIdentityHandler(identitySvc))
	wallethttp.RegisterWalletRoutes(api, wallethttp.NewWalletHandler(walletSvc))
	rafflehttp.RegisterRaffleRoutes(api, rafflehttp.NewRaffleHandler(raffleSvc))
	tickethttp.RegisterTicketRoutes(api, tickethttp.NewTicketHandler(ticketSvc))
	drawhttp.RegisterDrawRoutes(api, drawhttp.NewDrawHandler(drawSvc))
	winnerhttp.RegisterWinnerRoutes(api, winnerhttp.NewWinnerHandler(winnerSvc))
	notificationhttp.RegisterRoutes(api, notificationhttp.NewHandler(notificationSvc))
	audithttp.RegisterAuditRoutes(api, audithttp.NewAuditHandler(auditSvc))
	reporthttp.RegisterReportRoutes(api, reporthttp.NewReportHandler(reportSvc))
	smshttp.RegisterSMSRoutes(api, smshttp.NewSMSHandler(smsSvc), cfg.SMSAPIKey)

	if err := r.Run(fmt.Sprintf(":%d", cfg.AppPort)); err != nil {
		panic(err)
	}
}
