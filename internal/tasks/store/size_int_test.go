package store

import (
	"errors"
	"strconv"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestSizeIntRefusesSizesAboveInt: an inventory Size the platform int cannot
// hold is refused LIMIT_EXCEEDED instead of wrapping into a negative length.
func TestSizeIntRefusesSizesAboveInt(t *testing.T) {
	if n, err := sizeInt("receipts/x", wire.Size("42")); err != nil || n != 42 {
		t.Fatalf("sizeInt(42) = %d, %v", n, err)
	}
	if strconv.IntSize < 64 {
		t.Skip("uint64 overflow case needs a 64-bit int to state")
	}
	_, err := sizeInt("receipts/x", wire.Size("18446744073709551615"))
	var we *wire.Error
	if !errors.As(err, &we) || we.Code != wire.CodeLimitExceeded {
		t.Fatalf("sizeInt(max uint64) error = %v, want LIMIT_EXCEEDED", err)
	}
}
