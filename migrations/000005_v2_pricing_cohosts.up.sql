-- 000005_v2_pricing_cohosts.up.sql
-- Phase 2: Host Supply (Pricing Rules, Date Overrides, Reservation Nights, iCal Feeds, Co-hosts, Payout Splits)

CREATE TABLE IF NOT EXISTS listing_pricing_rules (
    listing_id uuid PRIMARY KEY REFERENCES listings(id) ON DELETE CASCADE,
    weekend_price_minor bigint,
    weekend_days smallint[] NOT NULL DEFAULT '{5,6}',
    min_nights int NOT NULL DEFAULT 1 CHECK (min_nights >= 1),
    weekly_discount_bps int NOT NULL DEFAULT 0 CHECK (weekly_discount_bps >= 0 AND weekly_discount_bps <= 10000),
    monthly_discount_bps int NOT NULL DEFAULT 0 CHECK (monthly_discount_bps >= 0 AND monthly_discount_bps <= 10000),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS listing_date_overrides (
    listing_id uuid NOT NULL REFERENCES listings(id) ON DELETE CASCADE,
    night date NOT NULL,
    price_minor bigint NOT NULL CHECK (price_minor >= 0),
    min_nights int CHECK (min_nights IS NULL OR min_nights >= 1),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (listing_id, night)
);

CREATE TABLE IF NOT EXISTS reservation_nights (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    reservation_id uuid NOT NULL REFERENCES reservations(id) ON DELETE CASCADE,
    night date NOT NULL,
    price_minor bigint NOT NULL,
    rate_type text NOT NULL CHECK (rate_type IN ('base', 'weekend', 'override')),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (reservation_id, night)
);
CREATE INDEX IF NOT EXISTS idx_reservation_nights_res ON reservation_nights(reservation_id);

ALTER TABLE reservations
  ADD COLUMN IF NOT EXISTS discount_minor bigint NOT NULL DEFAULT 0;

ALTER TABLE listings
  ADD COLUMN IF NOT EXISTS ical_export_token text UNIQUE;

CREATE TABLE IF NOT EXISTS ical_feeds (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    listing_id uuid NOT NULL REFERENCES listings(id) ON DELETE CASCADE,
    name text NOT NULL,
    url text NOT NULL,
    last_synced_at timestamptz,
    last_sync_error text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_ical_feeds_listing ON ical_feeds(listing_id);

ALTER TABLE listing_calendar
  ADD COLUMN IF NOT EXISTS ical_feed_id uuid REFERENCES ical_feeds(id) ON DELETE SET NULL;

ALTER TABLE listing_hosts
  ADD COLUMN IF NOT EXISTS payout_share_bps int NOT NULL DEFAULT 0 CHECK (payout_share_bps >= 0 AND payout_share_bps <= 10000);

CREATE TABLE IF NOT EXISTS cohost_invitations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    listing_id uuid NOT NULL REFERENCES listings(id) ON DELETE CASCADE,
    inviter_user_id uuid NOT NULL REFERENCES users(id),
    email citext NOT NULL,
    token_hash text NOT NULL UNIQUE,
    permissions jsonb NOT NULL DEFAULT '{}'::jsonb,
    payout_share_bps int NOT NULL DEFAULT 0 CHECK (payout_share_bps >= 0 AND payout_share_bps <= 10000),
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'declined', 'cancelled', 'expired')),
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_cohost_invitations_listing ON cohost_invitations(listing_id);
CREATE INDEX IF NOT EXISTS idx_cohost_invitations_email ON cohost_invitations(email);

CREATE TABLE IF NOT EXISTS payout_splits (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    payout_id uuid NOT NULL REFERENCES payouts(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id),
    share_bps int NOT NULL CHECK (share_bps >= 0 AND share_bps <= 10000),
    amount_minor bigint NOT NULL CHECK (amount_minor >= 0),
    status text NOT NULL DEFAULT 'scheduled' CHECK (status IN ('scheduled', 'released', 'cancelled')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (payout_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_payout_splits_payout ON payout_splits(payout_id);
