CREATE TABLE email_tokens (
 token_hash bytea PRIMARY KEY,
 user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 purpose text NOT NULL CHECK (purpose IN ('verify','reset')),
 expires_at timestamptz NOT NULL
);
CREATE INDEX email_tokens_user ON email_tokens(user_id, purpose);
CREATE TABLE email_outbox (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 recipient text NOT NULL,
 subject text NOT NULL,
 body text NOT NULL,
 attempts integer NOT NULL DEFAULT 0,
 next_attempt timestamptz NOT NULL DEFAULT now(),
 created_at timestamptz NOT NULL DEFAULT now(),
 expires_at timestamptz NOT NULL DEFAULT now() + interval '24 hours'
);
