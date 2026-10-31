-- =====================================================================
-- StayHub — Migration 000004: V2 Admin Catalog
-- =====================================================================

ALTER TABLE listing_versions
  ADD COLUMN IF NOT EXISTS decided_by uuid REFERENCES admin_users(id),
  ADD COLUMN IF NOT EXISTS decision_note text;
