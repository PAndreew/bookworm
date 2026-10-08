-- New presentation cards may span immutable source pages. Old rows survive only
-- as minimal historical read/report anchors; they are never served as cards.
ALTER TABLE reader_cards ADD COLUMN archived boolean NOT NULL DEFAULT false;
ALTER TABLE reader_cards ADD COLUMN content text;
CREATE INDEX reader_cards_active ON reader_cards(edition_id,idx) WHERE NOT archived;
CREATE TABLE reader_card_parts (
 card_id uuid NOT NULL REFERENCES reader_cards(id) ON DELETE CASCADE,
 ordinal integer NOT NULL,
 page_id uuid NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
 start_off integer NOT NULL CHECK(start_off>=0),
 end_off integer NOT NULL CHECK(end_off>start_off),
 card_start integer NOT NULL CHECK(card_start>=0),
 PRIMARY KEY(card_id,ordinal)
);
CREATE INDEX reader_card_parts_page ON reader_card_parts(page_id,card_id);
CREATE TABLE card_reports (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 card_id uuid NOT NULL REFERENCES reader_cards(id) ON DELETE CASCADE,
 user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 category text NOT NULL CHECK(category IN ('layout','text','break')),
 note text NOT NULL DEFAULT '' CHECK(char_length(note)<=1000),
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(user_id,card_id,category)
);
CREATE INDEX card_reports_queue ON card_reports(created_at);
