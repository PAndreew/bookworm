package main

import (
	"context"
	_ "embed"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/bookworm/api/internal/chunk"
	"github.com/bookworm/api/internal/ingest"
)

//go:embed catalogue.csv
var catalogue string

type catalogueEntry struct {
	ID            int
	Title, Author string
	Genres        []string
}

var titleHeader = regexp.MustCompile(`(?m)^Title:[ \t]*(.+)`)

func matchesSourceTitle(expected, text string) bool {
	match := titleHeader.FindStringSubmatch(text)
	if len(match) != 2 {
		return false
	}
	normalize := func(s string) string {
		return strings.Map(func(r rune) rune {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				return unicode.ToLower(r)
			}
			return -1
		}, s)
	}
	want, actual := normalize(expected), normalize(match[1])
	return want != "" && actual != "" && (strings.HasPrefix(actual, want) || strings.HasPrefix(want, actual))
}

func readCatalogue() ([]catalogueEntry, error) {
	reader := csv.NewReader(strings.NewReader(catalogue))
	reader.Comma = '|'
	if _, err := reader.Read(); err != nil {
		return nil, err
	}
	entries := []catalogueEntry{}
	seen := map[int]bool{}
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		id, err := strconv.Atoi(row[0])
		if err != nil || id <= 0 || seen[id] || row[1] == "" || row[2] == "" {
			return nil, fmt.Errorf("invalid catalogue entry: %v", row)
		}
		genres := strings.Split(row[3], ",")
		if err := ingest.ValidateGenres(genres); err != nil {
			return nil, err
		}
		seen[id] = true
		entries = append(entries, catalogueEntry{id, row[1], row[2], genres})
	}
	return entries, nil
}

// expandShelf is deliberately sequential and paced. Re-running is safe: existing
// source IDs are skipped, and Publish never replaces an existing edition's pages.
func expandShelf(ctx context.Context, ing *ingest.Ingester) error {
	entries, err := readCatalogue()
	if err != nil {
		return err
	}
	successful, added, failed := 0, 0, 0
	for _, entry := range entries {
		source := fmt.Sprintf("gutenberg:%d", entry.ID)
		exists, err := ing.HasSource(ctx, source)
		if err != nil {
			return err
		}
		if exists {
			successful++
			continue
		}
		text, err := fetchGutenbergText(ctx, entry.ID)
		if err != nil {
			log.Printf("skipping %d: %v", entry.ID, err)
			failed++
			continue
		}
		// Do not publish an HTML error page or an unrecognised source as a book.
		if !strings.Contains(strings.ToUpper(text), "*** START OF") || !strings.Contains(strings.ToUpper(text), "GUTENBERG") {
			log.Printf("skipping %d: missing Gutenberg text markers", entry.ID)
			failed++
			continue
		}
		if !matchesSourceTitle(entry.Title, text) {
			log.Printf("skipping %d: source title does not match %q", entry.ID, entry.Title)
			failed++
			continue
		}
		cleaned := chunk.StripGutenbergBoilerplate(text)
		book := ingest.Book{
			Slug: fmt.Sprintf("gutenberg-%d", entry.ID), Title: entry.Title,
			Author: entry.Author, Genres: entry.Genres, Language: "en",
			License: "public_domain", SourceRef: source,
			Pages: chunk.Split(cleaned), WordCount: len(strings.Fields(cleaned)),
		}
		result, err := ing.Publish(ctx, book)
		if err != nil {
			log.Printf("failed %d: %v", entry.ID, err)
			failed++
			continue
		}
		successful++
		if result.Published {
			added++
		}
		log.Printf("published %d: %s (%d pages)", entry.ID, book.Title, result.PageCount)
		time.Sleep(time.Second)
	}
	log.Printf("expanded shelf: %d new, %d available, %d failed", added, successful, failed)
	if successful < 100 {
		return fmt.Errorf("only %d expanded titles available; need at least 100; rerun to retry failed sources", successful)
	}
	return nil
}
