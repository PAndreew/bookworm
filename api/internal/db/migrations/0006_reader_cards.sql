-- A non-destructive presentation layer. Original pages and annotation offsets stay immutable.
ALTER TABLE book_editions ADD COLUMN reader_card_count integer NOT NULL DEFAULT 0;
ALTER TABLE reading_progress ADD COLUMN last_card_idx integer;
CREATE TABLE reader_cards (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 edition_id uuid NOT NULL REFERENCES book_editions(id) ON DELETE CASCADE,
 idx integer NOT NULL CHECK (idx >= 0),
 page_id uuid REFERENCES pages(id) ON DELETE CASCADE,
 kind text NOT NULL CHECK (kind IN ('title','chapter','text')),
 title text,
 start_off integer NOT NULL DEFAULT 0,
 end_off integer NOT NULL DEFAULT 0,
 CHECK (start_off >= 0 AND end_off >= start_off),
 CHECK (kind <> 'text' OR (page_id IS NOT NULL AND end_off > start_off)),
 UNIQUE(edition_id,idx)
);
CREATE INDEX reader_cards_page ON reader_cards(page_id,idx);
CREATE TABLE card_reads (
 user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 card_id uuid NOT NULL REFERENCES reader_cards(id) ON DELETE CASCADE,
 local_day date NOT NULL,
 metered boolean NOT NULL,
 PRIMARY KEY(user_id,card_id,local_day)
);
CREATE INDEX card_reads_quota ON card_reads(user_id,local_day) WHERE metered;
