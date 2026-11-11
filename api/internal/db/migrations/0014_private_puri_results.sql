CREATE TABLE reader_puri_results (
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    card_id uuid NOT NULL REFERENCES reader_cards(id) ON DELETE CASCADE,
    source_hash text NOT NULL,
    mode text NOT NULL CHECK (mode IN ('explain','translate','card_translate')),
    language text NOT NULL,
    phrase text NOT NULL DEFAULT '',
    phrase_hash text GENERATED ALWAYS AS (md5(phrase)) STORED,
    content text NOT NULL,
    model text NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, card_id, source_hash, mode, language, phrase_hash)
);
