package store

import (
	"context"
	"strings"

	"github.com/bookworm/api/internal/chunk"
	"github.com/jackc/pgx/v5"
)

type cardSource struct {
	id, content, chapter string
	idx                  int
}
type CardPart struct {
	PageID    string `json:"page_id"`
	StartOff  int    `json:"start_off"`
	EndOff    int    `json:"end_off"`
	CardStart int    `json:"card_start"`
}
type layoutCard struct {
	kind, content, page string
	start, end          int
	parts               []CardPart
}

// buildCardLayout balances whole chapters while keeping precise original anchors.
// A source boundary inside a sentence becomes a space, not a fake paragraph.
func buildCardLayout(sources []cardSource, title, author string) []layoutCard {
	out := []layoutCard{{kind: "title", content: title + "\n" + author}}
	var text []rune
	var parts []CardPart
	chapter := ""
	flush := func() {
		for _, span := range chunk.Cards(string(text)) {
			card := layoutCard{kind: "text", content: string(text[span.Start:span.End])}
			for _, p := range parts {
				a, b := max(span.Start, p.CardStart), min(span.End, p.CardStart+p.EndOff-p.StartOff)
				if a >= b {
					continue
				}
				card.parts = append(card.parts, CardPart{p.PageID, p.StartOff + a - p.CardStart, p.StartOff + b - p.CardStart, a - span.Start})
			}
			if len(card.parts) > 0 {
				first := card.parts[0]
				card.page, card.start, card.end = first.PageID, first.StartOff, first.EndOff
				out = append(out, card)
			}
		}
		text = nil
		parts = nil
	}
	for _, p := range sources {
		if p.chapter != chapter {
			flush()
			chapter = p.chapter
			if chapter != "" {
				out = append(out, layoutCard{kind: "chapter", content: chapter, page: p.id})
			}
		}
		if len(text) > 0 {
			separator := " "
			if chunk.SentenceBoundary(text, len(text)) {
				separator = "\n\n"
			}
			text = append(text, []rune(separator)...)
		}
		parts = append(parts, CardPart{p.id, 0, len([]rune(p.content)), len(text)})
		text = append(text, []rune(p.content)...)
	}
	flush()
	return out
}

