// card-sample applies the production sentence chunker to a source-page export.
// It is used for static guest fixtures; it never connects to the database.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/bookworm/api/internal/chunk"
	"os"
	"strings"
)

func main() {
	var input struct {
		Pages []struct {
			Content string `json:"content"`
		} `json:"pages"`
	}
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		panic(err)
	}
	var text strings.Builder
	for _, p := range input.Pages {
		if text.Len() > 0 {
			r := []rune(text.String())
			if chunk.SentenceBoundary(r, len(r)) {
				text.WriteString("\n\n")
			} else {
				text.WriteString(" ")
			}
		}
		text.WriteString(p.Content)
	}
	r := []rune(text.String())
	sentences, err := chunk.Sentences(context.Background(), []string{string(r)})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	type card struct {
		Content      string `json:"content"`
		SentenceEnds []int  `json:"sentence_ends"`
	}
	cards := []card{}
	i := 0
	for _, span := range chunk.Cards(string(r), sentences[0]) {
		c := card{Content: string(r[span.Start:span.End])}
		for i < len(sentences[0]) && sentences[0][i].End <= span.End {
			c.SentenceEnds = append(c.SentenceEnds, sentences[0][i].End-span.Start)
			i++
		}
		cards = append(cards, c)
	}
	if err := json.NewEncoder(os.Stdout).Encode(cards); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
