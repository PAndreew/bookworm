package store

import (
	"context"

	"github.com/bookworm/api/internal/chunk"
	"github.com/jackc/pgx/v5"
)

// refineReaderCards upgrades the first presentation layout's paragraph-heavy
// cards. Source pages/annotations never change. Existing card IDs are retained
// for the first piece; historical coverage carries to additional pieces without
// charging quota or inflating historical streak counts. Caller holds the edition lock.
func refineReaderCards(ctx context.Context, tx pgx.Tx, edition string) error {
	rows, err := tx.Query(ctx, `SELECT c.id,c.idx,c.page_id,c.kind,c.start_off,c.end_off,COALESCE(substring(p.content FROM c.start_off+1 FOR c.end_off-c.start_off),'') FROM reader_cards c LEFT JOIN pages p ON p.id=c.page_id WHERE c.edition_id=$1 ORDER BY c.idx`, edition)
	if err != nil {
		return err
	}
	type oldCard struct {
		id, kind, text  string
		idx, start, end int
		page            *string
		parts           []chunk.CardSpan
	}
	cards := []oldCard{}
	changed := false
	for rows.Next() {
		var c oldCard
		if err = rows.Scan(&c.id, &c.idx, &c.page, &c.kind, &c.start, &c.end, &c.text); err != nil {
			rows.Close()
			return err
		}
		c.parts = []chunk.CardSpan{{Start: 0, End: c.end - c.start}}
		if c.kind == "text" {
			parts := chunk.FitLines(c.text)
			if len(parts) > 0 {
				c.parts = parts
			}
			if len(c.parts) > 1 {
				changed = true
			}
		}
		cards = append(cards, c)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if !changed {
		_, err = tx.Exec(ctx, `UPDATE book_editions SET reader_layout_version=2 WHERE id=$1`, edition)
		return err
	}
	// Reserve a disjoint positive index range until the entire new order is ready.
	if _, err = tx.Exec(ctx, `UPDATE reader_cards SET idx=idx+1000000000 WHERE edition_id=$1`, edition); err != nil {
		return err
	}
	batch := &pgx.Batch{}
	oldIdx, newIdx := []int{}, []int{}
	idx := 0
	for _, c := range cards {
		oldIdx = append(oldIdx, c.idx)
		newIdx = append(newIdx, idx)
		for i, part := range c.parts {
			start, end := c.start+part.Start, c.start+part.End
			if i == 0 {
				batch.Queue(`UPDATE reader_cards SET idx=$2,start_off=$3,end_off=$4 WHERE id=$1`, c.id, idx, start, end)
			} else {
				batch.Queue(`WITH child AS (INSERT INTO reader_cards(edition_id,idx,page_id,kind,start_off,end_off) VALUES($1,$2,$3,'text',$4,$5) RETURNING id)
     INSERT INTO card_reads(user_id,card_id,local_day,metered,inherited)
     SELECT r.user_id,child.id,r.local_day,false,true FROM card_reads r CROSS JOIN child WHERE r.card_id=$6`, edition, idx, c.page, start, end, c.id)
			}
			idx++
		}
	}
	if err = tx.SendBatch(ctx, batch).Close(); err != nil {
		return err
	}
	// One UPDATE avoids cascading a remapped position through later old indexes.
	if _, err = tx.Exec(ctx, `UPDATE reading_progress p SET last_card_idx=m.new_idx FROM unnest($2::integer[],$3::integer[]) m(old_idx,new_idx) WHERE p.edition_id=$1 AND p.last_card_idx=m.old_idx`, edition, oldIdx, newIdx); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE book_editions SET reader_card_count=$2,reader_layout_version=2 WHERE id=$1`, edition, idx)
	return err
}
