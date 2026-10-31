DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS outbox_events;
ALTER TABLE payouts DROP COLUMN IF EXISTS held_by;
ALTER TABLE reservations DROP COLUMN IF EXISTS fee_config_id;
DROP TABLE IF EXISTS fee_configs;
DROP TABLE IF EXISTS admin_permissions;
DROP TABLE IF EXISTS admin_role_permissions;
ALTER TABLE admin_users DROP COLUMN IF EXISTS role;
