package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/enums"
)

// CohostInvitation maps to `cohost_invitations`.
type CohostInvitation struct {
	ID             uuid.UUID                `db:"id" json:"id"`
	ListingID      uuid.UUID                `db:"listing_id" json:"listing_id"`
	InviterUserID  uuid.UUID                `db:"inviter_user_id" json:"inviter_user_id"`
	Email          string                   `db:"email" json:"email"`
	TokenHash      string                   `db:"token_hash" json:"-"`
	Permissions    json.RawMessage          `db:"permissions" json:"permissions"`
	PayoutShareBPS int                      `db:"payout_share_bps" json:"payout_share_bps"`
	Status         enums.CohostInviteStatus `db:"status" json:"status"`
	ExpiresAt      time.Time                `db:"expires_at" json:"expires_at"`
	CreatedAt      time.Time                `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time                `db:"updated_at" json:"updated_at"`
}

// PayoutSplit maps to `payout_splits`.
type PayoutSplit struct {
	ID          uuid.UUID               `db:"id" json:"id"`
	PayoutID    uuid.UUID               `db:"payout_id" json:"payout_id"`
	UserID      uuid.UUID               `db:"user_id" json:"user_id"`
	ShareBPS    int                     `db:"share_bps" json:"share_bps"`
	AmountMinor int64                   `db:"amount_minor" json:"amount_minor"`
	Status      enums.PayoutSplitStatus `db:"status" json:"status"`
	CreatedAt   time.Time               `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time               `db:"updated_at" json:"updated_at"`
}
