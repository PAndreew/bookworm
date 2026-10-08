package store

import (
	"testing"
	"time"
)

func TestStreakDatabaseDateAndLocalDay(t *testing.T) {
	for _, zone := range []string{"Europe/Budapest", "Asia/Tokyo", "America/New_York"} {
		t.Run(zone, func(t *testing.T) {
			loc, err := time.LoadLocation(zone)
			if err != nil {
				t.Fatal(err)
			}
			// PostgreSQL DATE is decoded at UTC midnight, not in the user's zone.
			s := Streak{CurrentDays: 1, LongestDays: 1, LastDay: day(2026, 10, 7)}
			today := time.Date(2026, 10, 8, 0, 0, 0, 0, loc)
			got := ApplyRead(s, today, 7)
			if !got.Extended || got.CurrentDays != 2 {
				t.Fatalf("qualifying local day did not advance: %+v", got)
			}
			if !s.AtRisk(today) {
				t.Fatal("yesterday's streak must be at risk")
			}
			if s.IsAlive(today.AddDate(0, 0, 1)) {
				t.Fatal("two missed calendar days must expire the streak")
			}
			if ApplyRead(got.Streak, today, 8).Extended {
				t.Fatal("same local day counted twice")
			}
		})
	}
}

func TestStreakAcrossSpringDST(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Budapest")
	if err != nil {
		t.Fatal(err)
	}
	last := time.Date(2026, 3, 29, 0, 0, 0, 0, loc)
	today := last.AddDate(0, 0, 1)
	s := Streak{CurrentDays: 4, LongestDays: 4, LastDay: last}
	if got := ApplyRead(s, today, 3); !got.Extended || got.CurrentDays != 5 {
		t.Fatalf("23-hour calendar day failed: %+v", got)
	}
}
