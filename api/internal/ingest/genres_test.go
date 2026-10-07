package ingest

import "testing"

func TestValidateGenres(t *testing.T) {
	for _, genres := range [][]string{nil, {}, {"fantasy", "children"}, {"a", "b", "c", "d", "e"}} {
		if err := ValidateGenres(genres); err != nil {
			t.Fatal(err)
		}
	}
	for _, genres := range [][]string{{""}, {"Fantasy"}, {"science fiction"}, {"fantasy", "fantasy"}, {"a", "b", "c", "d", "e", "f"}} {
		if ValidateGenres(genres) == nil {
			t.Fatalf("accepted invalid genres: %v", genres)
		}
	}
}
