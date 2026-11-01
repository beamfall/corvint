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

// TestCAPV0820_VerbGroupsRefuseUnknownVerb: every verb group refuses an
// unknown word MALFORMED, naming the group and the word, and suggests the
// nearest verb of that group (CAP-V0-820).
func TestCAPV0820_VerbGroupsRefuseUnknownVerb(t *testing.T) {
	r := fixture.TempRepo(t)
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"queue", "stats"}, "unknown queue verb `stats`; did you mean `corvint-tasks queue status --help`?"},
		{[]string{"plan", "previwe"}, "unknown plan verb `previwe`; did you mean `corvint-tasks plan preview --help`?"},
		{[]string{"policy", "shwo"}, "unknown policy verb `shwo`; did you mean `corvint-tasks policy show --help`?"},
		{[]string{"program", "shwo"}, "unknown program verb `shwo`; did you mean `corvint-tasks program show --help`?"},
		{[]string{"slots", "acquir"}, "unknown slots verb `acquir`; did you mean `corvint-tasks slots acquire --help`?"},
		{[]string{"service", "statsu"}, "unknown service verb `statsu`; did you mean `corvint-tasks service status --help`?"},
		{[]string{"worktree", "ensur"}, "unknown worktree verb `ensur`; did you mean `corvint-tasks worktree ensure --help`?"},
		{[]string{"criterion-binding", "captur"}, "unknown criterion-binding verb `captur`; did you mean `corvint-tasks criterion-binding capture --help`?"},
		{[]string{"release", "promot"}, "unknown release verb `promot`; did you mean `corvint-tasks release promote --help`?"},
		{[]string{"attempt", "shwo"}, "unknown attempt verb `shwo`; did you mean `corvint-tasks attempt show --help`?"},
	} {
		x := atm(t, r.Root, nil, tc.args...)
		warnings := strings.Join(x.res.Warnings, ";")
		if x.code == 0 || x.res.Outcome != wire.OutcomeError || !reflect.DeepEqual(x.res.Codes, []string{wire.CodeMalformed}) ||
			!reflect.DeepEqual(x.res.Command, tc.args) || !strings.HasPrefix(warnings, tc.want) {
			t.Errorf("%q: exit %d %s", tc.args, x.code, x.stdout)
		}
	}
}

// TestCAPV0820_NoSuggestionBeyondThreshold: a word with no verb within the
// edit-distance bound gets the plain help pointer, never a farther verb
// (`zz` is 3 edits from `run`, above the bound of 2) (CAP-V0-820).
func TestCAPV0820_NoSuggestionBeyondThreshold(t *testing.T) {
	r := fixture.TempRepo(t)
	x := atm(t, r.Root, nil, "zz")
	if warnings := strings.Join(x.res.Warnings, ";"); x.code == 0 || !reflect.DeepEqual(x.res.Codes, []string{wire.CodeMalformed}) || warnings != "unknown verb `zz`; run `corvint-tasks help`" {
		t.Errorf("zz: exit %d %s", x.code, x.stdout)
	}
}
