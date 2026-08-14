package textedit

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerbTableIsTheOnlyPlaceANameIsWritten(t *testing.T) {
	infos := Verbs()
	require.Len(t, infos, 35, "the verb count in the specification's table")

	seen := map[string]bool{}
	for i, info := range infos {
		require.Equal(t, Verb(i), info.Verb, "the table is indexed by verb value")
		require.NotEmpty(t, info.Name)
		require.False(t, seen[info.Name], "duplicate verb name %q", info.Name)
		seen[info.Name] = true

		v, ok := LookupVerb(info.Name)
		require.True(t, ok, "LookupVerb(%q)", info.Name)
		require.Equal(t, info.Verb, v)
		require.Equal(t, info.Name, v.String(), "String round-trips the name")
		require.Equal(t, info, v.Info())
	}

	// The returned slice is a copy: a host cannot corrupt the table.
	infos[0].Name = "clobbered"
	require.Equal(t, "left", Left.String())

	_, ok := LookupVerb("noSuchVerb")
	require.False(t, ok)
	require.Equal(t, "unknown", Verb(999).String())
}

func TestVerbGroupsMatchTheSpecification(t *testing.T) {
	var motions, takesText, takesPos int
	for _, info := range Verbs() {
		if info.Motion {
			motions++
		}
		if info.TakesText {
			takesText++
		}
		if info.TakesPos {
			takesPos++
		}
	}
	assert.Equal(t, 13, motions, "13 motion verbs, all accepting Extend")
	assert.Equal(t, 2, takesText, "InsertText and Paste read Op.Text")
	assert.Equal(t, 3, takesPos, "MoveTo, SelectWordAt and SelectParagraphAt read Op.Pos")

	for _, v := range []Verb{Left, Right, Up, Down, WordLeft, WordRight, LineStart,
		LineEnd, ParagraphStart, ParagraphEnd, BufferStart, BufferEnd, MoveTo} {
		assert.True(t, v.Info().Motion, "%s is a motion", v)
	}
	assert.Equal(t, "deleteWordBack", DeleteWordBack.String())
}

func TestNewAndOptions(t *testing.T) {
	b := New()
	assert.Empty(t, b.Text())
	assert.Equal(t, Position{}, b.Cursor())
	assert.Empty(t, b.Clipboard())
	assert.False(t, b.CanUndo())
	assert.False(t, b.CanRedo())
	_, ok := b.Selection()
	assert.False(t, ok)

	w := New(WithText("one\ntwo"), WithWidth(40))
	assert.Equal(t, "one\ntwo", w.Text())
	assert.Equal(t, Position{Line: 1, Col: 3}, w.Cursor(), "WithText leaves the cursor at the end")
	assert.False(t, w.CanUndo(), "starting text is not an undo entry")
}

func TestSetTextResetsUndoAndKeepsTheClipboard(t *testing.T) {
	b := New(WithText("hello"))
	b.Do(Op{Verb: SelectAll})
	b.Do(Op{Verb: Copy})
	b.Do(Op{Verb: InsertText, Text: "x"})
	require.True(t, b.CanUndo())

	b.SetText("fresh\ntext")
	assert.Equal(t, "fresh\ntext", b.Text())
	assert.Equal(t, Position{Line: 1, Col: 4}, b.Cursor())
	assert.False(t, b.CanUndo(), "SetText resets undo")
	assert.False(t, b.CanRedo())
	assert.Equal(t, "hello", b.Clipboard(), "SetText keeps the clipboard")
	_, ok := b.Selection()
	assert.False(t, ok)
}

func TestSealEndsTheTypingRun(t *testing.T) {
	b := New()
	c1 := b.Do(Op{Verb: InsertText, Text: "a"})
	assert.True(t, c1.NewEntry)
	assert.True(t, c1.Undoable)

	c2 := b.Do(Op{Verb: InsertText, Text: "b"})
	assert.False(t, c2.NewEntry, "a continued run merges")
	assert.Len(t, b.undo, 1)

	b.Seal()
	c3 := b.Do(Op{Verb: InsertText, Text: "c"})
	assert.True(t, c3.NewEntry, "Seal starts a new entry")
	assert.Len(t, b.undo, 2)
}

