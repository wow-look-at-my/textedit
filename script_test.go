package textedit

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The script DSL: steps separated by "|", each step a verb name with an
// optional argument. A quoted argument is Op.Text, a bare number is Op.Pos for
// the verbs that take one and Op.N for everything else, "shift+" in front sets
// Op.Extend, and "type" is an alias for insertText. Two pseudo-steps are not
// verbs: seal and setWidth.
//
// The expectation after every step is the whole observable state: the text with
// the cursor marked "|", then the selection and the clipboard when there is one.

type step struct {
	op   string
	want string
}

func byteIndex(b *Buffer, p Position) int {
	off := 0
	for i := range p.Line {
		off += len(b.d.lines[i].text) + 1
	}
	return off + b.d.byteOff(p)
}

func render(b *Buffer) string {
	txt := b.Text()
	i := byteIndex(b, b.Cursor())
	out := txt[:i] + "|" + txt[i:]
	if r, ok := b.Selection(); ok {
		out += fmt.Sprintf(" sel=%d:%d-%d:%d", r.Start.Line, r.Start.Col, r.End.Line, r.End.Col)
	}
	if c := b.Clipboard(); c != "" {
		out += " clip=" + strconv.Quote(c)
	}
	return out
}

func parsePos(t *testing.T, arg string) Position {
	t.Helper()
	line, col := 0, arg
	if l, c, ok := strings.Cut(arg, ":"); ok {
		n, err := strconv.Atoi(l)
		require.NoError(t, err)
		line, col = n, c
	}
	n, err := strconv.Atoi(col)
	require.NoError(t, err)
	return Position{Line: line, Col: n}
}

func parseStep(t *testing.T, s string) Op {
	t.Helper()
	s = strings.TrimSpace(s)
	op := Op{}
	if rest, ok := strings.CutPrefix(s, "shift+"); ok {
		op.Extend = true
		s = rest
	}
	name, arg := s, ""
	if i := strings.IndexByte(s, '('); i >= 0 {
		require.True(t, strings.HasSuffix(s, ")"), "step %q missing )", s)
		name, arg = s[:i], s[i+1:len(s)-1]
	}
	if name == "type" {
		name = "insertText"
	}
	v, ok := LookupVerb(name)
	require.True(t, ok, "unknown verb %q", name)
	op.Verb = v
	switch {
	case arg == "":
	case strings.HasPrefix(arg, `"`):
		text, err := strconv.Unquote(arg)
		require.NoError(t, err)
		op.Text = text
	case v.Info().TakesPos:
		op.Pos = parsePos(t, arg)
	default:
		n, err := strconv.Atoi(arg)
		require.NoError(t, err)
		op.N = n
	}
	return op
}

func runScript(t *testing.T, b *Buffer, steps []step) {
	t.Helper()
	for i, s := range steps {
		switch {
		case s.op == "seal":
			b.Seal()
		case strings.HasPrefix(s.op, "setWidth("):
			w, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(s.op, "setWidth("), ")"))
			require.NoError(t, err)
			b.SetWidth(w)
		default:
			b.Do(parseStep(t, s.op))
		}
		require.Equal(t, s.want, render(b), "after step %d: %s", i+1, s.op)
	}
}

