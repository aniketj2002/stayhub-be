ALTER TABLE listing_versions
  DROP COLUMN IF EXISTS decision_note,
  DROP COLUMN IF EXISTS decided_by;
