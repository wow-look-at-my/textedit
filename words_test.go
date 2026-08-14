package textedit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The word-boundary corpus. The classifier is the most opinionated thing in the
// library, so the record is a table: runs of letters, digits and underscore are
// words, runs of punctuation are their own unit, whitespace is a third.
func TestWordBoundaryCorpus(t *testing.T) {
	cases := []struct {
		text      string
		col       int
		wantLeft  int
		wantRight int
	}{
		// A path breaks into its parts. readline's Ctrl-W would take the lot.
		{"src/foo.go", 10, 8, 10},
		{"src/foo.go", 8, 7, 10},
		{"src/foo.go", 7, 4, 8},
		{"src/foo.go", 4, 3, 7},
		{"src/foo.go", 3, 0, 4},
		{"src/foo.go", 0, 0, 3},

		// Whitespace behind the cursor goes with the unit before it.
		{"foo bar", 7, 4, 7},
		{"foo bar", 4, 0, 7},
		{"foo   ", 6, 0, 6},
		{"  foo", 5, 2, 5},
		{"  foo", 0, 0, 5},
		{"tabs\tand", 8, 5, 8},

		// Underscores and digits are inside the word; punctuation is not.
		{"a_b1 c", 4, 0, 6},
		{"snake_case_name", 15, 0, 15},
		{"hello, world!", 13, 12, 13},
		{"hello, world!", 12, 7, 13},
		{"hello, world!", 7, 5, 12},
		{"hello, world!", 5, 0, 7},

		// A URL is several units, which is the whole point of the ruling.
		{"https://ex.com/a", 16, 15, 16},
		{"https://ex.com/a", 15, 11, 16},
		{"https://ex.com/a", 11, 10, 15},
		{"", 0, 0, 0},
	}
	for _, c := range cases {
		b := New(WithText(c.text))
		at := Position{Col: c.col}
		assert.Equal(t, Position{Col: c.wantLeft}, b.wordLeft(at),
			"wordLeft in %q from %d", c.text, c.col)
		assert.Equal(t, Position{Col: c.wantRight}, b.wordRight(at),
			"wordRight in %q from %d", c.text, c.col)

		// A word delete is exactly the span the matching word motion covers.
		b.Do(Op{Verb: MoveTo, Pos: at})
		b.Do(Op{Verb: DeleteWordBack})
		want := c.text[:byteAt(c.text, c.wantLeft)] + c.text[byteAt(c.text, c.col):]
		assert.Equal(t, want, b.Text(), "deleteWordBack in %q from %d", c.text, c.col)

		f := New(WithText(c.text))
		f.Do(Op{Verb: MoveTo, Pos: at})
		f.Do(Op{Verb: DeleteWordForward})
		want = c.text[:byteAt(c.text, c.col)] + c.text[byteAt(c.text, c.wantRight):]
		assert.Equal(t, want, f.Text(), "deleteWordForward in %q from %d", c.text, c.col)
	}
}

func byteAt(s string, col int) int { return clusterOffsets(s)[col] }

func TestDefaultClassifier(t *testing.T) {
	cases := map[string]CharClass{
		"a": ClassWord, "Z": ClassWord, "7": ClassWord, "_": ClassWord,
		"é": ClassWord, "本": ClassWord,
		" ": ClassSpace, "\t": ClassSpace, "\n": ClassSpace, "": ClassSpace,
		"/": ClassPunct, ".": ClassPunct, "-": ClassPunct, "!": ClassPunct,
	}
	for cluster, want := range cases {
		assert.Equal(t, want, DefaultClassifier(cluster), "classifying %q", cluster)
	}
}

func TestClassifierIsAnOption(t *testing.T) {
	// A shell-flavoured host wants the readline boundary back: everything that
	// is not whitespace is one unit.
	shell := func(cluster string) CharClass {
		if DefaultClassifier(cluster) == ClassSpace {
			return ClassSpace
		}
		return ClassWord
	}
	b := New(WithText("cd src/foo.go"), WithClassifier(shell))
	b.Do(Op{Verb: DeleteWordBack})
	assert.Equal(t, "cd ", b.Text())

	// A nil classifier keeps the default rather than panicking.
	d := New(WithText("src/foo.go"), WithClassifier(nil))
	d.Do(Op{Verb: DeleteWordBack})
	require.Equal(t, "src/foo.", d.Text())
}

func TestWordRangeAtEdges(t *testing.T) {
	empty := New()
	assert.Equal(t, rangeAt(Position{}), empty.wordRangeAt(Position{}))

	b := New(WithText("foo bar"))
	// Past the end of the text, the run behind the position is the answer.
	assert.Equal(t, Range{Start: Position{Col: 4}, End: Position{Col: 7}},
		b.wordRangeAt(Position{Col: 99}))
	// A position out of range is clamped, never a panic.
	assert.Equal(t, Range{Start: Position{Col: 0}, End: Position{Col: 3}},
		b.wordRangeAt(Position{Line: -3, Col: -1}))
}
