-- Local inference has its own allowance; it never consumes hosted capacity.
CREATE TABLE assistant_local_usage (
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    month date NOT NULL,
    requests integer NOT NULL DEFAULT 0 CHECK (requests >= 0),
    last_request_at timestamptz,
    PRIMARY KEY (user_id, month)
);
