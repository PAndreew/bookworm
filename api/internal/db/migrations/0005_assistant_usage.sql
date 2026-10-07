-- Count attempts, not phrase text. UTC daily caps survive API/container restarts.
CREATE TABLE assistant_daily_usage (
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    day date NOT NULL,
    requests integer NOT NULL DEFAULT 1 CHECK (requests > 0),
    last_request_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, day)
);
CREATE TABLE assistant_global_usage (
    day date PRIMARY KEY,
    requests integer NOT NULL DEFAULT 1 CHECK (requests > 0)
);
