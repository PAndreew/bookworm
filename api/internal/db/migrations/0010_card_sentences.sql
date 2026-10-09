-- pySBD rune offsets within presentation cards; source page anchors are untouched.
ALTER TABLE reader_cards ADD COLUMN sentence_ends integer[];
