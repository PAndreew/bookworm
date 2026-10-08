-- Repair the UTC/local-date comparison bug for already-earned consecutive days.
-- Only actual reads count; inherited rechunking coverage is excluded.
-- Idempotent: after repair last_day is today and cannot match yesterday again.
BEGIN;
LOCK TABLE streaks IN SHARE ROW EXCLUSIVE MODE;
WITH local_dates AS (
  SELECT id, (now() AT TIME ZONE timezone)::date AS today FROM users
), qualifying AS (
  SELECT u.id, u.today FROM local_dates u
  WHERE (SELECT count(*) FROM page_reads p WHERE p.user_id=u.id AND p.local_day=u.today)
      + (SELECT count(*) FROM card_reads c WHERE c.user_id=u.id AND c.local_day=u.today AND NOT c.inherited) >= 3
)
UPDATE streaks s
SET current_days=s.current_days+1,
    longest_days=GREATEST(s.longest_days,s.current_days+1),
    last_day=q.today
FROM qualifying q
WHERE s.user_id=q.id AND s.last_day=q.today-1 AND s.current_days>0
RETURNING s.current_days,s.longest_days,s.last_day;
COMMIT;
