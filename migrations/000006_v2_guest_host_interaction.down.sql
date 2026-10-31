-- Revert Phase 3

ALTER TABLE listings DROP COLUMN IF EXISTS rating_value;
ALTER TABLE listings DROP COLUMN IF EXISTS rating_location;
ALTER TABLE listings DROP COLUMN IF EXISTS rating_communication;
ALTER TABLE listings DROP COLUMN IF EXISTS rating_checkin;
ALTER TABLE listings DROP COLUMN IF EXISTS rating_accuracy;
ALTER TABLE listings DROP COLUMN IF EXISTS rating_cleanliness;
ALTER TABLE listings DROP COLUMN IF EXISTS review_count;
ALTER TABLE listings DROP COLUMN IF EXISTS rating_avg;

ALTER TABLE reservations DROP COLUMN IF EXISTS review_deadline;

DROP TABLE IF EXISTS reservation_alterations;
DROP TYPE IF EXISTS alteration_status;

DROP TABLE IF EXISTS reviews;
DROP TYPE IF EXISTS review_target_type;

DROP TABLE IF EXISTS conversation_reads;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS conversations;
