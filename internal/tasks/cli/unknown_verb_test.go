package cli_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCAPV0820_UnknownVerbIsCodedAndSuggestsNearest (V1-3902): an unknown
// verb exits nonzero with MALFORMED, names the word and suggests the nearest
// verb, at the first word and after a verb group; help itself stays coded
// empty and OK.
func TestCAPV0820_UnknownVerbIsCodedAndSuggestsNearest(t *testing.T) {
	r := fixture.TempRepo(t)
	for _, tc := range []struct {
		args    []string
		command []string
		want    string
	}{
		{[]string{"nosuchverb"}, []string{"nosuchverb"}, "unknown verb `nosuchverb`; run `corvint-tasks help`"},
		{[]string{"roadmp"}, []string{"roadmp"}, "unknown verb `roadmp`; did you mean `corvint-tasks roadmap --help`?"},
		{[]string{"ticket", "shwo"}, []string{"ticket", "shwo"}, "unknown ticket verb `shwo`; did you mean `corvint-tasks ticket show --help`?"},
		{[]string{"receipt", "audti"}, []string{"receipt", "audti"}, "unknown receipt verb `audti`; did you mean `corvint-tasks receipt audit --help`?"},
		{[]string{"no such"}, []string{"help"}, "unknown verb; run `corvint-tasks help`"},
	} {
		x := atm(t, r.Root, nil, tc.args...)
		warnings := strings.Join(x.res.Warnings, ";")
		if x.code == 0 || x.res.Outcome != wire.OutcomeError || !reflect.DeepEqual(x.res.Codes, []string{wire.CodeMalformed}) ||
			!reflect.DeepEqual(x.res.Command, tc.command) || !strings.HasPrefix(warnings, tc.want) {
			t.Errorf("%q: exit %d %s", tc.args, x.code, x.stdout)
		}
	}
	if x := atm(t, r.Root, nil, "help"); x.code != 0 || len(x.res.Codes) != 0 || x.res.Outcome != wire.OutcomeOK {
		t.Errorf("help: exit %d %s", x.code, x.stdout)
	}
}
