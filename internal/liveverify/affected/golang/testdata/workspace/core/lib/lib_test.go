package lib

import "testing"

func TestAnswer(t *testing.T) {
	if Answer() != 42 {
		t.Fatal("answer")
	}
}
