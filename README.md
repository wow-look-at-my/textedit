# textedit

A text editing **model** for Go: text, cursor, selection, soft wrapping, undo, and a
one-slot clipboard. No terminal, no key decoding, no clock, no I/O.

It behaves the way most people expect a text box to behave, rather than the way readline
behaves. Line motions follow the wrapped line you can see; paragraph deletes never do.
Word motion stops at word boundaries, so `src/foo.go` does not vanish in one press.
Deleting never touches the clipboard — undo is the recovery path.

## The API

One mutating entry point. `Do(Op) Change` runs a named verb and returns what it did;
everything else reads state (`Text`, `Cursor`, `Selection`, `VisualLines`, `Clipboard`,
`CanUndo`, `CanRedo`) or configures the buffer (`SetWidth`, `SetText`, `Seal`). `Op`
carries the verb plus `Text`, `Pos`, a repeat count `N`, and `Extend` — which is how
every motion extends the selection when shifted, instead of a second set of verbs.
`Verbs()` and `LookupVerb` are the name table, so a host binds config strings to verbs
without writing the list down twice.

```go
b := textedit.New(textedit.WithWidth(80))

b.Do(textedit.Op{Verb: textedit.InsertText, Text: "foo bar"})
b.Do(textedit.Op{Verb: textedit.SelectWordAt, Pos: textedit.Position{Col: 4}})
b.Do(textedit.Op{Verb: textedit.Cut})   // clipboard: "bar", text: "foo "
b.Do(textedit.Op{Verb: textedit.Paste}) // text: "foo bar" — paste does not consume
b.Do(textedit.Op{Verb: textedit.Paste}) // text: "foo barbar"

b.Seal() // ends the current typing run; the host decides when, this library has no clock
b.Do(textedit.Op{Verb: textedit.Undo})

for _, line := range b.VisualLines() {
    fmt.Println(line.Text) // wrapped to 80 terminal cells
}
```

Binding by name, which is what a host with its own keymap does:

```go
verb, ok := textedit.LookupVerb("deleteWordBack") // from <bind action="text.deleteWordBack"/>
```

## Install

```sh
go get github.com/wow-look-at-my/textedit
```

One dependency: `github.com/rivo/uniseg`, for UAX #29 grapheme clusters and display width.
The unit of movement is the cluster and the unit of width is the terminal cell.

`textedit/keymap` is optional readline-ish chord defaults for a host that has no keymap of
its own. Nothing in the core depends on it.

## Development

```sh
go-toolchain
```

That runs mod tidy, vet, lint, the tests with their coverage gate, and the build.
