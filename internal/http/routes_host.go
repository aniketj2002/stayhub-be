package http

import (
	"github.com/gin-gonic/gin"
	cohostctrl "github.com/stayhub/backend/internal/controllers/cohost"
	icalctrl "github.com/stayhub/backend/internal/controllers/ical"
	listingctrl "github.com/stayhub/backend/internal/controllers/listing"
	pricingctrl "github.com/stayhub/backend/internal/controllers/pricing"
	reservationctrl "github.com/stayhub/backend/internal/controllers/reservation"
	alterationctrl "github.com/stayhub/backend/internal/controllers/reservation_alteration"
	"github.com/stayhub/backend/internal/http/middleware"
)

func registerHostRoutes(v1 *gin.RouterGroup, deps ServerDeps) {
	if deps.JWTManager == nil {
		return
	}

	hostListingCtrl := listingctrl.NewHostController(deps.ListingService, deps.CalendarService, deps.Clock)
	hostResCtrl := reservationctrl.NewHostController(deps.ReservationService)
	alterationCtrl := alterationctrl.NewController(deps.ReservationAlterationService)
	pricingCtrl := pricingctrl.NewController(deps.PricingService)
	icalCtrl := icalctrl.NewController(deps.ICalService)
	cohostCtrl := cohostctrl.NewController(deps.CohostService, deps.UserRepository)

	hostGroup := v1.Group("/host")
	{
		// Creating the first listing also promotes a guest user to is_host=true
		hostGroup.POST("/listings", middleware.RequireUser(deps.JWTManager), hostListingCtrl.CreateListing)

		hostAuthed := hostGroup.Group("", middleware.RequireHost(deps.JWTManager, deps.UserRepository))
		{
			hostAuthed.GET("/today", hostResCtrl.GetHostToday)
			hostAuthed.GET("/date-change-requests", alterationCtrl.ListPendingForHost)
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

			// Pricing Rules & Date Overrides
			hostListings.GET(
				"/:id/pricing-rules",
				middleware.RequireListingPermission(deps.ListingHostRepository, "id", middleware.PermAnyHostMember),
				pricingCtrl.GetPricingRules,
			)
			hostListings.PUT(
				"/:id/pricing-rules",
				middleware.RequireListingPermission(deps.ListingHostRepository, "id", middleware.PermManageListing),
				pricingCtrl.SetPricingRules,
			)
			hostListings.GET(
				"/:id/date-overrides",
				middleware.RequireListingPermission(deps.ListingHostRepository, "id", middleware.PermAnyHostMember),
				pricingCtrl.GetDateOverrides,
			)
			hostListings.PUT(
				"/:id/date-overrides",
				middleware.RequireListingPermission(deps.ListingHostRepository, "id", middleware.PermManageListing),
				pricingCtrl.SetDateOverrides,
			)
			hostListings.DELETE(
				"/:id/date-overrides",
				middleware.RequireListingPermission(deps.ListingHostRepository, "id", middleware.PermManageListing),
				pricingCtrl.DeleteDateOverrides,
			)

			// iCal Feeds & Export Token
			hostListings.POST(
				"/:id/ical-feeds",
				middleware.RequireListingPermission(deps.ListingHostRepository, "id", middleware.PermManageCalendar),
				icalCtrl.CreateFeed,
			)
			hostListings.GET(
				"/:id/ical-feeds",
				middleware.RequireListingPermission(deps.ListingHostRepository, "id", middleware.PermAnyHostMember),
				icalCtrl.ListFeeds,
			)
			hostListings.DELETE(
				"/:id/ical-feeds/:feedId",
				middleware.RequireListingPermission(deps.ListingHostRepository, "id", middleware.PermManageCalendar),
				icalCtrl.DeleteFeed,
			)
			hostListings.POST(
				"/:id/ical-export-token/regenerate",
				middleware.RequireListingPermission(deps.ListingHostRepository, "id", middleware.PermManageListing),
				icalCtrl.RegenerateExportToken,
			)

			// Co-hosts Management
			hostListings.POST(
				"/:id/cohosts/invitations",
				middleware.RequireListingPermission(deps.ListingHostRepository, "id", middleware.PermAnyHostMember),
				cohostCtrl.InviteCohost,
			)
			hostListings.GET(
				"/:id/cohosts/invitations",
				middleware.RequireListingPermission(deps.ListingHostRepository, "id", middleware.PermAnyHostMember),
				cohostCtrl.ListInvitations,
			)
			hostListings.DELETE(
				"/:id/cohosts/invitations/:inviteId",
				middleware.RequireListingPermission(deps.ListingHostRepository, "id", middleware.PermAnyHostMember),
				cohostCtrl.CancelInvitation,
			)
			hostListings.GET(
				"/:id/cohosts",
				middleware.RequireListingPermission(deps.ListingHostRepository, "id", middleware.PermAnyHostMember),
				cohostCtrl.ListCohosts,
			)
			hostListings.PATCH(
				"/:id/cohosts/:userId",
				middleware.RequireListingPermission(deps.ListingHostRepository, "id", middleware.PermAnyHostMember),
				cohostCtrl.UpdateCohost,
			)
			hostListings.DELETE(
				"/:id/cohosts/:userId",
				middleware.RequireListingPermission(deps.ListingHostRepository, "id", middleware.PermAnyHostMember),
				cohostCtrl.RemoveCohost,
			)
		}
	}
}
