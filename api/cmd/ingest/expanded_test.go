package main

import (
	"strings"
	"testing"
)

func TestMatchesSourceTitle(t *testing.T) {
	for _, text := range []string{"Title: Jane Eyre: An Autobiography\r\n", "Title: Essays — First Series\n"} {
		expected := "Jane Eyre"
		if strings.Contains(text, "Essays") {
			expected = "Essays, First Series"
		}
		if !matchesSourceTitle(expected, text) {
			t.Fatalf("rejected %q", text)
		}
	}
	for _, text := range []string{"Title: Familiar Studies of Men and Books\n", "<html>Error</html>", "Title: \n"} {
		if matchesSourceTitle("Robinson Crusoe", text) {
			t.Fatalf("accepted wrong source: %q", text)
		}
	}
}

func TestExpandedCatalogue(t *testing.T) {
	entries, err := readCatalogue()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) < 100 {
		t.Fatalf("only %d titles", len(entries))
	}
	genres := map[string]bool{}
	starterIDs := map[int]bool{}
	for _, entry := range starterShelf {
		starterIDs[entry.ID] = true
	}
	for _, entry := range entries {
		if starterIDs[entry.ID] {
			t.Fatalf("duplicate starter title: %d", entry.ID)
		}
		for _, genre := range entry.Genres {
			genres[genre] = true
		}
	}
	if len(genres) < 15 {
		t.Fatalf("not enough genre diversity: %d", len(genres))
	}
}
