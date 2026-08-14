package textedit

import "strings"

// Do runs one operation and returns what it did. It is the only mutating entry
// point, and it never panics.
func (b *Buffer) Do(op Op) Change {
	n := op.N
	if n < 1 {
		n = 1
	}
	before := b.cursor

	if op.Verb != InsertText {
		b.openRun = false
	}
	if op.Verb != Up && op.Verb != Down {
		b.goal = -1
	}

	switch op.Verb {
	case Left, Right, Up, Down, WordLeft, WordRight, LineStart, LineEnd,
		ParagraphStart, ParagraphEnd, BufferStart, BufferEnd, MoveTo:
		return b.motion(op, n, before)

	case InsertText:
		if op.Text == "" {
			return b.noop(before)
		}
		return b.insert(b.target(), strings.Repeat(op.Text, n))

	case DeleteBack:
		return b.deleteTo(b.repeat(n, b.prevPos))
	case DeleteForward:
		return b.deleteTo(b.repeat(n, b.nextPos))
	case DeleteWordBack:
		return b.deleteTo(b.repeat(n, b.wordLeft))
	case DeleteWordForward:
		return b.deleteTo(b.repeat(n, b.wordRight))
	case DeleteToParagraphStart:
		return b.deleteTo(b.repeat(n, b.paragraphStartTarget))
	case DeleteToParagraphEnd:
		return b.deleteTo(b.repeat(n, b.paragraphEndTarget))
	case DeleteAll:
		return b.edit(Range{Start: Position{}, End: b.d.end()}, "")

	case Cut:
		if !b.hasSelection() {
			return b.noop(before)
		}
		b.clip = b.d.slice(b.selection())
		return b.edit(b.selection(), "")
	case Copy:
		if !b.hasSelection() {
			return b.noop(before)
		}
		b.clip = b.d.slice(b.selection())
		return b.noop(before)
	case Paste:
		// The slot is the only source: text the host already holds goes in through
		// InsertText, so the slot has one writer and one reader.
		if b.clip == "" {
			return b.noop(before)
		}
		return b.edit(b.target(), strings.Repeat(b.clip, n))

	case SelectAll:
		b.anchor = Position{}
		b.cursor = b.d.end()
		return b.noop(before)
	case SelectNone:
		b.anchor = b.cursor
		return b.noop(before)
	case SelectWordAt:
		r := b.wordRangeAt(op.Pos)
		b.anchor, b.cursor = r.Start, r.End
		return b.noop(before)
	case SelectParagraphAt:
		p := b.d.clamp(op.Pos)
		b.anchor = Position{Line: p.Line}
		b.cursor = Position{Line: p.Line, Col: b.d.lineLen(p.Line)}
		return b.noop(before)

	case TransposeChars:
		return b.loop(n, before, b.transposeChars)
	case TransposeWords:
		return b.loop(n, before, b.transposeWords)
	case UpcaseWord:
		return b.caseWord(n, strings.ToUpper)
	case DowncaseWord:
		return b.caseWord(n, strings.ToLower)
	case CapitalizeWord:
		return b.caseWord(n, b.capitalize)

	case Undo:
		return b.loop(n, before, b.undoOnce)
	case Redo:
		return b.loop(n, before, b.redoOnce)
	}
	return b.noop(before)
}

// target is what an insertion replaces: the selection, or nothing at the cursor.
func (b *Buffer) target() Range {
	if b.hasSelection() {
		return b.selection()
	}
	return rangeAt(b.cursor)
}

// repeat applies a position function n times from the cursor.
func (b *Buffer) repeat(n int, f func(Position) Position) Position {
	p := b.cursor
	for range n {
		p = f(p)
	}
	return p
}

// loop runs a sub-operation n times and reports the combined move. The
// mutation fields describe the last application.
func (b *Buffer) loop(n int, before Position, f func(Position) Change) Change {
	c := b.noop(before)
	for range n {
		c = f(b.cursor)
	}
	c.Before = before
	c.After = b.cursor
	return c
}

// deleteTo removes the selection if there is one, otherwise the span between
// the cursor and p. No delete verb touches the clipboard.
func (b *Buffer) deleteTo(p Position) Change {
	if b.hasSelection() {
		return b.edit(b.selection(), "")
	}
	return b.edit(Range{Start: b.cursor, End: p}, "")
}

func (b *Buffer) paragraphStartTarget(p Position) Position {
	if p.Col > 0 {
		return Position{Line: p.Line}
	}
	if p.Line > 0 {
		return Position{Line: p.Line - 1, Col: b.d.lineLen(p.Line - 1)}
	}
	return p
}

