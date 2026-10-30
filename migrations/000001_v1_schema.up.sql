-- =====================================================================
-- StayHub — VERSION 1 schema (PostgreSQL 15+)
--
-- Forward-compat rule: V2 may ADD tables, ADD nullable/defaulted columns and
-- ADD enum values. It must never rename, retype, drop or tighten anything
-- below.
--
-- Conventions
--   * ids: uuid; money: bigint minor units (EUR cents) + char(3) currency
--   * rates: basis points (14% = 1400); no floats in money math
--   * the enums below hold the full vocabulary from the business rules, so
--     V2 does not need to touch them (except payment_kind: ADD VALUE later)
-- =====================================================================

CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TYPE reservation_status AS ENUM (
  'inquiry','pending','confirmed','checked_in','completed','reviewed',
  'declined','cancelled_by_guest','cancelled_by_host','disputed');
CREATE TYPE listing_status    AS ENUM ('draft','pending_review','live','paused','rejected','archived');
CREATE TYPE version_status    AS ENUM ('draft','submitted','approved','changes_requested','rejected','superseded');
CREATE TYPE calendar_status   AS ENUM ('blocked','booked');
CREATE TYPE payment_kind      AS ENUM ('charge','refund');            -- V2: ALTER TYPE ADD VALUE
CREATE TYPE payment_status    AS ENUM ('pending','succeeded','failed');
CREATE TYPE payout_status     AS ENUM ('scheduled','held','released','failed','cancelled');
CREATE TYPE actor_type        AS ENUM ('guest','host','cohost','admin','system');
CREATE TYPE listing_host_role AS ENUM ('owner','cohost');

-- ---------------------------------------------------------------------
-- ACCOUNTS
-- ---------------------------------------------------------------------
CREATE TABLE users (                        -- guests and hosts; one account can be both
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email           citext NOT NULL UNIQUE,
  password_hash   text   NOT NULL,          -- or the identity-provider subject if using Cognito
  full_name       text   NOT NULL,
  avatar_media_id uuid,                     -- FK to media added below (circular); shown on listing detail as host
  is_guest        boolean NOT NULL DEFAULT true,
  is_host         boolean NOT NULL DEFAULT false,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  CHECK (is_guest OR is_host)
);

