package store

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
)

var ErrTranslationBusy = errors.New("store: translation already generating")

type Translation struct {
	Language string `json:"language"`
	Content  string `json:"content"`
	Model    string `json:"model"`
}

func (s *Store) LockSubscription(ctx context.Context, id string) (pgx.Tx, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,44))`, id); err != nil {
		tx.Rollback(ctx)
		return nil, err
	}
	return tx, nil
}
func (s *Store) TranslationSource(ctx context.Context, id, user string) (ReaderCard, *Book, error) {
	var slug string
	var idx int
	err := s.pool.QueryRow(ctx, `SELECT b.slug,c.idx FROM reader_cards c JOIN book_editions e ON e.id=c.edition_id AND e.state='published' JOIN books b ON b.id=e.book_id WHERE c.id=$1`, id).Scan(&slug, &idx)
	if err != nil {
		return ReaderCard{}, nil, err
	}
	book, err := s.BookBySlug(ctx, slug)
	if err != nil {
		return ReaderCard{}, nil, err
	}
	cards, _, err := s.CardWindow(ctx, book.EditionID, user, idx, 0, 1)
	if err != nil {
		return ReaderCard{}, nil, err
	}
	if len(cards) != 1 || cards[0].CardID != id || cards[0].Kind != "text" {
		return ReaderCard{}, nil, pgx.ErrNoRows
	}
	return cards[0], book, nil
}
func (s *Store) Translations(ctx context.Context, id, hash string) ([]Translation, error) {
	rows, err := s.pool.Query(ctx, `SELECT language,content,model FROM card_translations WHERE card_id=$1 AND source_hash=$2 ORDER BY language`, id, hash)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Translation{}
	for rows.Next() {
		var t Translation
		if err = rows.Scan(&t.Language, &t.Content, &t.Model); err != nil {
			return nil, err
		}
		result = append(result, t)
	}
	return result, rows.Err()
}
func (s *Store) LockTranslation(ctx context.Context, id, language, hash string) (pgx.Tx, *Translation, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	var locked bool
	err = tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock(hashtextextended($1,45))`, id+":"+language).Scan(&locked)
	if err != nil || !locked {
		tx.Rollback(ctx)
		if err == nil {
			err = ErrTranslationBusy
		}
		return nil, nil, err
	}
	var cached Translation
	err = tx.QueryRow(ctx, `SELECT language,content,model FROM card_translations WHERE card_id=$1 AND language=$2 AND source_hash=$3`, id, language, hash).Scan(&cached.Language, &cached.Content, &cached.Model)
	if errors.Is(err, pgx.ErrNoRows) {
		return tx, nil, nil
	}
	if err != nil {
		tx.Rollback(ctx)
		return nil, nil, err
	}
	return tx, &cached, nil
}
func SaveTranslation(ctx context.Context, tx pgx.Tx, id, hash string, t Translation) error {
	if _, err := tx.Exec(ctx, `INSERT INTO card_translations(card_id,source_hash,language,content,model) VALUES($1,$2,$3,$4,$5) ON CONFLICT(card_id,language,source_hash) DO NOTHING`, id, hash, t.Language, t.Content, t.Model); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
