// Package textedit is a text editing model: text, cursor, selection, wrapping,
// undo and a one-slot clipboard, driven by named operations. No terminal, no
// key decoding, no clock, no I/O.
package textedit

// Op is one operation. Verb selects what happens; the other fields are read
// only by the verbs that declare them in Verbs().
type Op struct {
	Verb   Verb
	Text   string   // InsertText only
	Pos    Position // MoveTo, SelectWordAt, SelectParagraphAt
	N      int      // repeat count; 0 and 1 both mean once
	Extend bool     // any motion verb: move the selection's free end instead of collapsing
}

// Change is what an operation did. It is returned, not published: the host
// called Do, so the host gets the answer.
type Change struct {
	Replaced Range    // what the operation removed
	Inserted string   // what it put there
	Before   Position // cursor before
	After    Position // cursor after
	Undoable bool     // the operation recorded an undo entry
	NewEntry bool     // false when this merged into the current undo entry
}

// Buffer is the editing model. It is not safe for concurrent use.
type Buffer struct {
	d      *doc
	cursor Position
	anchor Position // the selection's fixed end; equal to cursor means no selection
	clip   string
	width  int

	classify Classifier

	visual   []VisualLine
	visualOK bool

	undo      []entry
	redo      []entry
	undoBytes int
	openRun   bool // the newest undo entry is still accepting typed text

	goal int // sticky visual cell for consecutive Up/Down; -1 when unset
}

// Option configures a Buffer at construction.
type Option func(*Buffer)

// WithText starts the buffer with text, the cursor at its end.
func WithText(s string) Option {
	return func(b *Buffer) {
		b.d.setText(s)
		b.cursor = b.d.end()
		b.anchor = b.cursor
	}
}

// WithWidth sets the wrap width in terminal cells. Zero or less means no wrapping.
func WithWidth(cells int) Option {
	return func(b *Buffer) { b.width = cells }
}

// WithClassifier replaces the word classifier that word motions and word
// deletes step by.
func WithClassifier(c Classifier) Option {
	return func(b *Buffer) {
		if c != nil {
			b.classify = c
		}
	}
}

// New returns an empty buffer.
func New(opts ...Option) *Buffer {
	b := &Buffer{d: newDoc(""), classify: DefaultClassifier, goal: -1}
	for _, o := range opts {
		o(b)
	}
	return b
}

// Text returns the whole buffer.
func (b *Buffer) Text() string { return b.d.text() }

// Cursor is the caret: a line and a grapheme-cluster index.
func (b *Buffer) Cursor() Position { return b.cursor }

// Anchor is the selection's fixed end. It equals the cursor when nothing is selected.
func (b *Buffer) Anchor() Position { return b.anchor }

// Selection returns the selected range, and false when nothing is selected.
func (b *Buffer) Selection() (Range, bool) {
	return b.selection(), b.hasSelection()
}

// VisualLines returns the text wrapped to the configured width. The slice is
// cached and must not be modified.
func (b *Buffer) VisualLines() []VisualLine { return b.visualLines() }

// Clipboard returns what the last Cut or Copy put there. Nothing else in the
// library writes it.
func (b *Buffer) Clipboard() string { return b.clip }

// CanUndo reports whether there is an undo entry.
func (b *Buffer) CanUndo() bool { return len(b.undo) > 0 }

// CanRedo reports whether there is a redo entry.
func (b *Buffer) CanRedo() bool { return len(b.redo) > 0 }

// SetWidth sets the wrap width in terminal cells. Zero or less means no wrapping.
func (b *Buffer) SetWidth(cells int) {
	if cells == b.width {
		return
	}
	b.width = cells
	b.visualOK = false
}

// SetText replaces the buffer and resets undo. The clipboard is kept; the
// cursor goes to the end of the new text.
func (b *Buffer) SetText(s string) {
	b.d.setText(s)
	b.cursor = b.d.end()
	b.anchor = b.cursor
	b.undo, b.redo, b.undoBytes = nil, nil, 0
	b.openRun = false
	b.goal = -1
	b.visualOK = false
}

// Seal ends the current typing run, so the next InsertText starts a new undo
// entry. The library has no clock: an idle timer is the host's.
func (b *Buffer) Seal() { b.openRun = false }

func (b *Buffer) hasSelection() bool { return b.anchor != b.cursor }

func (b *Buffer) selection() Range {
	return Range{Start: b.anchor, End: b.cursor}.normalized()
}

func (b *Buffer) setCursor(p Position) {
	b.cursor = p
	b.anchor = p
}

func (b *Buffer) noop(before Position) Change {
	return Change{Replaced: rangeAt(before), Before: before, After: b.cursor}
}
