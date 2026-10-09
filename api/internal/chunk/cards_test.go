package chunk

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func testCards(t *testing.T, text string) []CardSpan {
	t.Helper()
	spans, err := Sentences(context.Background(), []string{text})
	if err != nil {
		t.Fatal(err)
	}
	return Cards(text, spans[0])
}

func TestCardSpansPreserveAnchors(t *testing.T) {
	for _, text := range []string{"", "Short sentence.", strings.Repeat("Hello world. Καλημέρα! 😀 Another sentence.\n\n", 60), strings.Repeat("x", 4001)} {
		spans := testCards(t, text)
		runes := []rune(text)
		var pieces []string
		for _, span := range spans {
			if span.Start < 0 || span.End > len(runes) || span.End <= span.Start {
				t.Fatalf("invalid span %+v", span)
			}
			pieces = append(pieces, string(runes[span.Start:span.End]))
		}
		normalize := func(s string) string { return strings.Join(strings.Fields(s), "") }
		if normalize(strings.Join(pieces, "")) != normalize(text) {
			t.Fatal("source characters lost")
		}
		if !reflect.DeepEqual(spans, testCards(t, text)) {
			t.Fatal("non-deterministic cards")
		}
	}
}
func TestCardsPreferWholeSentences(t *testing.T) {
	for _, tc := range []struct {
		sentence string
		count    int
	}{
		{"A short sentence. ", 5},
		{strings.Repeat("word ", 15) + "ends here. ", 4},
		{strings.Repeat("word ", 27) + "ends here. ", 2},
	} {
		text := strings.Repeat(tc.sentence, 20)
		first := testCards(t, text)[0]
		got := string([]rune(text)[first.Start:first.End])
		if strings.Count(got, ".") != tc.count {
			t.Fatalf("wanted %d sentences, got %q", tc.count, got)
		}
		if len(strings.Fields(got)) > CardWordLimit || first.End-first.Start > CardMaximum {
			t.Fatal("budget exceeded")
		}
	}
}
func TestHonorificsAndSentenceBoundaries(t *testing.T) {
	for _, text := range []string{"Mr.", "Mrs.", "Dr.", "Prof.", "J.", "J.R.R.", "e.g.", "i.e.", "Fig."} {
		if SentenceBoundary([]rune(text), len([]rune(text))) {
			t.Errorf("abbreviation treated as sentence: %s", text)
		}
	}
	for _, text := range []string{"Finished.", "Finished!", "Finished?\"", "Finished.”", "Finished.\n\n"} {
		if !SentenceBoundary([]rune(text), len([]rune(text))) {
			t.Errorf("missed sentence: %s", text)
		}
	}
	if SentenceBoundary([]rune("3.14"), 2) {
		t.Fatal("decimal treated as sentence")
	}
	// Put Mr. exactly at the desired cut: it must stay with its name.
	text := strings.Repeat("word ", 67) + "Mr. Bingley " + strings.Repeat("word ", 70)
	for _, span := range testCards(t, text) {
		if strings.HasSuffix(string([]rune(text)[span.Start:span.End]), "Mr.") {
			t.Fatal("card orphaned Mr.")
		}
	}
}
func TestLongSentenceIsNotCutByAnEstimatedBudget(t *testing.T) {
	long := strings.Repeat("a word ", 200) + "ends here."
	text := "A short sentence. " + long + " Another complete sentence."
	spans := testCards(t, text)
	if len(spans) != 3 {
		t.Fatalf("expected short / long / short, got %d", len(spans))
	}
	if string([]rune(text)[spans[1].Start:spans[1].End]) != long {
		t.Fatal("long sentence split by estimated budget")
	}
	for _, span := range spans {
		if !SentenceBoundary([]rune(text), span.End) {
			t.Fatal("card does not end on a sentence")
		}
	}
}
