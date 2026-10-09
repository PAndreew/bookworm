package chunk

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestPySBDOffsetsAndAbbreviations(t *testing.T) {
	text := "Mr. Bingley met Dr. Smith at 5 p.m. on Tuesday. They paid $3.14. 😀 It worked!"
	result, err := Sentences(context.Background(), []string{text, "", "“Done!”\n\nNext sentence."})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, span := range result[0] {
		got = append(got, string([]rune(text)[span.Start:span.End]))
	}
	want := []string{"Mr. Bingley met Dr. Smith at 5 p.m. on Tuesday.", "They paid $3.14.", "😀 It worked!"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
	if len(result[1]) != 0 {
		t.Fatal("empty chapter has sentences")
	}
}

func TestBalanceDoesNotLeaveATinyTail(t *testing.T) {
	sentence := strings.Repeat("word ", 17) + "ends."
	text := strings.Repeat(sentence+" ", 6)
	sentences, err := Sentences(context.Background(), []string{text})
	if err != nil {
		t.Fatal(err)
	}
	cards := Cards(text, sentences[0])
	if len(cards) != 2 {
		t.Fatalf("wanted two balanced cards, got %d", len(cards))
	}
	if cards[0].End-cards[0].Start != cards[1].End-cards[1].Start {
		t.Fatal("greedy packing left uneven cards", cards)
	}
	for _, card := range cards {
		found := false
		for _, sentence := range sentences[0] {
			if sentence.End == card.End {
				found = true
			}
		}
		if !found {
			t.Fatal("balancing invented a sentence boundary")
		}
	}
}

func TestPySBDFailureDoesNotGuess(t *testing.T) {
	t.Setenv("PYSBD_PYTHON", "/nonexistent/python")
	if _, err := Sentences(context.Background(), []string{"Hello."}); err == nil {
		t.Fatal("missing segmenter silently accepted")
	}
}
