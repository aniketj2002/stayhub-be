package dtos

import (
	"time"

	"github.com/google/uuid"
)

type CohostPermissions struct {
	CanManageListing      bool `json:"can_manage_listing"`
	CanManageCalendar     bool `json:"can_manage_calendar"`
	CanManageReservations bool `json:"can_manage_reservations"`
	CanMessageGuests      bool `json:"can_message_guests"`
	CanViewInsights       bool `json:"can_view_insights"`
}

type CreateCohostInviteInput struct {
	Email          string            `json:"email"`
	Permissions    CohostPermissions `json:"permissions"`
	PayoutShareBPS int               `json:"payout_share_bps"`
}

type CohostInviteResponse struct {
	ID             uuid.UUID         `json:"id"`
	ListingID      uuid.UUID         `json:"listing_id"`
	Email          string            `json:"email"`
	Permissions    CohostPermissions `json:"permissions"`
	PayoutShareBPS int               `json:"payout_share_bps"`
	Status         string            `json:"status"`
	ExpiresAt      time.Time         `json:"expires_at"`
	CreatedAt      time.Time         `json:"created_at"`
}

type UpdateCohostInput struct {
	Permissions    *CohostPermissions `json:"permissions,omitempty"`
	PayoutShareBPS *int               `json:"payout_share_bps,omitempty"`
}

type CohostMemberResponse struct {
	UserID         uuid.UUID         `json:"user_id"`
	FullName       string            `json:"full_name"`
	Email          string            `json:"email"`
	Role           string            `json:"role"` // "owner" or "cohost"
	Permissions    CohostPermissions `json:"permissions"`
	PayoutShareBPS int               `json:"payout_share_bps"`
	CreatedAt      time.Time         `json:"created_at"`
}

type AcceptCohostInviteResponse struct {
	ListingID uuid.UUID `json:"listing_id"`
	Status    string    `json:"status"`
}

type CohostInvitationDetailsResponse struct {
	ID             uuid.UUID         `json:"id"`
	ListingID      uuid.UUID         `json:"listing_id"`
	ListingTitle   string            `json:"listing_title"`
	HostName       string            `json:"host_name"`
	InvitedEmail   string            `json:"invited_email"`
	Permissions    CohostPermissions `json:"permissions"`
	PayoutShareBPS int               `json:"payout_share_bps"`
	Status         string            `json:"status"`
	ExpiresAt      time.Time         `json:"expires_at"`
}
