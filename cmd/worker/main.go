package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/stayhub/backend/internal/config"
	"github.com/stayhub/backend/internal/platform/clock"
	"github.com/stayhub/backend/internal/platform/db"
	"github.com/stayhub/backend/internal/platform/redis"
	"github.com/stayhub/backend/internal/platform/storage"
	"github.com/stayhub/backend/internal/repositories"
	"github.com/stayhub/backend/internal/services"
	"github.com/stayhub/backend/internal/worker"
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

	auditRepo := repositories.NewAuditRepository(database)
	userRepo := repositories.NewUserRepository(database)
	mediaRepo := repositories.NewMediaRepository(database)
	listingHostRepo := repositories.NewListingHostRepository(database)
	catalogRepo := repositories.NewCatalogRepository(database)
	listingRepo := repositories.NewListingRepository(database)
	calendarRepo := repositories.NewCalendarRepository(database)
	reservationRepo := repositories.NewReservationRepository(database)
	paymentRepo := repositories.NewPaymentRepository(database)

	auditService := services.NewAuditService(auditRepo, clk)
	catalogService := services.NewCatalogService(catalogRepo, rdb)
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
	)

	scheduler := worker.NewScheduler(reservationService, rdb, clk, 5*time.Second, 100)
	if err := scheduler.Run(ctx); err != nil {
		log.Fatalf("worker error: %v", err)
	}
}
