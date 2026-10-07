package store

import (
	"testing"
	"time"
)

// ------------------------------------------------------------------ metering

func TestPublicDomainIsFreeOnEveryTier(t *testing.T) {
	// The core promise of the free tier. If this ever fails, the product is broken.
	for _, tier := range []Tier{TierFree, TierPlus, TierPro} {
		d := DecideMeter(tier, true, false, 99999)
		if !d.Allowed {
			t.Errorf("tier %s was denied a public-domain page", tier)
		}
		if d.Metered {
			t.Errorf("tier %s was metered for a public-domain page", tier)
		}
	}
}

func TestFreeTierCannotReadLicensedContent(t *testing.T) {
	d := DecideMeter(TierFree, false, false, 0)
	if d.Allowed {
		t.Error("free tier was allowed a licensed page")
	}
	if d.Reason != "upgrade_required" {
		t.Errorf("Reason = %q, want upgrade_required", d.Reason)
	}
}

func TestLicensedPagesCountAgainstTheDailyLimit(t *testing.T) {
	tests := []struct {
		tier      Tier
		usedToday int
		want      bool
	}{
		{TierPlus, 0, true},
		{TierPlus, 29, true},  // last page of the allowance
		{TierPlus, 30, false}, // exhausted
		{TierPro, 149, true},
		{TierPro, 150, false},
	}

	for _, tc := range tests {
		d := DecideMeter(tc.tier, false, false, tc.usedToday)
		if d.Allowed != tc.want {
			t.Errorf("%s with %d used: Allowed = %v, want %v",
				tc.tier, tc.usedToday, d.Allowed, tc.want)
		}
		if !tc.want && d.Reason != "daily_limit_reached" {
			t.Errorf("%s with %d used: Reason = %q, want daily_limit_reached",
				tc.tier, tc.usedToday, d.Reason)
		}
	}
}

func TestRereadingAPageTodayIsFree(t *testing.T) {
	// Swiping back to re-read must not burn quota, even at the cap -- otherwise
	// the meter punishes ordinary reading behaviour.
	d := DecideMeter(TierPlus, false, true, 30)
	if !d.Allowed {
		t.Error("re-reading a page already read today was denied at the cap")
	}
	if d.Metered {
		t.Error("re-reading a page already read today consumed quota")
	}
}

func TestRemainingCountsDown(t *testing.T) {
	d := DecideMeter(TierPlus, false, false, 10)
	if d.Remaining != 19 {
		t.Errorf("Remaining = %d, want 19 (30 limit - 10 used - 1 for this page)", d.Remaining)
	}
}

func TestLocalDayUsesTheUsersTimezone(t *testing.T) {
	// 2026-03-15 23:30 UTC is already the 16th in Auckland and still the 15th in
	// Los Angeles. Two users reading at the same instant are on different days.
	at := time.Date(2026, 3, 15, 23, 30, 0, 0, time.UTC)

	if got := LocalDay("Pacific/Auckland", at).Format("2006-01-02"); got != "2026-03-16" {
		t.Errorf("Auckland local day = %s, want 2026-03-16", got)
	}
	if got := LocalDay("America/Los_Angeles", at).Format("2006-01-02"); got != "2026-03-15" {
		t.Errorf("Los Angeles local day = %s, want 2026-03-15", got)
	}
}

func TestLocalDayFallsBackToUTCOnBadTimezone(t *testing.T) {
	at := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	for _, tz := range []string{"", "Not/AZone"} {
		if got := LocalDay(tz, at).Format("2006-01-02"); got != "2026-03-15" {
			t.Errorf("LocalDay(%q) = %s, want the UTC date 2026-03-15", tz, got)
		}
	}
}

func TestNextResetIsLocalMidnight(t *testing.T) {
	at := time.Date(2026, 3, 15, 23, 30, 0, 0, time.UTC)
	reset := NextResetAt("America/Los_Angeles", at)

	if got := reset.Format("2006-01-02"); got != "2026-03-16" {
		t.Errorf("reset date = %s, want 2026-03-16", got)
	}
	if reset.Hour() != 0 {
		t.Errorf("reset at hour %d, want local midnight", reset.Hour())
	}
}

// ------------------------------------------------------------------- streaks

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestStreakStartsAtOne(t *testing.T) {
	got := ApplyRead(Streak{}, day(2026, 3, 15), PagesPerStreakDay)

	if got.CurrentDays != 1 {
		t.Errorf("CurrentDays = %d, want 1", got.CurrentDays)
	}
	if !got.Extended {
		t.Error("Extended = false on the first qualifying day")
	}
}

