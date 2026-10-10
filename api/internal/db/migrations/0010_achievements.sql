-- Earned once, independent of mutable reader layout and book visibility.
-- Historical card reads only stored their local date. Use midday on that date
-- for ordering those rows; new reads record their exact instant.
ALTER TABLE card_reads ADD COLUMN read_at timestamptz;
UPDATE card_reads SET read_at=local_day::timestamp + interval '12 hours';
ALTER TABLE card_reads ALTER COLUMN read_at SET DEFAULT now();
ALTER TABLE card_reads ALTER COLUMN read_at SET NOT NULL;
CREATE TABLE achievements (
 user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 code text NOT NULL CHECK (code IN ('first-card','hundred-cards','full-book')),
 earned_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(user_id,code)
);
