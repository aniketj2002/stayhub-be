package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/stayhub/backend/internal/config"
	platformaws "github.com/stayhub/backend/internal/platform/aws"
	"github.com/stayhub/backend/internal/platform/clock"
	"github.com/stayhub/backend/internal/platform/db"
	"github.com/stayhub/backend/internal/platform/redis"
	"github.com/stayhub/backend/internal/platform/storage"
	"github.com/stayhub/backend/internal/repositories"
	"github.com/stayhub/backend/internal/services"
	"github.com/stayhub/backend/internal/worker"
	"github.com/stayhub/backend/internal/worker/jobs"
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
		log.Printf("warning: redis connection failed (%v); running worker with postgres sweep only", err)
	} else {
		defer func() { _ = rdb.Close() }()
	}

	clk := clock.NewRealClock()
	store := storage.NewStorage(cfg)

	// AWS Platform Clients
	payoutScheduler, err := platformaws.NewEventBridgePayoutScheduler(ctx, cfg)
	if err != nil {
		log.Printf("warning: EventBridge payout scheduler init error: %v", err)
	}

	sqsClient, err := platformaws.NewSQSClient(ctx, cfg)
	if err != nil {
		log.Printf("warning: SQS client init error: %v", err)
	}

	auditRepo := repositories.NewAuditRepository(database)
	userRepo := repositories.NewUserRepository(database)
	mediaRepo := repositories.NewMediaRepository(database)
	listingHostRepo := repositories.NewListingHostRepository(database)
	catalogRepo := repositories.NewCatalogRepository(database)
	listingRepo := repositories.NewListingRepository(database)
	calendarRepo := repositories.NewCalendarRepository(database)
	reservationRepo := repositories.NewReservationRepository(database)
	paymentRepo := repositories.NewPaymentRepository(database)
	outboxRepo := repositories.NewOutboxRepository(database)
	icalRepo := repositories.NewICalRepository(database)
	pricingRepo := repositories.NewPricingRepository(database)
	feeConfigRepo := repositories.NewFeeConfigRepository(database)

	auditService := services.NewAuditService(auditRepo, clk)
	feeConfigService := services.NewFeeConfigService(cfg, feeConfigRepo, auditService, clk)
	pricingService := services.NewPricingService(database, pricingRepo, listingRepo, feeConfigService, clk)
	catalogService := services.NewCatalogService(catalogRepo, rdb)
	ledgerService := services.NewLedgerService(paymentRepo, auditService)
	payoutService := services.NewPayoutService(cfg, database, rdb, paymentRepo, reservationRepo, ledgerService, auditService, clk, payoutScheduler)
	icalService := services.NewICalService(database, icalRepo, calendarRepo, listingRepo, clk)
	reviewRepo := repositories.NewReviewRepository(database)
	reviewService := services.NewReviewService(database, reviewRepo, reservationRepo, listingRepo, userRepo, clk)
	alterationRepo := repositories.NewReservationAlterationRepository(database)
	alterationService := services.NewReservationAlterationService(database, alterationRepo, reservationRepo, calendarRepo, listingRepo, listingHostRepo, pricingService, pricingRepo, paymentRepo, ledgerService, payoutService, clk)

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

	scheduler := worker.NewScheduler(reservationService, rdb, clk, 5*time.Second, 100)
	scheduler.Registry.Register(jobs.NewDispatchOutboxJob(outboxRepo))
	scheduler.Registry.Register(jobs.NewSyncICalFeedsJob(icalService))
	scheduler.Registry.Register(jobs.NewCloseReviewWindowsJob(reviewService))
	scheduler.Registry.Register(jobs.NewExpireReservationAlterationsJob(alterationService))
	scheduler.Registry.Register(jobs.NewRecomputeHostBadgesJob(database))
	scheduler.Registry.Register(jobs.NewRefreshListingRatingsJob(database))
	payoutConsumer := worker.NewPayoutConsumer(sqsClient, cfg.PayoutQueueURL, payoutService)

	errCh := make(chan error, 2)
	go func() {
		errCh <- scheduler.Run(ctx)
	}()
	go func() {
		errCh <- payoutConsumer.Run(ctx)
	}()

	select {
	case <-ctx.Done():
		log.Println("StayHub background worker stopped gracefully")
	case err := <-errCh:
		if err != nil {
			log.Fatalf("worker component error: %v", err)
		}
	}
}
