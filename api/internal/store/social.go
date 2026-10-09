package store

import (
	"context"
	"time"
)

// Friendships store the TRUE direction of the request in requester_id/addressee_id.
// Uniqueness is enforced on the unordered pair by the friendships_unique_pair index,
// so A->B and B->A cannot both exist. Queries therefore always match on
// "either column is me".

// RequestFriend creates a pending request from one user to another.
//
// ON CONFLICT DO NOTHING against the unordered-pair index means a second request --
// in either direction -- leaves the existing row untouched. That matters: re-asking
// must not reset an accepted friendship, and must not quietly undo a block.
func (s *Store) RequestFriend(ctx context.Context, fromID, toID string) error {
	if fromID == toID {
		return ErrForbidden
	}

	blocked, err := s.IsBlocked(ctx, fromID, toID)
	if err != nil {
		return err
	}
	if blocked {
		// Deliberately the same error a stranger would get: do not confirm to a
		// blocked user that they have been blocked.
		return ErrForbidden
	}

	const q = `
		INSERT INTO friendships (requester_id, addressee_id, state)
		VALUES ($1, $2, 'pending')
		ON CONFLICT (LEAST(requester_id, addressee_id), GREATEST(requester_id, addressee_id))
		DO NOTHING`
	_, err = s.pool.Exec(ctx, q, fromID, toID)
	return err
}

