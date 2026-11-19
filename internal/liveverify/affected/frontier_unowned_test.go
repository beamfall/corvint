// SPDX-License-Identifier: Apache-2.0
package affected

import (
	"context"
	"slices"
	"testing"
)

type sourceOnlyDirected struct {
	directed
	only []string
}

func (language sourceOnlyDirected) SourceOnlyFrontiers() []string { return language.only }

// AFP-V0-511 and AZU-V1-960: a static source-only proof closes bare entries,
// but never replaces located read evidence or the validated source-bound declaration.
func TestSourceOnlyUnownedFrontiers_AFPV0511(t *testing.T) {
	for _, tc := range []struct {
		name, entry                          string
		unclassified, steered, bounded, want bool
	}{
		{name: "bare", entry: "y:import"},
		{name: "unclassified", entry: "y:import", unclassified: true, want: true},
		{name: "resolution input", entry: "y:import", steered: true, want: true},
		{name: "located read", entry: "y:import:app.y:1 `./data.json`", want: true},
		{name: "unowned site", entry: "y:import:config.json", want: true},
		{name: "unknown site", entry: "y:import:future(site=app.y)", want: true},
		{name: "overflow", entry: "y:import:+2", want: true},
		{name: "bounded site", entry: "y:import:app.y", bounded: true, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			language := sourceOnlyDirected{
				directed: directed{reaching: reaching{
					fake: fake{name: "y", frontier: []string{tc.entry}, units: []Unit{{
						ID: "y:app", Sources: []string{"app.y"}, PathTokensBounded: tc.bounded,
					}}},
					inputs: func(string) bool { return tc.steered },
				}},
				only: []string{"y:import"},
			}
			if !tc.unclassified {
				language.sourceBound = []string{"y:import"}
			}
			graph, err := Build(context.Background(), t.TempDir(), language)
			if err != nil {
				t.Fatal(err)
			}
			plan := Select(graph, []string{"data.json"})
			if got := slices.Contains(languageFrontier(plan), tc.entry); got != tc.want {
				t.Fatalf("frontier bears=%v want %v: %+v", got, tc.want, plan.Unknown)
			}
		})
	}
}

// AZU-V1-960: a source-bound declaration cannot erase a named foreign read,
// including an overflow site's tokens or a resolution table with no owner.
func TestUnownedSourceBoundReads_AZU_V1_960(t *testing.T) {
	for _, tc := range []struct {
		name, entry string
		tokens      []string
		bounded     bool
		sites       map[string][]string
		want        bool
	}{
		{name: "bare", entry: "y:import", want: true},
		{name: "unrecorded overflow", entry: "y:import:+2", want: true},
		{name: "unknown site form", entry: "y:import:future(site=app.y)", want: true},
		{name: "unrelated", entry: "y:import:app.y:1 `./other.json`"},
		{name: "literal", entry: "y:import:app.y:1 `./data.json`", want: true},
		{name: "alias target", entry: "y:import:app.y:1 `@cfg`", tokens: []string{"data.json"}, want: true},
		{name: "bounded", entry: "y:import:app.y", bounded: true, want: true},
		{name: "unowned table", entry: "y:import:config.json", want: true},
		{name: "complete unrelated overflow", entry: "y:import:+1", sites: map[string][]string{"y:import": {"app.y:1 `./other.json`"}}},
		{name: "overflow", entry: "y:import:+1", sites: map[string][]string{"y:import": {"app.y:1 `./data.json`"}}, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			language := directed{reaching: reaching{fake: fake{name: "y", frontier: []string{tc.entry}, units: []Unit{{ID: "y:app", Sources: []string{"app.y"}, PathTokens: tc.tokens, PathTokensBounded: tc.bounded}}}, sourceBound: []string{"y:import"}}, sites: tc.sites}
			graph, err := Build(context.Background(), t.TempDir(), language)
			if err != nil {
				t.Fatal(err)
			}
			plan := Select(graph, []string{"data.json"})
			if got := slices.Contains(languageFrontier(plan), tc.entry); got != tc.want {
				t.Fatalf("frontier bears=%v want %v: %+v", got, tc.want, plan.Unknown)
			}
		})
	}
}

// AZU-V1-961: a nonparticipating plugin's hosted fixture runner cannot bind
// unrelated data. A dirty resolution input, explicit read, or touched fixture
// keeps the unknown; a computed edge stays open unless its host is selected.
func TestUnownedHostedFrontier_AZU_V1_961(t *testing.T) {
	const entry = "y:runner:pkg/testdata/repo/test.y"
	for _, tc := range []struct {
		name, dirty                  string
		tokens                       []string
		input, open, hostReads, want bool
	}{
		{name: "unrelated", dirty: "data.json"},
		{name: "resolution", dirty: "config.json", input: true, want: true},
		{name: "named read", dirty: "data.json", tokens: []string{"data.json"}, want: true},
		{name: "inside fixture", dirty: "pkg/testdata/repo/data.json", want: true},
		{name: "path open", dirty: "data.json", open: true, want: true},
		{name: "selected host", dirty: "data.json", open: true, hostReads: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reach := reaching{fake: fake{name: "y", frontier: []string{entry}, units: []Unit{{ID: "y:fixture", Tests: []string{"pkg/testdata/repo/test.y"}, PathTokens: tc.tokens}}}, inputs: func(string) bool { return tc.input }}
			if !tc.open {
				reach.fixtureClosed = []string{"y:runner"}
			}
			host := Unit{ID: "go:pkg", Sources: []string{"pkg/pkg.go"}, Tests: []string{"pkg/pkg_test.go"}}
			if tc.hostReads {
				host.PathTokens = []string{"data.json"}
			}
			graph, err := Build(context.Background(), t.TempDir(), reach, fake{name: "go", units: []Unit{host}})
			if err != nil {
				t.Fatal(err)
			}
			plan := Select(graph, []string{tc.dirty})
			if got := slices.Contains(languageFrontier(plan), entry); got != tc.want {
				t.Fatalf("frontier bears=%v want %v: %+v", got, tc.want, plan.Unknown)
			}
			if !tc.want && !hostedFixtureClosed(plan, entry) {
				t.Fatalf("missing fixture proof: %+v", plan.FrontierExcluded)
			}
		})
	}
}
