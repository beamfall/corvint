package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gokernel"
)

// A hostile invalid choice or help topic is rendered as a Python repr, as
// argparse's %r does, so a quote or newline cannot end the quoted span or forge
// a second diagnostic line (CodeQL alerts 27 and 38). Ordinary values keep
// their exact argparse bytes.
func TestInvalidChoiceAndHelpTopicEscapeHostileValue(t *testing.T) {
	t.Parallel()
	message := func(err error) string {
		var kernel *gokernel.Error
		if !errors.As(err, &kernel) {
			t.Fatalf("not a kernel error: %v", err)
		}
		return kernel.Message
	}
	got := message(invalidChoice("command", "it's \"x\"\nnext", []string{"a", "b"}))
	if want := `argument command: invalid choice: 'it\'s "x"\nnext' (choose from 'a', 'b')`; got != want {
		t.Fatalf("hostile choice = %q, want %q", got, want)
	}
	if got := message(invalidChoice("command", "it's", []string{"a"})); got != `argument command: invalid choice: "it's" (choose from 'a')` {
		t.Fatalf("apostrophe choice = %q", got)
	}
	if got := message(invalidChoice("command", "nope", []string{"a", "b"})); got != `argument command: invalid choice: 'nope' (choose from 'a', 'b')` {
		t.Fatalf("plain choice bytes changed: %q", got)
	}
	topic := message(unknownHelpTopic([]string{"x'", "\ny"}))
	if !strings.HasPrefix(topic, `unknown help topic: "x' \ny" (choose from 'init', `) || strings.Contains(topic, "\n") {
		t.Fatalf("hostile help topic = %q", topic)
	}
	if plain := message(unknownHelpTopic([]string{"nope"})); !strings.HasPrefix(plain, `unknown help topic: 'nope' (choose from 'init', `) {
		t.Fatalf("plain help topic bytes changed: %q", plain)
	}
}
