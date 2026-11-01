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
