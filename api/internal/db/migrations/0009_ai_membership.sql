-- AI-only founding membership. Book licensing data is preserved, not deleted.
CREATE TABLE reader_subscriptions (
 user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
 customer_id text NOT NULL,
 subscription_id text NOT NULL UNIQUE,
 status text NOT NULL,
 period_end timestamptz NOT NULL,
 cancel_at_period_end boolean NOT NULL DEFAULT false,
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE founding_slots (
 slot integer PRIMARY KEY CHECK(slot BETWEEN 1 AND 50),
 user_id uuid UNIQUE REFERENCES users(id) ON DELETE SET NULL,
 paid boolean NOT NULL DEFAULT false,
 lease_id uuid NOT NULL DEFAULT gen_random_uuid(),
 expires_at timestamptz NOT NULL DEFAULT now(),
 checkout_id text,
 checkout_url text
);
INSERT INTO founding_slots(slot) SELECT generate_series(1,50);
CREATE TABLE reader_stripe_events (
 id text PRIMARY KEY,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE assistant_monthly_usage (
 user_id uuid REFERENCES users(id) ON DELETE CASCADE,
 month date NOT NULL,
 requests integer NOT NULL DEFAULT 0 CHECK(requests>=0),
 last_request_at timestamptz,
 PRIMARY KEY(user_id,month)
);
-- Preserve current-month attempts already counted under the former daily rules.
INSERT INTO assistant_monthly_usage(user_id,month,requests,last_request_at)
 SELECT user_id,date_trunc('month',day)::date,sum(requests),max(last_request_at)
 FROM assistant_daily_usage GROUP BY user_id,date_trunc('month',day)::date;
CREATE TABLE card_translations (
 card_id uuid REFERENCES reader_cards(id) ON DELETE CASCADE,
 language text NOT NULL CHECK(language IN ('French','Spanish','German')),
 source_hash text NOT NULL,
 content text NOT NULL CHECK(char_length(content) BETWEEN 1 AND 4000),
 model text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(card_id,language,source_hash)
);
