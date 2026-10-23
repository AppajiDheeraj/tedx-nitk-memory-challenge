-- Keep the database constraints aligned with game.GetLayout().
-- Easy uses a 4x4 board (8 pairs), medium 4x5, and hard 5x6.
ALTER TABLE games DROP CONSTRAINT IF EXISTS games_dimensions_check;

ALTER TABLE games ADD CONSTRAINT games_dimensions_check CHECK (
    (difficulty = 'easy' AND row_count = 4 AND col_count = 4)
    OR (difficulty = 'medium' AND row_count = 4 AND col_count = 5)
    OR (difficulty = 'hard' AND row_count = 5 AND col_count = 6)
);
