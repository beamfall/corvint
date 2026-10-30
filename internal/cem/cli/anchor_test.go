package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

// TestAnchorActionsParseLikeTheirSiblings: anchor and provenance (FPK-V0-037,
// FPK-V0-038) and export (RCB-V0-001) pass the same argparse stages as every
// other action, are listed after cover, and are an invalid choice until the
// binary installs GitNotes or Export.
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
		{[]string{"bogus"}, "'report', 'cover', 'discriminate', 'anchor', 'provenance', 'export', 'defer')"},
		// cem defer and the deferral bound (CEM-V1-644, CEM-V1-645).
		{[]string{"defer"}, "the following arguments are required: --map, --intent, --reason"},
		{[]string{"defer", "--map", "m.json", "--intent", "/abs.md", "--reason", "r"}, "argument --intent: path must be repository-relative"},
		{[]string{"verify", "--map", "m.json", "--max-deferred-intents", "1"}, "unrecognized arguments: --max-deferred-intents"},
		{[]string{"status", "--map", "m.json", "--max-deferred-intents", "-1"}, "--max-deferred-intents must be a non-negative integer"},
		{[]string{"report", "--map", "m.json", "--max-deferred-intents", "x"}, "--max-deferred-intents must be a non-negative integer"},
		{[]string{"anchor", "--map", "m.json"}, "argument cem_command: invalid choice: 'anchor'"},
		{[]string{"provenance", "--commit", "HEAD"}, "argument cem_command: invalid choice: 'provenance'"},
		{[]string{"provenance", "--commit", "HEAD", "--notes-tip", "abc"}, "argument cem_command: invalid choice: 'provenance'"},
		{[]string{"export"}, "the following arguments are required: --map, --expected-base, --target, --output"},
		{[]string{"export", "--map", "/abs.json", "--expected-base", "HEAD", "--target", "HEAD", "--output", "/o"}, "argument --map: path must be repository-relative"},
		{[]string{"export", "--map", "m.json", "--expected-base", "HEAD", "--target", "HEAD", "--output", "/o"}, "argument cem_command: invalid choice: 'export'"},
	}
	if GitNotes != nil || Export != nil {
		t.Fatal("GitNotes and Export must be uninstalled in this package's tests")
	}
	for _, test := range cases {
		_, err := dispatchCEM(context.Background(), root, test.arguments)
		if cemcode.CodeOf(err) != cemcode.InvalidArguments || !strings.Contains(cemcode.MessageOf(err), test.message) {
			t.Fatalf("%v: %v, want %q", test.arguments, err, test.message)
		}
	}
}
