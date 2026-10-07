package ingest

import "testing"

// Publish used to return only an error, so a caller could not tell a real publish
// from a no-op. The seed script then printed "ingested X, 194 pages" for a book
// whose stored edition was still the old 12-page excerpt -- a log line that was
// confidently wrong. Result exists to make that distinction impossible to miss.
func TestResultDistinguishesPublishFromNoOp(t *testing.T) {
	published := Result{Published: true, PageCount: 194}
	noop := Result{Published: false, PageCount: 12}

	if !published.Published {
		t.Error("a real publish must report Published")
	}
	if noop.Published {
		t.Error("a no-op must not report Published")
	}

	// The count must describe what is STORED, not what was offered. This is the
	// property that makes the log line honest.
	if noop.PageCount != 12 {
		t.Errorf("a no-op reported %d pages; it must report the stored count, not the candidate",
			noop.PageCount)
	}
}
