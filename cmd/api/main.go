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
	adminRepo := repositories.NewAdminUserRepository(database)
	refreshRepo := repositories.NewRefreshTokenRepository(database)
	mediaRepo := repositories.NewMediaRepository(database)
	listingHostRepo := repositories.NewListingHostRepository(database)
	catalogRepo := repositories.NewCatalogRepository(database)
	listingRepo := repositories.NewListingRepository(database)
	calendarRepo := repositories.NewCalendarRepository(database)
	reservationRepo := repositories.NewReservationRepository(database)
	paymentRepo := repositories.NewPaymentRepository(database)

	// Services
	auditService := services.NewAuditService(auditRepo, clk)
	authService := services.NewAuthService(
		database,
		userRepo,
		adminRepo,
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
	searchService := services.NewSearchService(
		cfg,
		listingRepo,
		calendarRepo,
		userRepo,
		mediaRepo,
		catalogService,
		store,
		clk,
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
	)

	router := apphttp.NewRouter(apphttp.ServerDeps{
		Config:                cfg,
		DB:                    database,
		Redis:                 rdb,
		Clock:                 clk,
		JWTManager:            jwtMgr,
		UserRepository:        userRepo,
		ListingHostRepository: listingHostRepo,
		AuditService:          auditService,
		AuthService:           authService,
		MediaService:          mediaService,
		CatalogService:        catalogService,
		ListingService:        listingService,
		CalendarService:       calendarService,
		SearchService:         searchService,
		ReservationService:    reservationService,
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
