package http

import (
	"github.com/gin-gonic/gin"
	"github.com/stayhub/backend/internal/config"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/http/middleware"
	"github.com/stayhub/backend/internal/http/response"
	"github.com/stayhub/backend/internal/platform/clock"
	"github.com/stayhub/backend/internal/platform/db"
	"github.com/stayhub/backend/internal/platform/jwt"
	"github.com/stayhub/backend/internal/platform/logger"
	"github.com/stayhub/backend/internal/platform/redis"
)

type ServerDeps struct {
	Config                *config.Config
	DB                    *db.DB
	Redis                 *redis.Client
	Clock                 clock.Clock
	JWTManager            *jwt.Manager
	UserRepository        interfaces.UserRepository
	ListingHostRepository interfaces.ListingHostRepository
	AuditService          interfaces.AuditService
	AuthService           interfaces.AuthService
	MediaService          interfaces.MediaService
	CatalogService        interfaces.CatalogService
	ListingService        interfaces.ListingService
	CalendarService       interfaces.CalendarService
	SearchService         interfaces.SearchService
	ReservationService    interfaces.ReservationService

	AdminRepository           interfaces.AdminRepository
	AdminReservationService   interfaces.AdminReservationService
	AdminListingReviewService interfaces.AdminListingReviewService
	AdminService              interfaces.AdminService
	FeeConfigService          interfaces.FeeConfigService
	PayoutService             interfaces.PayoutService
	LedgerService             interfaces.LedgerService

	PricingService interfaces.PricingService
	ICalService    interfaces.ICalService
	CohostService  interfaces.CohostService

	MessagingService             interfaces.MessagingService
	ReviewService                interfaces.ReviewService
	ItineraryService             interfaces.ItineraryService
	ReservationAlterationService interfaces.ReservationAlterationService
	WishlistService              interfaces.WishlistService
}

func NewRouter(deps ServerDeps) *gin.Engine {
	if deps.Config.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	jsonLogger := logger.NewJSONLogger(deps.Config.AppEnv)
	r.Use(middleware.RequestID())
	r.Use(logger.RequestLogger(jsonLogger))
	r.Use(gin.Recovery())
	r.Use(middleware.CORS(deps.Config.CORSAllowedOrigins))

	r.NoRoute(func(c *gin.Context) {
		response.NotFound(c, "Route not found")
	})

	registerRoutes(r, deps)

	return r
}
