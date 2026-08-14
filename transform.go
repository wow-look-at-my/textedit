package textedit

import (
	"strings"

	"github.com/rivo/uniseg"
)

// caseWord recases the selection if there is one, otherwise the text from the
// cursor to the end of the next word, leaving the cursor there.
func (b *Buffer) caseWord(n int, f func(string) string) Change {
	before := b.cursor
	sel := b.hasSelection()
	r := Range{Start: b.cursor, End: b.repeat(n, b.wordRight)}
	if sel {
		r = b.selection()
	}
	r = r.normalized()
	if r.Empty() {
		return b.noop(before)
	}
	c := b.edit(r, f(b.d.slice(r)))
	if sel {
		b.anchor = r.Start
	}
	return c
}

// capitalize uppercases the first cluster of every word run and lowercases the rest.
func (b *Buffer) capitalize(s string) string {
	var out strings.Builder
	rest, state, inWord := s, -1, false
	for len(rest) > 0 {
		var c string
		c, rest, _, state = uniseg.FirstGraphemeClusterInString(rest, state)
		switch {
		case b.classify(c) != ClassWord:
			out.WriteString(c)
			inWord = false
		case inWord:
			out.WriteString(strings.ToLower(c))
		default:
			out.WriteString(strings.ToUpper(c))
			inWord = true
		}
	}
	return out.String()
}

// transposeChars swaps the cluster before the cursor with the one at it and
// steps over both. At the end of a line it swaps the last two.
func (b *Buffer) transposeChars(before Position) Change {
	p := b.cursor
	n := b.d.lineLen(p.Line)
	var r Range
	switch {
	case p.Col >= n:
		if n < 2 {
			return b.noop(before)
		}
		r = Range{Start: Position{Line: p.Line, Col: n - 2}, End: Position{Line: p.Line, Col: n}}
	case p.Col == 0:
		return b.noop(before)
	default:
		r = Range{Start: Position{Line: p.Line, Col: p.Col - 1}, End: Position{Line: p.Line, Col: p.Col + 1}}
	}
	cl := b.d.clusters(p.Line)
	return b.edit(r, cl[r.Start.Col+1]+cl[r.Start.Col])
}

// transposeWords swaps the word before the cursor with the word after it,
// keeping whatever separated them, and leaves the cursor past both.
func (b *Buffer) transposeWords(before Position) Change {
	end2 := b.wordRight(b.cursor)
	if b.classBefore(end2) == ClassSpace {
		return b.noop(before)
	}
	start2 := b.runStart(end2)
	start1 := b.wordLeft(start2)
	end1 := b.runEnd(start1)
	if start1 == start2 || start1 == end1 || start2.Before(end1) {
		return b.noop(before)
	}
	w1 := b.d.slice(Range{Start: start1, End: end1})
	mid := b.d.slice(Range{Start: end1, End: start2})
	w2 := b.d.slice(Range{Start: start2, End: end2})
	return b.edit(Range{Start: start1, End: end2}, w2+mid+w1)
}
