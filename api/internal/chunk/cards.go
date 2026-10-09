package chunk

import (
	"math"
	"strings"
	"unicode"
)

// CardSpan uses rune offsets into immutable source text.
type CardSpan struct{ Start, End int }

// Grouping budgets fit ordinary prose on a compact phone. A single longer
// sentence stays intact; only the measured viewport may require continuation.
const CardTarget = 360
const CardMaximum = 480
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

// Cards balances the whole chapter by choosing only pySBD sentence boundaries.
// Squared deviation discourages tiny tails and alternating full/empty cards.
// Four sentences is a soft preference, not a reason to split a sentence. Single
// oversized sentences are outliers; the browser measures whether they really fit.
func Cards(text string, sentences []CardSpan) []CardSpan {
	r := []rune(text)
	cost := make([]float64, len(sentences)+1)
	next := make([]int, len(sentences))
	for i := len(sentences) - 1; i >= 0; i-- {
		cost[i] = math.Inf(1)
		for j := i; j < len(sentences) && j < i+5; j++ {
			chars := sentences[j].End - sentences[i].Start
			words := len(strings.Fields(string(r[sentences[i].Start:sentences[j].End])))
			n := j - i + 1
			if n > 1 && (chars > CardMaximum || words > CardWordLimit) {
				break
			}
			if n == 5 && (chars > 360 || words > 60) {
				break
			}
			deviation := float64(chars-CardTarget) / 180
			score := 1 + deviation*deviation + .03*float64((n-4)*(n-4)) + cost[j+1]
			if score < cost[i] {
				cost[i], next[i] = score, j+1
			}
		}
	}
	var out []CardSpan
	for i := 0; i < len(sentences); i = next[i] {
		out = append(out, CardSpan{sentences[i].Start, sentences[next[i]-1].End})
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
