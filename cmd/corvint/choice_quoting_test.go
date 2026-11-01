package main

import (
	"bytes"
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

// PRS-V1-910 (V1-3313): `corvint version` names --version; other unknown
// commands keep the exact argparse text.
func TestVersionCommandNamesTheVersionFlag_PRSV1910(t *testing.T) {
	for _, test := range []struct {
		arguments []string
		suffix    bool
	}{{[]string{"version"}, true}, {[]string{"versions"}, false}} {
		var stdout, stderr bytes.Buffer
		if code := run(test.arguments, strings.NewReader(""), &stdout, &stderr); code != 2 || stdout.Len() != 0 {
			t.Fatalf("%v: exit %d stdout %q", test.arguments, code, stdout.String())
		}
		hint := "; to print the version, run corvint --version (or corvint --version --json)"
		if got := strings.Contains(stderr.String(), hint); got != test.suffix || !strings.Contains(stderr.String(), `"code": "invalid-arguments"`) || !strings.Contains(stderr.String(), "argument command: invalid choice: '"+test.arguments[len(test.arguments)-1]+"' (choose from 'init'") {
			t.Fatalf("%v: stderr %s", test.arguments, stderr.String())
		}
	}
}
