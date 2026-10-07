ALTER TABLE books ADD COLUMN genres text[] NOT NULL DEFAULT '{}'
    CHECK (cardinality(genres) <= 5);

CREATE TABLE book_ratings (
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    book_id uuid NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    stars smallint NOT NULL CHECK (stars BETWEEN 1 AND 5),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, book_id)
);

-- Metadata only: editions, pages, annotations and reading progress are untouched.
UPDATE books SET genres = CASE slug
    WHEN 'pride-and-prejudice' THEN ARRAY['romance', 'classic', 'social-fiction']
    WHEN 'frankenstein' THEN ARRAY['science-fiction', 'horror', 'gothic']
    WHEN 'the-adventures-of-sherlock-holmes' THEN ARRAY['mystery', 'crime', 'short-stories']
    WHEN 'moby-dick' THEN ARRAY['adventure', 'classic', 'sea-stories']
    WHEN 'dracula' THEN ARRAY['horror', 'gothic', 'fantasy']
    WHEN 'alices-adventures-in-wonderland' THEN ARRAY['fantasy', 'children', 'adventure']
    WHEN 'the-picture-of-dorian-gray' THEN ARRAY['gothic', 'philosophy', 'classic']
    WHEN 'the-yellow-wallpaper' THEN ARRAY['short-stories', 'psychological', 'gothic']
    WHEN 'the-great-gatsby' THEN ARRAY['classic', 'social-fiction', 'romance']
    WHEN 'a-tale-of-two-cities' THEN ARRAY['historical-fiction', 'classic']
    ELSE ARRAY['contemporary', 'fiction']
END;