func TestScriptClipboard(t *testing.T) {
	t.Run("cut then paste twice, the spec's own example", func(t *testing.T) {
		runScript(t, New(), []step{
			{`type("foo bar")`, `foo bar|`},
			{`selectWordAt(4)`, `foo bar| sel=0:4-0:7`},
			{`cut`, `foo | clip="bar"`},
			// Paste does not consume the slot, so twice pastes twice.
			{`paste`, `foo bar| clip="bar"`},
			{`paste`, `foo barbar| clip="bar"`},
		})
	})

	t.Run("cut and copy with no selection do nothing at all", func(t *testing.T) {
		// readline: kill-region on an empty region, or a fallback to the line.
		// Here: no text change, no clipboard write, not even a cleared slot.
		runScript(t, New(), []step{
			{`type("hello")`, `hello|`},
			{`cut`, `hello|`},
			{`copy`, `hello|`},
			{`selectAll`, `hello| sel=0:0-0:5`},
			{`copy`, `hello| sel=0:0-0:5 clip="hello"`},
			{`selectNone`, `hello| clip="hello"`},
			{`cut`, `hello| clip="hello"`},
		})
	})

	t.Run("no delete verb touches the clipboard", func(t *testing.T) {
		// readline: every kill command feeds the ring.
		runScript(t, New(), []step{
			{`type("foo bar")`, `foo bar|`},
			{`selectAll`, `foo bar| sel=0:0-0:7`},
			{`copy`, `foo bar| sel=0:0-0:7 clip="foo bar"`},
			{`selectNone`, `foo bar| clip="foo bar"`},
			{`deleteWordBack`, `foo | clip="foo bar"`},
			{`deleteBack`, `foo| clip="foo bar"`},
			{`deleteToParagraphStart`, `| clip="foo bar"`},
			{`type("x")`, `x| clip="foo bar"`},
			{`deleteAll`, `| clip="foo bar"`},
		})
	})

	t.Run("paste replaces the selection", func(t *testing.T) {
		runScript(t, New(), []step{
			{`type("foo bar")`, `foo bar|`},
			{`selectWordAt(0)`, `foo| sel=0:0-0:3`},
			{`copy`, `foo| sel=0:0-0:3 clip="foo"`},
			{`selectWordAt(4)`, `foo bar| sel=0:4-0:7 clip="foo"`},
			{`paste`, `foo foo| clip="foo"`},
		})
	})

	t.Run("paste with explicit text leaves the slot alone", func(t *testing.T) {
		runScript(t, New(), []step{
			{`type("a")`, `a|`},
			{`selectAll`, `a| sel=0:0-0:1`},
			{`copy`, `a| sel=0:0-0:1 clip="a"`},
			{`selectNone`, `a| clip="a"`},
			{`paste("zz")`, `azz| clip="a"`},
			{`paste`, `azza| clip="a"`},
		})
	})
}

func TestScriptWordBoundaries(t *testing.T) {
	t.Run("deleteWordBack stops at a word boundary, not at whitespace", func(t *testing.T) {
		// readline's Ctrl-W: back to the previous whitespace, so the whole
		// path "src/foo.go" goes in one bite.
		runScript(t, New(), []step{
			{`type("cd src/foo.go")`, `cd src/foo.go|`},
			{`deleteWordBack`, `cd src/foo.|`},
			{`deleteWordBack`, `cd src/foo|`},
			{`deleteWordBack`, `cd src/|`},
			{`deleteWordBack`, `cd src|`},
		})
	})

	t.Run("deleteWordBack takes the whitespace behind the cursor plus the unit before it", func(t *testing.T) {
		runScript(t, New(), []step{
			{`type("foo bar   ")`, `foo bar   |`},
			{`deleteWordBack`, `foo |`},
		})
	})

	t.Run("deleteWordForward is the mirror", func(t *testing.T) {
		runScript(t, New(), []step{
			{`type("foo   bar")`, `foo   bar|`},
			{`moveTo(3)`, `foo|   bar`},
			{`deleteWordForward`, `foo|`},
		})
	})

	t.Run("word motions cross the line break", func(t *testing.T) {
		runScript(t, New(), []step{
			{`type("one\ntwo")`, "one\ntwo|"},
			{`wordLeft`, "one\n|two"},
			{`wordLeft`, "|one\ntwo"},
			{`wordRight`, "one|\ntwo"},
			{`wordRight`, "one\ntwo|"},
		})
	})
}

