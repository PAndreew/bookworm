// Package chunk turns a book's plain text into the immutable "pages" that the feed
// renders, the meter counts, and likes and highlights anchor to.
//
// Because page ids are permanent anchors for user annotations, the output of this
// package must be deterministic: the same input and the same Version must always
// produce byte-identical pages. If you change the algorithm, bump Version -- that
// creates a new edition and leaves existing annotations pointing at the old one.
package chunk

import (
	"crypto/sha256"
	"regexp"
	"strings"
	"unicode"
)

// Version identifies the chunking algorithm. BUMP THIS on any behavioural change.
const Version = 1

const (
	// targetChars is the size we aim for per page: roughly a comfortable phone
	// screen at default type size.
	targetChars = 900
	// maxChars is the hard ceiling. A paragraph longer than this is split on
	// sentence boundaries rather than being allowed to overflow.
	maxChars = 1400
	// minTailChars stops a 3-word orphan page at the end of a chapter; a tail
	// smaller than this is merged back into the previous page.
	minTailChars = 120
)

// Page is one card in the feed.
type Page struct {
	Idx          int
	ChapterTitle string
	Content      string
	CharCount    int
	SHA256       []byte
}

// chapterRe matches common chapter headings in Project Gutenberg plain text.
var chapterRe = regexp.MustCompile(`(?i)^\s*(chapter|part|book|canto|act)\s+([0-9]+|[ivxlcdm]+)\b.*$`)

// sentenceEnd matches a sentence terminator followed by whitespace. Used only when a
// single paragraph exceeds maxChars.
var sentenceEnd = regexp.MustCompile(`[.!?]["')\]]?\s+`)

// Split turns raw text into pages. It is pure and deterministic.
func Split(raw string) []Page {
	paras := paragraphs(raw)

	var pages []Page
	var buf strings.Builder
	chapter := ""
	bufChapter := ""

	flush := func() {
		if buf.Len() == 0 {
			return
		}
		content := strings.TrimSpace(buf.String())
		buf.Reset()
		if content == "" {
			return
		}
		pages = append(pages, Page{ChapterTitle: bufChapter, Content: content})
	}

	for _, p := range paras {
		// A chapter heading starts a new page and relabels everything after it.
		if m := chapterRe.FindString(p); m != "" && len(p) < 90 {
			flush()
			chapter = strings.Join(strings.Fields(p), " ")
			bufChapter = chapter
			continue
		}

		if bufChapter == "" {
			bufChapter = chapter
		}

		// Oversized paragraph: split it on sentence boundaries.
		if len(p) > maxChars {
			flush()
			for _, part := range splitLong(p) {
				bufChapter = chapter
				buf.WriteString(part)
				flush()
			}
			continue
		}

		// Would appending overflow the target? Close the current page first.
		if buf.Len() > 0 && buf.Len()+len(p)+2 > targetChars {
			// Only break if what we have is already substantial; otherwise accept
			// a slightly over-target page rather than emitting a stub.
			if buf.Len() >= targetChars/2 {
				flush()
				bufChapter = chapter
			}
		}

		if buf.Len() > 0 {
			buf.WriteString("\n\n")
		}
		buf.WriteString(p)
	}
	flush()

	pages = mergeShortTail(pages)

	for i := range pages {
		pages[i].Idx = i
		pages[i].CharCount = len([]rune(pages[i].Content))
		sum := sha256.Sum256([]byte(pages[i].Content))
		pages[i].SHA256 = sum[:]
	}
	return pages
}

// paragraphs normalises line endings and splits on blank lines. Project Gutenberg
// hard-wraps at ~70 columns, so single newlines inside a paragraph are joined.
func paragraphs(raw string) []string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")

	var out []string
	for _, block := range regexp.MustCompile(`\n[ \t]*\n+`).Split(raw, -1) {
		// Join the hard-wrapped lines back into one logical paragraph.
		joined := strings.Join(strings.Fields(block), " ")
		if joined == "" {
			continue
		}
		out = append(out, joined)
	}
	return out
}

// splitLong breaks one over-long paragraph into target-sized pieces at sentence
// boundaries, falling back to a word boundary if a single "sentence" is enormous.
func splitLong(p string) []string {
	locs := sentenceEnd.FindAllStringIndex(p, -1)

	var parts []string
	start, last := 0, 0
	for _, loc := range locs {
		if loc[1]-start >= targetChars {
			end := loc[1]
			if last > start {
				end = last // prefer the previous boundary: closer to target
			}
			parts = append(parts, strings.TrimSpace(p[start:end]))
			start = end
		}
		last = loc[1]
	}
	if start < len(p) {
		parts = append(parts, strings.TrimSpace(p[start:]))
	}

	// A sentence longer than maxChars on its own: fall back to word boundaries.
	var out []string
	for _, part := range parts {
		for len(part) > maxChars {
			cut := strings.LastIndexFunc(part[:maxChars], unicode.IsSpace)
			if cut <= 0 {
				cut = maxChars
			}
			out = append(out, strings.TrimSpace(part[:cut]))
			part = strings.TrimSpace(part[cut:])
		}
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// mergeShortTail folds a runt final page into its predecessor, but only when they
// belong to the same chapter.
func mergeShortTail(pages []Page) []Page {
	if len(pages) < 2 {
		return pages
	}
	last := len(pages) - 1
	if len(pages[last].Content) < minTailChars &&
		pages[last].ChapterTitle == pages[last-1].ChapterTitle {
		pages[last-1].Content += "\n\n" + pages[last].Content
		return pages[:last]
	}
	return pages
}

// StripGutenbergBoilerplate removes the Project Gutenberg licence header and footer,
// which must not appear in the feed. It is tolerant: if the markers are missing the
// text is returned unchanged.
func StripGutenbergBoilerplate(raw string) string {
	const startMark = "*** START OF TH"
	const endMark = "*** END OF TH"

	if i := strings.Index(raw, startMark); i >= 0 {
		if nl := strings.IndexByte(raw[i:], '\n'); nl >= 0 {
			raw = raw[i+nl+1:]
		}
	}
	if i := strings.Index(raw, endMark); i >= 0 {
		raw = raw[:i]
	}
	return strings.TrimSpace(raw)
}
