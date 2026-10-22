package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	auditapp "github.com/raffle-app/backend/internal/audit/application"
	audithttp "github.com/raffle-app/backend/internal/audit/interfaces/http"
	auditmw "github.com/raffle-app/backend/internal/audit/interfaces/http/middleware"
	auditrepo "github.com/raffle-app/backend/internal/audit/interfaces/repository"
	crashapp "github.com/raffle-app/backend/internal/crash/application"
	crashdomain "github.com/raffle-app/backend/internal/crash/domain"
	crashinfra "github.com/raffle-app/backend/internal/crash/infrastructure"
	crashhttp "github.com/raffle-app/backend/internal/crash/interfaces/http"
	crashrepo "github.com/raffle-app/backend/internal/crash/interfaces/repository"
	drawapp "github.com/raffle-app/backend/internal/draw/application"
	drawinfra "github.com/raffle-app/backend/internal/draw/infrastructure"
	drawhttp "github.com/raffle-app/backend/internal/draw/interfaces/http"
	drawrepo "github.com/raffle-app/backend/internal/draw/interfaces/repository"
	identityapp "github.com/raffle-app/backend/internal/identity/application"
	identityhttp "github.com/raffle-app/backend/internal/identity/interfaces/http"
	identityrepo "github.com/raffle-app/backend/internal/identity/interfaces/repository"
	notificationapp "github.com/raffle-app/backend/internal/notification/application"
	notificationhttp "github.com/raffle-app/backend/internal/notification/interfaces/http"
	notificationqueue "github.com/raffle-app/backend/internal/notification/interfaces/queue"
	notificationrepo "github.com/raffle-app/backend/internal/notification/interfaces/repository"
	p2papp "github.com/raffle-app/backend/internal/p2p/application"
	p2phttp "github.com/raffle-app/backend/internal/p2p/interfaces/http"
	p2prepo "github.com/raffle-app/backend/internal/p2p/interfaces/repository"
	raffleapp "github.com/raffle-app/backend/internal/raffle/application"
	rafflehttp "github.com/raffle-app/backend/internal/raffle/interfaces/http"
	rafflerepo "github.com/raffle-app/backend/internal/raffle/interfaces/repository"
	realtime "github.com/raffle-app/backend/internal/realtime"
	reportapp "github.com/raffle-app/backend/internal/reporting/application"
	reporthttp "github.com/raffle-app/backend/internal/reporting/interfaces/http"
	reportrepo "github.com/raffle-app/backend/internal/reporting/interfaces/repository"
	smsapp "github.com/raffle-app/backend/internal/sms/application"
	smsinfra "github.com/raffle-app/backend/internal/sms/infrastructure"
	smshttp "github.com/raffle-app/backend/internal/sms/interfaces/http"
	smsrepo "github.com/raffle-app/backend/internal/sms/interfaces/repository"
	ticketapp "github.com/raffle-app/backend/internal/ticket/application"
	tickethttp "github.com/raffle-app/backend/internal/ticket/interfaces/http"
	ticketrepo "github.com/raffle-app/backend/internal/ticket/interfaces/repository"
	walletapp "github.com/raffle-app/backend/internal/wallet/application"
	walletinfra "github.com/raffle-app/backend/internal/wallet/infrastructure"
	"github.com/raffle-app/backend/internal/wallet/infrastructure/verifier"
	wallethttp "github.com/raffle-app/backend/internal/wallet/interfaces/http"
	walletrepo "github.com/raffle-app/backend/internal/wallet/interfaces/repository"
	winnerapp "github.com/raffle-app/backend/internal/winner/application"
	winnerhttp "github.com/raffle-app/backend/internal/winner/interfaces/http"
	winnerrepo "github.com/raffle-app/backend/internal/winner/interfaces/repository"
	"github.com/raffle-app/backend/pkg/config"
	"github.com/raffle-app/backend/pkg/database"
	"github.com/raffle-app/backend/pkg/idempotency"
	"github.com/raffle-app/backend/pkg/logger"
	"github.com/raffle-app/backend/pkg/middleware"
	"github.com/raffle-app/backend/pkg/money"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	if err := logger.Init(cfg.AppEnv, cfg.AppDebug); err != nil {
		panic(err)
	}

	// Receipt OCR and the Telebirr proxy are optional, but when they are missing
	// deposits degrade quietly (screenshots cannot be read at all). Say so once
	// at startup instead of failing later with a vague extraction error.
	if cfg.GeminiAPIKey == "" {
		logger.Warn("GEMINI_API_KEY is not set: receipt screenshot OCR is disabled, deposits must use a typed transaction number")
	}
	if cfg.TeleVerifyKey == "" {
		logger.Warn("TELE_VERIFY_KEY is not set: Telebirr verification falls back to scraping the public receipt page")
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

	// Real-time Service
	realtimeSvc := realtime.NewService(rdb, logger.With())
	go realtimeSvc.Start(context.Background())

	// Services
	auditSvc := auditapp.NewAuditService(auditrepo.NewAuditRepo(db))

	identitySvc := identityapp.NewIdentityService(
		identityrepo.NewUserRepo(db),
		auditSvc,
		[]byte(cfg.JWT.Secret),
		cfg.JWT.AccessExpiry,
	)

	depositRepo := walletrepo.NewDepositRepo(db)
	cbeVerifier := verifier.NewCBEDirectVerifier(cfg.CBEReceiptAPI, cfg.CBEAppID, cfg.CBEAppVersion, 30*time.Second)
	telebirrVerifier := verifier.NewTelebirrDirectVerifier("", cfg.TeleVerifyURL, cfg.TeleVerifyKey, cfg.LinksETAPIKey, cfg.LinksETBaseURL, 30*time.Second)
	imageExtractor := verifier.NewImageExtractor(cfg.GeminiAPIKey)
	verificationMgr := verifier.NewVerificationManager(
		cbeVerifier,
		telebirrVerifier,
		imageExtractor,
		cfg.CBEAccountName,
		cfg.CBEAccountNumber,
		cfg.TelebirrAccountName,
		cfg.TelebirrPhone,
		cfg.AutoCreditEnabled,
	)

	receiptVerifier := walletinfra.NewReceiptVerifier(cfg.TeleVerifyURL, cfg.TeleVerifyKey, cfg.CBEReceiptAPI, cfg.GeminiAPIKey)
	walletSvc := walletapp.NewWalletService(walletrepo.NewWalletRepo(db), auditSvc)
	walletSvc.SetDepositRepository(depositRepo)
	walletSvc.SetVerificationManager(verificationMgr)
	walletSvc.SetRealtimeService(realtimeSvc)
	walletSvc.SetReceiptVerifier(receiptVerifier)

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
	ticketSvc.SetRealtimeService(realtimeSvc)

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
		walletSvc,
	)
	drawSvc.SetRealtimeService(realtimeSvc)

	notificationSvc := notificationapp.NewNotificationService(
		notificationrepo.NewNotificationRepo(db),
		notificationqueue.NewRedisQueue(rdb),
	)

	reportSvc := reportapp.NewReportService(reportrepo.NewReportRepo(db))

	// Multiplayer crash game: shared authoritative round engine + player service.
	crashCfg := crashdomain.RoundConfig{
		Room:              cfg.Crash.Room,
		WaitingSeconds:    cfg.Crash.WaitingSeconds,
		BettingSeconds:    cfg.Crash.BettingSeconds,
		CrashedSeconds:    cfg.Crash.CrashedSeconds,
		NextRoundSeconds:  cfg.Crash.NextRoundSeconds,
		GrowthRate:        cfg.Crash.GrowthRate,
		HouseEdgeBP:       cfg.Crash.HouseEdgeBP,
		MaxMultiplierBP:   cfg.Crash.MaxMultiplierBP,
		MinBetCents:       money.FromFloat(cfg.Crash.MinBet),
		MaxBetCents:       money.FromFloat(cfg.Crash.MaxBet),
		TickInterval:      cfg.Crash.TickInterval,
		BroadcastInterval: cfg.Crash.BroadcastInterval,
		Enabled:           cfg.Crash.Enabled,
		ServiceFeeBP:      cfg.Crash.ServiceFeeBP,
		MinParticipants:   cfg.Crash.MinParticipants,
		MaxParticipants:   cfg.Crash.MaxParticipants,
		PlatformUserID:    cfg.Crash.PlatformUserID,
	}
	if err := crashCfg.Validate(); err != nil {
		panic(err)
	}

	crashRoundRepo := crashrepo.NewRoundRepo(db)
	crashBetRepo := crashrepo.NewBetRepo(db)
	crashPointSource := crashinfra.NewCrashPointSource(crashCfg.HouseEdgeBP, crashCfg.MaxMultiplierBP)
	crashSettingsRepo := crashrepo.NewSettingsRepo(db)

	crashSvc := crashapp.NewCrashService(
		crashRoundRepo,
		crashBetRepo,
		crashrepo.NewWalletRepo(db),
		crashPointSource,
		crashCfg,
	)
	crashSvc.SetPlayerRepository(crashrepo.NewPlayerRepo(db))
	crashSvc.SetSettingsRepository(crashSettingsRepo)
	crashSvc.SetAuditService(auditSvc)
	crashSvc.SetRealtimeService(realtimeSvc)
	crashSvc.SetIdempotencyStore(idempotencyStore)

	crashEngine := crashapp.NewEngine(
		crashRoundRepo,
		crashBetRepo,
		crashPointSource,
		crashSvc,
		crashrepo.NewAdvisoryLocker(db),
		crashCfg,
		logger.With(),
	)
	crashEngine.SetAuditService(auditSvc)
	crashEngine.SetRealtimeService(realtimeSvc)
	go crashEngine.Run(context.Background())

	userRepo := identityrepo.NewUserRepo(db)
	smsSvc := smsapp.NewSMSService(
		db,
		userRepo,
		walletrepo.NewWalletRepo(db),
		smsrepo.NewSMSLogRepo(db),
		smsinfra.NewReceiptFetcher(30*time.Second, cfg.TeleVerifyURL, cfg.TeleVerifyKey),
		cfg.SMSAPIKey,
	)

	// P2P Multiplayer Platform
	p2pGameRepo := p2prepo.NewGameRepo(db)
	p2pMatchRepo := p2prepo.NewMatchRepo(db)
	p2pPlayerRepo := p2prepo.NewPlayerRepo(db)
	p2pMoveRepo := p2prepo.NewMoveRepo(db)
	p2pSettingsRepo := p2prepo.NewSettingsRepo(db)
	p2pEscrowRepo := p2prepo.NewEscrowRepo(db, cfg.Crash.PlatformUserID)

	p2pSvc := p2papp.NewP2PService(
		p2pGameRepo,
		p2pMatchRepo,
		p2pPlayerRepo,
		p2pMoveRepo,
		p2pSettingsRepo,
		p2pEscrowRepo,
		realtimeSvc,
		auditSvc,
	)

	p2pEngine := p2papp.NewP2PEngine(
		p2pMatchRepo,
		p2pPlayerRepo,
		p2pEscrowRepo,
		realtimeSvc,
		logger.With(),
	)
	go p2pEngine.Run(context.Background())

	// Router
	r := gin.New()
	r.Use(middleware.RecoveryMiddleware())
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.AuditContextMiddleware())

	// Serve uploaded files (avatars, etc.)
	r.Static("/uploads", "./uploads")

	api := r.Group("/api/v1")
	api.Use(auditmw.NewAuditMiddleware(auditSvc).Middleware())

	// Liveness probe used by the container healthcheck and load balancers.
	api.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

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

	crashHandler := crashhttp.NewCrashHandler(crashSvc)
	crashHandler.SetRateLimiter(crashinfra.NewRateLimiter(rdb, cfg.Crash.RateLimitPerSecond, time.Second))
	crashhttp.RegisterCrashRoutes(api, crashHandler)

	p2pHandler := p2phttp.NewP2PHandler(p2pSvc)
	p2phttp.RegisterP2PRoutes(api, p2pHandler)

	realtimeHandler := realtime.NewHandler(realtimeSvc, cfg.JWT.Secret, logger.With())
	api.GET("/realtime/stream", realtimeHandler.Stream)

	if err := r.Run(fmt.Sprintf(":%d", cfg.AppPort)); err != nil {
		panic(err)
	}
}
