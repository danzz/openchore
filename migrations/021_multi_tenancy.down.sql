-- Rollback multi-tenancy migration

DROP TABLE IF EXISTS household_settings;
DROP TABLE IF EXISTS household_devices;
DROP TABLE IF EXISTS households;
DROP TABLE IF EXISTS accounts;

DROP INDEX IF EXISTS idx_users_household;
DROP INDEX IF EXISTS idx_chores_household;
DROP INDEX IF EXISTS idx_rewards_household;
DROP INDEX IF EXISTS idx_streak_rewards_household;
DROP INDEX IF EXISTS idx_webhooks_household;

ALTER TABLE users DROP COLUMN household_id;
ALTER TABLE users DROP COLUMN account_id;
ALTER TABLE chores DROP COLUMN household_id;
ALTER TABLE rewards DROP COLUMN household_id;
ALTER TABLE streak_rewards DROP COLUMN household_id;
ALTER TABLE webhooks DROP COLUMN household_id;
