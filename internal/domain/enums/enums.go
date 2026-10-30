package enums

type ReservationStatus string

const (
	ReservationStatusInquiry          ReservationStatus = "inquiry"
	ReservationStatusPending          ReservationStatus = "pending"
	ReservationStatusConfirmed        ReservationStatus = "confirmed"
	ReservationStatusCheckedIn        ReservationStatus = "checked_in"
	ReservationStatusCompleted        ReservationStatus = "completed"
	ReservationStatusReviewed         ReservationStatus = "reviewed"
	ReservationStatusDeclined         ReservationStatus = "declined"
	ReservationStatusCancelledByGuest ReservationStatus = "cancelled_by_guest"
	ReservationStatusCancelledByHost  ReservationStatus = "cancelled_by_host"
	ReservationStatusDisputed         ReservationStatus = "disputed"
)

type ListingStatus string

const (
	ListingStatusDraft         ListingStatus = "draft"
	ListingStatusPendingReview ListingStatus = "pending_review"
	ListingStatusLive          ListingStatus = "live"
	ListingStatusPaused        ListingStatus = "paused"
	ListingStatusRejected      ListingStatus = "rejected"
	ListingStatusArchived      ListingStatus = "archived"
)

type VersionStatus string

const (
	VersionStatusDraft            VersionStatus = "draft"
	VersionStatusSubmitted        VersionStatus = "submitted"
	VersionStatusApproved         VersionStatus = "approved"
	VersionStatusChangesRequested VersionStatus = "changes_requested"
	VersionStatusRejected         VersionStatus = "rejected"
	VersionStatusSuperseded       VersionStatus = "superseded"
)

type CalendarStatus string

const (
	CalendarStatusBlocked CalendarStatus = "blocked"
	CalendarStatusBooked  CalendarStatus = "booked"
)

type PaymentKind string

const (
	PaymentKindCharge PaymentKind = "charge"
	PaymentKindRefund PaymentKind = "refund"
)

type PaymentStatus string

const (
	PaymentStatusPending   PaymentStatus = "pending"
	PaymentStatusSucceeded PaymentStatus = "succeeded"
	PaymentStatusFailed    PaymentStatus = "failed"
)

type PayoutStatus string

const (
	PayoutStatusScheduled PayoutStatus = "scheduled"
	PayoutStatusHeld      PayoutStatus = "held"
	PayoutStatusReleased  PayoutStatus = "released"
	PayoutStatusFailed    PayoutStatus = "failed"
	PayoutStatusCancelled PayoutStatus = "cancelled"
)

type ActorType string

const (
	ActorGuest  ActorType = "guest"
	ActorHost   ActorType = "host"
	ActorCohost ActorType = "cohost"
	ActorAdmin  ActorType = "admin"
	ActorSystem ActorType = "system"
)

type ListingHostRole string

const (
	ListingHostRoleOwner  ListingHostRole = "owner"
	ListingHostRoleCohost ListingHostRole = "cohost"
)

type LedgerDirection string

const (
	LedgerDirectionDebit  LedgerDirection = "debit"
	LedgerDirectionCredit LedgerDirection = "credit"
)

const (
	LedgerAccountGuestCash        = "guest_cash"
	LedgerAccountPlatformGuestFee = "platform_guest_fee"
	LedgerAccountPlatformHostFee  = "platform_host_fee"
	LedgerAccountHostPayable      = "host_payable"
	LedgerAccountHostPaid         = "host_paid"
	LedgerAccountRefunds          = "refunds"
)
