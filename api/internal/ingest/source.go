package ingest

import "context"

func (i *Ingester) HasSource(ctx context.Context, source string) (bool, error) {
	var exists bool
	err := i.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM books b
 JOIN book_editions e ON e.book_id = b.id AND e.state = 'published'
 WHERE b.source_ref = $1)`, source).Scan(&exists)
	return exists, err
}
