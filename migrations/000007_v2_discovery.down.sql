-- Phase 4: Discovery Schema Changes (Down)

DROP TABLE IF EXISTS wishlist_items;
DROP TABLE IF EXISTS wishlists;

ALTER TABLE users
    DROP COLUMN IF EXISTS status,
    DROP COLUMN IF EXISTS phone,
    DROP COLUMN IF EXISTS hosting_since,
    DROP COLUMN IF EXISTS languages,
    DROP COLUMN IF EXISTS bio,
    DROP COLUMN IF EXISTS is_superhost;

ALTER TABLE listings
    DROP COLUMN IF EXISTS is_guest_favorite;

ALTER TABLE listing_versions
    DROP COLUMN IF EXISTS bathrooms,
    DROP COLUMN IF EXISTS beds,
    DROP COLUMN IF EXISTS bedrooms,
    DROP COLUMN IF EXISTS longitude,
    DROP COLUMN IF EXISTS latitude;