// Caller holds the edition advisory lock. Replacement and bookmark/history
// migration commit together. Published source pages and annotations never change.
func replaceReaderCards(ctx context.Context, tx pgx.Tx, edition, title, author string) error {
	rows, err := tx.Query(ctx, `SELECT id,content,COALESCE(chapter_title,''),idx FROM pages WHERE edition_id=$1 ORDER BY idx`, edition)
	if err != nil {
		return err
	}
	var sources []cardSource
	for rows.Next() {
		var p cardSource
		if err = rows.Scan(&p.id, &p.content, &p.chapter, &p.idx); err != nil {
			rows.Close()
			return err
		}
		sources = append(sources, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	cards := buildCardLayout(sources, title, author)
	// Capture exact old bookmarks before moving the obsolete layout out of range.
	if _, err = tx.Exec(ctx, `CREATE TEMP TABLE card_bookmark_map ON COMMIT DROP AS SELECT p.user_id,c.page_id,c.start_off,c.kind FROM reading_progress p JOIN reader_cards c ON c.edition_id=p.edition_id AND c.idx=p.last_card_idx AND NOT c.archived WHERE p.edition_id=$1`, edition); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE reader_cards SET archived=true,idx=idx+(SELECT COALESCE(max(idx),0)+$2+1 FROM reader_cards WHERE edition_id=$1) WHERE edition_id=$1 AND NOT archived`, edition, len(cards)); err != nil {
		return err
	}
	batch := &pgx.Batch{}
	for idx, c := range cards {
		var page *string
		if c.page != "" {
			page = &c.page
		}
		var heading *string
		if c.kind != "text" {
			heading = &c.content
		}
		batch.Queue(`INSERT INTO reader_cards(edition_id,idx,page_id,kind,title,content,start_off,end_off) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, edition, idx, page, c.kind, heading, c.content, c.start, c.end)
		for ordinal, p := range c.parts {
			batch.Queue(`INSERT INTO reader_card_parts(card_id,ordinal,page_id,start_off,end_off,card_start) SELECT id,$3,$4,$5,$6,$7 FROM reader_cards WHERE edition_id=$1 AND idx=$2 AND NOT archived`, edition, idx, ordinal, p.PageID, p.StartOff, p.EndOff, p.CardStart)
		}
	}
	if err = tx.SendBatch(ctx, batch).Close(); err != nil {
		return err
	}
	// Historical reads stay on old minimal anchors, preserving exact billed totals
	// and streak counts. New overlapping cards receive unmetered inherited coverage.
	if _, err = tx.Exec(ctx, `INSERT INTO card_reads(user_id,card_id,local_day,metered,inherited)
 SELECT DISTINCT r.user_id,part.card_id,r.local_day,false,true FROM card_reads r
 JOIN reader_cards old ON old.id=r.card_id AND old.edition_id=$1 AND old.archived
 LEFT JOIN reader_card_parts oldpart ON oldpart.card_id=old.id
 JOIN reader_card_parts part ON part.page_id=COALESCE(oldpart.page_id,old.page_id) AND part.start_off<COALESCE(oldpart.end_off,old.end_off) AND part.end_off>COALESCE(oldpart.start_off,old.start_off)
 JOIN reader_cards fresh ON fresh.id=part.card_id AND NOT fresh.archived
 ON CONFLICT DO NOTHING`, edition); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE reading_progress p SET last_card_idx=COALESCE((
 SELECT min(c.idx) FROM reader_cards c LEFT JOIN reader_card_parts part ON part.card_id=c.id
 WHERE c.edition_id=$1 AND NOT c.archived AND
 ((m.kind='title' AND c.kind='title') OR (m.kind='chapter' AND c.kind='chapter' AND c.page_id=m.page_id)
 OR (m.kind='text' AND part.page_id=m.page_id AND part.end_off>m.start_off))),0)
 FROM card_bookmark_map m WHERE p.edition_id=$1 AND p.user_id=m.user_id`, edition); err != nil {
		return err
	}
	// Discard obsolete card chunks. Keep only tiny referenced rows for the ledger
	// and reports, with no presentation text. Keep parts for subsequent migrations.
	if _, err = tx.Exec(ctx, `DELETE FROM reader_cards c WHERE c.edition_id=$1 AND c.archived AND NOT EXISTS(SELECT 1 FROM card_reads r WHERE r.card_id=c.id) AND NOT EXISTS(SELECT 1 FROM card_reports r WHERE r.card_id=c.id)`, edition); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE reader_cards SET content=NULL,title=NULL WHERE edition_id=$1 AND archived`, edition); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE book_editions SET reader_card_count=$2,reader_layout_version=4 WHERE id=$1`, edition, len(cards))
	return err
}

// ReportCard deduplicates reports and bounds daily submissions atomically. Only
// published cards accessible to this completed-auth user may be reported.
func (s *Store) ReportCard(ctx context.Context, userID, cardID, category, note string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// Same lock order as reads: edition, then user. A simultaneous replacement
	// cannot delete this report's target between validation and insertion.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock_shared(hashtextextended(edition_id::text,0)) FROM reader_cards WHERE id=$1`, cardID); err != nil {
		return err
	}
	var tier Tier
	if err = tx.QueryRow(ctx, `SELECT tier FROM users WHERE id=$1 FOR UPDATE`, userID).Scan(&tier); err != nil {
		return err
	}
	var license string
	if err = tx.QueryRow(ctx, `SELECT b.license FROM reader_cards c JOIN book_editions e ON e.id=c.edition_id AND e.state='published' JOIN books b ON b.id=e.book_id WHERE c.id=$1`, cardID).Scan(&license); err == pgx.ErrNoRows {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	if license != "public_domain" && tier == TierFree {
		return ErrForbidden
	}
	var exists bool
	var count int
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM card_reports WHERE user_id=$1 AND card_id=$2 AND category=$3),(SELECT count(*) FROM card_reports WHERE user_id=$1 AND created_at>=date_trunc('day',now()))`, userID, cardID, category).Scan(&exists, &count); err != nil {
		return err
	}
	if !exists && count >= 20 {
		return ErrReportLimit
	}
	_, err = tx.Exec(ctx, `INSERT INTO card_reports(user_id,card_id,category,note) VALUES($1,$2,$3,$4) ON CONFLICT(user_id,card_id,category) DO NOTHING`, userID, cardID, category, strings.TrimSpace(note))
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
