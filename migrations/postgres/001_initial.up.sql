-- OpenChore PostgreSQL Baseline Schema

-- Compatibility functions for SQLite expressions
CREATE OR REPLACE FUNCTION julianday(val text) RETURNS double precision AS $$
BEGIN
    RETURN EXTRACT(EPOCH FROM val::timestamptz)/86400.0 + 2440587.5;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

CREATE OR REPLACE FUNCTION julianday(val date) RETURNS double precision AS $$
BEGIN
    RETURN EXTRACT(EPOCH FROM val::timestamp)/86400.0 + 2440587.5;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

CREATE OR REPLACE FUNCTION julianday(val timestamptz) RETURNS double precision AS $$
BEGIN
    RETURN EXTRACT(EPOCH FROM val)/86400.0 + 2440587.5;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

CREATE OR REPLACE FUNCTION strftime(format text, val date) RETURNS text AS $$
BEGIN
    IF format = '%w' THEN
        RETURN EXTRACT(DOW FROM val)::text;
    END IF;
    RETURN to_char(val, format);
END;
$$ LANGUAGE plpgsql IMMUTABLE;

CREATE OR REPLACE FUNCTION strftime(format text, val timestamptz) RETURNS text AS $$
BEGIN
    IF format = '%w' THEN
        RETURN EXTRACT(DOW FROM val)::text;
    END IF;
    RETURN to_char(val, format);
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- 1. Accounts (global platform logins)
CREATE TABLE accounts (
    id SERIAL PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX idx_accounts_email ON accounts(email);

-- 2. Households (tenants)
CREATE TABLE households (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    owner_account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    invite_code TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_households_owner ON households(owner_account_id);

-- 3. Household Devices (wall tablets)
CREATE TABLE household_devices (
    id SERIAL PRIMARY KEY,
    household_id INTEGER NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    device_token TEXT NOT NULL UNIQUE,
    device_name TEXT NOT NULL DEFAULT 'Wall Tablet',
    pair_code TEXT,
    pair_code_expires_at TIMESTAMPTZ,
    paired_at TIMESTAMPTZ,
    last_seen_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
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

-- 5. Global App Settings
CREATE TABLE app_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- 6. Users
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    avatar_url TEXT NOT NULL DEFAULT '',
    role TEXT NOT NULL CHECK (role IN ('admin', 'child')) DEFAULT 'child',
    age INTEGER,
    theme TEXT NOT NULL DEFAULT '',
    paused INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    line_color TEXT NOT NULL DEFAULT '',
    pin_hash TEXT NOT NULL DEFAULT '',
    session_version INTEGER NOT NULL DEFAULT 0,
    color TEXT NOT NULL DEFAULT '',
    pin_length INTEGER NOT NULL DEFAULT 0,
    household_id INTEGER NOT NULL DEFAULT 1 REFERENCES households(id) ON DELETE CASCADE,
    account_id INTEGER REFERENCES accounts(id) ON DELETE SET NULL
);
CREATE INDEX idx_users_household ON users(household_id);

-- 7. Chores
CREATE TABLE chores (
    id SERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    category TEXT NOT NULL CHECK (category IN ('required', 'core', 'bonus')) DEFAULT 'core',
    icon TEXT NOT NULL DEFAULT '',
    points_value INTEGER NOT NULL DEFAULT 0,
    missed_penalty_value INTEGER NOT NULL DEFAULT 0,
    estimated_minutes INTEGER,
    requires_approval INTEGER NOT NULL DEFAULT 0,
    requires_photo INTEGER NOT NULL DEFAULT 0,
    photo_source TEXT NOT NULL DEFAULT 'child' CHECK (photo_source IN ('child', 'external', 'both')),
    source TEXT NOT NULL DEFAULT 'manual',
    external_id TEXT NOT NULL DEFAULT '',
    created_by INTEGER NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    tts_audio_url TEXT NOT NULL DEFAULT '',
    household_id INTEGER NOT NULL DEFAULT 1 REFERENCES households(id) ON DELETE CASCADE
);
CREATE INDEX idx_chores_household ON chores(household_id);

-- 8. Rewards
CREATE TABLE rewards (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    icon TEXT NOT NULL DEFAULT '',
    cost INTEGER NOT NULL CHECK (cost > 0),
    stock INTEGER,
    active INTEGER NOT NULL DEFAULT 1,
    created_by INTEGER NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    shareable INTEGER NOT NULL DEFAULT 0 CHECK (shareable IN (0, 1)),
    household_id INTEGER NOT NULL DEFAULT 1 REFERENCES households(id) ON DELETE CASCADE
);
CREATE INDEX idx_rewards_household ON rewards(household_id);

-- 9. Reward Redemptions
CREATE TABLE reward_redemptions (
    id SERIAL PRIMARY KEY,
    reward_id INTEGER NOT NULL REFERENCES rewards(id),
    user_id INTEGER NOT NULL REFERENCES users(id),
    points_spent INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_redemptions_user ON reward_redemptions(user_id);

-- 10. Reward Assignments
CREATE TABLE reward_assignments (
    id SERIAL PRIMARY KEY,
    reward_id INTEGER NOT NULL REFERENCES rewards(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    custom_cost INTEGER,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(reward_id, user_id)
);
CREATE INDEX idx_reward_assignments_reward ON reward_assignments(reward_id);
CREATE INDEX idx_reward_assignments_user ON reward_assignments(user_id);

-- 11. User Streaks
CREATE TABLE user_streaks (
    user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    current_streak INTEGER NOT NULL DEFAULT 0,
    longest_streak INTEGER NOT NULL DEFAULT 0,
    streak_start_date DATE,
    last_completed_date DATE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 12. Webhooks
CREATE TABLE webhooks (
    id SERIAL PRIMARY KEY,
    url TEXT NOT NULL,
    secret TEXT NOT NULL DEFAULT '',
    events TEXT NOT NULL DEFAULT '*',
    active INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    household_id INTEGER NOT NULL DEFAULT 1 REFERENCES households(id) ON DELETE CASCADE
);
CREATE INDEX idx_webhooks_household ON webhooks(household_id);

-- 13. Webhook Deliveries
CREATE TABLE webhook_deliveries (
    id SERIAL PRIMARY KEY,
    webhook_id INTEGER NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    event TEXT NOT NULL,
    payload TEXT NOT NULL,
    status_code INTEGER,
    response_body TEXT NOT NULL DEFAULT '',
    error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_webhook_deliveries_webhook ON webhook_deliveries(webhook_id);
CREATE INDEX idx_webhook_deliveries_created ON webhook_deliveries(created_at);

-- 14. User Decay Config
CREATE TABLE user_decay_config (
    user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    enabled INTEGER NOT NULL DEFAULT 0,
    decay_rate INTEGER NOT NULL DEFAULT 5,
    decay_interval_hours INTEGER NOT NULL DEFAULT 24,
    last_decay_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 15. Chore Triggers
CREATE TABLE chore_triggers (
    id SERIAL PRIMARY KEY,
    uuid TEXT NOT NULL UNIQUE,
    chore_id INTEGER NOT NULL REFERENCES chores(id) ON DELETE CASCADE,
    default_assigned_to INTEGER REFERENCES users(id) ON DELETE SET NULL,
    default_due_by TEXT,
    default_available_at TEXT,
    enabled INTEGER NOT NULL DEFAULT 1,
    cooldown_minutes INTEGER NOT NULL DEFAULT 0,
    last_triggered_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    assignment_type TEXT NOT NULL DEFAULT 'individual'
);
CREATE UNIQUE INDEX idx_chore_triggers_uuid ON chore_triggers(uuid);
CREATE INDEX idx_chore_triggers_chore ON chore_triggers(chore_id);

-- 16. API Tokens
CREATE TABLE api_tokens (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    last_used_at TIMESTAMPTZ,
    revoked INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_api_tokens_hash ON api_tokens(token_hash);

-- 17. Chore Schedules
CREATE TABLE chore_schedules (
    id SERIAL PRIMARY KEY,
    chore_id INTEGER NOT NULL REFERENCES chores(id) ON DELETE CASCADE,
    assigned_to INTEGER NOT NULL REFERENCES users(id),
    assignment_type TEXT NOT NULL CHECK (assignment_type IN ('individual', 'family', 'fcfs')) DEFAULT 'individual',
    fcfs_group_id TEXT,
    day_of_week INTEGER CHECK (day_of_week BETWEEN 0 AND 6),
    specific_date DATE,
    available_at TEXT,
    due_by TEXT,
    expiry_penalty TEXT NOT NULL DEFAULT 'block',
    expiry_penalty_value INTEGER NOT NULL DEFAULT 0,
    points_multiplier REAL NOT NULL DEFAULT 1.0,
    start_date DATE,
    end_date DATE,
    recurrence_interval INTEGER,
    recurrence_start DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (day_of_week IS NOT NULL OR specific_date IS NOT NULL OR recurrence_interval IS NOT NULL)
);
CREATE INDEX idx_schedules_assigned ON chore_schedules(assigned_to);
CREATE INDEX idx_schedules_chore ON chore_schedules(chore_id);
CREATE INDEX idx_schedules_fcfs_group ON chore_schedules(fcfs_group_id);

-- 18. Shared Commitment Pools
CREATE TABLE shared_commitment_pools (
    id SERIAL PRIMARY KEY,
    reward_id INTEGER NOT NULL REFERENCES rewards(id),
    target_cost INTEGER NOT NULL CHECK (target_cost > 0),
    status TEXT NOT NULL CHECK (status IN ('active', 'redeemed', 'cancelled')) DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    redeemed_at TIMESTAMPTZ,
    redeemed_by_user_id INTEGER REFERENCES users(id),
    cancelled_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX idx_shared_pools_one_active_per_reward ON shared_commitment_pools(reward_id) WHERE status = 'active';
CREATE INDEX idx_shared_pools_status ON shared_commitment_pools(status);

-- 19. Reward Commitments
CREATE TABLE reward_commitments (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reward_id INTEGER NOT NULL REFERENCES rewards(id),
    target_cost INTEGER NOT NULL CHECK (target_cost > 0),
    auto_contribute_percent INTEGER NOT NULL DEFAULT 0 CHECK (auto_contribute_percent BETWEEN 0 AND 100),
    status TEXT NOT NULL CHECK (status IN ('active', 'redeemed', 'cancelled')) DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    redeemed_at TIMESTAMPTZ,
    cancelled_at TIMESTAMPTZ,
    shared_pool_id INTEGER REFERENCES shared_commitment_pools(id)
);
CREATE INDEX idx_reward_commitments_user ON reward_commitments(user_id);
CREATE INDEX idx_reward_commitments_pool ON reward_commitments(shared_pool_id) WHERE shared_pool_id IS NOT NULL;
CREATE UNIQUE INDEX idx_reward_commitments_one_active_personal ON reward_commitments(user_id) WHERE status = 'active' AND shared_pool_id IS NULL;
CREATE UNIQUE INDEX idx_reward_commitments_one_active_share_per_pool ON reward_commitments(user_id, shared_pool_id) WHERE status = 'active' AND shared_pool_id IS NOT NULL;

-- 20. Point Transactions
CREATE TABLE point_transactions (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount INTEGER NOT NULL,
    reason TEXT NOT NULL CHECK (reason IN (
        'chore_complete', 'chore_uncomplete', 'reward_redeem',
        'streak_bonus', 'admin_adjust', 'expiry_penalty',
        'points_decay', 'missed_chore',
        'commit_to_goal', 'goal_break'
    )),
    reference_id INTEGER,
    note TEXT NOT NULL DEFAULT '',
    idempotency_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_point_tx_user ON point_transactions(user_id);
CREATE INDEX idx_point_tx_ref ON point_transactions(reason, reference_id);
CREATE UNIQUE INDEX idx_point_tx_idempotency_key ON point_transactions(idempotency_key) WHERE idempotency_key IS NOT NULL;

-- 21. User Identities
CREATE TABLE user_identities (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    subject TEXT NOT NULL,
    email TEXT NOT NULL DEFAULT '',
    display_name TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_login_at TIMESTAMPTZ,
    UNIQUE(provider, subject)
);
CREATE INDEX idx_user_identities_user ON user_identities(user_id);

-- 22. Chore Completions
CREATE TABLE chore_completions (
    id SERIAL PRIMARY KEY,
    chore_schedule_id INTEGER NOT NULL REFERENCES chore_schedules(id) ON DELETE CASCADE,
    completed_by INTEGER NOT NULL REFERENCES users(id),
    status TEXT NOT NULL CHECK (status IN ('pending', 'approved', 'rejected', 'excused')) DEFAULT 'approved',
    photo_url TEXT NOT NULL DEFAULT '',
    approved_by INTEGER REFERENCES users(id),
    approved_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completion_date DATE NOT NULL,
    ai_feedback TEXT NOT NULL DEFAULT '',
    ai_confidence REAL NOT NULL DEFAULT 0,
    ai_complete INTEGER,
    uncompleted_at TIMESTAMPTZ
);
CREATE INDEX idx_completions_schedule_date ON chore_completions(chore_schedule_id, completion_date);
CREATE INDEX idx_completions_completed_by ON chore_completions(completed_by);
CREATE INDEX idx_completions_uncompleted_at_null ON chore_completions(chore_schedule_id, completion_date) WHERE uncompleted_at IS NULL;

-- 23. Weekly Summaries
CREATE TABLE weekly_summaries (
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    week_start DATE NOT NULL,
    summary TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, week_start)
);

-- 24. OIDC Providers
CREATE TABLE oidc_providers (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    issuer TEXT NOT NULL,
    client_id TEXT NOT NULL,
    client_secret TEXT NOT NULL DEFAULT '',
    scopes TEXT NOT NULL DEFAULT '',
    prompt TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 25. Streak Rewards
CREATE TABLE streak_rewards (
    id SERIAL PRIMARY KEY,
    household_id INTEGER NOT NULL DEFAULT 1 REFERENCES households(id) ON DELETE CASCADE,
    streak_days INTEGER NOT NULL,
    bonus_points INTEGER NOT NULL,
    label TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (household_id, streak_days)
);
CREATE INDEX idx_streak_rewards_household ON streak_rewards(household_id);

-- 26. Default Bootstrap Seeds
INSERT INTO accounts (id, email, password_hash, display_name)
VALUES (1, 'admin@openchore.local', '', 'Admin')
ON CONFLICT (id) DO NOTHING;

INSERT INTO households (id, name, owner_account_id, invite_code)
VALUES (1, 'Default Household', 1, 'DEFAULT_HOUSEHOLD')
ON CONFLICT (id) DO NOTHING;

-- Reset sequence for accounts and households so auto-increment continues from max id
SELECT setval(pg_get_serial_sequence('accounts', 'id'), COALESCE((SELECT MAX(id) FROM accounts), 1));
SELECT setval(pg_get_serial_sequence('households', 'id'), COALESCE((SELECT MAX(id) FROM households), 1));
