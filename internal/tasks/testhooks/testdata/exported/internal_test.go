package p

import "testing"

func TestDirectWrite(t *testing.T) {
	publishFault = nil
}

func TestIncrementViaHelper(t *testing.T) {
	t.Parallel()
	bump()
}

func TestShadowedLocal(t *testing.T) {
	t.Parallel()
	count := 0
	count++
	_ = count
}

var enabled = true

// The parallel call sits inside an if: still parallel.
func TestNestedParallel(t *testing.T) {
	if enabled {
		t.Parallel()
	}
	publishFault = nil
}

// Only a subtest body calls Parallel: the test itself is not parallel.
func TestSubtestParallelOnly(t *testing.T) {
	publishFault = nil
	t.Run("sub", func(t *testing.T) { t.Parallel() })
}

// A nested parameter named like the hook hides only the writes inside that literal.
func TestNestedShadowKeepsOuterWrite(t *testing.T) {
	use := func(publishFault func() error) { publishFault = nil; _ = publishFault }
	use(nil)
	publishFault = nil
}

// A block variable named like the hook hides only the block's writes.
func TestBlockShadowThenWrite(t *testing.T) {
	{
		count := 0
		count++
		_ = count
	}
	count++
}

// Every write is to a shadowing name: not a hook user.
func TestNestedShadowOnly(t *testing.T) {
	t.Parallel()
	use := func(publishFault func() error) { publishFault = nil; _ = publishFault }
	use(nil)
}
