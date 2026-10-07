CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- Match the catalogue's stable ordering and substring-search expressions.
CREATE INDEX books_title_id ON books (title, id);
CREATE INDEX books_search_trgm ON books USING gin
    ((lower(title || ' ' || author)) gin_trgm_ops);
CREATE INDEX books_genres_gin ON books USING gin (genres);
