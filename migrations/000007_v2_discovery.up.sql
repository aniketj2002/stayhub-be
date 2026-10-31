-- Phase 4: Discovery Schema Changes (Up)

-- 1. Listing Version specs & location
ALTER TABLE listing_versions
    ADD COLUMN IF NOT EXISTS latitude DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS longitude DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS bedrooms INT DEFAULT 1,
    ADD COLUMN IF NOT EXISTS beds INT DEFAULT 1,
    ADD COLUMN IF NOT EXISTS bathrooms NUMERIC(3, 1) DEFAULT 1.0;

-- 2. Listing badges
ALTER TABLE listings
    ADD COLUMN IF NOT EXISTS is_guest_favorite BOOLEAN NOT NULL DEFAULT false;

-- 3. Host Profile metadata & badges
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS is_superhost BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS bio TEXT,
    ADD COLUMN IF NOT EXISTS languages TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS hosting_since TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS phone TEXT,
    ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active';

-- 4. Wishlists & Wishlist items
CREATE TABLE IF NOT EXISTS wishlists (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    is_shared   BOOLEAN NOT NULL DEFAULT false,
    share_token TEXT UNIQUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS wishlist_items (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    wishlist_id UUID NOT NULL REFERENCES wishlists(id) ON DELETE CASCADE,
    listing_id  UUID NOT NULL REFERENCES listings(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(wishlist_id, listing_id)
);

-- 5. Indexes for fast discovery & geo queries
CREATE INDEX IF NOT EXISTS lv_geo_idx ON listing_versions(latitude, longitude);
CREATE INDEX IF NOT EXISTS lv_price_idx ON listing_versions(base_price_minor);
CREATE INDEX IF NOT EXISTS listings_guest_fav_idx ON listings(is_guest_favorite);
CREATE INDEX IF NOT EXISTS users_superhost_idx ON users(is_superhost);
CREATE INDEX IF NOT EXISTS wishlists_user_idx ON wishlists(user_id);
CREATE UNIQUE INDEX IF NOT EXISTS wishlists_share_token_idx ON wishlists(share_token) WHERE share_token IS NOT NULL;
CREATE INDEX IF NOT EXISTS wishlist_items_wishlist_idx ON wishlist_items(wishlist_id);
CREATE INDEX IF NOT EXISTS wishlist_items_listing_idx ON wishlist_items(listing_id);
