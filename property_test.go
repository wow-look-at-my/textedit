package textedit

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/rivo/uniseg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// randomOps is the operation stream the properties below are checked over. It
// deliberately includes out-of-range positions and huge repeat counts.
func randomOps(rng *rand.Rand, n int) []Op {
	texts := []string{"a", "z", " ", "\n", "foo", "bar baz", "é", "\U0001F469‍\U0001F467", "x\ny", ""}
	verbs := Verbs()
	ops := make([]Op, n)
	for i := range ops {
		info := verbs[rng.Intn(len(verbs))]
		op := Op{Verb: info.Verb, Extend: rng.Intn(2) == 0}
		if rng.Intn(4) == 0 {
			op.N = rng.Intn(5)
		}
		if info.TakesText {
			op.Text = texts[rng.Intn(len(texts))]
		}
		if info.TakesPos {
			op.Pos = Position{Line: rng.Intn(5) - 1, Col: rng.Intn(9) - 1}
		}
		ops[i] = op
	}
	return ops
}

// checkInvariants holds after every operation, for any stream.
func checkInvariants(t *testing.T, b *Buffer) {
	t.Helper()
	require.Less(t, b.Cursor().Line, b.d.lineCount(), "cursor line inside the buffer")
	require.GreaterOrEqual(t, b.Cursor().Line, 0)
	require.LessOrEqual(t, b.Cursor().Col, b.d.lineLen(b.Cursor().Line), "cursor on a cluster boundary")
	require.GreaterOrEqual(t, b.Cursor().Col, 0)

	a := b.Anchor()
	require.Less(t, a.Line, b.d.lineCount(), "anchor line inside the buffer")
	require.LessOrEqual(t, a.Col, b.d.lineLen(a.Line), "anchor on a cluster boundary")

	// Concatenating the visual lines reproduces the text, with a line break
	// wherever the logical line changes.
	var sb strings.Builder
	prev := 0
	for i, v := range b.VisualLines() {
		if i > 0 && v.Line != prev {
			sb.WriteByte('\n')
		}
		sb.WriteString(v.Text)
		prev = v.Line
	}
	require.Equal(t, b.Text(), sb.String(), "visual lines reproduce the text")
}

func TestPropertyRandomStreams(t *testing.T) {
	for seed := int64(0); seed < 40; seed++ {
		rng := rand.New(rand.NewSource(seed))
		b := New(WithWidth(1 + rng.Intn(12)))
		for _, op := range randomOps(rng, 60) {
			clipBefore := b.Clipboard()
			c := b.Do(op)
			checkInvariants(t, b)

			if isDelete(op.Verb) {
				require.Equal(t, clipBefore, b.Clipboard(), "delete verb wrote the clipboard: %v", op)
			}
			// Only an edit or a history verb changes text; everything else
			// reports a pure move.
			if !c.Undoable && op.Verb != Undo && op.Verb != Redo {
				require.Empty(t, c.Inserted, "a non-undoable change inserted text: %v", op)
				require.True(t, c.Replaced.Empty(), "a non-undoable change replaced a range: %v", op)
			}
		}
	}
}

func isDelete(v Verb) bool {
	switch v {
	case DeleteBack, DeleteForward, DeleteWordBack, DeleteWordForward,
		DeleteToParagraphStart, DeleteToParagraphEnd, DeleteAll:
		return true
	}
	return false
}

func TestPropertyUndoToEmptyThenRedoToEnd(t *testing.T) {
	for seed := int64(0); seed < 40; seed++ {
		rng := rand.New(rand.NewSource(seed))
		b := New(WithWidth(1 + rng.Intn(12)))
		for _, op := range randomOps(rng, 40) {
			b.Do(op)
		}
		final := b.Text()
		finalCursor := b.Cursor()

		steps := 0
		for b.CanUndo() {
			b.Do(Op{Verb: Undo})
			checkInvariants(t, b)
			steps++
			require.Less(t, steps, 1000, "undo did not terminate")
		}
		require.Empty(t, b.Text(), "undoing every entry empties the buffer")

		for b.CanRedo() {
			b.Do(Op{Verb: Redo})
			checkInvariants(t, b)
		}
		require.Equal(t, final, b.Text(), "redo reproduces the final text exactly")
		require.Equal(t, finalCursor, b.Cursor(), "redo reproduces the final cursor")
	}
}

func TestPropertyClusterBoundariesSurviveEditing(t *testing.T) {
	// Every cursor position the buffer reports must be a real cluster boundary
	// of the line it is on, computed from scratch rather than from the cache.
	rng := rand.New(rand.NewSource(7))
	b := New()
	for _, op := range randomOps(rng, 200) {
		b.Do(op)
		p := b.Cursor()
		line := b.d.lines[p.Line].text
		off := b.d.byteOff(p)
		require.Equal(t, clusterOffsets(line), b.d.offsets(p.Line))
		require.Contains(t, clusterOffsets(line), off, "cursor byte offset is a boundary")
		require.Equal(t, uniseg.GraphemeClusterCount(line), b.d.lineLen(p.Line))
	}
}

func FuzzDoNeverPanics(f *testing.F) {
	f.Add("hello world", 8, 3, "x")
	f.Add("a\nb\nc", 2, 30, "\n")
	f.Add("", 0, 1, "é")
	f.Fuzz(func(t *testing.T, text string, width, seed int, insert string) {
		if width < -4 || width > 200 {
			t.Skip()
		}
		b := New(WithText(text), WithWidth(width))
		rng := rand.New(rand.NewSource(int64(seed)))
		for _, op := range randomOps(rng, 30) {
			if op.Verb.Info().TakesText {
				op.Text = insert
			}
			b.Do(op)
			checkInvariants(t, b)
		}
		assert.NotNil(t, b.VisualLines())
	})
}
