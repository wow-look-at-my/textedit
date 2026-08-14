package textedit

import (
	"strings"

	"github.com/rivo/uniseg"
)

// doc is the text: logical lines with lazily computed grapheme-cluster boundaries.
type doc struct {
	lines []docLine
}

type docLine struct {
	text string
	offs []int // byte offset of every cluster start, plus len(text); nil until computed
}

func newDoc(s string) *doc {
	d := &doc{}
	d.setText(s)
	return d
}

func (d *doc) setText(s string) {
	parts := strings.Split(s, "\n")
	d.lines = make([]docLine, len(parts))
	for i, p := range parts {
		d.lines[i] = docLine{text: p}
	}
}

func (d *doc) text() string {
	if len(d.lines) == 1 {
		return d.lines[0].text
	}
	parts := make([]string, len(d.lines))
	for i, l := range d.lines {
		parts[i] = l.text
	}
	return strings.Join(parts, "\n")
}

// offsets returns the cluster boundary table for a line, computing it on first use.
func (d *doc) offsets(line int) []int {
	l := &d.lines[line]
	if l.offs == nil {
		l.offs = clusterOffsets(l.text)
	}
	return l.offs
}

func clusterOffsets(s string) []int {
	offs := make([]int, 0, len(s)+1)
	state := -1
	rest := s
	at := 0
	for len(rest) > 0 {
		offs = append(offs, at)
		var cluster string
		cluster, rest, _, state = uniseg.FirstGraphemeClusterInString(rest, state)
		at += len(cluster)
	}
	return append(offs, len(s))
}

// clusters returns the grapheme clusters of a line.
func (d *doc) clusters(line int) []string {
	offs := d.offsets(line)
	out := make([]string, len(offs)-1)
	text := d.lines[line].text
	for i := range out {
		out[i] = text[offs[i]:offs[i+1]]
	}
	return out
}

func (d *doc) lineCount() int { return len(d.lines) }

// lineLen is the number of grapheme clusters in a line.
func (d *doc) lineLen(line int) int { return len(d.offsets(line)) - 1 }

func (d *doc) byteOff(p Position) int { return d.offsets(p.Line)[p.Col] }

// colAtByte converts a byte offset within a line to a cluster index, rounding
// down to the enclosing cluster boundary.
func (d *doc) colAtByte(line, off int) int {
	offs := d.offsets(line)
	lo, hi := 0, len(offs)-1
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if offs[mid] <= off {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo
}

func (d *doc) end() Position {
	last := len(d.lines) - 1
	return Position{Line: last, Col: d.lineLen(last)}
}

func (d *doc) clamp(p Position) Position {
	if p.Line < 0 {
		return Position{}
	}
	if p.Line >= len(d.lines) {
		return d.end()
	}
	if p.Col < 0 {
		p.Col = 0
	}
	if n := d.lineLen(p.Line); p.Col > n {
		p.Col = n
	}
	return p
}

func (d *doc) slice(r Range) string {
	r = r.normalized()
	if r.Start.Line == r.End.Line {
		return d.lines[r.Start.Line].text[d.byteOff(r.Start):d.byteOff(r.End)]
	}
	var b strings.Builder
	b.WriteString(d.lines[r.Start.Line].text[d.byteOff(r.Start):])
	for i := r.Start.Line + 1; i < r.End.Line; i++ {
		b.WriteByte('\n')
		b.WriteString(d.lines[i].text)
	}
	b.WriteByte('\n')
	b.WriteString(d.lines[r.End.Line].text[:d.byteOff(r.End)])
	return b.String()
}

// replace swaps the text in r for s and returns the position just past the
// inserted text.
func (d *doc) replace(r Range, s string) Position {
	r = r.normalized()
	prefix := d.lines[r.Start.Line].text[:d.byteOff(r.Start)]
	suffix := d.lines[r.End.Line].text[d.byteOff(r.End):]

	ins := strings.Split(s, "\n")
	fresh := make([]docLine, len(ins))
	if len(ins) == 1 {
		fresh[0] = docLine{text: prefix + ins[0] + suffix}
	} else {
		fresh[0] = docLine{text: prefix + ins[0]}
		for i := 1; i < len(ins)-1; i++ {
			fresh[i] = docLine{text: ins[i]}
		}
		fresh[len(ins)-1] = docLine{text: ins[len(ins)-1] + suffix}
	}

	tail := append([]docLine(nil), d.lines[r.End.Line+1:]...)
	d.lines = append(append(d.lines[:r.Start.Line:r.Start.Line], fresh...), tail...)

	endLine := r.Start.Line + len(ins) - 1
	endByte := len(ins[len(ins)-1])
	if len(ins) == 1 {
		endByte += len(prefix)
	}
	return Position{Line: endLine, Col: d.colAtByte(endLine, endByte)}
}

// advance returns the position just past s, for a document that already holds
// s at p. That is what the history path needs: the range an entry's text now
// occupies.
func (d *doc) advance(p Position, s string) Position {
	nl := strings.Count(s, "\n")
	line := p.Line + nl
	if nl == 0 {
		return Position{Line: line, Col: d.colAtByte(line, d.byteOff(p)+len(s))}
	}
	last := s[strings.LastIndexByte(s, '\n')+1:]
	return Position{Line: line, Col: d.colAtByte(line, len(last))}
}
