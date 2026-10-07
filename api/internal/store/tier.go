package store

import "time"

// Tier is the subscription level.
type Tier string

const (
	TierFree Tier = "free"
	TierPlus Tier = "plus"
	TierPro  Tier = "pro"
)

// DailyLicensedPageLimit is the number of pages of NON-public-domain content a tier
// may read per local day. Public-domain books are always unlimited on every tier --
// that is the promise of the free tier and it is enforced in MeterDecision below,
// not here.
func (t Tier) DailyLicensedPageLimit() int {
	switch t {
	case TierPlus:
		return 30
	case TierPro:
		return 150
	default:
		return 0
	}
}

// Valid reports whether t is a known tier.
func (t Tier) Valid() bool {
	switch t {
	case TierFree, TierPlus, TierPro:
		return true
	}
	return false
}

// LocalDay returns the calendar date at the given instant in the user's timezone.
// The streak day boundary and the quota reset both key off this, so a user in
// Auckland and a user in Los Angeles roll over at their own midnights.
//
// An unknown or empty timezone falls back to UTC rather than failing: a bad
// timezone string must never block someone from reading.
func LocalDay(tz string, at time.Time) time.Time {
	loc, err := time.LoadLocation(tz)
	if err != nil || tz == "" {
		loc = time.UTC
	}
	y, m, d := at.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}

// MeterDecision is the answer to "may this user turn to this page right now?".
type MeterDecision struct {
	Allowed bool
	// Metered reports whether this read consumes quota. Public-domain pages and
	// pages already read today are free.
	Metered bool
	// Remaining is the quota left AFTER this read, for licensed content only.
	Remaining int
	// Reason is a machine-readable code for the client when Allowed is false.
	Reason string
}

// DecideMeter applies the tier rules for a single page turn.
//
//   - Public-domain content is always allowed and never metered, on every tier.
//   - A page already read today is free: re-reading or swiping back must not
//     burn quota, or the meter would punish normal reading behaviour.
//   - Otherwise the read is metered against the tier's daily limit.
func DecideMeter(t Tier, isPublicDomain, alreadyReadToday bool, usedToday int) MeterDecision {
	if isPublicDomain {
		return MeterDecision{Allowed: true, Metered: false, Remaining: -1}
	}
	if alreadyReadToday {
		limit := t.DailyLicensedPageLimit()
		return MeterDecision{Allowed: true, Metered: false, Remaining: max(limit-usedToday, 0)}
	}

	limit := t.DailyLicensedPageLimit()
	if limit == 0 {
		return MeterDecision{Allowed: false, Reason: "upgrade_required"}
	}
	if usedToday >= limit {
		return MeterDecision{Allowed: false, Reason: "daily_limit_reached"}
	}

	return MeterDecision{Allowed: true, Metered: true, Remaining: limit - usedToday - 1}
}

// NextResetAt is when the user's quota rolls over: their next local midnight.
// The client shows this as "resumes in 4h".
func NextResetAt(tz string, at time.Time) time.Time {
	return LocalDay(tz, at).AddDate(0, 0, 1)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
