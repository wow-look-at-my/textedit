package keymap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultsResolveToRealVerbs(t *testing.T) {
	d := Defaults()
	require.NotEmpty(t, d)
	for chord, op := range d {
		require.NotEmpty(t, op.Verb.String(), "chord %q", chord)
		require.NotEqual(t, "unknown", op.Verb.String(), "chord %q", chord)
	}

	// The returned map is a copy.
	delete(d, "Left")
	_, ok := Lookup("Left")
	assert.True(t, ok)
}

func TestLookup(t *testing.T) {
	op, ok := Lookup("Ctrl+A")
	require.True(t, ok)
	assert.Equal(t, "lineStart", op.Verb.String())
	assert.False(t, op.Extend)

	// Shift in front of a motion sets Extend rather than needing a second entry.
	op, ok = Lookup("Shift+Left")
	require.True(t, ok)
	assert.Equal(t, "left", op.Verb.String())
	assert.True(t, op.Extend)

	// Shift in front of anything else is not a binding.
	_, ok = Lookup("Shift+Ctrl+X")
	assert.False(t, ok)
	_, ok = Lookup("Shift+Backspace")
	assert.False(t, ok)
	_, ok = Lookup("Ctrl+Q")
	assert.False(t, ok)

	nl, ok := Lookup("Enter")
	require.True(t, ok)
	assert.Equal(t, "insertText", nl.Verb.String())
	assert.Equal(t, "\n", nl.Text)
}
