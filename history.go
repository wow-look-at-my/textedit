package textedit

// Undo stack caps. Whichever binds first wins, oldest entries dropped: pasting
// a large diff two hundred times must not grow the buffer without bound.
const (
	maxUndoEntries = 200
	maxUndoBytes   = 4 << 20
)

// selState is the cursor and selection at one side of an edit. An undo that
// restores the text but not the selection is half an undo.
type selState struct {
	cursor Position
	anchor Position
}

// entry is one undo step: the range that was replaced, both texts, and the
// selection either side of it.
type entry struct {
	replaced Range
	old      string
	inserted string
	before   selState
	after    selState
}

func (e entry) size() int { return len(e.old) + len(e.inserted) }

func (b *Buffer) state() selState {
	return selState{cursor: b.cursor, anchor: b.anchor}
}

func (b *Buffer) restore(s selState) {
	b.cursor = b.d.clamp(s.cursor)
	b.anchor = b.d.clamp(s.anchor)
}

func (b *Buffer) pushUndo(e entry) {
	b.undo = append(b.undo, e)
	b.undoBytes += e.size()
	for len(b.undo) > maxUndoEntries || (b.undoBytes > maxUndoBytes && len(b.undo) > 1) {
		b.undoBytes -= b.undo[0].size()
		b.undo = b.undo[1:]
	}
}
