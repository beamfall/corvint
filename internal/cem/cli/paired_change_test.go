package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

// TestCEMV1648CEMRefusesAPairedChange: every cem action refuses the
// witness's paired-change flags with the reason, before its required-argument
// stage, so a paired change never reaches a single-repository producer and the
// caller is not told only that the flag is unknown.
func TestCEMV1648CEMRefusesAPairedChange(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, arguments := range [][]string{
		{"prepare", "--range", "app=a..b", "--range", "tests=c..d"},
		{"prepare", "--base", "HEAD~1", "--range=app=a..b"},
		{"begin", "--range-root", "app=/x"},
		{"status", "--map", "m.json", "--paired-from", "HEAD"},
		{"report", "--map", "m.json", "--range-root=tests=/y"},
	} {
		_, err := dispatchCEM(context.Background(), root, arguments)
		message := cemcode.MessageOf(err)
		if cemcode.CodeOf(err) != cemcode.InvalidArguments || !strings.Contains(message, "cem takes no paired change") || !strings.Contains(message, "CEM-V1-648") {
			t.Fatalf("%v: %v, want the paired-change refusal", arguments, err)
		}
	}
	// A flag that only shares the prefix stays an ordinary unknown argument.
	if _, err := dispatchCEM(context.Background(), root, []string{"status", "--map", "m.json", "--ranges", "x"}); !strings.Contains(cemcode.MessageOf(err), "unrecognized arguments: --ranges x") {
		t.Fatalf("--ranges: %v", err)
	}
}
