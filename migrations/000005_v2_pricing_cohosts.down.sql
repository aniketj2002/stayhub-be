-- 000005_v2_pricing_cohosts.down.sql

DROP TABLE IF EXISTS payout_splits CASCADE;
DROP TABLE IF EXISTS cohost_invitations CASCADE;
ALTER TABLE listing_hosts DROP COLUMN IF EXISTS payout_share_bps;
ALTER TABLE listing_calendar DROP COLUMN IF EXISTS ical_feed_id;
DROP TABLE IF EXISTS ical_feeds CASCADE;
ALTER TABLE listings DROP COLUMN IF EXISTS ical_export_token;
ALTER TABLE reservations DROP COLUMN IF EXISTS discount_minor;
DROP TABLE IF EXISTS reservation_nights CASCADE;
DROP TABLE IF EXISTS listing_date_overrides CASCADE;
DROP TABLE IF EXISTS listing_pricing_rules CASCADE;
