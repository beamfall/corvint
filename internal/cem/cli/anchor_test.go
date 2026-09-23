package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

// TestAnchorActionsParseLikeTheirSiblings: anchor and provenance (FPK-V0-037,
// FPK-V0-038) pass the same argparse stages as every other action, are listed
// after cover, and are an invalid choice until the binary installs GitNotes.
func TestAnchorActionsParseLikeTheirSiblings(t *testing.T) {
	root := t.TempDir()
	cases := []struct {
		arguments []string
		message   string
	}{
		{[]string{"anchor"}, "the following arguments are required: --map"},
		{[]string{"anchor", "--map", "/abs.json"}, "argument --map: path must be repository-relative"},
		{[]string{"anchor", "--map", "m.json", "--bogus"}, "unrecognized arguments: --bogus"},
		{[]string{"provenance"}, "the following arguments are required: --commit"},
		{[]string{"bogus"}, "'report', 'cover', 'discriminate', 'anchor', 'provenance')"},
		{[]string{"anchor", "--map", "m.json"}, "argument cem_command: invalid choice: 'anchor'"},
		{[]string{"provenance", "--commit", "HEAD"}, "argument cem_command: invalid choice: 'provenance'"},
	}
	if GitNotes != nil {
		t.Fatal("GitNotes must be uninstalled in this package's tests")
	}
	for _, test := range cases {
		_, err := dispatchCEM(context.Background(), root, test.arguments)
		if cemcode.CodeOf(err) != cemcode.InvalidArguments || !strings.Contains(cemcode.MessageOf(err), test.message) {
			t.Fatalf("%v: %v, want %q", test.arguments, err, test.message)
		}
	}
}
