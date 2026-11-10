-- Deliberately separate from paid inference usage. No keys, prompts or results.
CREATE TABLE IF NOT EXISTS byok_monthly_usage (
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    month date NOT NULL,
    requests integer NOT NULL DEFAULT 0 CHECK (requests BETWEEN 0 AND 50),
    PRIMARY KEY (user_id, month)
);
