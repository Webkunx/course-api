CREATE TABLE IF NOT EXISTS schema_version (
    version    INT NOT NULL,
    applied_at BIGINT NOT NULL,
    PRIMARY KEY (version)
);

CREATE TABLE IF NOT EXISTS meta (
    meta_key VARCHAR(255) NOT NULL,
    value    TEXT NOT NULL,
    PRIMARY KEY (meta_key)
);

CREATE TABLE IF NOT EXISTS exercises (
    exercise_id  BIGINT NOT NULL,
    canonical_id VARCHAR(255) NOT NULL,
    unit_id      INT NOT NULL,
    lesson_id    INT NOT NULL,
    content      MEDIUMTEXT NOT NULL,
    PRIMARY KEY (exercise_id)
);

CREATE TABLE IF NOT EXISTS user_progress (
    user_id    VARCHAR(36) NOT NULL,
    status     VARCHAR(20) NOT NULL DEFAULT 'real',
    `cursor`   BIGINT NOT NULL DEFAULT 0,
    is_active  TINYINT(1) NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL,
    PRIMARY KEY (user_id)
);

CREATE TABLE IF NOT EXISTS user_completion (
    user_id     VARCHAR(36) NOT NULL,
    slot        INT NOT NULL,
    exercise_id BIGINT NOT NULL,
    start_time  BIGINT NOT NULL,
    end_time    BIGINT,
    PRIMARY KEY (user_id, slot)
);

CREATE INDEX idx_completion_user ON user_completion (user_id);

CREATE TABLE IF NOT EXISTS user_daily (
    user_id   VARCHAR(36) NOT NULL,
    day       VARCHAR(10) NOT NULL,
    completed INT NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, day)
);
