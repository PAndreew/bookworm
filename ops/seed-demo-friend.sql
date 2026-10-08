-- Explicitly fictional fixture, only linked to the requested test account.
-- Requires the current reader-card schema. The randomly generated password was
-- discarded; no known login credential, mail delivery, read credit or ratings.
-- Run with psql -v ON_ERROR_STOP=1. Safe to rerun without resetting progress.
BEGIN;
DO $$
DECLARE target uuid; demo uuid; n integer;
BEGIN
 SELECT id INTO target FROM users WHERE email='test@test.com' AND deleted_at IS NULL;
 IF target IS NULL THEN RAISE EXCEPTION 'Requested test@test.com account does not exist'; END IF;

 INSERT INTO users(email,handle,display_name,password_hash,birthdate,timezone,visibility)
 VALUES('reading-demo@example.invalid','puri_demo_reader','Demo reader (fictional)',
 '$argon2id$v=19$m=19456,t=2,p=1$YeMQPdam/wEuq18dySf1ig$MFOvT34cG0kFpfxZ8eeuwGVYp3VoBvHd5ImoZXlkyzE',
 '1990-01-01','UTC','friends') ON CONFLICT(email) DO NOTHING;
 SELECT id INTO demo FROM users WHERE email='reading-demo@example.invalid'
 AND handle='puri_demo_reader' AND display_name='Demo reader (fictional)' AND deleted_at IS NULL;
 IF demo IS NULL THEN RAISE EXCEPTION 'Demo identity conflict; will not overwrite an existing account'; END IF;
 IF EXISTS(SELECT 1 FROM friendships WHERE state='blocked'
 AND LEAST(requester_id,addressee_id)=LEAST(target,demo)
 AND GREATEST(requester_id,addressee_id)=GREATEST(target,demo)) THEN
  RAISE EXCEPTION 'An existing block must not be overridden';
 END IF;
 INSERT INTO friendships(requester_id,addressee_id,state,decided_at)
 VALUES(demo,target,'accepted',now())
 ON CONFLICT(LEAST(requester_id,addressee_id),GREATEST(requester_id,addressee_id))
 DO UPDATE SET state='accepted',decided_at=COALESCE(friendships.decided_at,now());

 SELECT count(*) INTO n FROM books b JOIN book_editions e ON e.book_id=b.id AND e.state='published'
 WHERE b.slug IN ('gutenberg-46','gutenberg-4363','the-yellow-wallpaper','gutenberg-86');
 IF n<>4 THEN RAISE EXCEPTION 'Expected all four demo books to be published'; END IF;

 INSERT INTO reading_progress(user_id,edition_id,last_page_idx,furthest_idx,last_card_idx,last_read_at)
 SELECT demo,e.id,COALESCE(p.idx,0),COALESCE(p.idx,0),c.idx,
 now()-CASE b.slug WHEN 'gutenberg-46' THEN interval '1 hour'
 WHEN 'the-yellow-wallpaper' THEN interval '1 day'
 WHEN 'gutenberg-4363' THEN interval '2 days' ELSE interval '3 days' END
 FROM books b JOIN book_editions e ON e.book_id=b.id AND e.state='published'
 JOIN reader_cards c ON c.edition_id=e.id AND c.idx=5 AND NOT c.archived
 LEFT JOIN pages p ON p.id=c.page_id
 WHERE b.slug IN ('gutenberg-46','gutenberg-4363','the-yellow-wallpaper','gutenberg-86')
 ON CONFLICT(user_id,edition_id) DO NOTHING;
 SELECT count(*) INTO n FROM reading_progress rp JOIN book_editions e ON e.id=rp.edition_id
 JOIN books b ON b.id=e.book_id WHERE rp.user_id=demo
 AND b.slug IN ('gutenberg-46','gutenberg-4363','the-yellow-wallpaper','gutenberg-86');
 IF n<>4 THEN RAISE EXCEPTION 'Missing current cards; demo fixture rolled back'; END IF;
END $$;
COMMIT;
