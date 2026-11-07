// SPDX-License-Identifier: Apache-2.0
package gateselect

import (
	"strings"
	"testing"
)

// AFP-V0-2744 (V1-4933): the CI selector's rule (c) for a record entry, a
// sealed CEM or a build-log entry, keeps only the packages whose token
// resolves to the entry or its record directory, as the planner does
// (AFP-V0-2743). A fixture-root token that resolves inside the package
// (`docs/build-log/`), a lone `.corvint` or `docs` beside a climbing `..`,
// and a climbing token that stops at an ancestor (`../.corvint`, `../docs`)
// no longer select their package; a token that names the directory from the
// package and one that names the entry itself still do, so no real reader is
// dropped. Before the change the build-log entry selected the fixture and
// ancestor holders (beamfall/corvint#106: 13 such readers), and the sealed
// CEM selected the climbing holders (5).
func TestRecordEntryReadersResolveToTheDirectory_AFPV2744(t *testing.T) {
	const sealed = ".corvint/changes/0123456789abcdef0123456789abcdef01234567.cem.json" //corvint:fixture-path
	const entry = "docs/build-log/2026-11-06-selgo.md"                                  //corvint:fixture-path
	root := writeFixture(t, map[string]string{
		"lister/lister_test.go":       "package lister\n\nvar a, b = \"../.corvint/changes\", \"../docs/build-log/\"\n", //corvint:fixture-path
		"named/named_test.go":         "package named\n\nvar a, b = \"../" + sealed + "\", \"../" + entry + "\"\n",
		"temp/temp_test.go":           "package temp\n\nvar a, b = \".corvint/changes/\", \"docs/build-log/\"\n", //corvint:fixture-path
		"lone/lone_test.go":           "package lone\n\nvar up, a, b = \"..\", \".corvint\", \"docs\"\n",
		"climbs/climbs_test.go":       "package climbs\n\nvar a, b = \"../.corvint\", \"../docs\"\n",
		"unrelated/unrelated_test.go": "package unrelated\n\nvar a = \"../docs/specs/\"\n", //corvint:fixture-path
	})
	for _, dirty := range []string{sealed, entry} {
		t.Run(dirty, func(t *testing.T) {
			var plan Receipt
			plan.Provider.Go.State = "RUNNABLE"
			plan.Plan.Dirty = []string{dirty}
			lines := selectPackages(plan, fixtureModule, root, true)
			readers := map[string]bool{}
			for _, line := range lines {
				if rest, ok := strings.CutPrefix(line, "reader "+fixtureModule+"/"); ok {
					readers[strings.Fields(rest)[0]] = true
				}
			}
			for _, want := range []string{"lister", "named"} {
				if !readers[want] {
					t.Errorf("%s is not a reader of %s:\n%s", want, dirty, strings.Join(lines, "\n"))
				}
			}
			for _, not := range []string{"temp", "lone", "climbs", "unrelated"} {
				if readers[not] {
					t.Errorf("%s reads %s through no token that resolves to its record directory:\n%s", not, dirty, strings.Join(lines, "\n"))
				}
			}
			bounds, err := packageBounds(fixtureModule, root, []string{dirty})
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Join(bounds, ""); strings.Contains(got, fixtureModule+"/temp ") || !strings.Contains(got, "bound "+fixtureModule+"/lister reader "+dirty) {
				t.Fatalf("bounds disagree with selection:\n%s", got)
			}
		})
	}
}
