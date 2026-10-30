package cmd

import (
	"testing"

	"example.com/workspace/core/lib"
)

func TestAnswerAcrossModules(t *testing.T) {
	if lib.Answer() != 42 {
		t.Fatal("answer")
	}
}
