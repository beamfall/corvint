package cli

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

// A hostile invalid choice is rendered as a Python repr, as argparse's %r does,
// so a quote or newline cannot end the quoted span or forge a second diagnostic
// line (CodeQL alert 28). Ordinary values keep their exact argparse bytes.
func TestInvalidChoiceEscapesHostileValue(t *testing.T) {
	for _, test := range []struct{ value, want string }{
		{"it's \"x\"\nnext", `argument cem_command: invalid choice: 'it\'s "x"\nnext' (choose from 'a', 'b')`},
		{"it's", `argument cem_command: invalid choice: "it's" (choose from 'a', 'b')`},
		{"tab\there\x01\u200b", "argument cem_command: invalid choice: 'tab\\there\\x01\\u200b' (choose from 'a', 'b')"},
		{"nope", `argument cem_command: invalid choice: 'nope' (choose from 'a', 'b')`},
	} {
		err := invalidChoice("cem_command", test.value, []string{"a", "b"})
		if got := cemcode.MessageOf(err); got != test.want || strings.Contains(got, "\n") {
			t.Errorf("invalidChoice(%q) = %q, want %q", test.value, got, test.want)
		}
	}
}
