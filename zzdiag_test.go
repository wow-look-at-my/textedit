package textedit

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestZZDiag(t *testing.T) {
	for seed := int64(0); seed < 40; seed++ {
		rng := rand.New(rand.NewSource(seed))
		b := New(WithWidth(1 + rng.Intn(12)))
		ops := randomOps(rng, 40)
		var log []string
		for _, op := range ops {
			b.Do(op)
			log = append(log, fmt.Sprintf("%s(text=%q pos=%v ext=%v n=%d) -> %q undo=%d redo=%d",
				op.Verb, op.Text, op.Pos, op.Extend, op.N, b.Text(), len(b.undo), len(b.redo)))
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
		require.Equalf(t, final, b.Text(), "seed %d emptied=%q redoDepth=%d\n%s",
			seed, emptied, nredo, strings.Join(log, "\n"))
	}
}
