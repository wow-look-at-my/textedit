package textedit

// Position is a logical location in the buffer: a line index and a
// grapheme-cluster index within that line.
type Position struct {
	Line int
	Col  int
}

// Before reports whether p is strictly earlier in the buffer than q.
func (p Position) Before(q Position) bool {
	if p.Line != q.Line {
		return p.Line < q.Line
	}
	return p.Col < q.Col
}

// Range is a span from Start to End. A normalized range has Start at or before End.
type Range struct {
	Start Position
	End   Position
}

// Empty reports whether the range covers no text.
func (r Range) Empty() bool { return r.Start == r.End }

func (r Range) normalized() Range {
	if r.End.Before(r.Start) {
		return Range{Start: r.End, End: r.Start}
	}
	return r
}

func rangeAt(p Position) Range { return Range{Start: p, End: p} }
