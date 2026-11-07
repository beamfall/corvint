package store

import (
	"errors"
	"math"
	"strconv"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestSizeIntRefusesSizesAboveInt: an inventory Size the platform int cannot
// hold is refused LIMIT_EXCEEDED instead of wrapping into a negative length,
// and math.MaxInt itself is the largest size admitted.
func TestSizeIntRefusesSizesAboveInt(t *testing.T) {
	for _, admitted := range []uint64{0, 42, math.MaxInt} {
		if n, err := sizeInt("receipts/x", wire.SizeOf(admitted)); err != nil || uint64(n) != admitted {
			t.Fatalf("sizeInt(%d) = %d, %v", admitted, n, err)
		}
	}
	for _, refused := range []uint64{uint64(math.MaxInt) + 1, math.MaxUint64} {
		_, err := sizeInt("receipts/x", wire.SizeOf(refused))
		var we *wire.Error
		if !errors.As(err, &we) || we.Code != wire.CodeLimitExceeded {
			t.Fatalf("sizeInt(%s) error = %v, want LIMIT_EXCEEDED", strconv.FormatUint(refused, 10), err)
		}
	}
}
