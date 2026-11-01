package appflows

import (
	"os"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/playwrightruns"
)

// attribution is one test as a decoder attributes it (AFU-V1-950).
type attribution struct {
	file, title, project string
	attempts             int
	passed               bool
}

// TestAFUV1950PlaywrightDecodersAgree decodes one real Playwright 1.61.1 report through the three
// decoders and requires the same tests, attempts and pass verdicts from each. The report holds a
// plain pass, a test.fail() test that fails, a test.fail() test that passes on both attempts, and
// a test that passes on retry.
func TestAFUV1950PlaywrightDecodersAgree(t *testing.T) {
	raw, err := os.ReadFile("testdata/playwright-attribution.json")
	if err != nil {
		t.Fatal(err)
	}
	want := []attribution{
		{"a.spec.js", "plain pass", "p1", 1, true},
		{"a.spec.js", "expected fail fails", "p1", 1, false},
		{"a.spec.js", "expected fail passes", "p1", 2, false},
		{"a.spec.js", "flaky once", "p1", 2, true},
	}

	outcomes, infra, err := jstestprovider.ParsePlaywrightJSON(raw)
	if err != nil || infra != nil {
		t.Fatalf("ParsePlaywrightJSON: %v %+v", err, infra)
	}
	var provider []attribution
	var providerAttempts [][]string
	for _, o := range outcomes {
		passed := o.State == jstestprovider.StatePassed || o.State == jstestprovider.StateFlaky
		// A failed outcome's anchor is its failure location, which here is in the spec's own file.
		file := o.Anchor.File
		if o.FailureMessage != "" {
			file = path.Base(file)
		}
		provider = append(provider, attribution{file, o.Name, "p1", len(o.AttemptDetails), passed})
		var states []string
		for _, d := range o.AttemptDetails {
			states = append(states, string(d.State))
		}
		providerAttempts = append(providerAttempts, states)
	}

	run, err := playwrightruns.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	var runs []attribution
	for _, test := range run.Tests {
		last := ""
		if len(test.Results) > 0 {
			last = test.Results[len(test.Results)-1].Status
		}
		passed := test.Executed && last == "passed" && !test.ExpectedFail && test.Status != "unexpected"
		runs = append(runs, attribution{test.File, test.TitlePath[len(test.TitlePath)-1], test.Project, len(test.Results), passed})
		if got := test.TitlePath[0]; got != "group" {
			t.Errorf("playwrightruns title path %v lacks the describe block", test.TitlePath)
		}
	}

	ingested, err := parsePlaywrightRun(raw)
	if err != nil {
		t.Fatal(err)
	}
	var ingest []attribution
	var ingestAttempts [][]string
	for _, test := range ingested {
		file, title, _ := strings.Cut(test.key, " > ")
		ingest = append(ingest, attribution{file, title, test.project, len(test.attempts), slices.Contains([]string{"passed", "flaky"}, Classify(test.attempts))})
		var outcomes []string
		for _, a := range test.attempts {
			outcomes = append(outcomes, a.Outcome)
		}
		ingestAttempts = append(ingestAttempts, outcomes)
	}

	for name, got := range map[string][]attribution{"ParsePlaywrightJSON": provider, "playwrightruns": runs, "appflows run ingest": ingest} {
		if !slices.Equal(got, want) {
			t.Errorf("%s attributes\n%+v\nwant\n%+v", name, got, want)
		}
	}
	if !slices.EqualFunc(providerAttempts, ingestAttempts, slices.Equal[[]string]) {
		t.Errorf("attempt outcomes differ: ParsePlaywrightJSON %v, run ingest %v", providerAttempts, ingestAttempts)
	}
}