func TestScriptSelection(t *testing.T) {
	t.Run("extend moves the free end, a bare motion collapses and moves", func(t *testing.T) {
		runScript(t, New(), []step{
			{`type("hello")`, `hello|`},
			{`bufferStart`, `|hello`},
			{`shift+right(2)`, `he|llo sel=0:0-0:2`},
			{`shift+wordRight`, `hello| sel=0:0-0:5`},
			{`left`, `hell|o`},
			{`shift+left(2)`, `he|llo sel=0:2-0:4`},
			{`selectNone`, `he|llo`},
		})
	})

	t.Run("deleteBack with a selection deletes the selection and nothing else", func(t *testing.T) {
		runScript(t, New(), []step{
			{`type("hello")`, `hello|`},
			{`moveTo(1)`, `h|ello`},
			{`shift+right(3)`, `hell|o sel=0:1-0:4`},
			{`deleteBack`, `h|o`},
		})
	})

	t.Run("deleteForward with a selection deletes the selection and nothing else", func(t *testing.T) {
		runScript(t, New(), []step{
			{`type("hello")`, `hello|`},
			{`moveTo(1)`, `h|ello`},
			{`shift+right(3)`, `hell|o sel=0:1-0:4`},
			{`deleteForward`, `h|o`},
		})
	})

	t.Run("insertText replaces the selection", func(t *testing.T) {
		runScript(t, New(), []step{
			{`type("hello")`, `hello|`},
			{`selectAll`, `hello| sel=0:0-0:5`},
			{`type("bye")`, `bye|`},
		})
	})

	t.Run("selectParagraphAt takes the logical line", func(t *testing.T) {
		runScript(t, New(), []step{
			{`type("one\ntwo")`, "one\ntwo|"},
			{`selectParagraphAt(0:1)`, "one|\ntwo sel=0:0-0:3"},
			{`selectParagraphAt(1:0)`, "one\ntwo| sel=1:0-1:3"},
		})
	})

	t.Run("selectWordAt on whitespace takes the whitespace run", func(t *testing.T) {
		runScript(t, New(), []step{
			{`type("a   b")`, `a   b|`},
			{`selectWordAt(2)`, `a   |b sel=0:1-0:4`},
		})
	})
}

func TestScriptVisualVersusParagraph(t *testing.T) {
	t.Run("line motions are visual, paragraph deletes never are", func(t *testing.T) {
		// readline: Ctrl-A/Ctrl-E and kill-line all work on the logical line.
		runScript(t, New(), []step{
			{`setWidth(10)`, `|`},
			{`type("aaa bbb ccc")`, `aaa bbb ccc|`},
			{`lineStart`, `aaa bbb |ccc`},
			{`lineEnd`, `aaa bbb ccc|`},
			{`moveTo(0)`, `|aaa bbb ccc`},
			{`lineEnd`, `aaa bbb| ccc`},
			{`paragraphEnd`, `aaa bbb ccc|`},
			{`paragraphStart`, `|aaa bbb ccc`},
			// A delete stops at a boundary that is in the text, never at one
			// the window width invented.
			{`moveTo(4)`, `aaa |bbb ccc`},
			{`deleteToParagraphEnd`, `aaa |`},
		})
	})

	t.Run("up and down keep a goal column across visual rows", func(t *testing.T) {
		runScript(t, New(), []step{
			{`setWidth(10)`, `|`},
			{`type("aaa bbb ccc")`, `aaa bbb ccc|`},
			{`moveTo(10)`, `aaa bbb cc|c`},
			{`up`, `aa|a bbb ccc`},
			{`down`, `aaa bbb cc|c`},
			{`up`, `aa|a bbb ccc`},
			{`up`, `aa|a bbb ccc`},
		})
	})

	t.Run("up and down step logical lines when nothing wraps", func(t *testing.T) {
		runScript(t, New(), []step{
			{`type("one\ntwo\nthree")`, "one\ntwo\nthree|"},
			{`up`, "one\ntwo|\nthree"},
			{`up`, "one|\ntwo\nthree"},
			{`down(2)`, "one\ntwo\nthr|ee"},
			{`down`, "one\ntwo\nthr|ee"},
		})
	})

	t.Run("deleteToParagraphStart joins with the line above at a line start", func(t *testing.T) {
		runScript(t, New(), []step{
			{`type("one\ntwo")`, "one\ntwo|"},
			{`paragraphStart`, "one\n|two"},
			{`deleteToParagraphStart`, `one|two`},
		})
	})

	t.Run("deleteToParagraphEnd joins with the line below at a line end", func(t *testing.T) {
		runScript(t, New(), []step{
			{`type("one\ntwo")`, "one\ntwo|"},
			{`moveTo(0:3)`, "one|\ntwo"},
			{`deleteToParagraphEnd`, `one|two`},
		})
	})
}

