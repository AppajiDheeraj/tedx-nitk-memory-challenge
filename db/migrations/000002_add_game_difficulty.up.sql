ALTER TABLE games
    DROP CONSTRAINT IF EXISTS games_board_size_check;

ALTER TABLE games
    DROP COLUMN board_size;

ALTER TABLE games
    ADD COLUMN difficulty VARCHAR(10) NOT NULL DEFAULT 'easy'
        CHECK (difficulty IN ('easy', 'medium', 'hard'));

ALTER TABLE games
    ADD COLUMN row_count INT NOT NULL DEFAULT 3;

ALTER TABLE games
    ADD COLUMN col_count INT NOT NULL DEFAULT 4;

ALTER TABLE games
    ADD CONSTRAINT games_dimensions_check CHECK (
        (difficulty = 'easy' AND row_count = 3 AND col_count = 4)
        OR
        (difficulty = 'medium' AND row_count = 4 AND col_count = 5)
        OR
        (difficulty = 'hard' AND row_count = 5 AND col_count = 6)
    );