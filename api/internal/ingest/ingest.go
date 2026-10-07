// Package ingest writes books and their immutable pages into the catalogue.
package ingest

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bookworm/api/internal/chunk"
)

type Book struct {
	Slug        string
	Title       string
	Author      string
	Description string
	Genres      []string
	CoverURL    string
	Language    string
	License     string // public_domain | licensed
	SourceRef   string
	PublisherID string // empty for our own public-domain ingests
	WordCount   int
	Pages       []chunk.Page
}

type Ingester struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Ingester { return &Ingester{pool: pool} }

// Result reports what Publish actually did, so a caller can tell a real publish
// from a no-op. Without this the log line "ingested X, 194 pages" is a lie when
// the edition already existed and the pages on disk are still the old ones.
type Result struct {
	// Published is false when an edition at this chunker version already existed
	// and only metadata was refreshed.
	Published bool
	// PageCount is what is actually stored, which is NOT necessarily len(b.Pages).
	PageCount int
}

// Publish writes a book and one published edition in a single transaction.
//
// Idempotency: re-running with the same slug and the same chunker version leaves
// pages untouched. That matters because pages are permanent anchors for likes and
// highlights -- a careless re-ingest must not silently replace them with new rows.
//
// To re-chunk a book, bump chunk.Version. That creates a second edition and leaves
// every existing annotation pointing at the old one, which stays readable.
//
// If you need to replace the *source text* at the same chunker version -- swapping
// a sample excerpt for the full book, say -- use Replace, which is explicit about
// destroying the old pages and the annotations hanging off them.
func (i *Ingester) Publish(ctx context.Context, b Book) (Result, error) {
	if len(b.Pages) == 0 {
		return Result{}, errors.New("ingest: refusing to publish a book with no pages")
	}
	if err := ValidateGenres(b.Genres); err != nil {
		return Result{}, err
	}
	if b.License != "public_domain" && b.License != "licensed" {
		return Result{}, fmt.Errorf("ingest: unknown licence %q", b.License)
	}

	tx, err := i.pool.Begin(ctx)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback(ctx)

	var bookID string
	err = tx.QueryRow(ctx, `
		INSERT INTO books (slug, title, author, description, cover_url, language,
		                   license, source_ref, publisher_id)
		VALUES ($1, $2, $3, $4, NULLIF($5,''), $6, $7::license_kind, NULLIF($8,''),
		        NULLIF($9,'')::uuid)
		ON CONFLICT (slug) DO UPDATE
		SET title = EXCLUDED.title, author = EXCLUDED.author,
		    description = EXCLUDED.description, cover_url = EXCLUDED.cover_url
		RETURNING id`,
		b.Slug, b.Title, b.Author, b.Description, b.CoverURL, b.Language,
		b.License, b.SourceRef, b.PublisherID).Scan(&bookID)
	if err != nil {
		return Result{}, fmt.Errorf("ingest: upserting book: %w", err)
	}

	if b.Genres != nil {
		if _, err := tx.Exec(ctx, `UPDATE books SET genres=$2 WHERE id=$1`, bookID, b.Genres); err != nil {
			return Result{}, fmt.Errorf("ingest: updating genres: %w", err)
		}
	}

	// Already have this edition? Leave its pages exactly as they are and say so.
	var existingID string
	var existingPages int
	err = tx.QueryRow(ctx,
		`SELECT id, page_count FROM book_editions
		 WHERE book_id = $1 AND chunker_version = $2`,
		bookID, chunk.Version).Scan(&existingID, &existingPages)
	if err == nil {
		if cerr := tx.Commit(ctx); cerr != nil {
			return Result{}, cerr
		}
		// Metadata refreshed, pages untouched.
		return Result{Published: false, PageCount: existingPages}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Result{}, fmt.Errorf("ingest: checking for an existing edition: %w", err)
	}

	// A book may have only one published edition, so retire the previous one.
	if _, err := tx.Exec(ctx, `
		UPDATE book_editions SET state = 'draft'
		WHERE book_id = $1 AND state = 'published'`, bookID); err != nil {
		return Result{}, fmt.Errorf("ingest: retiring the previous edition: %w", err)
	}

	var editionID string
	err = tx.QueryRow(ctx, `
		INSERT INTO book_editions (book_id, chunker_version, state, page_count,
		                           word_count, published_at)
		VALUES ($1, $2, 'published', $3, $4, now())
		RETURNING id`,
		bookID, chunk.Version, len(b.Pages), b.WordCount).Scan(&editionID)
	if err != nil {
		return Result{}, fmt.Errorf("ingest: creating the edition: %w", err)
	}

	rows := make([][]any, 0, len(b.Pages))
	for _, p := range b.Pages {
		var title *string
		if p.ChapterTitle != "" {
			t := p.ChapterTitle
			title = &t
		}
		rows = append(rows, []any{editionID, p.Idx, title, p.Content, p.CharCount, p.SHA256})
	}

	_, err = tx.CopyFrom(ctx,
		pgx.Identifier{"pages"},
		[]string{"edition_id", "idx", "chapter_title", "content", "char_count", "content_sha256"},
		pgx.CopyFromRows(rows))
	if err != nil {
		return Result{}, fmt.Errorf("ingest: writing pages: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Result{}, err
	}
	return Result{Published: true, PageCount: len(b.Pages)}, nil
}

// Replace swaps the source text of an existing edition at the current chunker
// version. It deletes the old pages and writes new ones.
//
// This is destructive and it is meant to look it. Pages cascade-delete their
// likes and highlights, so calling this on a book people have read throws away
// their annotations. It exists for one narrow job: promoting a seed excerpt to
// the full text before anyone has read it.
//
// For a live book, bump chunk.Version and publish a new edition instead -- that
// keeps the old edition and its annotations intact.
func (i *Ingester) Replace(ctx context.Context, b Book) (Result, error) {
	if len(b.Pages) == 0 {
		return Result{}, errors.New("ingest: refusing to replace with no pages")
	}

	tx, err := i.pool.Begin(ctx)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback(ctx)

	var bookID, editionID string
	err = tx.QueryRow(ctx, `
		SELECT b.id, e.id FROM books b
		JOIN book_editions e ON e.book_id = b.id AND e.chunker_version = $2
		WHERE b.slug = $1`, b.Slug, chunk.Version).Scan(&bookID, &editionID)
	if errors.Is(err, pgx.ErrNoRows) {
		// Nothing to replace: a plain publish is the right thing.
		return i.Publish(ctx, b)
	}
	if err != nil {
		return Result{}, fmt.Errorf("ingest: locating the edition to replace: %w", err)
	}

	// Refuse to silently destroy annotations. The caller can delete the book
	// outright if that is really what they want.
	var annotations int
	if err := tx.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM page_likes l
		        JOIN pages p ON p.id = l.page_id WHERE p.edition_id = $1)
		     + (SELECT count(*) FROM highlights h
		        JOIN pages p ON p.id = h.page_id WHERE p.edition_id = $1)`,
		editionID).Scan(&annotations); err != nil {
		return Result{}, err
	}
	if annotations > 0 {
		return Result{}, fmt.Errorf(
			"ingest: %s has %d likes/highlights on its pages; bump chunk.Version "+
				"and publish a new edition instead of replacing this one",
			b.Slug, annotations)
	}

	if _, err := tx.Exec(ctx,
		`UPDATE books SET title=$2, author=$3, description=$4, source_ref=NULLIF($5,'')
		 WHERE id=$1`, bookID, b.Title, b.Author, b.Description, b.SourceRef); err != nil {
		return Result{}, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM pages WHERE edition_id = $1`, editionID); err != nil {
		return Result{}, fmt.Errorf("ingest: clearing old pages: %w", err)
	}

	rows := make([][]any, 0, len(b.Pages))
	for _, p := range b.Pages {
		var title *string
		if p.ChapterTitle != "" {
			t := p.ChapterTitle
			title = &t
		}
		rows = append(rows, []any{editionID, p.Idx, title, p.Content, p.CharCount, p.SHA256})
	}
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"pages"},
		[]string{"edition_id", "idx", "chapter_title", "content", "char_count", "content_sha256"},
		pgx.CopyFromRows(rows)); err != nil {
		return Result{}, fmt.Errorf("ingest: writing replacement pages: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`UPDATE book_editions SET page_count=$2, word_count=$3, published_at=now()
		 WHERE id=$1`, editionID, len(b.Pages), b.WordCount); err != nil {
		return Result{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Result{}, err
	}
	return Result{Published: true, PageCount: len(b.Pages)}, nil
}

// Publisher is a publishing account, owned by a user.
type Publisher struct {
	ID       string
	Name     string
	Verified bool
}

// EnsurePublisher creates (or finds) a publisher and the user account that owns
// it. Idempotent, so seeding can be re-run.
//
// The owning user is real: a publisher is a role a person holds, not a free-floating
// entity, and the schema enforces that with a NOT NULL owner. passwordHash is
// supplied by the caller so this package never has to know how passwords are made.
func (i *Ingester) EnsurePublisher(ctx context.Context, name, email, handle, passwordHash string, verified bool) (*Publisher, error) {
	tx, err := i.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// A publisher account is still an account: it needs a birthdate to satisfy
	// the 13+ check constraint, so use a fixed adult date for these service users.
	var ownerID string
	err = tx.QueryRow(ctx, `
		INSERT INTO users (email, handle, display_name, password_hash, birthdate,
		                   timezone, role, email_verified)
		VALUES ($1, $2, $3, $4, DATE '1990-01-01', 'UTC', 'publisher', true)
		ON CONFLICT (email) DO UPDATE SET role = 'publisher'
		RETURNING id`, email, handle, name, passwordHash).Scan(&ownerID)
	if err != nil {
		return nil, fmt.Errorf("ingest: creating publisher owner: %w", err)
	}

	var p Publisher
	var verifiedAt *time.Time
	err = tx.QueryRow(ctx, `
		INSERT INTO publishers (owner_user_id, name, verified_at)
		VALUES ($1, $2, CASE WHEN $3 THEN now() ELSE NULL END)
		ON CONFLICT DO NOTHING
		RETURNING id, name, verified_at`, ownerID, name, verified).
		Scan(&p.ID, &p.Name, &verifiedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		// Already present: fetch it rather than failing.
		err = tx.QueryRow(ctx,
			`SELECT id, name, verified_at FROM publishers WHERE owner_user_id = $1`,
			ownerID).Scan(&p.ID, &p.Name, &verifiedAt)
	}
	if err != nil {
		return nil, fmt.Errorf("ingest: creating publisher: %w", err)
	}
	p.Verified = verifiedAt != nil

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &p, nil
}
