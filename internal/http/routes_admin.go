package http

import (
	"github.com/gin-gonic/gin"
	adminctrl "github.com/stayhub/backend/internal/controllers/admin"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/http/middleware"
)

func registerAdminRoutes(v1 *gin.RouterGroup, deps ServerDeps) {
	if deps.JWTManager == nil || deps.AdminRepository == nil {
		return
	}

	adminAuditCtrl := adminctrl.NewAuditController(deps.AuditService)
	adminResCtrl := adminctrl.NewReservationsController(deps.AdminReservationService)
	adminPayoutCtrl := adminctrl.NewPayoutsController(deps.PayoutService)
	adminReviewCtrl := adminctrl.NewListingReviewController(deps.AdminListingReviewService)
	adminFeeCtrl := adminctrl.NewFeeConfigsController(deps.FeeConfigService)
	adminAdminsCtrl := adminctrl.NewAdminsController(deps.AdminService)

	// Secured legacy /events endpoint (moved behind audit.read)
	v1.GET(
		"/events",
		middleware.RequireAdminPermission(deps.JWTManager, deps.AdminRepository, enums.PermAuditRead),
		adminAuditCtrl.ListEvents,
	)

	admin := v1.Group("/admin")
	{
		// 1. Audit Log & CSV Export
		events := admin.Group("/events", middleware.RequireAdminPermission(deps.JWTManager, deps.AdminRepository, enums.PermAuditRead))
		{
			events.GET("", adminAuditCtrl.ListEvents)
			events.GET("/export", adminAuditCtrl.ExportCSV)
		}

		// 2. Reservation Management & Financial Adjustments
		reservations := admin.Group("/reservations")
		{
			reservations.GET(
				"",
				middleware.RequireAdminPermission(deps.JWTManager, deps.AdminRepository, enums.PermReservationsRead),
				adminResCtrl.ListReservations,
			)
			reservations.GET(
				"/:id",
				middleware.RequireAdminPermission(deps.JWTManager, deps.AdminRepository, enums.PermReservationsRead),
				adminResCtrl.GetReservationDetail,
			)
			reservations.POST(
				"/:id/cancel",
				middleware.RequireAdminPermission(deps.JWTManager, deps.AdminRepository, enums.PermReservationsCancel),
				middleware.RequireIdempotency(deps.DB, deps.Redis),
				adminResCtrl.CancelReservation,
			)
			reservations.POST(
				"/:id/refund",
				middleware.RequireAdminPermission(deps.JWTManager, deps.AdminRepository, enums.PermReservationsRefund),
				middleware.RequireIdempotency(deps.DB, deps.Redis),
				adminResCtrl.RefundReservation,
			)
		}

		// 3. Host Payout Controls (Hold / Release)
		payouts := admin.Group("/payouts", middleware.RequireAdminPermission(deps.JWTManager, deps.AdminRepository, enums.PermPayoutsHoldRelease))
		{
			payouts.POST(
				"/:id/hold",
				middleware.RequireIdempotency(deps.DB, deps.Redis),
				adminPayoutCtrl.HoldPayout,
			)
			payouts.POST(
				"/:id/release",
				middleware.RequireIdempotency(deps.DB, deps.Redis),
				adminPayoutCtrl.ReleasePayout,
			)
		}

		// 4. Listing Review Queue, Diffs & Decisions
		reviews := admin.Group("/listing-reviews", middleware.RequireAdminPermission(deps.JWTManager, deps.AdminRepository, enums.PermListingsReview))
		{
			reviews.GET("", adminReviewCtrl.ListQueue)
			reviews.GET("/:versionId/diff", adminReviewCtrl.GetDiff)
			reviews.POST(
				"/:versionId/approve",
				middleware.RequireIdempotency(deps.DB, deps.Redis),
				adminReviewCtrl.Approve,
			)
			reviews.POST(
				"/:versionId/request-changes",
				middleware.RequireIdempotency(deps.DB, deps.Redis),
				adminReviewCtrl.RequestChanges,
			)
			reviews.POST(
				"/:versionId/reject",
				middleware.RequireIdempotency(deps.DB, deps.Redis),
				adminReviewCtrl.Reject,
			)
		}

		// 5. Fee Configuration (Propose & Two-Person Approval)
		feeConfigs := admin.Group("/fee-configs")
		{
			feeConfigs.GET(
				"",
				middleware.RequireAdminPermission(deps.JWTManager, deps.AdminRepository, enums.PermFeesPropose),
				adminFeeCtrl.ListFeeConfigs,
			)
			feeConfigs.POST(
				"",
				middleware.RequireAdminPermission(deps.JWTManager, deps.AdminRepository, enums.PermFeesPropose),
				middleware.RequireIdempotency(deps.DB, deps.Redis),
				adminFeeCtrl.ProposeFeeConfig,
			)
			feeConfigs.POST(
				"/:id/approve",
				middleware.RequireAdminPermission(deps.JWTManager, deps.AdminRepository, enums.PermFeesApprove),
				middleware.RequireIdempotency(deps.DB, deps.Redis),
				adminFeeCtrl.ApproveFeeConfig,
			)
			feeConfigs.POST(
				"/:id/reject",
				middleware.RequireAdminPermission(deps.JWTManager, deps.AdminRepository, enums.PermFeesApprove),
				middleware.RequireIdempotency(deps.DB, deps.Redis),
				adminFeeCtrl.RejectFeeConfig,
			)
		}

		// 6. Admin Roles & Permissions Management
		admins := admin.Group("/admins", middleware.RequireAdminPermission(deps.JWTManager, deps.AdminRepository, enums.PermAdminsManage))
		{
			admins.GET("", adminAdminsCtrl.ListAdmins)
			admins.POST("", adminAdminsCtrl.CreateAdmin)
			admins.PATCH("/:id", adminAdminsCtrl.UpdateAdmin)
		}
	}
}
