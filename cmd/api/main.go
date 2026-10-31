package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/stayhub/backend/internal/config"
	apphttp "github.com/stayhub/backend/internal/http"
	platformaws "github.com/stayhub/backend/internal/platform/aws"
	"github.com/stayhub/backend/internal/platform/clock"
	"github.com/stayhub/backend/internal/platform/db"
	"github.com/stayhub/backend/internal/platform/jwt"
	"github.com/stayhub/backend/internal/platform/redis"
	"github.com/stayhub/backend/internal/platform/storage"
	"github.com/stayhub/backend/internal/repositories"
	"github.com/stayhub/backend/internal/services"
)

func main() {
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	database, err := db.NewPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to postgres: %v", err)
	}
	defer database.Close()

	rdb, err := redis.NewClient(ctx, cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		log.Printf("warning: redis connection failed (%v); continuing with postgres fallback", err)
	} else {
		defer func() { _ = rdb.Close() }()
	}

	clk := clock.NewRealClock()
	store := storage.NewStorage(cfg)
	jwtMgr := jwt.NewManager(cfg.JWTSecret, cfg.JWTAccessTTL, cfg.JWTRefreshTTL, clk)

	// Repositories
	auditRepo := repositories.NewAuditRepository(database)
	userRepo := repositories.NewUserRepository(database)
	adminUserRepo := repositories.NewAdminUserRepository(database)
	adminRepo := repositories.NewAdminRepository(database)
	feeConfigRepo := repositories.NewFeeConfigRepository(database)
	_ = repositories.NewOutboxRepository(database)
	refreshRepo := repositories.NewRefreshTokenRepository(database)
	mediaRepo := repositories.NewMediaRepository(database)
	listingHostRepo := repositories.NewListingHostRepository(database)
	catalogRepo := repositories.NewCatalogRepository(database)
	listingRepo := repositories.NewListingRepository(database)
	calendarRepo := repositories.NewCalendarRepository(database)
	reservationRepo := repositories.NewReservationRepository(database)
	paymentRepo := repositories.NewPaymentRepository(database)
	pricingRepo := repositories.NewPricingRepository(database)
	icalRepo := repositories.NewICalRepository(database)
	cohostRepo := repositories.NewCohostRepository(database)

	// AWS EventBridge Scheduler & SES Email Sender
	payoutScheduler, err := platformaws.NewEventBridgePayoutScheduler(ctx, cfg)
	if err != nil {
		log.Printf("warning: EventBridge payout scheduler init error: %v", err)
	}
	sesEmailSender, err := platformaws.NewSESEmailSender(ctx, cfg)
	if err != nil {
		log.Printf("warning: AWS SES email sender init error: %v", err)
	}

	// Services
	auditService := services.NewAuditService(auditRepo, clk)
	feeConfigService := services.NewFeeConfigService(cfg, feeConfigRepo, auditService, clk)
	ledgerService := services.NewLedgerService(paymentRepo, auditService)
	payoutService := services.NewPayoutService(cfg, database, rdb, paymentRepo, reservationRepo, ledgerService, auditService, clk, payoutScheduler)
	authService := services.NewAuthService(
		database,
		userRepo,
		adminUserRepo,
		refreshRepo,
		mediaRepo,
		store,
		jwtMgr,
		auditService,
		clk,
	)
	mediaService := services.NewMediaService(database, mediaRepo, store, auditService, clk)
	catalogService := services.NewCatalogService(catalogRepo, rdb)
	listingService := services.NewListingService(
		cfg,
		database,
		listingRepo,
		listingHostRepo,
		userRepo,
		mediaRepo,
		catalogService,
		store,
		auditService,
		clk,
	)
	calendarService := services.NewCalendarService(database, listingRepo, calendarRepo, auditService)
	pricingService := services.NewPricingService(database, pricingRepo, listingRepo, feeConfigService, clk)
	searchService := services.NewSearchService(
		cfg,
		listingRepo,
		calendarRepo,
		userRepo,
		mediaRepo,
		catalogService,
		store,
		clk,
		pricingService,
		rdb,
	)
	reservationService := services.NewReservationService(
		cfg,
		database,
		rdb,
		reservationRepo,
		paymentRepo,
		listingRepo,
		listingHostRepo,
		calendarRepo,
		userRepo,
		mediaRepo,
		catalogService,
		store,
		auditService,
		clk,
		payoutScheduler,
	)
	icalService := services.NewICalService(database, icalRepo, calendarRepo, listingRepo, clk)
	cohostService := services.NewCohostService(database, cohostRepo, listingHostRepo, userRepo, listingRepo, clk, sesEmailSender)
	if cs, ok := cohostService.(*services.CohostService); ok {
		cs.SetEmailConfig(sesEmailSender, cfg.AppBaseURL)
	}

	messagingRepo := repositories.NewMessagingRepository(database)
	reviewRepo := repositories.NewReviewRepository(database)
	alterationRepo := repositories.NewReservationAlterationRepository(database)
	wishlistRepo := repositories.NewWishlistRepository(database)

	messagingService := services.NewMessagingService(database, messagingRepo, reservationRepo, listingRepo, listingHostRepo, userRepo, clk)
	if ms, ok := messagingService.(*services.MessagingService); ok {
		ms.SetMediaDependencies(mediaRepo, store)
	}
	reviewService := services.NewReviewService(database, reviewRepo, reservationRepo, listingRepo, userRepo, clk)
	itineraryService := services.NewItineraryService(reservationRepo, listingRepo, listingHostRepo, userRepo, paymentRepo)
	alterationService := services.NewReservationAlterationService(database, alterationRepo, reservationRepo, calendarRepo, listingRepo, listingHostRepo, pricingService, pricingRepo, paymentRepo, ledgerService, payoutService, clk)
	if as, ok := alterationService.(*services.ReservationAlterationService); ok {
		as.SetMessagingService(messagingService)
	}
	wishlistService := services.NewWishlistService(wishlistRepo, listingRepo, mediaRepo, store, clk)

	if ps, ok := payoutService.(*services.PayoutService); ok {
		ps.SetCohostDependencies(listingHostRepo, cohostRepo)
	}

	if rs, ok := reservationService.(*services.ReservationService); ok {
		rs.SetFeeConfigService(feeConfigService)
		rs.SetPricingRepo(pricingRepo)
		rs.SetMessagingService(messagingService)
	}

	adminReservationService := services.NewAdminReservationService(
		database,
		reservationRepo,
		paymentRepo,
		listingRepo,
		calendarRepo,
		userRepo,
		mediaRepo,
		catalogService,
		ledgerService,
		payoutService,
		auditService,
		store,
		clk,
	)
	adminReviewService := services.NewAdminListingReviewService(
		database,
		listingRepo,
		userRepo,
		auditService,
		clk,
	)
	adminService := services.NewAdminService(
		adminRepo,
		auditService,
		clk,
	)

	router := apphttp.NewRouter(apphttp.ServerDeps{
		Config:                       cfg,
		DB:                           database,
		Redis:                        rdb,
		Clock:                        clk,
		JWTManager:                   jwtMgr,
		UserRepository:               userRepo,
		ListingHostRepository:        listingHostRepo,
		AuditService:                 auditService,
		AuthService:                  authService,
		MediaService:                 mediaService,
		CatalogService:               catalogService,
		ListingService:               listingService,
		CalendarService:              calendarService,
		SearchService:                searchService,
		ReservationService:           reservationService,
		AdminRepository:              adminRepo,
		AdminReservationService:      adminReservationService,
		AdminListingReviewService:    adminReviewService,
		AdminService:                 adminService,
		FeeConfigService:             feeConfigService,
		PayoutService:                payoutService,
		LedgerService:                ledgerService,
		PricingService:               pricingService,
		ICalService:                  icalService,
		CohostService:                cohostService,
		MessagingService:             messagingService,
		ReviewService:                reviewService,
		ItineraryService:             itineraryService,
		ReservationAlterationService: alterationService,
		WishlistService:              wishlistService,
	})

	srv := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("StayHub API listening on :%s (mode=%s, env=%s)", cfg.HTTPPort, cfg.VersionMode, cfg.AppEnv)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down StayHub API server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown error: %v", err)
	}
}
