package solo

import "testing"

func TestAlone(t *testing.T) {
	if Alone != 9 {
		t.Fatal("alone")
	}
}
