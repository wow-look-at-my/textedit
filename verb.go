package textedit

// Verb names an operation. It is the only thing Op selects on.
type Verb int

// The verbs, grouped as the specification groups them.
const (
	// Motion. Every one of these accepts Op.Extend.
	Left Verb = iota
	Right
	Up
	Down
	WordLeft
	WordRight
	LineStart
	LineEnd
	ParagraphStart
	ParagraphEnd
	BufferStart
	BufferEnd
	MoveTo

	// Insert.
	InsertText

	// Delete. None of these touch the clipboard.
	DeleteBack
	DeleteForward
	DeleteWordBack
	DeleteWordForward
	DeleteToParagraphStart
	DeleteToParagraphEnd
	DeleteAll

	// Clipboard.
	Cut
	Copy
	Paste

	// Selection.
	SelectAll
	SelectNone
	SelectWordAt
	SelectParagraphAt

	// Transform.
	TransposeChars
	TransposeWords
	UpcaseWord
	DowncaseWord
	CapitalizeWord

	// History.
	Undo
	Redo
)

// VerbInfo describes one verb: its canonical name and what an Op carrying it uses.
type VerbInfo struct {
	Name      string
	Verb      Verb
	TakesText bool // reads Op.Text
	TakesPos  bool // reads Op.Pos
	Motion    bool // accepts Op.Extend
}

var verbTable = []VerbInfo{
	{Name: "left", Verb: Left, Motion: true},
	{Name: "right", Verb: Right, Motion: true},
	{Name: "up", Verb: Up, Motion: true},
	{Name: "down", Verb: Down, Motion: true},
	{Name: "wordLeft", Verb: WordLeft, Motion: true},
	{Name: "wordRight", Verb: WordRight, Motion: true},
	{Name: "lineStart", Verb: LineStart, Motion: true},
	{Name: "lineEnd", Verb: LineEnd, Motion: true},
	{Name: "paragraphStart", Verb: ParagraphStart, Motion: true},
	{Name: "paragraphEnd", Verb: ParagraphEnd, Motion: true},
	{Name: "bufferStart", Verb: BufferStart, Motion: true},
	{Name: "bufferEnd", Verb: BufferEnd, Motion: true},
	{Name: "moveTo", Verb: MoveTo, TakesPos: true, Motion: true},

	{Name: "insertText", Verb: InsertText, TakesText: true},

	{Name: "deleteBack", Verb: DeleteBack},
	{Name: "deleteForward", Verb: DeleteForward},
	{Name: "deleteWordBack", Verb: DeleteWordBack},
	{Name: "deleteWordForward", Verb: DeleteWordForward},
	{Name: "deleteToParagraphStart", Verb: DeleteToParagraphStart},
	{Name: "deleteToParagraphEnd", Verb: DeleteToParagraphEnd},
	{Name: "deleteAll", Verb: DeleteAll},

	{Name: "cut", Verb: Cut},
	{Name: "copy", Verb: Copy},
	{Name: "paste", Verb: Paste},

	{Name: "selectAll", Verb: SelectAll},
	{Name: "selectNone", Verb: SelectNone},
	{Name: "selectWordAt", Verb: SelectWordAt, TakesPos: true},
	{Name: "selectParagraphAt", Verb: SelectParagraphAt, TakesPos: true},

	{Name: "transposeChars", Verb: TransposeChars},
	{Name: "transposeWords", Verb: TransposeWords},
	{Name: "upcaseWord", Verb: UpcaseWord},
	{Name: "downcaseWord", Verb: DowncaseWord},
	{Name: "capitalizeWord", Verb: CapitalizeWord},

	{Name: "undo", Verb: Undo},
	{Name: "redo", Verb: Redo},
}

var verbByName = func() map[string]Verb {
	m := make(map[string]Verb, len(verbTable))
	for _, v := range verbTable {
		m[v.Name] = v.Verb
	}
	return m
}()

// Verbs returns every verb with its canonical name. A host builds its action
// registry by iterating this, so a name is never written down twice.
func Verbs() []VerbInfo {
	out := make([]VerbInfo, len(verbTable))
	copy(out, verbTable)
	return out
}

// LookupVerb resolves a canonical name, as a host's config parser does.
func LookupVerb(name string) (Verb, bool) {
	v, ok := verbByName[name]
	return v, ok
}

// Info returns the verb's table entry.
func (v Verb) Info() VerbInfo {
	if int(v) < 0 || int(v) >= len(verbTable) {
		return VerbInfo{Name: "unknown", Verb: v}
	}
	return verbTable[v]
}

// String is the canonical name LookupVerb accepts.
func (v Verb) String() string { return v.Info().Name }
