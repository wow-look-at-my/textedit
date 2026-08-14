package textedit

import (
	"unicode"
	"unicode/utf8"
)

// CharClass is what a word classifier answers for one grapheme cluster.
type CharClass int

const (
	// ClassSpace is whitespace, including the line break between two lines.
	ClassSpace CharClass = iota
	// ClassWord is letters, digits and underscore.
	ClassWord
	// ClassPunct is everything else.
	ClassPunct
)

// Classifier assigns a grapheme cluster to a class. Word motions and word
// deletes step over one run of one class at a time.
type Classifier func(cluster string) CharClass

// DefaultClassifier is the editor rule rather than the shell one: runs of
// letters, digits and underscore are words, runs of punctuation are their own
// unit, whitespace is a third. It is what makes Ctrl-Backspace stop inside
// src/foo.go instead of eating the whole path.
func DefaultClassifier(cluster string) CharClass {
	if cluster == "" {
		return ClassSpace
	}
	r, _ := utf8.DecodeRuneInString(cluster)
	switch {
	case r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r):
		return ClassWord
	case unicode.IsSpace(r):
		return ClassSpace
	default:
		return ClassPunct
	}
}

// hasPrev reports whether a position has a cluster (or line break) behind it.
func (b *Buffer) hasPrev(p Position) bool { return p.Line > 0 || p.Col > 0 }

func (b *Buffer) hasNext(p Position) bool {
	return p.Line < b.d.lineCount()-1 || p.Col < b.d.lineLen(p.Line)
}

func (b *Buffer) prevPos(p Position) Position {
	if p.Col > 0 {
		return Position{Line: p.Line, Col: p.Col - 1}
	}
	if p.Line > 0 {
		return Position{Line: p.Line - 1, Col: b.d.lineLen(p.Line - 1)}
	}
	return p
}

func (b *Buffer) nextPos(p Position) Position {
	if p.Col < b.d.lineLen(p.Line) {
		return Position{Line: p.Line, Col: p.Col + 1}
	}
	if p.Line < b.d.lineCount()-1 {
		return Position{Line: p.Line + 1, Col: 0}
	}
	return p
}

// classBefore classifies the cluster behind p; a line break is whitespace.
func (b *Buffer) classBefore(p Position) CharClass {
	if p.Col == 0 {
		return ClassSpace
	}
	return b.classify(b.d.slice(Range{Start: Position{Line: p.Line, Col: p.Col - 1}, End: p}))
}

func (b *Buffer) classAfter(p Position) CharClass {
	if p.Col == b.d.lineLen(p.Line) {
		return ClassSpace
	}
	return b.classify(b.d.slice(Range{Start: p, End: Position{Line: p.Line, Col: p.Col + 1}}))
}

// wordLeft steps back over whitespace and then over the unit before it.
func (b *Buffer) wordLeft(p Position) Position {
	for b.hasPrev(p) && b.classBefore(p) == ClassSpace {
		p = b.prevPos(p)
	}
	if !b.hasPrev(p) {
		return p
	}
	c := b.classBefore(p)
	for b.hasPrev(p) && b.classBefore(p) == c {
		p = b.prevPos(p)
	}
	return p
}

// wordRight is the mirror of wordLeft.
func (b *Buffer) wordRight(p Position) Position {
	for b.hasNext(p) && b.classAfter(p) == ClassSpace {
		p = b.nextPos(p)
	}
	if !b.hasNext(p) {
		return p
	}
	c := b.classAfter(p)
	for b.hasNext(p) && b.classAfter(p) == c {
		p = b.nextPos(p)
	}
	return p
}

// runStart walks back to the start of the run of the class behind p.
func (b *Buffer) runStart(p Position) Position {
	if !b.hasPrev(p) {
		return p
	}
	c := b.classBefore(p)
	for b.hasPrev(p) && b.classBefore(p) == c {
		p = b.prevPos(p)
	}
	return p
}

// runEnd walks forward to the end of the run of the class at p.
func (b *Buffer) runEnd(p Position) Position {
	if !b.hasNext(p) {
		return p
	}
	c := b.classAfter(p)
	for b.hasNext(p) && b.classAfter(p) == c {
		p = b.nextPos(p)
	}
	return p
}

// wordRangeAt is the run of one class containing p, which is what a
// double-click selects.
func (b *Buffer) wordRangeAt(p Position) Range {
	p = b.d.clamp(p)
	if !b.hasNext(p) && !b.hasPrev(p) {
		return rangeAt(p)
	}
	if b.hasNext(p) {
		return Range{Start: b.runStart(b.nextPos(p)), End: b.runEnd(p)}
	}
	return Range{Start: b.runStart(p), End: p}
}
