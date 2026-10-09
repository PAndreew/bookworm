package chunk

import (
	"strings"
	"unicode"
)

// CardSpan uses rune offsets into immutable source text.
type CardSpan struct{ Start, End int }

// Budgets fit ordinary prose on a compact phone without reducing type size.
const CardTarget = 480
const CardWordLimit = 85

// SentenceBoundary recognizes punctuation, quotation endings, abbreviations,
// initials and decimals. It is deliberately conservative, not an NLP model.
func SentenceBoundary(r []rune, cut int) bool {
	end := cut
	for end > 0 && unicode.IsSpace(r[end-1]) {
		end--
	}
	for end > 0 && strings.ContainsRune("\"'”’)]}", r[end-1]) {
		end--
	}
	if end == 0 {
		return false
	}
	ch := r[end-1]
	if ch == '!' || ch == '?' || ch == '。' || ch == '！' || ch == '？' {
		return true
	}
	if ch != '.' {
		return false
	}
	start := end - 1
	for start > 0 && (unicode.IsLetter(r[start-1]) || r[start-1] == '.') {
		start--
	}
	token := strings.ToLower(string(r[start : end-1]))
	switch token {
	case "mr", "mrs", "ms", "dr", "prof", "rev", "st", "sr", "jr", "vs", "etc", "e.g", "i.e", "a.m", "p.m", "no", "fig", "vol", "mt", "messrs", "mme", "mlle":
		return false
	}
	if end-start == 2 && unicode.IsUpper(r[start]) {
		return false
	}
	if strings.Contains(token, ".") {
		return false
	} // dotted initials
	if end >= 2 && unicode.IsDigit(r[end-2]) && cut < len(r) && unicode.IsDigit(r[cut]) {
		return false
	}
	return true
}

// Cards prefers four whole sentences, or five when especially short. Long
// sentences produce fewer per card; only over-budget sentences split at words.
func Cards(text string) []CardSpan {
	r := []rune(text)
	var sentences []CardSpan
	start := 0
	for start < len(r) && unicode.IsSpace(r[start]) {
		start++
	}
	for cut := start + 1; cut <= len(r); cut++ {
		if cut < len(r) && (!unicode.IsSpace(r[cut]) || unicode.IsSpace(r[cut-1])) {
			continue
		}
		if cut == len(r) || SentenceBoundary(r, cut) {
			end := cut
			for end > start && unicode.IsSpace(r[end-1]) {
				end--
			}
			if end > start {
				sentences = append(sentences, CardSpan{start, end})
			}
			start = cut
			for start < len(r) && unicode.IsSpace(r[start]) {
				start++
			}
		}
	}
	var units []CardSpan
	for _, sentence := range sentences {
		start := sentence.Start
		for start < sentence.End {
			end, words, best := start, 0, start
			for end < sentence.End {
				for end < sentence.End && unicode.IsSpace(r[end]) {
					end++
				}
				for end < sentence.End && !unicode.IsSpace(r[end]) {
					end++
				}
				words++
				if (end-start > CardTarget || words > CardWordLimit) && best > start {
					break
				}
				best = end
				// Never orphan an abbreviation at a forced word break.
				if r[end-1] == '.' && !SentenceBoundary(r, end) && end < sentence.End {
					continue
				}
				if end-start >= CardTarget || words >= CardWordLimit {
					break
				}
			}
			// Keep a trailing honorific with its following word.
			if best < sentence.End && r[best-1] == '.' && !SentenceBoundary(r, best) {
				back := best - 1
				for back > start && !unicode.IsSpace(r[back-1]) {
					back--
				}
				for back > start && unicode.IsSpace(r[back-1]) {
					back--
				}
				if back > start {
					best = back
				}
			}
			units = append(units, CardSpan{start, best})
			start = best
			for start < sentence.End && unicode.IsSpace(r[start]) {
				start++
			}
		}
	}
	var out []CardSpan
	for i := 0; i < len(units); {
		span := units[i]
		i++
		n := 1
		for i < len(units) && n < 5 {
			candidate := string(r[span.Start:units[i].End])
			chars, words := units[i].End-span.Start, len(strings.Fields(candidate))
			if chars > CardTarget || words > CardWordLimit {
				break
			}
			if n == 4 && (chars > 360 || words > 60) {
				break
			}
			span.End = units[i].End
			i++
			n++
		}
		out = append(out, span)
	}
	return out
}

// FitLines is retained solely for the legacy v1→v2 migration.
func FitLines(text string) []CardSpan {
	r := []rune(text)
	var out []CardSpan
	var fit func(int, int)
	fit = func(start, end int) {
		for start < end && unicode.IsSpace(r[start]) {
			start++
		}
		for end > start && unicode.IsSpace(r[end-1]) {
			end--
		}
		if start == end {
			return
		}
		lines, columns := 1, 0.0
		for _, ch := range r[start:end] {
			if ch == '\n' {
				lines++
				columns = 0
				continue
			}
			width := 1.0
			if unicode.IsUpper(ch) {
				width = 1.3
			}
			if ch == 'W' || ch == 'M' || ch >= 0x2e80 {
				width = 1.9
			}
			columns += width
			if columns > 32 {
				lines++
				columns = width
			}
		}
		if lines <= 13 || end-start < 2 {
			out = append(out, CardSpan{start, end})
			return
		}
		mid := start + (end-start)/2
		cut := mid
		for distance := 0; distance <= 30; distance++ {
			if mid-distance > start && unicode.IsSpace(r[mid-distance]) {
				cut = mid - distance
				break
			}
			if mid+distance < end && unicode.IsSpace(r[mid+distance]) {
				cut = mid + distance
				break
			}
		}
		fit(start, cut)
		fit(cut, end)
	}
	fit(0, len(r))
	return out
}
