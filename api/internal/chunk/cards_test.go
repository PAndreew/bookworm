package chunk

import (
	"reflect"
	"strings"
	"testing"
)

func TestCardSpansPreserveAnchors(t *testing.T) {
	if len(Cards(strings.Repeat("Short heading.\n\n", 12))) < 2 {
		t.Fatal("paragraph-heavy front matter was not split to fit")
	}
	for _, text := range []string{"", "Short sentence.", strings.Repeat("Hello world. Καλημέρα! 😀 Another sentence.\n\n", 60), strings.Repeat("x", 4001)} {
		spans := Cards(text)
		runes := []rune(text)
		pieces := []string{}
		for _, span := range spans {
			if span.Start < 0 || span.End > len(runes) || span.End <= span.Start || span.End-span.Start > 400 {
				t.Fatalf("invalid span %+v", span)
			}
			pieces = append(pieces, string(runes[span.Start:span.End]))
		}
		// Non-whitespace text is never dropped or changed, including Unicode.
		normalize := func(s string) string { return strings.Join(strings.Fields(s), "") }
		if normalize(strings.Join(pieces, "")) != normalize(text) {
			t.Fatal("source characters lost")
		}
		if !reflect.DeepEqual(spans, Cards(text)) {
			t.Fatal("non-deterministic cards")
		}
	}
	text := strings.Repeat("a normal sentence with some words. ", 30)
	for _, span := range Cards(text) {
		if n := span.End - span.Start; n < 240 || n > 400 {
			t.Fatalf("imbalanced card: %d", n)
		}
	}
}