func TestScriptUndo(t *testing.T) {
	t.Run("a typing run is one entry", func(t *testing.T) {
		// readline: one entry per inserted character.
		runScript(t, New(), []step{
			{`type("h")`, `h|`},
			{`type("i")`, `hi|`},
			{`undo`, `|`},
			{`redo`, `hi|`},
		})
	})

	t.Run("seal ends the run", func(t *testing.T) {
		runScript(t, New(), []step{
			{`type("h")`, `h|`},
			{`seal`, `h|`},
			{`type("i")`, `hi|`},
			{`undo`, `h|`},
			{`undo`, `|`},
		})
	})

	t.Run("a newline ends the run", func(t *testing.T) {
		runScript(t, New(), []step{
			{`type("a")`, `a|`},
			{`type("\n")`, "a\n|"},
			{`type("b")`, "a\nb|"},
			{`undo`, "a\n|"},
			{`undo`, `a|`},
			{`undo`, `|`},
		})
	})

	t.Run("a motion ends the run", func(t *testing.T) {
		runScript(t, New(), []step{
			{`type("ab")`, `ab|`},
			{`left`, `a|b`},
			{`type("x")`, `ax|b`},
			{`undo`, `a|b`},
		})
	})

	t.Run("undo restores the selection", func(t *testing.T) {
		runScript(t, New(), []step{
			{`type("hello")`, `hello|`},
			{`selectAll`, `hello| sel=0:0-0:5`},
			{`type("x")`, `x|`},
			{`undo`, `hello| sel=0:0-0:5`},
			{`redo`, `x|`},
		})
	})

	t.Run("a mutating operation clears redo", func(t *testing.T) {
		runScript(t, New(), []step{
			{`type("a")`, `a|`},
			{`undo`, `|`},
			{`type("b")`, `b|`},
			{`redo`, `b|`},
		})
	})

	t.Run("undo brings back a deleted word, which is the recovery path", func(t *testing.T) {
		runScript(t, New(), []step{
			{`type("foo bar")`, `foo bar|`},
			{`deleteWordBack`, `foo |`},
			{`undo`, `foo bar|`},
		})
	})
}

func TestScriptTransforms(t *testing.T) {
	t.Run("transposeChars", func(t *testing.T) {
		runScript(t, New(), []step{
			{`type("ab")`, `ab|`},
			{`transposeChars`, `ba|`},
			{`moveTo(1)`, `b|a`},
			{`transposeChars`, `ab|`},
			{`bufferStart`, `|ab`},
			{`transposeChars`, `|ab`},
		})
	})

	t.Run("transposeWords", func(t *testing.T) {
		runScript(t, New(), []step{
			{`type("foo bar")`, `foo bar|`},
			{`moveTo(3)`, `foo| bar`},
			{`transposeWords`, `bar foo|`},
			{`bufferStart`, `|bar foo`},
			{`transposeWords`, `|bar foo`},
		})
	})

	t.Run("case transforms run to the end of the word", func(t *testing.T) {
		runScript(t, New(), []step{
			{`type("hello world")`, `hello world|`},
			{`bufferStart`, `|hello world`},
			{`upcaseWord`, `HELLO| world`},
			{`downcaseWord`, `HELLO world|`},
			{`bufferStart`, `|HELLO world`},
			{`downcaseWord`, `hello| world`},
			{`bufferStart`, `|hello world`},
			{`capitalizeWord(2)`, `Hello World|`},
		})
	})

	t.Run("case transforms take the selection when there is one", func(t *testing.T) {
		runScript(t, New(), []step{
			{`type("hello world")`, `hello world|`},
			{`selectAll`, `hello world| sel=0:0-0:11`},
			{`upcaseWord`, `HELLO WORLD| sel=0:0-0:11`},
			{`undo`, `hello world| sel=0:0-0:11`},
		})
	})
}

func TestScriptUnicode(t *testing.T) {
	t.Run("motion is by grapheme cluster", func(t *testing.T) {
		runScript(t, New(), []step{
			{`type("éx")`, "éx|"},
			{`left`, "é|x"},
			{`left`, "|éx"},
			{`right`, "é|x"},
			{`deleteForward`, "é|"},
			{`deleteBack`, `|`},
		})
	})

	t.Run("a family emoji is one cluster", func(t *testing.T) {
		runScript(t, New(), []step{
			{`type("a\U0001F469‍\U0001F467b")`, "a👩‍👧b|"},
			{`left(2)`, "a|👩‍👧b"},
			{`deleteForward`, `ab|`},
		})
	})
}