func TestChangeReportsWhatHappened(t *testing.T) {
	b := New(WithText("hello"))
	b.Do(Op{Verb: BufferStart})
	b.Do(Op{Verb: SelectAll})

	c := b.Do(Op{Verb: InsertText, Text: "bye"})
	assert.Equal(t, Range{Start: Position{}, End: Position{Col: 5}}, c.Replaced)
	assert.Equal(t, "bye", c.Inserted)
	assert.Equal(t, Position{Col: 5}, c.Before)
	assert.Equal(t, Position{Col: 3}, c.After)
	assert.True(t, c.Undoable)
	assert.True(t, c.NewEntry)

	// A motion reports a move and nothing else.
	m := b.Do(Op{Verb: BufferStart})
	assert.False(t, m.Undoable)
	assert.False(t, m.NewEntry)
	assert.Empty(t, m.Inserted)
	assert.True(t, m.Replaced.Empty())
	assert.Equal(t, Position{Col: 3}, m.Before)
	assert.Equal(t, Position{}, m.After)
}

func TestNoOpsDoNothing(t *testing.T) {
	b := New()
	for _, op := range []Op{
		{Verb: InsertText, Text: ""},
		{Verb: Paste},
		{Verb: Cut},
		{Verb: Copy},
		{Verb: DeleteBack},
		{Verb: DeleteForward},
		{Verb: DeleteAll},
		{Verb: DeleteWordBack},
		{Verb: DeleteToParagraphEnd},
		{Verb: Undo},
		{Verb: Redo},
		{Verb: TransposeChars},
		{Verb: TransposeWords},
		{Verb: UpcaseWord},
		{Verb: Left},
		{Verb: Verb(999)},
	} {
		c := b.Do(op)
		assert.Empty(t, b.Text(), "%s changed an empty buffer", op.Verb)
		assert.False(t, c.Undoable, "%s recorded an undo entry on an empty buffer", op.Verb)
		assert.False(t, b.CanUndo())
	}
}

func TestOpNIsARepeatCount(t *testing.T) {
	b := New(WithText("abcdef"))
	b.Do(Op{Verb: Left, N: 3})
	assert.Equal(t, Position{Col: 3}, b.Cursor())
	b.Do(Op{Verb: Left, N: 0})
	assert.Equal(t, Position{Col: 2}, b.Cursor(), "N of 0 means once")
	b.Do(Op{Verb: Left, N: -5})
	assert.Equal(t, Position{Col: 1}, b.Cursor(), "a negative N means once")

	b.Do(Op{Verb: DeleteForward, N: 3})
	assert.Equal(t, "aef", b.Text())

	i := New()
	i.Do(Op{Verb: InsertText, Text: "ab", N: 3})
	assert.Equal(t, "ababab", i.Text())
	i.Do(Op{Verb: Undo})
	assert.Empty(t, i.Text(), "a repeated insert is one entry")

	// Repeats past the end of the buffer clamp instead of erroring.
	b.Do(Op{Verb: Right, N: 100})
	assert.Equal(t, b.d.end(), b.Cursor())
	b.Do(Op{Verb: Left, N: 100})
	assert.Equal(t, Position{}, b.Cursor())
}

func TestUndoStackEntryCap(t *testing.T) {
	b := New()
	for range maxUndoEntries + 50 {
		b.Do(Op{Verb: InsertText, Text: "x"})
		b.Seal()
	}
	assert.Len(t, b.undo, maxUndoEntries, "oldest entries are dropped")
	assert.Equal(t, maxUndoEntries+50, len(b.Text()))

	for b.CanUndo() {
		b.Do(Op{Verb: Undo})
	}
	assert.Len(t, b.Text(), 50, "the dropped entries are not recoverable")
}

func TestUndoStackByteCap(t *testing.T) {
	big := strings.Repeat("x", 1<<20)
	b := New()
	for range 6 {
		b.Do(Op{Verb: InsertText, Text: big})
		b.Seal()
	}
	assert.Less(t, len(b.undo), 6, "the byte cap dropped an entry")
	assert.LessOrEqual(t, b.undoBytes, maxUndoBytes+len(big))
}

