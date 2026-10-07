-- Refine the new presentation layer for paragraph-heavy front matter, without
-- touching source pages, annotations or historical quota/streak counts.
ALTER TABLE book_editions ADD COLUMN reader_layout_version integer NOT NULL DEFAULT 1;
ALTER TABLE card_reads ADD COLUMN inherited boolean NOT NULL DEFAULT false;
