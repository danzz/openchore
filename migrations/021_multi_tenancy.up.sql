-- Multi-tenancy support: Accounts, Households, Household Devices, and scoped entities

-- 1. Accounts (global platform logins, e.g. parents)
CREATE TABLE accounts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email TEXT NOT NULL UNIQUE COLLATE NOCASE,
    password_hash TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX idx_accounts_email ON accounts(email);

-- 2. Households (tenants)
CREATE TABLE households (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    owner_account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    invite_code TEXT NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_households_owner ON households(owner_account_id);

-- 3. Household Devices (wall tablet pairing and kiosk access)
CREATE TABLE household_devices (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    household_id INTEGER NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    device_token TEXT NOT NULL UNIQUE,
    device_name TEXT NOT NULL DEFAULT 'Wall Tablet',
    pair_code TEXT,
    pair_code_expires_at DATETIME,
    paired_at DATETIME,
    last_seen_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_devices_household ON household_devices(household_id);
CREATE INDEX idx_devices_pair_code ON household_devices(pair_code);
CREATE UNIQUE INDEX idx_devices_token ON household_devices(device_token);

-- 4. Scoped household settings
CREATE TABLE household_settings (
    household_id INTEGER NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    value TEXT NOT NULL,
    PRIMARY KEY (household_id, key)
);

CREATE INDEX idx_household_settings_key ON household_settings(household_id, key);

-- 5. Seed default account & household to guarantee foreign key integrity on existing rows
INSERT OR IGNORE INTO accounts (id, email, password_hash, display_name)
VALUES (1, 'admin@openchore.local', '', 'Admin');

INSERT OR IGNORE INTO households (id, name, owner_account_id, invite_code)
VALUES (1, 'Default Household', 1, 'DEFAULT_HOUSEHOLD');

-- 6. Add household_id and account_id to existing scoped tables
ALTER TABLE users ADD COLUMN household_id INTEGER NOT NULL DEFAULT 1;
ALTER TABLE users ADD COLUMN account_id INTEGER;
CREATE INDEX idx_users_household ON users(household_id);

ALTER TABLE chores ADD COLUMN household_id INTEGER NOT NULL DEFAULT 1;
CREATE INDEX idx_chores_household ON chores(household_id);

ALTER TABLE rewards ADD COLUMN household_id INTEGER NOT NULL DEFAULT 1;
CREATE INDEX idx_rewards_household ON rewards(household_id);

ALTER TABLE streak_rewards ADD COLUMN household_id INTEGER NOT NULL DEFAULT 1;
CREATE INDEX idx_streak_rewards_household ON streak_rewards(household_id);

ALTER TABLE webhooks ADD COLUMN household_id INTEGER NOT NULL DEFAULT 1;
CREATE INDEX idx_webhooks_household ON webhooks(household_id);

-- 7. Copy any existing app_settings to default household
INSERT OR IGNORE INTO household_settings (household_id, key, value)
SELECT 1, key, value FROM app_settings;
