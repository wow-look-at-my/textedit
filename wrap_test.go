package textedit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func rows(b *Buffer) []string {
	out := make([]string, 0, len(b.VisualLines()))
	for _, v := range b.VisualLines() {
		out = append(out, v.Text)
	}
	return out
}

func TestWrapping(t *testing.T) {
	t.Run("no width means no wrapping", func(t *testing.T) {
		b := New(WithText("a very long line indeed"))
		assert.Equal(t, []string{"a very long line indeed"}, rows(b))
		b.SetWidth(-1)
		assert.Equal(t, []string{"a very long line indeed"}, rows(b))
	})

	t.Run("an empty buffer is one empty row", func(t *testing.T) {
		b := New(WithWidth(10))
		require.Len(t, b.VisualLines(), 1)
		assert.Equal(t, VisualLine{}, b.VisualLines()[0])
	})

	t.Run("breaks at word boundaries, keeping the space on the row it ends", func(t *testing.T) {
		b := New(WithText("aaa bbb ccc"), WithWidth(10))
		assert.Equal(t, []string{"aaa bbb ", "ccc"}, rows(b))
	})

	t.Run("hard-breaks a word too long to fit", func(t *testing.T) {
		b := New(WithText("abcdefgh"), WithWidth(3))
		assert.Equal(t, []string{"abc", "def", "gh"}, rows(b))
	})

	t.Run("a cluster wider than the width gets its own row", func(t *testing.T) {
		b := New(WithText("你好"), WithWidth(1))
		assert.Equal(t, []string{"你", "好"}, rows(b))
		assert.Equal(t, 2, b.VisualLines()[0].Width, "width is cells, not runes")
	})

	t.Run("width is measured in cells", func(t *testing.T) {
		b := New(WithText("你好世界"), WithWidth(5))
		assert.Equal(t, []string{"你好", "世界"}, rows(b))
	})

	t.Run("every logical line starts a row", func(t *testing.T) {
		b := New(WithText("one\n\ntwo three"), WithWidth(5))
		lines := b.VisualLines()
		require.Len(t, lines, 4)
		assert.Equal(t, VisualLine{Line: 0, Start: 0, Text: "one", Width: 3}, lines[0])
		assert.Equal(t, VisualLine{Line: 1, Start: 0, Text: "", Width: 0}, lines[1])
		assert.Equal(t, VisualLine{Line: 2, Start: 0, Text: "two ", Width: 4}, lines[2])
		assert.Equal(t, VisualLine{Line: 2, Start: 4, Text: "three", Width: 5}, lines[3])
	})

	t.Run("the cache is invalidated by an edit and by a width change", func(t *testing.T) {
		b := New(WithText("aaa bbb"), WithWidth(20))
		assert.Equal(t, []string{"aaa bbb"}, rows(b))
		b.SetWidth(4)
		assert.Equal(t, []string{"aaa ", "bbb"}, rows(b))
		b.SetWidth(4)
		assert.Equal(t, []string{"aaa ", "bbb"}, rows(b), "setting the same width is a no-op")
		b.Do(Op{Verb: DeleteWordBack})
		assert.Equal(t, []string{"aaa "}, rows(b))
		b.Do(Op{Verb: Undo})
		assert.Equal(t, []string{"aaa ", "bbb"}, rows(b))
	})
}

func TestVisualNavigationAcrossWraps(t *testing.T) {
	b := New(WithText("aaa bbb ccc"), WithWidth(10))

	// Rows are "aaa bbb " and "ccc".
	assert.Equal(t, 0, b.visualIndex(Position{Col: 7}))
	assert.Equal(t, 1, b.visualIndex(Position{Col: 8}), "a soft break belongs to the row it starts")
	assert.Equal(t, 1, b.visualIndex(Position{Col: 11}))
	assert.Equal(t, 3, b.visualCell(Position{Col: 11}))

	// End on a soft-broken row stops where the text stops, not on the space
	// the break consumed.
	assert.Equal(t, Position{Col: 7}, b.rowEndPos(0))
	assert.Equal(t, Position{Col: 11}, b.rowEndPos(1))

	// A goal column past the end of a row lands on the row, never past it.
	assert.Equal(t, Position{Col: 7}, b.positionAtCell(0, 99))
	assert.Equal(t, Position{Col: 11}, b.positionAtCell(1, 99))
	assert.Equal(t, Position{Col: 8}, b.positionAtCell(1, 0))
}

func TestWrapSpansTerminates(t *testing.T) {
	// A zero-width cluster must not loop forever.
	assert.Equal(t, [][2]int{{0, 1}, {1, 2}}, wrapSpans([]int{2, 2}, []bool{false, false}, 1))
	assert.Equal(t, [][2]int{{0, 3}}, wrapSpans([]int{0, 0, 0}, []bool{false, false, false}, 1))
	assert.Equal(t, [][2]int{{0, 2}, {2, 3}}, wrapSpans([]int{1, 1, 1}, []bool{false, true, false}, 2))
}
