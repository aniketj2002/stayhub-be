package http

import (
	"github.com/gin-gonic/gin"
	alterationctrl "github.com/stayhub/backend/internal/controllers/reservation_alteration"
	authctrl "github.com/stayhub/backend/internal/controllers/auth"
	itineraryctrl "github.com/stayhub/backend/internal/controllers/itinerary"
	mediactrl "github.com/stayhub/backend/internal/controllers/media"
	messagingctrl "github.com/stayhub/backend/internal/controllers/messaging"
	reservationctrl "github.com/stayhub/backend/internal/controllers/reservation"
	reviewctrl "github.com/stayhub/backend/internal/controllers/review"
	wishlistctrl "github.com/stayhub/backend/internal/controllers/wishlist"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/http/middleware"
)

func registerGuestRoutes(v1 *gin.RouterGroup, deps ServerDeps) {
	if deps.JWTManager == nil {
		return
	}

	authCtrl := authctrl.NewController(deps.AuthService)
	mediaCtrl := mediactrl.NewController(deps.MediaService)
	guestResCtrl := reservationctrl.NewGuestController(deps.ReservationService)
	itineraryCtrl := itineraryctrl.NewController(deps.ItineraryService)
	msgCtrl := messagingctrl.NewController(deps.MessagingService)
	if sb, ok := deps.MessagingService.(interface{ SetBroadcaster(interfaces.MessageBroadcaster) }); ok {
		sb.SetBroadcaster(msgCtrl.Hub())
	}
	reviewCtrl := reviewctrl.NewController(deps.ReviewService, deps.UserRepository)
	alterationCtrl := alterationctrl.NewController(deps.ReservationAlterationService)
	wishlistCtrl := wishlistctrl.NewController(deps.WishlistService)

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
	v1.GET("/guest/trips/:id/itinerary", middleware.RequireUser(deps.JWTManager), itineraryCtrl.GetItinerary)

	// In-app Messaging
	conversations := v1.Group("/conversations", middleware.RequireUser(deps.JWTManager))
	{
		conversations.GET("", msgCtrl.ListConversations)
		conversations.POST("", msgCtrl.GetOrCreateConversation)
		conversations.GET("/ws", msgCtrl.WebSocketHandler)
		conversations.GET("/:id", msgCtrl.GetConversation)
		conversations.GET("/:id/messages", msgCtrl.ListMessages)
		conversations.POST("/:id/messages", msgCtrl.SendMessage)
		conversations.POST("/:id/read", msgCtrl.MarkRead)
		conversations.GET("/:id/ws", msgCtrl.WebSocketHandler)
	}
	v1.GET("/ws", middleware.RequireUser(deps.JWTManager), msgCtrl.WebSocketHandler)

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
			"/:id/itinerary",
			middleware.RequireAuth(deps.JWTManager),
			itineraryCtrl.GetItinerary,
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

		// Double-blind Reviews
		reservations.POST("/:id/reviews", middleware.RequireUser(deps.JWTManager), reviewCtrl.SubmitReview)
		reservations.GET("/:id/reviews", middleware.RequireAuth(deps.JWTManager), reviewCtrl.GetReservationReviews)

		// Scalable Reservation Alterations
		reservations.POST("/:id/alterations", middleware.RequireUser(deps.JWTManager), alterationCtrl.RequestAlteration)
		reservations.GET("/:id/alterations", middleware.RequireAuth(deps.JWTManager), alterationCtrl.ListAlterations)
		reservations.POST("/:id/alterations/:alterationId/approve", middleware.RequireUser(deps.JWTManager), alterationCtrl.ApproveAlteration)
		reservations.POST("/:id/alterations/:alterationId/decline", middleware.RequireUser(deps.JWTManager), alterationCtrl.DeclineAlteration)
	}

	// Wishlists
	wishlists := v1.Group("/wishlists", middleware.RequireUser(deps.JWTManager))
	{
		wishlists.GET("", wishlistCtrl.ListWishlists)
		wishlists.POST("", wishlistCtrl.CreateWishlist)
		wishlists.GET("/:id", wishlistCtrl.GetWishlist)
		wishlists.PATCH("/:id", wishlistCtrl.UpdateWishlist)
		wishlists.DELETE("/:id", wishlistCtrl.DeleteWishlist)
		wishlists.POST("/:id/items/:listingId", wishlistCtrl.AddListing)
		wishlists.DELETE("/:id/items/:listingId", wishlistCtrl.RemoveListing)
		wishlists.POST("/:id/share", wishlistCtrl.ShareWishlist)
		wishlists.DELETE("/:id/share", wishlistCtrl.UnshareWishlist)
	}
}
