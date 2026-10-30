package leaf

import "testing"

func TestTripled(t *testing.T) {
	if Tripled != 3 {
		t.Fatal("tripled")
	}
}
