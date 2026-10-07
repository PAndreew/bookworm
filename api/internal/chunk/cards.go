package chunk

import "unicode"

// CardSpan uses rune offsets into an IMMUTABLE source page. This presentation
// algorithm does not change Split, its version, or annotation anchors.
type CardSpan struct{ Start, End int }

// Cards balances a source page into ~300–360-character cards, capped at 400.
// Prefer nearby sentence endings, then words. Large words still obey the cap.
func Cards(text string) []CardSpan {
	r := []rune(text)
	count := (len(r) + 359) / 360
	if count == 0 {
		return nil
	}
	spans := []CardSpan{}
	start := 0
	for i := 0; i < count; i++ {
		end := len(r)
		if i < count-1 {
			ideal := start + (len(r)-start)/(count-i)
			lo, hi := ideal-35, ideal+35
			if lo <= start {
				lo = start + 1
			}
			if hi > start+400 {
				hi = start + 400
			}
			if hi >= len(r) {
				hi = len(r) - 1
			}
			best, score := ideal, 100000
			for cut := lo; cut <= hi; cut++ {
				if !unicode.IsSpace(r[cut]) {
					continue
				}
				distance := cut - ideal
				if distance < 0 {
					distance = -distance
				}
				if cut > 0 && (r[cut-1] == '.' || r[cut-1] == '!' || r[cut-1] == '?') {
					distance -= 40
				}
				if distance < score {
					best, score = cut, distance
				}
			}
			end = best
		}
		if end > start+400 {
			end = start + 400
			count++
		} // hard bound even after preferred cuts
		a, b := start, end
		for a < b && unicode.IsSpace(r[a]) {
			a++
		}
		for b > a && unicode.IsSpace(r[b-1]) {
			b--
		}
		if a < b {
			spans = append(spans, CardSpan{a, b})
		}
		start = end
	}
	var fitted []CardSpan
	for _, span := range spans {
		for _, part := range FitLines(string(r[span.Start:span.End])) {
			fitted = append(fitted, CardSpan{span.Start + part.Start, span.Start + part.End})
		}
	}
	return fitted
}

// FitLines keeps paragraph-heavy cards within a conservative phone line budget.
// Recursive balanced word-boundary splits retain all non-whitespace characters.
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
