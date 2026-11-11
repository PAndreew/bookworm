package store

import "context"

type PuriResult struct {
	Mode     string `json:"mode"`
	Language string `json:"language"`
	Phrase   string `json:"phrase"`
	Content  string `json:"content"`
	Model    string `json:"model"`
}

func (s *Store) SavePuriResult(ctx context.Context, user, card, hash string, result PuriResult) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO reader_puri_results(user_id,card_id,source_hash,mode,language,phrase,content,model) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(user_id,card_id,source_hash,mode,language,phrase_hash) DO UPDATE SET content=excluded.content,model=excluded.model,updated_at=now()`, user, card, hash, result.Mode, result.Language, result.Phrase, result.Content, result.Model)
	return err
}

func (s *Store) PuriResults(ctx context.Context, user, card, hash string) ([]PuriResult, error) {
	rows, err := s.pool.Query(ctx, `SELECT mode,language,phrase,content,model FROM reader_puri_results WHERE user_id=$1 AND card_id=$2 AND source_hash=$3 ORDER BY updated_at DESC LIMIT 200`, user, card, hash)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	results := []PuriResult{}
	for rows.Next() {
		var result PuriResult
		if err = rows.Scan(&result.Mode, &result.Language, &result.Phrase, &result.Content, &result.Model); err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, rows.Err()
}
