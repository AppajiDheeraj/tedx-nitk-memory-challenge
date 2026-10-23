CREATE TABLE players (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE games (
    id BIGSERIAL PRIMARY KEY,
    player_id BIGINT REFERENCES players(id),
    board_size INT NOT NULL
        CHECK (board_size IN (3, 4)),
    moves INT NOT NULL DEFAULT 0
        CHECK (moves >= 0),
    pairs_found INT NOT NULL DEFAULT 0
        CHECK (pairs_found >= 0),
    total_pairs INT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'in_progress'
        CHECK (status IN ('in_progress', 'completed')),
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE TABLE game_cards (
    id BIGSERIAL PRIMARY KEY,
    game_id BIGINT NOT NULL
        REFERENCES games(id) ON DELETE CASCADE,
    position INT NOT NULL,
    pair_id INT,
    value VARCHAR(30) NOT NULL,
    is_matched BOOLEAN NOT NULL DEFAULT FALSE,
    is_revealed BOOLEAN NOT NULL DEFAULT FALSE,

    UNIQUE (game_id, position)
);

CREATE INDEX idx_games_player_id ON games(player_id);
CREATE INDEX idx_game_cards_game_id ON game_cards(game_id);