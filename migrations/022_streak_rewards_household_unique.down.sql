CREATE TABLE streak_rewards_old (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    household_id INTEGER NOT NULL DEFAULT 1,
    streak_days INTEGER NOT NULL UNIQUE,
    bonus_points INTEGER NOT NULL,
    label TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO streak_rewards_old (id, household_id, streak_days, bonus_points, label, created_at)
    SELECT id, household_id, streak_days, bonus_points, label, created_at
    FROM streak_rewards;

DROP TABLE streak_rewards;
ALTER TABLE streak_rewards_old RENAME TO streak_rewards;

CREATE INDEX idx_streak_rewards_household ON streak_rewards(household_id);
