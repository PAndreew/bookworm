package chunk

import (
	"bytes"
	"strings"
	"testing"
)

// The guarantee the whole annotation model rests on.
func TestSplitIsDeterministic(t *testing.T) {
	text := buildText(60)
	a, b := Split(text), Split(text)

	if len(a) != len(b) {
		t.Fatalf("page count differs between runs: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].Content != b[i].Content {
			t.Errorf("page %d content differs between runs", i)
		}
		if !bytes.Equal(a[i].SHA256, b[i].SHA256) {
			t.Errorf("page %d hash differs between runs", i)
		}
	}
}

func TestSplitRespectsMaxChars(t *testing.T) {
	pages := Split(buildText(80))
	if len(pages) == 0 {
		t.Fatal("no pages produced")
	}
	for _, p := range pages {
		if p.CharCount > maxChars {
			t.Errorf("page %d is %d chars, over the %d ceiling", p.Idx, p.CharCount, maxChars)
		}
	}
}

func TestSplitIndicesAreContiguous(t *testing.T) {
	pages := Split(buildText(40))
	for i, p := range pages {
		if p.Idx != i {
			t.Fatalf("page at slice position %d has Idx %d; indices must be contiguous from 0", i, p.Idx)
		}
	}
}

func TestSplitPreservesAllProse(t *testing.T) {
	// No sentence may be dropped on the floor: this is a reader, not a summariser.
	text := "Alpha one. Beta two.\n\nGamma three. Delta four.\n\nEpsilon five."
	pages := Split(text)

	got := strings.Join(contents(pages), " ")
	for _, want := range []string{"Alpha one.", "Beta two.", "Gamma three.", "Delta four.", "Epsilon five."} {
		if !strings.Contains(got, want) {
			t.Errorf("sentence %q was lost during chunking", want)
		}
	}
}

func TestChapterHeadingStartsNewPage(t *testing.T) {
	text := "Some closing prose from the first chapter.\n\n" +
		"CHAPTER II\n\n" +
		"The opening line of the second chapter."
	pages := Split(text)

	if len(pages) < 2 {
		t.Fatalf("expected a page break at the chapter heading, got %d page(s)", len(pages))
	}
	last := pages[len(pages)-1]
	if !strings.Contains(last.ChapterTitle, "CHAPTER II") {
		t.Errorf("chapter title not carried onto the page: got %q", last.ChapterTitle)
	}
	if strings.Contains(last.Content, "CHAPTER II") {
		t.Error("chapter heading leaked into page content; it belongs in ChapterTitle only")
	}
}

func TestLongParagraphIsSplitOnSentences(t *testing.T) {
	long := strings.TrimSpace(strings.Repeat("This is one sentence of reasonable length. ", 80))
	pages := Split(long)

	if len(pages) < 2 {
		t.Fatalf("a %d-char paragraph should split into several pages, got %d", len(long), len(pages))
	}
	for _, p := range pages {
		if p.CharCount > maxChars {
			t.Errorf("split page %d still exceeds the ceiling: %d chars", p.Idx, p.CharCount)
		}
	}
}

func TestHardWrappedLinesAreRejoined(t *testing.T) {
	// Gutenberg wraps at ~70 columns. Those newlines are an artefact, not structure.
	text := "It is a truth universally acknowledged, that a single man in\n" +
		"possession of a good fortune, must be in want of a wife."
	pages := Split(text)

	if len(pages) != 1 {
		t.Fatalf("hard-wrapped lines should form one paragraph, got %d pages", len(pages))
	}
	if strings.Contains(pages[0].Content, "\n") {
		t.Error("intra-paragraph newline survived; lines were not rejoined")
	}
}

func TestStripGutenbergBoilerplate(t *testing.T) {
	raw := "The Project Gutenberg eBook of Something\nLicence waffle here.\n" +
		"*** START OF THE PROJECT GUTENBERG EBOOK SOMETHING ***\n" +
		"The actual opening line.\n" +
		"*** END OF THE PROJECT GUTENBERG EBOOK SOMETHING ***\n" +
		"Footer waffle about donations."

	got := StripGutenbergBoilerplate(raw)

	if !strings.Contains(got, "The actual opening line.") {
		t.Error("stripped the real text along with the boilerplate")
	}
	if strings.Contains(got, "Licence waffle") || strings.Contains(got, "donations") {
		t.Errorf("boilerplate survived stripping: %q", got)
	}
}

func TestStripBoilerplateToleratesMissingMarkers(t *testing.T) {
	raw := "A plain text with no Gutenberg markers at all."
	if got := StripGutenbergBoilerplate(raw); got != raw {
		t.Errorf("text without markers should pass through unchanged, got %q", got)
	}
}

func TestEmptyInputProducesNoPages(t *testing.T) {
	for _, in := range []string{"", "   ", "\n\n\n"} {
		if pages := Split(in); len(pages) != 0 {
			t.Errorf("Split(%q) produced %d pages, want 0", in, len(pages))
		}
	}
}

// buildText fabricates n paragraphs of plausible prose.
func buildText(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		if i%12 == 0 {
			b.WriteString("\n\nCHAPTER ")
			b.WriteString(strings.Repeat("I", i/12+1))
			b.WriteString("\n\n")
		}
		b.WriteString("This is paragraph number with several clauses in it, ")
		b.WriteString("written so that the chunker has something realistic to break apart. ")
		b.WriteString("It continues for a little while longer to reach a useful length.\n\n")
	}
	return b.String()
}

func contents(pages []Page) []string {
	out := make([]string, len(pages))
	for i, p := range pages {
		out[i] = p.Content
	}
	return out
}
