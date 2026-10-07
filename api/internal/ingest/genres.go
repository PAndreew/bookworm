package ingest

import (
	"fmt"
	"regexp"
)

var genrePattern = regexp.MustCompile(`^[a-z]+(?:-[a-z]+)*$`)

func ValidateGenres(genres []string) error {
	if len(genres) > 5 {
		return fmt.Errorf("ingest: a book may have at most five genres")
	}
	seen := map[string]bool{}
	for _, genre := range genres {
		if !genrePattern.MatchString(genre) || seen[genre] {
			return fmt.Errorf("ingest: invalid or duplicate genre %q", genre)
		}
		seen[genre] = true
	}
	return nil
}