CREATE TABLE admin_users (                  -- separate login, separate table
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email         citext NOT NULL UNIQUE,
  password_hash text NOT NULL,
  full_name     text NOT NULL,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE refresh_tokens (               -- omit if the identity provider owns sessions
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  subject_type text NOT NULL CHECK (subject_type IN ('user','admin')),
  subject_id   uuid NOT NULL,
  token_hash   text NOT NULL UNIQUE,
  expires_at   timestamptz NOT NULL,
  revoked_at   timestamptz,
  created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX refresh_tokens_subject_idx ON refresh_tokens(subject_type, subject_id);

-- ---------------------------------------------------------------------
-- MEDIA — every uploaded file in the system lives here (listing photos, avatars,
-- and later message attachments / case evidence). Other tables store media ids.
-- ---------------------------------------------------------------------
CREATE TABLE media (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  s3_key       text NOT NULL UNIQUE,
  content_type text NOT NULL,                 -- 'image/jpeg', ...
  size_bytes   bigint NOT NULL CHECK (size_bytes > 0),
  width        int,
  height       int,
  status       text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','uploaded')),
                                              -- pending = presigned URL issued, upload not confirmed
  uploaded_by  uuid NOT NULL,                 -- users.id or admin_users.id (uuids are globally unique, so
                                              -- no type column; no FK because it spans two tables)
  created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX media_uploader_idx ON media(uploaded_by);

ALTER TABLE users
  ADD CONSTRAINT users_avatar_fk FOREIGN KEY (avatar_media_id) REFERENCES media(id);

-- ---------------------------------------------------------------------
-- REFERENCE DATA
-- ---------------------------------------------------------------------
CREATE TABLE amenities (                    -- catalog for labels/categories; listing_versions.amenities holds codes
  code     text PRIMARY KEY,
  label    text NOT NULL,
  category text NOT NULL
);

CREATE TABLE cancellation_policies (        -- flexible, moderate, firm, strict
  code        text PRIMARY KEY,
  name        text NOT NULL,
  description text NOT NULL
);

CREATE TABLE cancellation_policy_rules (    -- refund % by exact hours before check-in instant
  policy_code         text NOT NULL REFERENCES cancellation_policies(code),
  min_hours_before    int  NOT NULL,        -- applies when hours_before >= this
  refund_pct          int  NOT NULL CHECK (refund_pct BETWEEN 0 AND 100),
  refunds_service_fee boolean NOT NULL DEFAULT false,
  PRIMARY KEY (policy_code, min_hours_before)
);

-- ---------------------------------------------------------------------
-- LISTINGS (identity + versioned content + per-listing host/cohost permissions)
-- Content lives in listing_versions from day one, so V2's "live version stays
-- until the edit is approved" needs no restructuring. In V1 a version that
-- passes the automated checks is auto-approved by the system.
-- ---------------------------------------------------------------------
CREATE TABLE listings (
  id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  host_id            uuid NOT NULL REFERENCES users(id),    -- primary owner
  status             listing_status NOT NULL DEFAULT 'draft',
  live_version_id    uuid,                                  -- FK below (circular)
  working_version_id uuid,                                  -- draft / edit in progress
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX listings_host_idx   ON listings(host_id);
CREATE INDEX listings_status_idx ON listings(status);

-- Per-listing host and co-host permissions + separate per-listing configuration
CREATE TABLE listing_hosts (
  id                      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  listing_id              uuid NOT NULL REFERENCES listings(id) ON DELETE CASCADE,
  user_id                 uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role                    listing_host_role NOT NULL,
  -- Scoped permissions per listing (Spec p. 2 & p. 5)
  can_manage_listing      boolean NOT NULL DEFAULT false,   -- edit versions, photos, pricing, submit
  can_manage_calendar     boolean NOT NULL DEFAULT false,   -- block/unblock dates, iCal
  can_manage_reservations boolean NOT NULL DEFAULT false,   -- accept/decline requests, cancel, check-in
  can_message_guests      boolean NOT NULL DEFAULT false,   -- reservation messaging thread (V2)
  can_view_insights       boolean NOT NULL DEFAULT false,   -- earnings, payouts & insights for this listing
  -- Payout split & per-listing host configuration
  payout_share_bps        int     NOT NULL DEFAULT 0 CHECK (payout_share_bps BETWEEN 0 AND 10000),
  config                  jsonb   NOT NULL DEFAULT '{}'::jsonb,
  invited_by              uuid REFERENCES users(id),
  created_at              timestamptz NOT NULL DEFAULT now(),
  updated_at              timestamptz NOT NULL DEFAULT now(),
  UNIQUE (listing_id, user_id)
);
CREATE INDEX lh_user_idx    ON listing_hosts(user_id);
CREATE INDEX lh_listing_idx ON listing_hosts(listing_id);
CREATE UNIQUE INDEX lh_one_owner_per_listing ON listing_hosts(listing_id) WHERE role = 'owner';

CREATE TABLE listing_versions (
  id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  listing_id          uuid NOT NULL REFERENCES listings(id) ON DELETE CASCADE,
  version_no          int  NOT NULL,
  status              version_status NOT NULL DEFAULT 'draft',
  wizard_step         int  NOT NULL DEFAULT 1,               -- draft is saved between steps
  -- content (nullable while draft; completeness enforced at submit)
  property_type       text,
  title               text,
  description         text,
  address_line1       text,
  address_line2       text,
  city                text,
  region              text,
  country             char(2),
  postal_code         text,
  timezone            text NOT NULL DEFAULT 'UTC',           -- IANA, e.g. 'Europe/Lisbon'
  max_guests          int  CHECK (max_guests > 0),
  base_price_minor    bigint CHECK (base_price_minor >= 0),
  cleaning_fee_minor  bigint NOT NULL DEFAULT 0 CHECK (cleaning_fee_minor >= 0),
  currency            char(3) NOT NULL DEFAULT 'EUR',
  instant_book        boolean NOT NULL DEFAULT false,
  check_in_time       time NOT NULL DEFAULT '15:00',         -- local to listing timezone
  check_out_time      time NOT NULL DEFAULT '11:00',
  house_rules         text,
  cancellation_policy text REFERENCES cancellation_policies(code),
  amenities           text[] NOT NULL DEFAULT '{}',          -- codes from amenities.code; validated in API
  media_ids           uuid[] NOT NULL DEFAULT '{}',          -- ordered; first = cover. Validated in API: >= 5 at
                                                             -- submit, each id exists, status='uploaded', and
                                                             -- was uploaded by an authorized listing host
  submitted_at        timestamptz,
  decided_at          timestamptz,
  created_at          timestamptz NOT NULL DEFAULT now(),
  updated_at          timestamptz NOT NULL DEFAULT now(),
  UNIQUE (listing_id, version_no)
);
CREATE INDEX lv_city_idx      ON listing_versions(city, country);
CREATE INDEX lv_amenities_idx ON listing_versions USING GIN (amenities);

ALTER TABLE listings
  ADD CONSTRAINT listings_live_fk    FOREIGN KEY (live_version_id)    REFERENCES listing_versions(id),
  ADD CONSTRAINT listings_working_fk FOREIGN KEY (working_version_id) REFERENCES listing_versions(id);

CREATE TABLE listing_check_results (        -- automated checks run on submit
  id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  listing_version_id uuid NOT NULL REFERENCES listing_versions(id) ON DELETE CASCADE,
  check_code         text NOT NULL,         -- 'min_photos','house_rules_policy','required_fields'
  outcome            text NOT NULL CHECK (outcome IN ('pass','warn','fail')),
  detail             text,
  created_at         timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX lcr_version_idx ON listing_check_results(listing_version_id);

-- ---------------------------------------------------------------------
-- CALENDAR — primary key = no double booking, no booking over a block
-- available night = no row. Pending requests hold nights via 'booked' rows,
-- released on decline / expiry / cancellation.
-- ---------------------------------------------------------------------
CREATE TABLE listing_calendar (
  listing_id     uuid NOT NULL REFERENCES listings(id) ON DELETE CASCADE,
  night          date NOT NULL,
  status         calendar_status NOT NULL,
  reservation_id uuid,                      -- FK below; set iff booked
  block_reason   text,
  created_at     timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (listing_id, night),
  CHECK ((status = 'booked') = (reservation_id IS NOT NULL))
);
CREATE INDEX cal_reservation_idx ON listing_calendar(reservation_id);

-- ---------------------------------------------------------------------
-- RESERVATIONS
-- Everything needed to price, refund and pay out is snapshotted here, so later
-- listing edits or fee changes never alter an existing booking.
-- nightly_total_minor is the NET accommodation subtotal (after any discounts).
-- ---------------------------------------------------------------------
CREATE TABLE reservations (
  id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code                text NOT NULL UNIQUE,                 -- 'SH-7K3QZ9'
  listing_id          uuid NOT NULL REFERENCES listings(id),
  listing_version_id  uuid NOT NULL REFERENCES listing_versions(id),
  guest_id            uuid NOT NULL REFERENCES users(id),
  host_id             uuid NOT NULL REFERENCES users(id),   -- owner at booking time
  status              reservation_status NOT NULL,
  is_instant_book     boolean NOT NULL,
  check_in            date NOT NULL,                        -- booking start date (first night)
  check_out           date NOT NULL,                        -- booking end date (departure day)
  check_in_time       time NOT NULL,                        -- snapshot of listing check-in time (local)
  check_out_time      time NOT NULL,                        -- snapshot of listing check-out time (local)
  timezone            text NOT NULL,                        -- snapshot of listing IANA timezone
  check_in_at         timestamptz NOT NULL,                 -- check_in + check_in_time in timezone = exact start instant
                                                            -- (this is the refund-policy clock)
  check_out_at        timestamptz NOT NULL,                 -- check_out + check_out_time in timezone = exact end instant
  guests              int  NOT NULL CHECK (guests > 0),
  guest_message       text,
  -- money snapshot
  currency            char(3) NOT NULL DEFAULT 'EUR',
  nightly_total_minor bigint NOT NULL CHECK (nightly_total_minor >= 0),
  cleaning_fee_minor  bigint NOT NULL CHECK (cleaning_fee_minor >= 0),
  guest_fee_bps       int    NOT NULL,
  guest_fee_minor     bigint NOT NULL,
  guest_total_minor   bigint NOT NULL,
  host_fee_bps        int    NOT NULL,
  host_fee_minor      bigint NOT NULL,
  host_net_minor      bigint NOT NULL,
  cancellation_policy text   NOT NULL REFERENCES cancellation_policies(code),
  -- lifecycle
  request_expires_at  timestamptz,                          -- 24h host deadline while pending
  decided_at          timestamptz,
  decline_reason      text,
  cancelled_at        timestamptz,
  cancelled_by        actor_type,
  cancel_reason       text,
  checked_in_at       timestamptz,
  completed_at        timestamptz,
  version             int NOT NULL DEFAULT 1,               -- optimistic lock
  created_at          timestamptz NOT NULL DEFAULT now(),
  updated_at          timestamptz NOT NULL DEFAULT now(),
  CHECK (check_out > check_in),
  CHECK (check_out_at > check_in_at),
  CHECK (guest_total_minor = nightly_total_minor + cleaning_fee_minor + guest_fee_minor),
  CHECK (host_net_minor    = nightly_total_minor + cleaning_fee_minor - host_fee_minor)
);
CREATE INDEX res_guest_idx   ON reservations(guest_id, check_in);
CREATE INDEX res_host_idx    ON reservations(host_id, check_in);
CREATE INDEX res_listing_idx ON reservations(listing_id, check_in);
CREATE INDEX res_status_idx  ON reservations(status);
CREATE INDEX res_expiry_idx  ON reservations(request_expires_at) WHERE status = 'pending';

ALTER TABLE listing_calendar
  ADD CONSTRAINT cal_res_fk FOREIGN KEY (reservation_id) REFERENCES reservations(id) ON DELETE CASCADE;

-- ---------------------------------------------------------------------
-- MONEY (all simulated)
-- ---------------------------------------------------------------------
CREATE TABLE payments (                     -- guest-side money: charge and refunds
  id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  reservation_id     uuid NOT NULL REFERENCES reservations(id),
  kind               payment_kind NOT NULL,
  status             payment_status NOT NULL DEFAULT 'pending',
  amount_minor       bigint NOT NULL CHECK (amount_minor > 0),   -- direction implied by kind
  currency           char(3) NOT NULL,
  idempotency_key    text NOT NULL UNIQUE,
  simulated_ref      text,
  related_payment_id uuid REFERENCES payments(id),               -- refund -> original charge
  refund_pct         int,
  refund_basis       jsonb,                                      -- hours_before, matched rule, inputs
  failure_reason     text,
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX pay_res_idx ON payments(reservation_id);
CREATE UNIQUE INDEX one_charge_per_res ON payments(reservation_id)
  WHERE kind = 'charge' AND status IN ('pending','succeeded');

CREATE TABLE payouts (                      -- host-side money, one per reservation
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  reservation_id  uuid NOT NULL UNIQUE REFERENCES reservations(id),
  host_id         uuid NOT NULL REFERENCES users(id),
  status          payout_status NOT NULL DEFAULT 'scheduled',
  gross_minor     bigint NOT NULL,
  host_fee_minor  bigint NOT NULL,
  net_minor       bigint NOT NULL,
  currency        char(3) NOT NULL,
  release_at      timestamptz,                -- checked_in_at + 24h; NULL until check-in
  released_at     timestamptz,
  held_reason     text,
  idempotency_key text NOT NULL UNIQUE,
  simulated_ref   text,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  CHECK (net_minor = gross_minor - host_fee_minor)
);
CREATE INDEX payouts_due_idx  ON payouts(release_at) WHERE status = 'scheduled';
CREATE INDEX payouts_host_idx ON payouts(host_id, status);

CREATE TABLE ledger_entries (               -- append-only; each txn_id must balance debit = credit
  id             bigserial PRIMARY KEY,
  txn_id         uuid NOT NULL,
  reservation_id uuid REFERENCES reservations(id),
  payment_id     uuid REFERENCES payments(id),
  payout_id      uuid REFERENCES payouts(id),
  account        text NOT NULL,               -- 'guest_cash','platform_guest_fee','platform_host_fee',
                                              -- 'host_payable','host_paid','refunds' (no CHECK: V2 may add)
  direction      text NOT NULL CHECK (direction IN ('debit','credit')),
  amount_minor   bigint NOT NULL CHECK (amount_minor > 0),
  currency       char(3) NOT NULL,
  created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ledger_txn_idx ON ledger_entries(txn_id);
CREATE INDEX ledger_res_idx ON ledger_entries(reservation_id);

CREATE TABLE idempotency_keys (             -- API-level replay protection
  scope         text NOT NULL,              -- e.g. 'POST /api/v1/reservations'
  key           text NOT NULL,
  subject_id    uuid,
  request_hash  text NOT NULL,
  response_code int,
  response_body jsonb,
  created_at    timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (scope, key)
);

-- ---------------------------------------------------------------------
-- EVENT LOG (append-only; V2 audit screen is just a view over this)
-- ---------------------------------------------------------------------
CREATE TABLE event_log (
  id           bigserial PRIMARY KEY,
  occurred_at  timestamptz NOT NULL DEFAULT now(),
  actor_type   actor_type NOT NULL,
  actor_id     uuid,                         -- NULL for system
  action       text NOT NULL,                -- 'reservation.transition','payment.charge','payout.release'...
  object_type  text NOT NULL,                -- 'reservation','listing','payment','payout'
  object_id    uuid NOT NULL,
  before_state jsonb,
  after_state  jsonb,
  reason       text,
  request_id   text
);
CREATE INDEX evt_object_idx ON event_log(object_type, object_id, occurred_at);
CREATE INDEX evt_actor_idx  ON event_log(actor_type, actor_id, occurred_at);
CREATE INDEX evt_action_idx ON event_log(action, occurred_at);
