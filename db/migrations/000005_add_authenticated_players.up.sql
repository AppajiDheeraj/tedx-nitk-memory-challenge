ALTER TABLE players RENAME COLUMN name TO username;
ALTER TABLE players ALTER COLUMN username DROP NOT NULL;
ALTER TABLE players ADD COLUMN google_subject VARCHAR(255);
ALTER TABLE players ADD COLUMN email VARCHAR(320);
ALTER TABLE players ADD COLUMN email_verified BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE players ADD COLUMN picture_url TEXT;
ALTER TABLE players ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

CREATE UNIQUE INDEX players_google_subject_key
    ON players (google_subject)
    WHERE google_subject IS NOT NULL;
CREATE UNIQUE INDEX players_username_lower_key
    ON players (LOWER(username))
    WHERE username IS NOT NULL;

ALTER TABLE games ALTER COLUMN player_id SET NOT NULL;

CREATE TABLE sessions (
    token_hash CHAR(64) PRIMARY KEY,
    player_id BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX sessions_player_id_idx ON sessions(player_id);
CREATE INDEX sessions_expires_at_idx ON sessions(expires_at);
