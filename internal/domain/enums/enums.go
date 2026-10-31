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
	PaymentKindCharge           PaymentKind = "charge"
	PaymentKindRefund           PaymentKind = "refund"
	PaymentKindDateChangeCharge PaymentKind = "date_change_charge"
	PaymentKindDateChangeRefund PaymentKind = "date_change_refund"
	PaymentKindCaseRefund       PaymentKind = "case_refund"
)

type AdminRole string

const (
	AdminRoleSupportL1      AdminRole = "support_l1"
	AdminRoleSupportL2      AdminRole = "support_l2"
	AdminRoleTrustSafety    AdminRole = "trust_safety"
	AdminRoleFinance        AdminRole = "finance"
	AdminRoleCatalogManager AdminRole = "catalog_manager"
	AdminRoleSuperAdmin     AdminRole = "super_admin"
)

type FeeConfigStatus string

const (
	FeeConfigStatusPendingApproval FeeConfigStatus = "pending_approval"
	FeeConfigStatusActive          FeeConfigStatus = "active"
	FeeConfigStatusRejected        FeeConfigStatus = "rejected"
	FeeConfigStatusSuperseded      FeeConfigStatus = "superseded"
)

const (
	PermReservationsRead   = "reservations.read"
	PermReservationsCancel = "reservations.cancel"
	PermReservationsRefund = "reservations.refund"
	PermPayoutsHoldRelease = "payouts.hold_release"
	PermAuditRead          = "audit.read"
	PermListingsReview     = "listings.review"
	PermCasesHandle        = "cases.handle"
	PermCasesDecide        = "cases.decide"
	PermFeesPropose        = "fees.propose"
	PermFeesApprove        = "fees.approve"
	PermAdminsManage       = "admins.manage"
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

type RateType string

const (
	RateTypeBase     RateType = "base"
	RateTypeWeekend  RateType = "weekend"
	RateTypeOverride RateType = "override"
)

type CohostInviteStatus string

const (
	CohostInvitePending   CohostInviteStatus = "pending"
	CohostInviteAccepted  CohostInviteStatus = "accepted"
	CohostInviteDeclined  CohostInviteStatus = "declined"
	CohostInviteCancelled CohostInviteStatus = "cancelled"
	CohostInviteExpired   CohostInviteStatus = "expired"
)

type PayoutSplitStatus string

const (
	PayoutSplitScheduled PayoutSplitStatus = "scheduled"
	PayoutSplitReleased  PayoutSplitStatus = "released"
	PayoutSplitCancelled PayoutSplitStatus = "cancelled"
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

type ReviewTargetType string

const (
	ReviewTargetListing ReviewTargetType = "listing"
	ReviewTargetGuest   ReviewTargetType = "guest"
)

type AlterationStatus string

const (
	AlterationStatusPending   AlterationStatus = "pending"
	AlterationStatusApproved  AlterationStatus = "approved"
	AlterationStatusDeclined  AlterationStatus = "declined"
	AlterationStatusExpired   AlterationStatus = "expired"
	AlterationStatusCancelled AlterationStatus = "cancelled"
)
