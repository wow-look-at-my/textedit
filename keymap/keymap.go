// Package keymap is readline-ish chord defaults for a host that has no keymap
// of its own. It is optional: nothing in textedit depends on it, and a host
// that already parses chords should build its own table from textedit.Verbs().
package keymap

import (
	"strings"

	"github.com/wow-look-at-my/textedit"
)

var defaults = map[string]textedit.Op{
	"Left":       {Verb: textedit.Left},
	"Right":      {Verb: textedit.Right},
	"Up":         {Verb: textedit.Up},
	"Down":       {Verb: textedit.Down},
	"Home":       {Verb: textedit.LineStart},
	"End":        {Verb: textedit.LineEnd},
	"Ctrl+A":     {Verb: textedit.LineStart},
	"Ctrl+E":     {Verb: textedit.LineEnd},
	"Alt+B":      {Verb: textedit.WordLeft},
	"Alt+F":      {Verb: textedit.WordRight},
	"Ctrl+Left":  {Verb: textedit.WordLeft},
	"Ctrl+Right": {Verb: textedit.WordRight},
	"Alt+<":      {Verb: textedit.BufferStart},
	"Alt+>":      {Verb: textedit.BufferEnd},
	"Ctrl+Home":  {Verb: textedit.BufferStart},
	"Ctrl+End":   {Verb: textedit.BufferEnd},

	"Enter": {Verb: textedit.InsertText, Text: "\n"},

	"Backspace":      {Verb: textedit.DeleteBack},
	"Delete":         {Verb: textedit.DeleteForward},
	"Ctrl+D":         {Verb: textedit.DeleteForward},
	"Ctrl+Backspace": {Verb: textedit.DeleteWordBack},
	"Alt+Backspace":  {Verb: textedit.DeleteWordBack},
	"Ctrl+Delete":    {Verb: textedit.DeleteWordForward},
	"Alt+Delete":     {Verb: textedit.DeleteWordForward},
	"Ctrl+U":         {Verb: textedit.DeleteToParagraphStart},
	"Ctrl+K":         {Verb: textedit.DeleteToParagraphEnd},

	"Ctrl+X": {Verb: textedit.Cut},
	"Alt+W":  {Verb: textedit.Copy},
	"Ctrl+V": {Verb: textedit.Paste},

	"Ctrl+Shift+A": {Verb: textedit.SelectAll},
	"Escape":       {Verb: textedit.SelectNone},

	"Ctrl+T": {Verb: textedit.TransposeChars},
	"Alt+T":  {Verb: textedit.TransposeWords},
	"Alt+U":  {Verb: textedit.UpcaseWord},
	"Alt+L":  {Verb: textedit.DowncaseWord},
	"Alt+C":  {Verb: textedit.CapitalizeWord},

	"Ctrl+_":      {Verb: textedit.Undo},
	"Alt+Z":       {Verb: textedit.Undo},
	"Alt+Shift+_": {Verb: textedit.Redo},
}

// Defaults returns the chord table.
func Defaults() map[string]textedit.Op {
	out := make(map[string]textedit.Op, len(defaults))
	for k, v := range defaults {
		out[k] = v
	}
	return out
}

// Lookup resolves one chord. Shift in front of a motion sets Extend rather than
// needing a second table entry.
func Lookup(chord string) (textedit.Op, bool) {
	if op, ok := defaults[chord]; ok {
		return op, true
	}
	if rest, cut := strings.CutPrefix(chord, "Shift+"); cut {
		if op, ok := defaults[rest]; ok && op.Verb.Info().Motion {
			op.Extend = true
			return op, true
		}
	}
	return textedit.Op{}, false
}