func (b *Buffer) paragraphEndTarget(p Position) Position {
	if p.Col < b.d.lineLen(p.Line) {
		return Position{Line: p.Line, Col: b.d.lineLen(p.Line)}
	}
	if p.Line < b.d.lineCount()-1 {
		return Position{Line: p.Line + 1}
	}
	return p
}

// motion moves the cursor, or the selection's free end when Extend is set.
func (b *Buffer) motion(op Op, n int, before Position) Change {
	p := b.cursor
	for range n {
		p = b.motionTarget(op.Verb, p, op.Pos)
	}
	if op.Extend {
		b.cursor = p
	} else {
		b.setCursor(p)
	}
	return b.noop(before)
}

func (b *Buffer) motionTarget(v Verb, p Position, to Position) Position {
	switch v {
	case Left:
		return b.prevPos(p)
	case Right:
		return b.nextPos(p)
	case Up:
		return b.vertical(p, -1)
	case Down:
		return b.vertical(p, 1)
	case WordLeft:
		return b.wordLeft(p)
	case WordRight:
		return b.wordRight(p)
	case LineStart:
		rows := b.visualLines()
		return Position{Line: p.Line, Col: rows[b.visualIndex(p)].Start}
	case LineEnd:
		return b.rowEndPos(b.visualIndex(p))
	case ParagraphStart:
		return Position{Line: p.Line}
	case ParagraphEnd:
		return Position{Line: p.Line, Col: b.d.lineLen(p.Line)}
	case BufferStart:
		return Position{}
	case BufferEnd:
		return b.d.end()
	case MoveTo:
		return b.d.clamp(to)
	}
	return p
}

// vertical moves one visual row, keeping the sticky goal column. At the first
// or last row it stays put, which is what lets a host bind history recall there.
func (b *Buffer) vertical(p Position, delta int) Position {
	rows := b.visualLines()
	idx := b.visualIndex(p)
	if b.goal < 0 {
		b.goal = b.visualCell(p)
	}
	t := idx + delta
	if t < 0 || t >= len(rows) {
		return p
	}
	return b.positionAtCell(t, b.goal)
}

// edit swaps r for s as a discrete undo entry.
func (b *Buffer) edit(r Range, s string) Change { return b.editRun(r, s, false) }

// insert swaps r for s as typed text, which may merge into the open typing run.
func (b *Buffer) insert(r Range, s string) Change { return b.editRun(r, s, true) }

// editRun is the single mutation: swap r for s, record history, invalidate the
// wrap cache.
func (b *Buffer) editRun(r Range, s string, isTyping bool) Change {
	r = r.normalized()
	r.Start, r.End = b.d.clamp(r.Start), b.d.clamp(r.End)
	before := b.state()
	if r.Empty() && s == "" {
		return b.noop(before.cursor)
	}

	old := b.d.slice(r)
	end := b.d.replace(r, s)
	b.setCursor(end)
	b.visualOK = false
	b.redo = nil

	newEntry := true
	if isTyping && b.openRun && len(b.undo) > 0 && r.Empty() && !strings.Contains(s, "\n") {
		last := &b.undo[len(b.undo)-1]
		if last.after.cursor == r.Start {
			b.undoBytes -= last.size()
			last.inserted += s
			last.after = b.state()
			b.undoBytes += last.size()
			newEntry = false
		}
	}
	if newEntry {
		b.pushUndo(entry{replaced: r, old: old, inserted: s, before: before, after: b.state()})
	}
	b.openRun = isTyping && !strings.Contains(s, "\n")

	return Change{
		Replaced: r,
		Inserted: s,
		Before:   before.cursor,
		After:    b.cursor,
		Undoable: true,
		NewEntry: newEntry,
	}
}

func (b *Buffer) undoOnce(before Position) Change {
	if len(b.undo) == 0 {
		return b.noop(before)
	}
	e := b.undo[len(b.undo)-1]
	b.undo = b.undo[:len(b.undo)-1]
	b.undoBytes -= e.size()

	r := Range{Start: e.replaced.Start, End: b.d.advance(e.replaced.Start, e.inserted)}
	b.d.replace(r, e.old)
	b.visualOK = false
	b.restore(e.before)
	b.redo = append(b.redo, e)

	return Change{Replaced: r, Inserted: e.old, Before: before, After: b.cursor}
}

func (b *Buffer) redoOnce(before Position) Change {
	if len(b.redo) == 0 {
		return b.noop(before)
	}
	e := b.redo[len(b.redo)-1]
	b.redo = b.redo[:len(b.redo)-1]

	r := Range{Start: e.replaced.Start, End: b.d.advance(e.replaced.Start, e.old)}
	b.d.replace(r, e.inserted)
	b.visualOK = false
	b.restore(e.after)
	b.pushUndo(e)

	return Change{Replaced: r, Inserted: e.inserted, Before: before, After: b.cursor}
}