func TestUndoAndRedoAcrossLines(t *testing.T) {
	b := New()
	b.Do(Op{Verb: InsertText, Text: "one\ntwo\nthree"})
	b.Do(Op{Verb: MoveTo, Pos: Position{Line: 1, Col: 1}})
	b.Do(Op{Verb: DeleteToParagraphEnd})
	require.Equal(t, "one\nt\nthree", b.Text())

	b.Do(Op{Verb: Undo})
	assert.Equal(t, "one\ntwo\nthree", b.Text())
	assert.Equal(t, Position{Line: 1, Col: 1}, b.Cursor())

	b.Do(Op{Verb: Redo})
	assert.Equal(t, "one\nt\nthree", b.Text())
	assert.Equal(t, Position{Line: 1, Col: 1}, b.Cursor())

	b.Do(Op{Verb: Undo, N: 2})
	assert.Empty(t, b.Text(), "a repeated undo walks the stack")
	b.Do(Op{Verb: Redo, N: 2})
	assert.Equal(t, "one\nt\nthree", b.Text())
}

func TestSelectionAccessors(t *testing.T) {
	b := New(WithText("hello"))
	b.Do(Op{Verb: MoveTo, Pos: Position{Col: 1}})
	b.Do(Op{Verb: Right, N: 3, Extend: true})

	r, ok := b.Selection()
	require.True(t, ok)
	assert.Equal(t, Range{Start: Position{Col: 1}, End: Position{Col: 4}}, r)
	assert.Equal(t, Position{Col: 1}, b.Anchor())
	assert.Equal(t, Position{Col: 4}, b.Cursor())

	// Extending back onto the anchor is no selection, not an empty one.
	b.Do(Op{Verb: Left, N: 3, Extend: true})
	_, ok = b.Selection()
	assert.False(t, ok)
}

func TestMoveToClampsOutOfRangePositions(t *testing.T) {
	b := New(WithText("one\ntwo"))
	b.Do(Op{Verb: MoveTo, Pos: Position{Line: 9, Col: 9}})
	assert.Equal(t, Position{Line: 1, Col: 3}, b.Cursor())
	b.Do(Op{Verb: MoveTo, Pos: Position{Line: -1, Col: -1}})
	assert.Equal(t, Position{}, b.Cursor())
	b.Do(Op{Verb: MoveTo, Pos: Position{Line: 0, Col: 99}})
	assert.Equal(t, Position{Line: 0, Col: 3}, b.Cursor())
}

func TestPositionAndRangeHelpers(t *testing.T) {
	a := Position{Line: 1, Col: 2}
	assert.True(t, Position{Line: 0, Col: 9}.Before(a))
	assert.True(t, Position{Line: 1, Col: 1}.Before(a))
	assert.False(t, a.Before(a))
	assert.False(t, Position{Line: 2}.Before(a))

	assert.True(t, Range{Start: a, End: a}.Empty())
	backwards := Range{Start: Position{Col: 5}, End: Position{Col: 1}}
	assert.Equal(t, Range{Start: Position{Col: 1}, End: Position{Col: 5}}, backwards.normalized())
}

func TestDocSliceAndAdvance(t *testing.T) {
	d := newDoc("one\ntwo\nthree")
	assert.Equal(t, "one\ntwo\nthree", d.text())
	assert.Equal(t, "ne\ntwo\nth", d.slice(Range{
		Start: Position{Line: 0, Col: 1},
		End:   Position{Line: 2, Col: 2},
	}))
	assert.Equal(t, "wo", d.slice(Range{
		Start: Position{Line: 1, Col: 1},
		End:   Position{Line: 1, Col: 3},
	}))
	assert.Equal(t, Position{Line: 2, Col: 5}, d.end())

	// advance walks over text the document already holds, which is how the
	// undo path re-derives the range an edit inserted.
	held := newDoc("onexy\ntwo")
	assert.Equal(t, Position{Line: 0, Col: 5}, held.advance(Position{Line: 0, Col: 3}, "xy"))
	assert.Equal(t, Position{Line: 1, Col: 1}, held.advance(Position{Line: 0, Col: 3}, "xy\nt"))
	assert.Equal(t, 2, d.colAtByte(0, 2))
}