// AcceptFriend accepts a pending request. Only the addressee may accept: the
// requester cannot accept their own request, which is what makes this mutual
// consent rather than a follow.
func (s *Store) AcceptFriend(ctx context.Context, userID, otherID string) error {
	const q = `
		UPDATE friendships SET state = 'accepted', decided_at = now()
		WHERE requester_id = $2 AND addressee_id = $1 AND state = 'pending'`
	tag, err := s.pool.Exec(ctx, q, userID, otherID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// RemoveFriend deletes the relationship in either direction. Also used to decline
// a pending request.
func (s *Store) RemoveFriend(ctx context.Context, userID, otherID string) error {
	const q = `
		DELETE FROM friendships
		WHERE (requester_id = $1 AND addressee_id = $2)
		   OR (requester_id = $2 AND addressee_id = $1)`
	_, err := s.pool.Exec(ctx, q, userID, otherID)
	return err
}

// BlockUser blocks someone, replacing any existing relationship. requester_id is set
// to the blocker so we know who imposed it.
func (s *Store) BlockUser(ctx context.Context, userID, otherID string) error {
	if userID == otherID {
		return ErrForbidden
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		DELETE FROM friendships
		WHERE (requester_id = $1 AND addressee_id = $2)
		   OR (requester_id = $2 AND addressee_id = $1)`, userID, otherID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO friendships (requester_id, addressee_id, state, decided_at)
		VALUES ($1, $2, 'blocked', now())`, userID, otherID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// AreFriends reports whether two users have an accepted friendship.
func (s *Store) AreFriends(ctx context.Context, a, b string) (bool, error) {
	if a == b {
		return true, nil // you can always see yourself
	}
	const q = `
		SELECT EXISTS (
			SELECT 1 FROM friendships
			WHERE state = 'accepted'
			  AND ((requester_id = $1 AND addressee_id = $2)
			    OR (requester_id = $2 AND addressee_id = $1)))`
	var ok bool
	err := s.pool.QueryRow(ctx, q, a, b).Scan(&ok)
	return ok, err
}

// IsBlocked reports whether either party has blocked the other.
func (s *Store) IsBlocked(ctx context.Context, a, b string) (bool, error) {
	const q = `
		SELECT EXISTS (
			SELECT 1 FROM friendships
			WHERE state = 'blocked'
			  AND ((requester_id = $1 AND addressee_id = $2)
			    OR (requester_id = $2 AND addressee_id = $1)))`
	var ok bool
	err := s.pool.QueryRow(ctx, q, a, b).Scan(&ok)
	return ok, err
}

type Friend struct {
	ID              string `json:"id"`
	Handle          string `json:"handle"`
	DisplayName     string `json:"display_name"`
	StreakDays      int    `json:"streak_days"`
	CommonBooks     int    `json:"common_books"`
	LatestBookTitle string `json:"latest_book_title"`
	Pages30Days     int    `json:"pages_30_days"`
	// Incoming distinguishes "they asked you" from "you asked them" on pending rows.
	Incoming bool `json:"incoming,omitempty"`
}

// ListFriends compares started books and earned reading, respecting privacy.
// Only real reading credits contribute to recent pages; days use local dates.
func (s *Store) ListFriends(ctx context.Context, userID string) ([]Friend, error) {
	const q = `
	WITH peers AS (
	 SELECT u.* FROM friendships f JOIN users u ON u.id =
	 CASE WHEN f.requester_id=$1 THEN f.addressee_id ELSE f.requester_id END
	 WHERE (f.requester_id=$1 OR f.addressee_id=$1) AND f.state='accepted' AND u.deleted_at IS NULL
	), participants AS (SELECT id FROM peers UNION SELECT $1::uuid),
	reads AS (
	 SELECT pr.user_id,p.edition_id,pr.local_day FROM page_reads pr JOIN pages p ON p.id=pr.page_id
	 WHERE pr.user_id IN (SELECT id FROM participants)
	 UNION ALL
	 SELECT cr.user_id,c.edition_id,cr.local_day FROM card_reads cr JOIN reader_cards c ON c.id=cr.card_id
	 WHERE NOT cr.inherited AND cr.user_id IN (SELECT id FROM participants)
	), started AS (
	 SELECT user_id,edition_id FROM reads
	 UNION
	 SELECT user_id,edition_id FROM reading_progress WHERE user_id IN (SELECT id FROM participants)
	), read_books AS (
	 SELECT DISTINCT r.user_id,e.book_id FROM started r JOIN book_editions e ON e.id=r.edition_id
	 JOIN users u ON u.id=r.user_id LEFT JOIN reading_progress rp ON rp.user_id=r.user_id AND rp.edition_id=r.edition_id
	 WHERE r.user_id=$1 OR (u.visibility<>'private' AND NOT COALESCE(rp.is_private,false))
	), common AS (
	 SELECT r.user_id,count(*) AS n FROM read_books r
	 JOIN read_books mine ON mine.user_id=$1 AND mine.book_id=r.book_id
	 GROUP BY r.user_id
	), recent AS (
	 SELECT r.user_id,count(*) AS n FROM reads r JOIN peers u ON u.id=r.user_id
	 LEFT JOIN reading_progress rp ON rp.user_id=r.user_id AND rp.edition_id=r.edition_id
	 WHERE u.visibility<>'private' AND NOT COALESCE(rp.is_private,false)
	 AND r.local_day BETWEEN (now() AT TIME ZONE u.timezone)::date-29 AND (now() AT TIME ZONE u.timezone)::date
	 GROUP BY r.user_id
	)
	SELECT u.id,u.handle,u.display_name,
	 CASE WHEN st.last_day >= (now() AT TIME ZONE u.timezone)::date-1 THEN COALESCE(st.current_days,0) ELSE 0 END,
	 COALESCE(common.n,0),COALESCE(recent.n,0),COALESCE(latest.title,'')
	FROM peers u LEFT JOIN streaks st ON st.user_id=u.id
	LEFT JOIN common ON common.user_id=u.id LEFT JOIN recent ON recent.user_id=u.id
	LEFT JOIN LATERAL (
	 SELECT b.title FROM reading_progress rp
	 JOIN book_editions e ON e.id=rp.edition_id AND e.state='published'
	 JOIN books b ON b.id=e.book_id
	 WHERE rp.user_id=u.id AND u.visibility<>'private' AND NOT rp.is_private
	 ORDER BY rp.last_read_at DESC,e.id LIMIT 1
	) latest ON true
	ORDER BY COALESCE(common.n,0) DESC,u.handle`
	rows, err := s.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Friend{}
	for rows.Next() {
		var f Friend
		if err := rows.Scan(&f.ID, &f.Handle, &f.DisplayName, &f.StreakDays, &f.CommonBooks, &f.Pages30Days, &f.LatestBookTitle); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// ListPendingRequests returns pending requests involving this user, flagged by
// direction so the UI can show "accept" only on incoming ones.
func (s *Store) ListPendingRequests(ctx context.Context, userID string) ([]Friend, error) {
	const q = `
		SELECT u.id, u.handle, u.display_name, COALESCE(st.current_days, 0),
		       (f.addressee_id = $1) AS incoming
		FROM friendships f
		JOIN users u ON u.id = CASE WHEN f.requester_id = $1
		                            THEN f.addressee_id ELSE f.requester_id END
		LEFT JOIN streaks st ON st.user_id = u.id
		WHERE (f.requester_id = $1 OR f.addressee_id = $1)
		  AND f.state = 'pending'
		  AND u.deleted_at IS NULL
		ORDER BY f.created_at DESC`

	rows, err := s.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Friend{}
	for rows.Next() {
		var f Friend
		if err := rows.Scan(&f.ID, &f.Handle, &f.DisplayName, &f.StreakDays, &f.Incoming); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Store) queryFriends(ctx context.Context, q string, args ...any) ([]Friend, error) {
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Friend{}
	for rows.Next() {
		var f Friend
		if err := rows.Scan(&f.ID, &f.Handle, &f.DisplayName, &f.StreakDays); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// --------------------------------------------------------------- profiles

type Profile struct {
	ID          string `json:"id"`
	Handle      string `json:"handle"`
	DisplayName string `json:"display_name"`
	Visibility  string `json:"visibility"`
	StreakDays  int    `json:"streak_days"`
	LongestDays int    `json:"longest_days"`
	BooksRead   int    `json:"books_read"`
	// Reading is empty when the viewer is not allowed to see it.
	Reading []Activity `json:"reading"`
}

// ProfileFor returns another user's profile as this viewer is permitted to see it.
// The visibility decision lives here so that every caller gets it right.
func (s *Store) ProfileFor(ctx context.Context, viewerID, handle string) (*Profile, error) {
	target, err := s.UserByHandle(ctx, handle)
	if err != nil {
		return nil, err
	}

	blocked, err := s.IsBlocked(ctx, viewerID, target.ID)
	if err != nil {
		return nil, err
	}
	if blocked {
		// A blocked viewer sees what a stranger would see: nothing at all.
		return nil, ErrNotFound
	}

	friends, err := s.AreFriends(ctx, viewerID, target.ID)
	if err != nil {
		return nil, err
	}

	// Who may see the reading list at all.
	visible := false
	switch target.Visibility {
	case "public":
		visible = true
	case "friends":
		visible = friends
	case "private":
		visible = viewerID == target.ID
	}

	st, err := s.GetStreak(ctx, target.ID)
	if err != nil {
		return nil, err
	}

	p := &Profile{
		ID:          target.ID,
		Handle:      target.Handle,
		DisplayName: target.DisplayName,
		Visibility:  target.Visibility,
		StreakDays:  st.CurrentDays,
		LongestDays: st.LongestDays,
		Reading:     []Activity{},
	}
	// A stale streak must not keep showing a number.
	if !st.IsAlive(LocalDay(target.Timezone, time.Now())) {
		p.StreakDays = 0
	}

	if err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM reading_progress
		 WHERE user_id = $1 AND finished_at IS NOT NULL`, target.ID).
		Scan(&p.BooksRead); err != nil {
		return nil, err
	}

	if !visible {
		return p, nil
	}

	p.Reading, err = s.userReading(ctx, target.ID, viewerID == target.ID, 10)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// userReading lists what one user is reading. includePrivate is true only when the
// viewer is the owner.
func (s *Store) userReading(ctx context.Context, userID string, includePrivate bool, limit int) ([]Activity, error) {
	const q = `
		SELECT u.handle, u.display_name, b.title, b.slug, b.author,
		       CASE WHEN e.page_count > 0
		            THEN (rp.furthest_idx::float + 1) / e.page_count::float
		            ELSE 0 END,
		       rp.last_read_at
		FROM reading_progress rp
		JOIN users u ON u.id = rp.user_id
		JOIN book_editions e ON e.id = rp.edition_id AND e.state = 'published'
		JOIN books b ON b.id = e.book_id
		WHERE rp.user_id = $1
		  AND ($2 OR rp.is_private = false)
		ORDER BY rp.last_read_at DESC
		LIMIT $3`
	return s.queryActivity(ctx, q, userID, includePrivate, limit)
}

// --------------------------------------------------------------- activity feed

type Activity struct {
	UserHandle  string    `json:"user_handle"`
	DisplayName string    `json:"display_name"`
	BookTitle   string    `json:"book_title"`
	BookSlug    string    `json:"book_slug"`
	Author      string    `json:"author"`
	Progress    float64   `json:"progress"`
	LastReadAt  time.Time `json:"last_read_at"`
}

// FriendActivity returns what the viewer's friends are reading.
//
// Visibility is enforced in SQL rather than filtered in the handler, so a missing
// check in some future endpoint cannot leak private reading:
//   - the owner's account visibility must not be 'private'
//   - the individual book must not be flagged private
func (s *Store) FriendActivity(ctx context.Context, viewerID string, limit int) ([]Activity, error) {
	const q = `
		SELECT u.handle, u.display_name, b.title, b.slug, b.author,
		       CASE WHEN e.page_count > 0
		            THEN (rp.furthest_idx::float + 1) / e.page_count::float
		            ELSE 0 END,
		       rp.last_read_at
		FROM friendships f
		JOIN users u ON u.id = CASE WHEN f.requester_id = $1
		                            THEN f.addressee_id ELSE f.requester_id END
		JOIN reading_progress rp ON rp.user_id = u.id
		JOIN book_editions e ON e.id = rp.edition_id AND e.state = 'published'
		JOIN books b ON b.id = e.book_id
		WHERE (f.requester_id = $1 OR f.addressee_id = $1)
		  AND f.state = 'accepted'
		  AND u.deleted_at IS NULL
		  AND u.visibility <> 'private'
		  AND rp.is_private = false
		ORDER BY rp.last_read_at DESC
		LIMIT $2`
	return s.queryActivity(ctx, q, viewerID, limit)
}

func (s *Store) queryActivity(ctx context.Context, q string, args ...any) ([]Activity, error) {
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Activity{}
	for rows.Next() {
		var a Activity
		if err := rows.Scan(&a.UserHandle, &a.DisplayName, &a.BookTitle, &a.BookSlug,
			&a.Author, &a.Progress, &a.LastReadAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// SetBookPrivacy hides or reveals one book on the user's profile.
func (s *Store) SetBookPrivacy(ctx context.Context, userID, editionID string, private bool) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE reading_progress SET is_private = $3
		 WHERE user_id = $1 AND edition_id = $2`, userID, editionID, private)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SearchUsers finds people to add, excluding private accounts and anyone involved
// in a block with the viewer.
func (s *Store) SearchUsers(ctx context.Context, viewerID, query string, limit int) ([]Friend, error) {
	const q = `
		SELECT u.id, u.handle, u.display_name, COALESCE(st.current_days, 0)
		FROM users u
		LEFT JOIN streaks st ON st.user_id = u.id
		WHERE u.deleted_at IS NULL
		  AND u.id <> $1
		  AND u.visibility <> 'private'
		  AND (u.handle ILIKE $2 || '%' OR u.display_name ILIKE '%' || $2 || '%')
		  AND NOT EXISTS (
		      SELECT 1 FROM friendships f
		      WHERE f.state = 'blocked'
		        AND ((f.requester_id = $1 AND f.addressee_id = u.id)
		          OR (f.requester_id = u.id AND f.addressee_id = $1)))
		ORDER BY u.handle
		LIMIT $3`
	return s.queryFriends(ctx, q, viewerID, query, limit)
}
