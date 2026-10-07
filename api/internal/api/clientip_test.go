package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Regression: an IPv6 RemoteAddr ("[::1]:54321") was being split on the last colon,
// yielding "[::1]", which Postgres rejects as an inet. Signup returned a bare 500.
func TestClientIPHandlesIPv6(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		xff        string
		want       string
	}{
		{"ipv6 loopback", "[::1]:54321", "", "::1"},
		{"ipv6 full", "[2001:db8::1]:443", "", "2001:db8::1"},
		{"ipv4", "192.0.2.5:443", "", "192.0.2.5"},
		{"xff single", "10.0.0.1:1", "203.0.113.9", "203.0.113.9"},
		{"xff chain takes the client", "10.0.0.1:1", "203.0.113.9, 70.41.3.18", "203.0.113.9"},
		{"xff ipv6", "10.0.0.1:1", "2001:db8::2", "2001:db8::2"},
		// Anything unparseable must become "", which the store writes as NULL.
		// Returning junk here is what caused the original 500.
		{"garbage xff falls through", "192.0.2.5:443", "not-an-ip", "192.0.2.5"},
		{"garbage everything", "unparseable", "", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = tc.remoteAddr
			if tc.xff != "" {
				r.Header.Set("X-Forwarded-For", tc.xff)
			}

			if got := clientIP(r); got != tc.want {
				t.Errorf("clientIP() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestAgeOn(t *testing.T) {
	// ageOn decides who is allowed an account, so the boundary matters.
	tests := []struct {
		birth string
		at    string
		want  int
	}{
		{"2013-06-15", "2026-06-15", 13}, // exactly 13 today
		{"2013-06-15", "2026-06-14", 12}, // one day short
		{"2013-06-15", "2026-06-16", 13},
		{"2000-01-01", "2026-12-31", 26},
	}

	for _, tc := range tests {
		birth := mustDate(t, tc.birth)
		at := mustDate(t, tc.at)

		if got := ageOn(birth, at); got != tc.want {
			t.Errorf("ageOn(%s, %s) = %d, want %d", tc.birth, tc.at, got, tc.want)
		}
	}
}

func TestMinAgeIsThirteen(t *testing.T) {
	// A guard on the product decision: changing this is a legal change, not a
	// tuning knob. See docs/LEGAL.md.
	if MinAge != 13 {
		t.Errorf("MinAge = %d, want 13", MinAge)
	}
}

func mustDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("parsing %s: %v", s, err)
	}
	return d
}
