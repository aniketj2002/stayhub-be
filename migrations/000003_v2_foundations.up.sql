-- =====================================================================
-- StayHub — Migration 000003: V2 Foundations
-- =====================================================================

-- 1. Payment kind additions
ALTER TYPE payment_kind ADD VALUE IF NOT EXISTS 'date_change_charge';
ALTER TYPE payment_kind ADD VALUE IF NOT EXISTS 'date_change_refund';
ALTER TYPE payment_kind ADD VALUE IF NOT EXISTS 'case_refund';

-- 2. Admin roles
ALTER TABLE admin_users
  ADD COLUMN IF NOT EXISTS role text NOT NULL DEFAULT 'support_l1'
  CHECK (role IN ('support_l1', 'support_l2', 'trust_safety', 'finance', 'catalog_manager', 'super_admin'));

UPDATE admin_users
  SET role = 'super_admin'
  WHERE email = 'admin@stayhub.dev';

-- 3. Admin role permissions (seeded lookup table)
CREATE TABLE IF NOT EXISTS admin_role_permissions (
  role       text NOT NULL,
  permission text NOT NULL,
  PRIMARY KEY (role, permission)
);

-- Seed role-permission matrix
INSERT INTO admin_role_permissions (role, permission) VALUES
  -- support_l1
  ('support_l1', 'reservations.read'),
  ('support_l1', 'audit.read'),
  ('support_l1', 'cases.handle'),

  -- support_l2
  ('support_l2', 'reservations.read'),
  ('support_l2', 'audit.read'),
  ('support_l2', 'reservations.cancel'),
  ('support_l2', 'reservations.refund'),
  ('support_l2', 'payouts.hold_release'),
  ('support_l2', 'cases.handle'),
  ('support_l2', 'cases.decide'),

  -- trust_safety
  ('trust_safety', 'reservations.read'),
  ('trust_safety', 'audit.read'),
  ('trust_safety', 'reservations.cancel'),
  ('trust_safety', 'listings.review'),
  ('trust_safety', 'cases.handle'),
  ('trust_safety', 'cases.decide'),

  -- finance
  ('finance', 'reservations.read'),
  ('finance', 'audit.read'),
  ('finance', 'reservations.refund'),
  ('finance', 'payouts.hold_release'),
  ('finance', 'cases.decide'),
  ('finance', 'fees.propose'),
  ('finance', 'fees.approve'),

  -- catalog_manager
  ('catalog_manager', 'reservations.read'),
  ('catalog_manager', 'audit.read'),
  ('catalog_manager', 'listings.review'),

  -- super_admin (all permissions)
  ('super_admin', 'reservations.read'),
  ('super_admin', 'audit.read'),
  ('super_admin', 'reservations.cancel'),
  ('super_admin', 'reservations.refund'),
  ('super_admin', 'payouts.hold_release'),
  ('super_admin', 'listings.review'),
  ('super_admin', 'cases.handle'),
  ('super_admin', 'cases.decide'),
  ('super_admin', 'fees.propose'),
  ('super_admin', 'fees.approve'),
  ('super_admin', 'admins.manage')
ON CONFLICT (role, permission) DO NOTHING;

-- 4. Admin per-user permission overrides
CREATE TABLE IF NOT EXISTS admin_permissions (
  admin_id   uuid NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
  permission text NOT NULL,
  granted    boolean NOT NULL DEFAULT true,
  PRIMARY KEY (admin_id, permission)
);

-- 5. Fee configs (two-person approval required for updates)
CREATE TABLE IF NOT EXISTS fee_configs (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  guest_fee_bps    int NOT NULL CHECK (guest_fee_bps >= 0),
  host_fee_bps     int NOT NULL CHECK (host_fee_bps >= 0),
  status           text NOT NULL DEFAULT 'pending_approval'
                   CHECK (status IN ('pending_approval','active','rejected','superseded')),
  proposed_by      uuid NOT NULL REFERENCES admin_users(id),
  proposed_reason  text NOT NULL,
  approved_by      uuid REFERENCES admin_users(id),
  approved_at      timestamptz,
  rejection_reason text,
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  CHECK (approved_by IS NULL OR approved_by <> proposed_by)
);

CREATE UNIQUE INDEX IF NOT EXISTS fee_configs_one_active
  ON fee_configs(status) WHERE status = 'active';

-- Seed initial active fee config matching V1 defaults (14% guest, 3% host)
INSERT INTO fee_configs (id, guest_fee_bps, host_fee_bps, status, proposed_by, proposed_reason, approved_at)
SELECT
  '00000000-0000-4000-9000-000000000001'::uuid,
  1400,
  300,
  'active',
  id,
  'Initial baseline fee configuration (1400 bps guest / 300 bps host)',
  now()
FROM admin_users
WHERE email = 'admin@stayhub.dev'
ON CONFLICT DO NOTHING;

-- 6. Reservations reference fee_config_id
ALTER TABLE reservations
  ADD COLUMN IF NOT EXISTS fee_config_id uuid REFERENCES fee_configs(id);

-- 7. Payouts hold tracking
ALTER TABLE payouts
  ADD COLUMN IF NOT EXISTS held_by uuid REFERENCES admin_users(id);

-- 8. Outbox events (transactional outbox pattern)
CREATE TABLE IF NOT EXISTS outbox_events (
  id             bigserial PRIMARY KEY,
  event_name     text NOT NULL,
  aggregate_type text NOT NULL,
  aggregate_id   uuid NOT NULL,
  payload        jsonb NOT NULL,
  status         text NOT NULL DEFAULT 'pending'
                 CHECK (status IN ('pending','processing','published','failed')),
  retry_count    int NOT NULL DEFAULT 0,
  next_retry_at  timestamptz NOT NULL DEFAULT now(),
  published_at   timestamptz,
  error_detail   text,
  created_at     timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS outbox_events_dispatch_idx
  ON outbox_events(status, next_retry_at)
  WHERE status IN ('pending', 'processing');

-- 9. Notifications table (in-app notifications)
CREATE TABLE IF NOT EXISTS notifications (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    uuid NOT NULL,
  title      text NOT NULL,
  message    text NOT NULL,
  channel    text NOT NULL DEFAULT 'in_app',
  status     text NOT NULL DEFAULT 'unread'
             CHECK (status IN ('unread','read','archived')),
  metadata   jsonb NOT NULL DEFAULT '{}'::jsonb,
  read_at    timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS notifications_user_idx
  ON notifications(user_id, status, created_at DESC);
