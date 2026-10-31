package http

import (
	"github.com/gin-gonic/gin"
	authctrl "github.com/stayhub/backend/internal/controllers/auth"
	catalogctrl "github.com/stayhub/backend/internal/controllers/catalog"
	cohostctrl "github.com/stayhub/backend/internal/controllers/cohost"
	icalctrl "github.com/stayhub/backend/internal/controllers/ical"
	listingctrl "github.com/stayhub/backend/internal/controllers/listing"
	mediactrl "github.com/stayhub/backend/internal/controllers/media"
	pricingctrl "github.com/stayhub/backend/internal/controllers/pricing"
	reviewctrl "github.com/stayhub/backend/internal/controllers/review"
	systemctrl "github.com/stayhub/backend/internal/controllers/system"
	wishlistctrl "github.com/stayhub/backend/internal/controllers/wishlist"
	"github.com/stayhub/backend/internal/http/middleware"
)

func registerPublicRoutes(r *gin.Engine, v1 *gin.RouterGroup, deps ServerDeps) {
	systemCtrl := systemctrl.NewController(deps.Config, deps.DB, deps.Redis, deps.Clock)
	authCtrl := authctrl.NewController(deps.AuthService)
	mediaCtrl := mediactrl.NewController(deps.MediaService)
	catalogCtrl := catalogctrl.NewController(deps.CatalogService)
	guestListingCtrl := listingctrl.NewGuestController(deps.SearchService)
	pricingCtrl := pricingctrl.NewController(deps.PricingService)
	icalCtrl := icalctrl.NewController(deps.ICalService)
	cohostCtrl := cohostctrl.NewController(deps.CohostService, deps.UserRepository)
	reviewCtrl := reviewctrl.NewController(deps.ReviewService, deps.UserRepository)
	wishlistCtrl := wishlistctrl.NewController(deps.WishlistService)

	// Infrastructure & Health Checks
	r.GET("/healthz", systemCtrl.Healthz)
	r.GET("/readyz", systemCtrl.Readyz)

	// Public iCal export subscription (both root and v1 paths)
	r.GET("/ical/:token", icalCtrl.ExportFeed)
	v1.GET("/ical/:token", icalCtrl.ExportFeed)

	// Reference Catalogs (public, Redis-cached)
	catalog := v1.Group("/catalog")
	{
		catalog.GET("/amenities", catalogCtrl.ListAmenities)
		catalog.GET("/cancellation-policies", catalogCtrl.ListCancellationPolicies)
	}

	// Guest: Public Search, Listing Detail & Stay Quote
	listings := v1.Group("/listings")
	{
		listings.GET("/search", guestListingCtrl.SearchListings)
		listings.GET("/:id", guestListingCtrl.GetListingDetail)
		listings.POST("/:id/quote", guestListingCtrl.QuoteStay)
		listings.GET("/:id/quote", pricingCtrl.GetQuote)
		listings.GET("/:id/reviews", reviewCtrl.ListListingReviews)
	}

	// Co-host Invitation Details & Acceptance
	v1.GET("/cohost-invitations/:token", cohostCtrl.GetInvitationDetails)
	if deps.JWTManager != nil {
		v1.POST("/cohost-invitations/:token/accept", middleware.RequireUser(deps.JWTManager), cohostCtrl.AcceptInvitation)
	}

	// Public Shared Wishlists
	v1.GET("/shared/wishlists/:token", wishlistCtrl.GetSharedWishlist)

	// Authentication & Sessions
	auth := v1.Group("/auth")
	{
		auth.POST("/register", authCtrl.Register)
		auth.POST("/login", authCtrl.LoginUser)
		auth.POST("/admin/login", authCtrl.LoginAdmin)
		auth.POST("/refresh", authCtrl.Refresh)
		auth.POST("/logout", authCtrl.Logout)
	}

	// Public Media Upload PUT & File Serving
	v1.PUT("/media/:id/upload", mediaCtrl.UploadContent)
	v1.GET("/media/:id/raw", mediaCtrl.ServeByID)
	v1.GET("/media/file/*filepath", mediaCtrl.ServeFile)
}