func TestStreakNeedsEnoughPages(t *testing.T) {
	got := ApplyRead(Streak{}, day(2026, 3, 15), PagesPerStreakDay-1)

	if got.CurrentDays != 0 {
		t.Errorf("CurrentDays = %d after too few pages, want 0", got.CurrentDays)
	}
	if got.Extended {
		t.Error("Extended = true despite not reaching the page threshold")
	}
}

func TestStreakExtendsOnConsecutiveDays(t *testing.T) {
	s := Streak{CurrentDays: 4, LongestDays: 4, LastDay: day(2026, 3, 15)}

	got := ApplyRead(s, day(2026, 3, 16), PagesPerStreakDay)

	if got.CurrentDays != 5 {
		t.Errorf("CurrentDays = %d, want 5", got.CurrentDays)
	}
	if got.LongestDays != 5 {
		t.Errorf("LongestDays = %d, want 5", got.LongestDays)
	}
}

func TestStreakAdvancesOncePerDayNotPerPage(t *testing.T) {
	s := Streak{CurrentDays: 4, LongestDays: 9, LastDay: day(2026, 3, 15)}

	got := ApplyRead(s, day(2026, 3, 15), 50) // same day, lots of pages

	if got.CurrentDays != 4 {
		t.Errorf("CurrentDays = %d after more reading on the same day, want 4", got.CurrentDays)
	}
	if got.Extended {
		t.Error("Extended = true for a second read on a day already counted")
	}
}

func TestStreakBreaksAfterAGap(t *testing.T) {
	s := Streak{CurrentDays: 12, LongestDays: 12, LastDay: day(2026, 3, 10)}

	got := ApplyRead(s, day(2026, 3, 15), PagesPerStreakDay)

	if got.CurrentDays != 1 {
		t.Errorf("CurrentDays = %d after a five-day gap, want 1", got.CurrentDays)
	}
	if !got.Broken {
		t.Error("Broken = false after a five-day gap")
	}
	if got.LongestDays != 12 {
		t.Errorf("LongestDays = %d, want the record 12 to be preserved", got.LongestDays)
	}
}

func TestFreezeAbsorbsASingleMissedDay(t *testing.T) {
	s := Streak{CurrentDays: 7, LongestDays: 7, LastDay: day(2026, 3, 14), FreezesLeft: 1}

	got := ApplyRead(s, day(2026, 3, 16), PagesPerStreakDay) // the 15th was missed

	if got.Broken {
		t.Error("Broken = true despite a freeze being available")
	}
	if !got.FreezeUsed {
		t.Error("FreezeUsed = false; the freeze should have been spent")
	}
	if got.CurrentDays != 8 {
		t.Errorf("CurrentDays = %d, want 8", got.CurrentDays)
	}
	if got.FreezesLeft != 0 {
		t.Errorf("FreezesLeft = %d, want 0 after spending the only freeze", got.FreezesLeft)
	}
}

func TestFreezeDoesNotCoverALongerGap(t *testing.T) {
	s := Streak{CurrentDays: 7, LongestDays: 7, LastDay: day(2026, 3, 10), FreezesLeft: 1}

	got := ApplyRead(s, day(2026, 3, 16), PagesPerStreakDay)

	if !got.Broken {
		t.Error("Broken = false; one freeze must not cover a six-day gap")
	}
	if got.FreezesLeft != 1 {
		t.Errorf("FreezesLeft = %d, want the freeze left unspent on an uncoverable gap", got.FreezesLeft)
	}
}

func TestStreakGoesStaleWithoutBeingWrittenTo(t *testing.T) {
	// A streak row is only updated when the user reads. The UI must not keep
	// showing "7 day streak" to someone who stopped reading three days ago.
	s := Streak{CurrentDays: 7, LastDay: day(2026, 3, 10)}

	if s.IsAlive(day(2026, 3, 14)) {
		t.Error("IsAlive = true four days after the last read")
	}
	if !s.IsAlive(day(2026, 3, 11)) {
		t.Error("IsAlive = false the day after the last read; there is still time to read today")
	}
	if !s.IsAlive(day(2026, 3, 10)) {
		t.Error("IsAlive = false on the day of the last read")
	}
}

func TestAtRiskOnlyOnTheDeadlineDay(t *testing.T) {
	s := Streak{CurrentDays: 7, LastDay: day(2026, 3, 10)}

	if s.AtRisk(day(2026, 3, 10)) {
		t.Error("AtRisk = true on a day already read")
	}
	if !s.AtRisk(day(2026, 3, 11)) {
		t.Error("AtRisk = false on the day the streak must be renewed")
	}
	if s.AtRisk(day(2026, 3, 13)) {
		t.Error("AtRisk = true for an already-dead streak; it is lost, not at risk")
	}
}

func TestZeroStreakIsNeverAtRisk(t *testing.T) {
	if (Streak{}).AtRisk(day(2026, 3, 15)) {
		t.Error("a user who has never read should not be told their streak is at risk")
	}
}
