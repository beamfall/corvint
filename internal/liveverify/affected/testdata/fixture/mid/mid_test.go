package mid

import "testing"

func TestDoubled(t *testing.T) {
	if Doubled != 2 {
		t.Fatal("doubled")
	}
}
