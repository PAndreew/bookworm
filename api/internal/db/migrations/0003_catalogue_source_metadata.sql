-- Correct metadata for source IDs found during the first catalogue audit.
-- Never alter the source text, page anchors, editions or existing progress.
UPDATE books b SET title = c.title, author = c.author, genres = c.genres, language = c.language
FROM (VALUES
 ('gutenberg:668', 'The Gutenberg Webster''s Unabridged Dictionary: Section S', 'Project Gutenberg', ARRAY['reference', 'language'], 'en'),
 ('gutenberg:425', 'Familiar Studies of Men and Books', 'Robert Louis Stevenson', ARRAY['essays', 'biography'], 'en'),
 ('gutenberg:16769', 'Orthodoxy', 'G. K. Chesterton', ARRAY['religion', 'philosophy', 'essays'], 'en'),
 ('gutenberg:17396', 'The Secret Garden', 'Frances Hodgson Burnett', ARRAY['children', 'classic'], 'en'),
 ('gutenberg:690', 'Proposed Roads to Freedom', 'Bertrand Russell', ARRAY['politics', 'philosophy'], 'en'),
 ('gutenberg:1120', 'Julius Caesar', 'William Shakespeare', ARRAY['drama', 'history'], 'en'),
 ('gutenberg:2000', 'Don Quijote', 'Miguel de Cervantes Saavedra', ARRAY['adventure', 'satire', 'classic'], 'es')
) AS c(source, title, author, genres, language)
WHERE b.source_ref = c.source AND b.slug = replace(c.source, ':', '-');
