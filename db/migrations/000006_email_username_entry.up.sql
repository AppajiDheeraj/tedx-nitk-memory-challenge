-- Players now enter with an email and a public username instead of Google.
-- Emails are stored lower-cased; one email maps to one player.
UPDATE players SET email = LOWER(TRIM(email)) WHERE email IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS players_email_lower_key
    ON players (LOWER(email))
    WHERE email IS NOT NULL;
