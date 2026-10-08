# Streak calendar dates

Streaks compare calendar dates, not elapsed hours. PostgreSQL `DATE` values
are decoded at UTC midnight, whereas `LocalDay` uses the reader's timezone.
Normalize each value's own year/month/day to UTC before subtracting dates;
never convert the stored date into a different timezone. This also handles
23-hour spring DST days.

The October 8, 2026 production incident had seven actual card reads that day,
but the stored streak remained on October 7. Budapest local midnight was only
22 hours after the stored UTC date, so the old integer hour division returned
zero. Rechunking had not removed the reads. Regression coverage lives in
`api/internal/store/streak_timezone_test.go`.

After deploying the fix, `ops/restore-streak-date.sql` restores a qualifying
consecutive day missed by this bug. Back up streaks before running it. It is
idempotent, preserves freeze balances, and excludes inherited card coverage.
It does not fabricate reading days or rebuild missing historical evidence.
