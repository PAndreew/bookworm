package store

import "time"

// PagesPerStreakDay is how many pages count as "read today". Three is low enough to
// be achievable on a bad day -- a streak you cannot keep is a streak you abandon --
// but high enough that it is not satisfied by an accidental swipe.
const PagesPerStreakDay = 3

// Streak is a user's reading streak.
type Streak struct {
	CurrentDays int
	LongestDays int
	LastDay     time.Time // zero means "never read"
	FreezesLeft int
}

// StreakUpdate is the result of applying a day's reading to a streak.
type StreakUpdate struct {
	Streak
	// Extended is true when this update pushed the streak to a new day, which is
	// the moment worth celebrating in the UI.
	Extended bool
	// FreezeUsed is true when a gap was absorbed by a freeze instead of resetting.
	FreezeUsed bool
	// Broken is true when a gap reset the count to 1.
	Broken bool
}

// ApplyRead folds a day's reading into a streak.
//
//   - Fewer than PagesPerStreakDay pages: nothing changes. The day does not count.
//   - Same day again: nothing changes. The streak advances once per day, not per page.
//   - The next day: the streak extends.
//   - A gap of exactly one missed day with a freeze available: the freeze is spent
//     and the streak survives.
//   - Any larger gap, or no freeze: the streak resets to 1.
//
// day must be the user's LOCAL day (see LocalDay), not a UTC date.
func ApplyRead(s Streak, day time.Time, pagesReadThatDay int) StreakUpdate {
	if pagesReadThatDay < PagesPerStreakDay {
		return StreakUpdate{Streak: s}
	}

	day = truncateDay(day)

	// First ever qualifying day.
	if s.LastDay.IsZero() {
		s.CurrentDays = 1
		s.LongestDays = maxInt(s.LongestDays, 1)
		s.LastDay = day
		return StreakUpdate{Streak: s, Extended: true}
	}

	last := truncateDay(s.LastDay)
	gap := int(day.Sub(last).Hours() / 24)

	switch {
	case gap <= 0:
		// Already counted today, or a late-arriving read for an earlier day.
		return StreakUpdate{Streak: s}

	case gap == 1:
		s.CurrentDays++
		s.LongestDays = maxInt(s.LongestDays, s.CurrentDays)
		s.LastDay = day
		return StreakUpdate{Streak: s, Extended: true}

	case gap == 2 && s.FreezesLeft > 0:
		// Exactly one day missed and a freeze to spend: the streak survives.
		s.FreezesLeft--
		s.CurrentDays++
		s.LongestDays = maxInt(s.LongestDays, s.CurrentDays)
		s.LastDay = day
		return StreakUpdate{Streak: s, Extended: true, FreezeUsed: true}

	default:
		s.CurrentDays = 1
		s.LongestDays = maxInt(s.LongestDays, 1)
		s.LastDay = day
		return StreakUpdate{Streak: s, Extended: true, Broken: true}
	}
}

// IsAlive reports whether the streak is still running as of today. A streak shown in
// the UI must go stale on its own: a user who last read three days ago should not
// still see "7 day streak" just because nothing has written to the row since.
func (s Streak) IsAlive(today time.Time) bool {
	if s.LastDay.IsZero() {
		return false
	}
	gap := int(truncateDay(today).Sub(truncateDay(s.LastDay)).Hours() / 24)
	return gap <= 1
}

// AtRisk reports whether the user must read today or lose the streak.
func (s Streak) AtRisk(today time.Time) bool {
	if s.LastDay.IsZero() || s.CurrentDays == 0 {
		return false
	}
	gap := int(truncateDay(today).Sub(truncateDay(s.LastDay)).Hours() / 24)
	return gap == 1
}

// truncateDay drops any time component, keeping the location.
func truncateDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
