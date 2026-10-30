package http

import (
	"github.com/gin-gonic/gin"
	auditctrl "github.com/stayhub/backend/internal/controllers/audit"
	authctrl "github.com/stayhub/backend/internal/controllers/auth"
	catalogctrl "github.com/stayhub/backend/internal/controllers/catalog"
	listingctrl "github.com/stayhub/backend/internal/controllers/listing"
	mediactrl "github.com/stayhub/backend/internal/controllers/media"
	reservationctrl "github.com/stayhub/backend/internal/controllers/reservation"
	systemctrl "github.com/stayhub/backend/internal/controllers/system"
	"github.com/stayhub/backend/internal/http/middleware"
)

func registerRoutes(r *gin.Engine, deps ServerDeps) {
	systemCtrl := systemctrl.NewController(deps.Config, deps.DB, deps.Redis, deps.Clock)
	eventsCtrl := auditctrl.NewController(deps.AuditService)
	authCtrl := authctrl.NewController(deps.AuthService)
	mediaCtrl := mediactrl.NewController(deps.MediaService)
	catalogCtrl := catalogctrl.NewController(deps.CatalogService)
	hostListingCtrl := listingctrl.NewHostController(deps.ListingService, deps.CalendarService, deps.Clock)
	guestListingCtrl := listingctrl.NewGuestController(deps.SearchService)
	guestResCtrl := reservationctrl.NewGuestController(deps.ReservationService)
	hostResCtrl := reservationctrl.NewHostController(deps.ReservationService)

	// Infrastructure & health
	r.GET("/healthz", systemCtrl.Healthz)

	// API v1 route groups
	v1 := r.Group("/api/v1")
	{
		// Platform / Admin Event Log
		v1.GET("/events", eventsCtrl.ListEvents)

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
		}

		// Authentication & Sessions
		auth := v1.Group("/auth")
		{
			auth.POST("/register", authCtrl.Register)
			auth.POST("/login", authCtrl.LoginUser)
			auth.POST("/admin/login", authCtrl.LoginAdmin)
			auth.POST("/refresh", authCtrl.Refresh)
			auth.POST("/logout", authCtrl.Logout)
		}

		// Public Media Upload PUT & File Serving (avoids browser-to-S3 CORS errors)
		v1.PUT("/media/:id/upload", mediaCtrl.UploadContent)
		v1.GET("/media/file/*filepath", mediaCtrl.ServeFile)

		// Authenticated Endpoints
		if deps.JWTManager != nil {
			v1.GET("/me", middleware.RequireAuth(deps.JWTManager), authCtrl.GetMe)
			v1.PATCH("/me", middleware.RequireUser(deps.JWTManager), authCtrl.UpdateProfile)
			v1.POST("/me/become-host", middleware.RequireUser(deps.JWTManager), authCtrl.BecomeHost)

			// Media Presigned Upload & Confirmation
			media := v1.Group("/media", middleware.RequireAuth(deps.JWTManager))
			{
				media.POST("/presign", mediaCtrl.PresignUpload)
				media.POST("/:id/confirm", mediaCtrl.ConfirmUpload)
				media.GET("/:id", mediaCtrl.GetMedia)
			}

			// Guest Trips & Shared Reservation Lifecycle
			v1.GET("/guest/trips", middleware.RequireUser(deps.JWTManager), guestResCtrl.ListGuestTrips)

			reservations := v1.Group("/reservations")
			{
				reservations.POST(
					"",
					middleware.RequireUser(deps.JWTManager),
					middleware.RequireIdempotency(deps.DB, deps.Redis),
					guestResCtrl.CreateReservation,
				)
				reservations.GET(
					"/:id",
					middleware.RequireAuth(deps.JWTManager),
					guestResCtrl.GetReservationDetail,
				)
				reservations.GET(
					"/:id/cancel-preview",
					middleware.RequireUser(deps.JWTManager),
					guestResCtrl.GetCancelPreview,
				)
				reservations.POST(
					"/:id/cancel",
					middleware.RequireUser(deps.JWTManager),
					middleware.RequireIdempotency(deps.DB, deps.Redis),
					guestResCtrl.CancelByGuest,
				)
			}

			// Host: Listings, Wizard Versioning, Per-Listing Config, Calendar, Today & Reservations
			hostGroup := v1.Group("/host")
			{
				// Creating the first listing also promotes a guest user to is_host=true
				hostGroup.POST("/listings", middleware.RequireUser(deps.JWTManager), hostListingCtrl.CreateListing)

				hostAuthed := hostGroup.Group("", middleware.RequireHost(deps.JWTManager, deps.UserRepository))
				{
					hostAuthed.GET("/today", hostResCtrl.GetHostToday)
					hostAuthed.GET("/earnings", hostResCtrl.GetHostEarnings)

					hostReservations := hostAuthed.Group("/reservations")
					{
						hostReservations.POST(
							"/:id/accept",
							middleware.RequireIdempotency(deps.DB, deps.Redis),
							hostResCtrl.AcceptReservation,
						)
						hostReservations.POST(
							"/:id/decline",
							middleware.RequireIdempotency(deps.DB, deps.Redis),
							hostResCtrl.DeclineReservation,
						)
						hostReservations.POST(
							"/:id/cancel",
							middleware.RequireIdempotency(deps.DB, deps.Redis),
							hostResCtrl.CancelReservation,
						)
						hostReservations.POST(
							"/:id/check-in",
							middleware.RequireIdempotency(deps.DB, deps.Redis),
							hostResCtrl.CheckInReservation,
						)
					}
				}

				hostListings := hostGroup.Group("/listings", middleware.RequireHost(deps.JWTManager, deps.UserRepository))
				{
					hostListings.GET("", hostListingCtrl.ListHostListings)
					hostListings.GET(
						"/:id",
						middleware.RequireListingPermission(deps.ListingHostRepository, "id", middleware.PermAnyHostMember),
						hostListingCtrl.GetHostListing,
					)
					hostListings.PATCH(
						"/:id/working-version",
						middleware.RequireListingPermission(deps.ListingHostRepository, "id", middleware.PermManageListing),
						hostListingCtrl.UpdateWorkingVersion,
					)
					hostListings.POST(
						"/:id/submit",
						middleware.RequireListingPermission(deps.ListingHostRepository, "id", middleware.PermManageListing),
						hostListingCtrl.SubmitListing,
					)
					hostListings.PATCH(
						"/:id/host-config",
						middleware.RequireListingPermission(deps.ListingHostRepository, "id", middleware.PermAnyHostMember),
						hostListingCtrl.UpdateHostConfig,
					)
					hostListings.GET(
						"/:id/calendar",
						middleware.RequireListingPermission(deps.ListingHostRepository, "id", middleware.PermAnyHostMember),
						hostListingCtrl.GetCalendar,
					)
					hostListings.PUT(
						"/:id/calendar/block",
						middleware.RequireListingPermission(deps.ListingHostRepository, "id", middleware.PermManageCalendar),
						hostListingCtrl.BlockCalendar,
					)
					hostListings.DELETE(
						"/:id/calendar/block",
						middleware.RequireListingPermission(deps.ListingHostRepository, "id", middleware.PermManageCalendar),
						hostListingCtrl.UnblockCalendar,
					)
				}
			}
		}
	}
}
