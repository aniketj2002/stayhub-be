-- ---------------------------------------------------------------------
-- Phase 3: Guest-Host Interaction (Messaging, Reviews, Itinerary & Reservation Alterations)
-- ---------------------------------------------------------------------

-- 1. Conversations & Messaging
CREATE TABLE IF NOT EXISTS conversations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reservation_id  UUID NOT NULL UNIQUE REFERENCES reservations(id) ON DELETE CASCADE,
    listing_id      UUID NOT NULL REFERENCES listings(id) ON DELETE CASCADE,
    guest_id        UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS conversations_res_idx ON conversations(reservation_id);
CREATE INDEX IF NOT EXISTS conversations_guest_idx ON conversations(guest_id);
CREATE INDEX IF NOT EXISTS conversations_listing_idx ON conversations(listing_id);

CREATE TABLE IF NOT EXISTS messages (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    sender_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    body            TEXT NOT NULL,
    media_ids       UUID[] NOT NULL DEFAULT '{}',
    flags           TEXT[] NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS messages_conv_created_idx ON messages(conversation_id, created_at DESC);
CREATE INDEX IF NOT EXISTS messages_sender_idx ON messages(sender_id);

CREATE TABLE IF NOT EXISTS conversation_reads (
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    last_read_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (conversation_id, user_id)
);

-- 2. Reviews (Double-Blind)
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'review_target_type') THEN
        CREATE TYPE review_target_type AS ENUM ('listing', 'guest');
    END IF;
END$$;

CREATE TABLE IF NOT EXISTS reviews (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reservation_id        UUID NOT NULL REFERENCES reservations(id) ON DELETE CASCADE,
    author_id             UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_type           review_target_type NOT NULL,
    listing_id            UUID NOT NULL REFERENCES listings(id) ON DELETE CASCADE,
    target_user_id        UUID REFERENCES users(id) ON DELETE CASCADE,
    rating                INT NOT NULL CHECK (rating >= 1 AND rating <= 5),
    cleanliness_rating    INT CHECK (cleanliness_rating >= 1 AND cleanliness_rating <= 5),
    accuracy_rating       INT CHECK (accuracy_rating >= 1 AND accuracy_rating <= 5),
    check_in_rating       INT CHECK (check_in_rating >= 1 AND check_in_rating <= 5),
    communication_rating  INT CHECK (communication_rating >= 1 AND communication_rating <= 5),
    location_rating       INT CHECK (location_rating >= 1 AND location_rating <= 5),
    value_rating          INT CHECK (value_rating >= 1 AND value_rating <= 5),
    public_review         TEXT NOT NULL,
    private_note          TEXT,
    published_at          TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT unique_reservation_author UNIQUE (reservation_id, author_id)
);
CREATE INDEX IF NOT EXISTS reviews_res_idx ON reviews(reservation_id);
CREATE INDEX IF NOT EXISTS reviews_listing_pub_idx ON reviews(listing_id, published_at) WHERE published_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS reviews_target_user_pub_idx ON reviews(target_user_id, published_at) WHERE published_at IS NOT NULL;

-- 3. Scalable Reservation Alterations Resource (Dates, Guests, Configuration & Addons)
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'alteration_status') THEN
        CREATE TYPE alteration_status AS ENUM ('pending', 'approved', 'declined', 'expired', 'cancelled');
    END IF;
END$$;

CREATE TABLE IF NOT EXISTS reservation_alterations (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reservation_id     UUID NOT NULL REFERENCES reservations(id) ON DELETE CASCADE,
    requested_by       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    current_check_in   DATE NOT NULL,
    current_check_out  DATE NOT NULL,
    new_check_in       DATE NOT NULL,
    new_check_out      DATE NOT NULL,
    
    original_snapshot  JSONB NOT NULL DEFAULT '{}'::jsonb,
    requested_changes  JSONB NOT NULL DEFAULT '{}'::jsonb,
    
    delta_minor        BIGINT NOT NULL,
    currency           CHAR(3) NOT NULL DEFAULT 'EUR',
    status             alteration_status NOT NULL DEFAULT 'pending',
    
    expires_at         TIMESTAMPTZ NOT NULL,
    decided_at         TIMESTAMPTZ,
    decision_reason    TEXT,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (new_check_out > new_check_in)
);
CREATE INDEX IF NOT EXISTS alterations_res_idx ON reservation_alterations(reservation_id);
CREATE INDEX IF NOT EXISTS alterations_expiry_idx ON reservation_alterations(expires_at) WHERE status = 'pending';

-- 4. Reservation & Listing review metadata columns
ALTER TABLE reservations ADD COLUMN IF NOT EXISTS review_deadline TIMESTAMPTZ;

ALTER TABLE listings ADD COLUMN IF NOT EXISTS rating_avg NUMERIC(3,2) NOT NULL DEFAULT 0.00;
ALTER TABLE listings ADD COLUMN IF NOT EXISTS review_count INT NOT NULL DEFAULT 0;
ALTER TABLE listings ADD COLUMN IF NOT EXISTS rating_cleanliness NUMERIC(3,2);
ALTER TABLE listings ADD COLUMN IF NOT EXISTS rating_accuracy NUMERIC(3,2);
ALTER TABLE listings ADD COLUMN IF NOT EXISTS rating_checkin NUMERIC(3,2);
ALTER TABLE listings ADD COLUMN IF NOT EXISTS rating_communication NUMERIC(3,2);
ALTER TABLE listings ADD COLUMN IF NOT EXISTS rating_location NUMERIC(3,2);
ALTER TABLE listings ADD COLUMN IF NOT EXISTS rating_value NUMERIC(3,2);
