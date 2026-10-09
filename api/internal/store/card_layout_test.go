package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

func testLayout(t *testing.T, sources []cardSource, title, author string) []layoutCard {
	t.Helper()
	cards, err := buildCardLayout(context.Background(), sources, title, author)
	if err != nil {
		t.Fatal(err)
	}
	return cards
}

func TestLayoutRepairsMrSourceBoundaryAndPreservesOffsets(t *testing.T) {
	sources := []cardSource{{id: "p1", content: "Jane had heard of Mr.", chapter: "Chapter 1"}, {id: "p2", content: "Bingley and his sister. " + strings.Repeat("An ordinary sentence follows. ", 40), chapter: "Chapter 1"}}
	cards := testLayout(t, sources, "Title", "Author")
	var contents []string
	found := false
	for _, card := range cards {
		if card.kind != "text" {
			continue
		}
		contents = append(contents, card.content)
		if strings.Contains(card.content, "Mr. Bingley") {
			found = true
		}
		r := []rune(card.content)
		if len(card.sentenceEnds) == 0 || card.sentenceEnds[len(card.sentenceEnds)-1] != len(r) {
			t.Fatal("missing persisted sentence boundary", card.content, card.sentenceEnds)
		}
		for _, part := range card.parts {
			var original string
			for _, p := range sources {
				if p.id == part.PageID {
					original = p.content
				}
			}
			if string(r[part.CardStart:part.CardStart+part.EndOff-part.StartOff]) != string([]rune(original)[part.StartOff:part.EndOff]) {
				t.Fatal("lost original annotation anchors")
			}
		}
	}
	if !found || strings.Contains(strings.Join(contents, " "), "Mr.\n") {
		t.Fatal("fake paragraph after Mr.")
	}
	if cards[0].kind != "title" || cards[1].kind != "chapter" {
		t.Fatal("missing headings")
	}
}

// Optional real-text preview before any live layout is replaced.
func TestRealBookLayout(t *testing.T) {
	path := os.Getenv("CARD_TEST_SOURCE")
	if path == "" {
		t.Skip("set CARD_TEST_SOURCE to a JSON source-page export")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var source []struct {
		Content string `json:"content"`
		Chapter string `json:"chapter"`
	}
	if err = json.Unmarshal(data, &source); err != nil {
		t.Fatal(err)
	}
	sources := make([]cardSource, len(source))
	for i, p := range source {
		sources[i] = cardSource{id: fmt.Sprintf("source-%d", i), content: p.Content, chapter: p.Chapter, idx: i}
	}
	cards := testLayout(t, sources, "Pride and Prejudice", "Jane Austen")
	preview := make([]ReaderCard, len(cards))
	normal, total := 0, 0
	for i, c := range cards {
		preview[i] = ReaderCard{Page: Page{ID: c.page, Idx: i, Content: c.content, CharCount: len([]rune(c.content))}, CardID: fmt.Sprintf("preview-%d", i), Kind: c.kind, StartOff: c.start, Parts: c.parts, SentenceEnds: c.sentenceEnds}
		if c.kind != "text" {
			continue
		}
		total++
		n := len([]rune(c.content))
		if n <= 480 && len(strings.Fields(c.content)) <= 85 {
			normal++
		}
		for _, p := range c.parts {
			var original string
			for _, s := range sources {
				if s.id == p.PageID {
					original = s.content
					break
				}
			}
			if string([]rune(c.content)[p.CardStart:p.CardStart+p.EndOff-p.StartOff]) != string([]rune(original)[p.StartOff:p.EndOff]) {
				t.Fatal("real-text annotation mapping mismatch")
			}
		}
	}
	t.Logf("real book: %d text cards; %d (%.1f%%) within sentence-card budgets", total, normal, 100*float64(normal)/float64(total))
	// Oversized single sentences are intentionally retained for measured fitting.
	if output := os.Getenv("CARD_TEST_PREVIEW"); output != "" {
		data, err = json.Marshal(preview)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(output, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLayoutNeverCrossesChapter(t *testing.T) {
	cards := testLayout(t, []cardSource{{id: "a", content: "First short chapter.", chapter: "One"}, {id: "b", content: "Second short chapter.", chapter: "Two"}}, "Title", "Author")
	if len(cards) != 5 || cards[2].content != "First short chapter." || cards[4].content != "Second short chapter." {
		t.Fatal("chapters merged", cards)
	}
}
