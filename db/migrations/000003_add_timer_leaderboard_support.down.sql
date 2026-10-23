ALTER TABLE games DROP CONSTRAINT IF EXISTS games_dimensions_check;
ALTER TABLE games ADD CONSTRAINT games_dimensions_check CHECK (
    (difficulty = 'easy' AND ((row_count = 3 AND col_count = 4) OR (row_count = 4 AND col_count = 4)))
    OR (difficulty = 'medium' AND row_count = 4 AND col_count = 5)
    OR (difficulty = 'hard' AND row_count = 5 AND col_count = 6)
);

UPDATE games SET status = 'in_progress' WHERE status = 'expired';
ALTER TABLE games DROP CONSTRAINT IF EXISTS games_status_check;
ALTER TABLE games ADD CONSTRAINT games_status_check
    CHECK (status IN ('in_progress', 'completed'));
