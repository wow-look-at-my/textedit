package textedit

import "github.com/rivo/uniseg"

// VisualLine is one wrapped row: a slice of one logical line, with the
// grapheme-cluster index it starts at and its width in terminal cells.
type VisualLine struct {
	Line  int    // index of the logical line this row came from
	Start int    // grapheme-cluster index within that line
	Text  string // the row's text, never containing a line break
	Width int    // display width in terminal cells
}

func clusterWidth(cluster string) int { return uniseg.StringWidth(cluster) }

// wrap recomputes the visual lines. Width <= 0 means no wrapping.
func (b *Buffer) wrap() []VisualLine {
	var out []VisualLine
	for ln := range b.d.lineCount() {
		clusters := b.d.clusters(ln)
		if b.width <= 0 || len(clusters) == 0 {
			out = append(out, VisualLine{
				Line:  ln,
				Text:  b.d.lines[ln].text,
				Width: uniseg.StringWidth(b.d.lines[ln].text),
			})
			continue
		}
		widths := make([]int, len(clusters))
		space := make([]bool, len(clusters))
		for i, c := range clusters {
			widths[i] = clusterWidth(c)
			space[i] = b.classify(c) == ClassSpace
		}
		offs := b.d.offsets(ln)
		text := b.d.lines[ln].text
		for _, span := range wrapSpans(widths, space, b.width) {
			w := 0
			for i := span[0]; i < span[1]; i++ {
				w += widths[i]
			}
			out = append(out, VisualLine{
				Line:  ln,
				Start: span[0],
				Text:  text[offs[span[0]]:offs[span[1]]],
				Width: w,
			})
		}
	}
	return out
}

// wrapSpans breaks one line's clusters into rows at the last word boundary that
// fits, hard-breaking a word too long for the width. A single cluster wider
// than the width gets a row to itself.
func wrapSpans(widths []int, space []bool, width int) [][2]int {
	n := len(widths)
	var spans [][2]int
	for start := 0; start < n; {
		w, lastBreak, end := 0, -1, n
		for i := start; i < n; i++ {
			if i > start && w+widths[i] > width {
				if lastBreak > start {
					end = lastBreak
				} else {
					end = i
				}
				break
			}
			w += widths[i]
			if space[i] {
				lastBreak = i + 1
			}
		}
		spans = append(spans, [2]int{start, end})
		start = end
	}
	return spans
}

// visualLines returns the cached wrap, recomputing it if the text or width moved.
func (b *Buffer) visualLines() []VisualLine {
	if !b.visualOK {
		b.visual = b.wrap()
		b.visualOK = true
	}
	return b.visual
}

// visualIndex is the row p sits on. A position at a soft break belongs to the
// row that follows it, which is where the cursor is drawn.
func (b *Buffer) visualIndex(p Position) int {
	rows := b.visualLines()
	idx := 0
	for i, r := range rows {
		if r.Line != p.Line {
			continue
		}
		if r.Start <= p.Col {
			idx = i
		}
	}
	return idx
}

// visualCell is p's horizontal offset in cells from the start of its row.
func (b *Buffer) visualCell(p Position) int {
	row := b.visualLines()[b.visualIndex(p)]
	clusters := b.d.clusters(p.Line)
	cells := 0
	for i := row.Start; i < p.Col && i < len(clusters); i++ {
		cells += clusterWidth(clusters[i])
	}
	return cells
}

// positionAtCell is the position on row idx nearest the given cell offset.
func (b *Buffer) positionAtCell(idx, cell int) Position {
	rows := b.visualLines()
	row := rows[idx]
	clusters := b.d.clusters(row.Line)
	end := row.Start + uniseg.GraphemeClusterCount(row.Text)
	col, cells := row.Start, 0
	for col < end {
		w := clusterWidth(clusters[col])
		if cells+w > cell {
			break
		}
		cells += w
		col++
	}
	// Never land past the last cluster of a soft-wrapped row: that position is
	// the first cluster of the next row and would skip a line.
	if col == end && col > row.Start && idx+1 < len(rows) && rows[idx+1].Line == row.Line {
		col--
	}
	return Position{Line: row.Line, Col: col}
}

// rowEndPos is where LineEnd lands on a row. On a soft-broken row the trailing
// whitespace the break consumed is dropped, so End stops where the text stops.
func (b *Buffer) rowEndPos(idx int) Position {
	rows := b.visualLines()
	row := rows[idx]
	end := row.Start + uniseg.GraphemeClusterCount(row.Text)
	if idx+1 < len(rows) && rows[idx+1].Line == row.Line {
		clusters := b.d.clusters(row.Line)
		for end > row.Start && b.classify(clusters[end-1]) == ClassSpace {
			end--
		}
	}
	return Position{Line: row.Line, Col: end}
}
