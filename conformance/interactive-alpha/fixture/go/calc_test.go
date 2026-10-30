package fixture

import (
	"testing"
	"time"
)

func TestAdd(t *testing.T) {
	time.Sleep(1200 * time.Millisecond)
	if got := Add(2, 3); got != 5 {
		t.Fatalf("Add(2, 3) = %d, want 5", got)
	}
}
