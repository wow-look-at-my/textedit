# CLAUDE.md

Notes for Claude working in this repository.

## What this is

`textedit` — a text editing model: text, cursor, selection, wrapping, undo and a one-slot clipboard, driven by named
operations. No terminal, no key decoding, no clock, no I/O. Written from scratch as an extraction of nothing: the
specification is `simple-llm-harness/docs/design/tui/editor.md`, and it is authoritative. Its consumer doc,
`tui/input.md`, owns which keys a host binds to which verb; `config/keybindings.md` owns how a host ingests them.

- `buffer.go` — `Buffer`, `Op`, `Change`, the options, and every reader.
- `ops.go` — `Do`'s dispatch, the single `editRun` mutation, motions, deletes, clipboard, selection, history.
- `verb.go` — the verb constants and `verbTable`: the one place a verb's name is written down.
- `doc.go` — logical lines with lazily cached grapheme-cluster boundaries.
- `words.go` — `CharClass`, `DefaultClassifier`, and the position walkers word motion is built from.
- `wrap.go` — `VisualLine`, the greedy wrap, and cursor-to-row mapping.
- `history.go` — the undo entry, its selection states, and the stack caps.
- `transform.go` — transpose and the case verbs.
- `keymap/` — optional readline-ish chord defaults. Nothing in the core depends on it.

## Build & test

ALWAYS build and test with `go-toolchain` (no args) from the repo root. NEVER run bare `go build` / `go test` /
`go mod tidy`, and never pipe or redirect the toolchain's output.

- It refuses a dirty tree when it needs to auto-fix a file: commit first, run it, commit its rewrites as a follow-up.
- Its rewrites are canonical (formatting, imports, testify conversion in tests, `go.mod`). Never revert them.
- Tests use testify (`assert`/`require`). Write them that way from the start; the toolchain will convert them if not,
  and its conversion can flatten a hand-written failure message into a bare `assert.Equal`.

## Hard rules

- **One mutating entry point.** `Do(Op) Change` is the only writer of text; `SetWidth`/`SetText`/`Seal` are
  configuration. Never add a second mutator, a per-verb method, or a callback/observer — `Change` is returned to the
  caller that asked for it, never published.
- **`Extend` is a field, not a second set of verbs.** `Shift+Left` is `Left` with `Extend: true`. Never add
  `SelectLeft`-shaped verbs.
- **One clipboard slot, written only by `Cut` and `Copy`.** No ring, no history, no put-back-the-one-before state.
  `Cut`/`Copy` with no selection do nothing at all — not a line, not a word, not a cleared slot. `Paste` does not
  consume. **No delete verb ever touches the clipboard**; undo is the recovery path, and a property test enforces this
  over random streams.
- **No clock.** `Seal()` ends a typing run and the host decides when. Never add a timer, a duration, or an idle rule.
- **The verb table is the only place a name is written down.** A host builds its action registry by iterating
  `Verbs()`. Adding a verb means one row plus tests; never hand-maintain a second list.
- **Do not prune verbs.** The owner reserved that decision for after trying them (spec: *Everything else is in, and the
  pruning is yours*). The transforms and `N` stay until he says otherwise.
- **Motions are visual, paragraph deletes never are.** `LineStart`/`LineEnd`/`Up`/`Down` work on the wrapped row;
  `ParagraphStart`/`ParagraphEnd` and both paragraph deletes work on the logical line. A delete must never stop at a
  boundary the window width invented.
- **The unit of movement is the grapheme cluster; the unit of width is the terminal cell.** Both come from
  `github.com/rivo/uniseg`, which is the whole dependency list. Never index text by rune or byte for a cursor position,
  and never measure width with `len` or a rune count.
- **`Do` never panics and never errors.** An out-of-range `Pos`, a negative `N`, an operation with nothing to act on:
  each clamps or reports a no-op `Change`.
- Exact behaviors pinned by the golden tests are contract: the word-boundary table, the whitespace-plus-unit rule for
  `DeleteWordBack`, typing-run merge conditions, undo restoring the selection, and the stack caps (200 entries or 4 MB,
  oldest dropped). Do not "improve" them without the spec changing first.

## Testing

`script_test.go` is a script DSL over the operation stream — `type("foo bar")|selectWordAt(4)|cut|paste|paste` with the
text, cursor, selection and clipboard asserted after every step. Each place the library deliberately disagrees with
readline has a case whose comment states the readline answer, so quietly restoring it fails loudly.
`property_test.go` fuzzes random operation streams for the invariants (undo-to-empty then redo-to-end reproduces the
final text, the cursor stays on a cluster boundary inside the buffer, visual lines concatenate back to the text, no
delete verb writes the clipboard, `Do` never panics). `words_test.go` is the word-boundary corpus, because that table
is the most opinionated thing here.

## Interpretations where the spec was silent

Recorded so nobody has to re-derive them; each is the least-surprising answer, not a preference.

- `Paste` reads the slot and ignores `Op.Text`. The spec contradicted itself here (its prose said slot, its `Op`
  field comment listed `Text` for `Paste`) and was resolved toward the prose: text a host already holds goes in
  through `InsertText`, so the slot has one writer and one reader.
- A bare motion collapses the selection **and** moves, per the spec's wording — not the GUI habit of collapsing to the
  selection edge without moving.
- `Up`/`Down` keep a sticky goal column, and at the first/last row they stay put, which is what lets a host bind
  history recall there.
- `LineEnd` on a soft-broken row stops where the text stops, dropping the whitespace the break consumed.
- A paragraph delete at a paragraph boundary joins with the neighbouring line rather than doing nothing.
- Any delete verb with a selection deletes the selection; the case verbs recase a selection when there is one.
- `SelectParagraphAt` takes the logical line without its trailing newline.
- `SetText` leaves the cursor at the end of the new text.

## CI

`.github/workflows/ci.yml` runs the org go-toolchain action. Trigger stays `on: push:` only. The repo is private, so
`runs-on` is `${{ vars.CI_RUNNER || 'ubuntu-latest' }}`. The job is named `build`: never name a job, workflow or check
`all-builds` — the required status is posted by the org's required-builds-manager app, and a job by that name hard-fails
the run. The `permissions` block is required as written; don't trim it.

## Git workflow

One branch and one PR per session, named `claude/<name>`. Commit and push often — the VM is ephemeral. PRs are
squash-merged: add follow-up commits, never rebase or force-push a pushed branch.

## Documentation upkeep

A change to the API surface or to any pinned behavior updates `README.md` and this file in the same commit. If the
change contradicts `editor.md`, say so out loud rather than deviating silently.
