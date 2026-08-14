package textedit

import (
	"math/rand"
	"testing"
)

func TestZZDiag(t *testing.T) {
	for seed := int64(0); seed < 40; seed++ {
		rng := rand.New(rand.NewSource(seed))
		b := New(WithWidth(1 + rng.Intn(12)))
		ops := randomOps(rng, 40)
		for _, op := range ops {
			b.Do(op)
		}
		final := b.Text()
		for b.CanUndo() {
			b.Do(Op{Verb: Undo})
		}
		emptied := b.Text()
		nredo := len(b.redo)
		for b.CanRedo() {
			b.Do(Op{Verb: Redo})
		}
		if b.Text() != final {
			t.Logf("seed %d: final=%q emptied=%q redoDepth=%d after=%q", seed, final, emptied, nredo, b.Text())
			for i, op := range ops {
				t.Logf("  %2d %s text=%q pos=%v extend=%v n=%d", i, op.Verb, op.Text, op.Pos, op.Extend, op.N)
			}
			return
		}
	}
}
