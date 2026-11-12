// SPDX-License-Identifier: Apache-2.0
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// planLists is what PROD-INT-006 of the accepted plan says a release ships.
type planLists struct {
	targets  []string
	products []string
}

var (
	planTargetsClause = regexp.MustCompile(`archives for ((?:[a-z0-9]+/[a-z0-9]+(?:, | and ))*[a-z0-9]+/[a-z0-9]+) with`)
	planTargetItem    = regexp.MustCompile(`[a-z0-9]+/[a-z0-9]+`)
	planBinaryItem    = regexp.MustCompile("`(corvint(?:-[a-z0-9]+)*)`")
)

// parsePlanLists reads the PROD-INT-006 block: the targets from its "archives for ... with"
// clause and the shipped binaries from the "Binaries per product" note, up to the sentence
// about the Tasks binaries, which ship from another repository. Anything it cannot find fails:
// the test never falls back to a remembered list (ARTIFACT-BIN-V0-521).
func parsePlanLists(plan string) (planLists, error) {
	start := strings.Index(plan, "- `PROD-INT-006`:")
	if start < 0 {
		return planLists{}, fmt.Errorf("plan has no PROD-INT-006 definition")
	}
	block := plan[start:]
	if end := strings.Index(block[1:], "\n- `PROD-"); end >= 0 {
		block = block[:end+1]
	}
	flat := strings.Join(strings.Fields(block), " ")
	clause := planTargetsClause.FindStringSubmatch(flat)
	if clause == nil {
		return planLists{}, fmt.Errorf("PROD-INT-006 has no \"archives for ... with\" target clause")
	}
	lists := planLists{targets: planTargetItem.FindAllString(clause[1], -1)}
	note := strings.Index(flat, "Binaries per product")
	if note < 0 {
		return planLists{}, fmt.Errorf("PROD-INT-006 has no \"Binaries per product\" note")
	}
	note += strings.Index(flat[note:], ":")
	stop := strings.Index(flat[note:], "Tasks binaries ship")
	if stop < 0 {
		return planLists{}, fmt.Errorf("the \"Binaries per product\" note has no \"Tasks binaries ship\" end")
	}
	for _, match := range planBinaryItem.FindAllStringSubmatch(flat[note:note+stop], -1) {
		lists.products = append(lists.products, match[1])
	}
	if len(lists.targets) == 0 || len(lists.products) == 0 {
		return planLists{}, fmt.Errorf("PROD-INT-006 names %d targets and %d binaries", len(lists.targets), len(lists.products))
	}
	slices.Sort(lists.targets)
	slices.Sort(lists.products)
	return lists, nil
}

// planDrift names every difference between the catalog and the plan's lists.
func planDrift(catalog Catalog, plan planLists) []string {
	var targets []string
	for _, target := range catalog.Targets {
		targets = append(targets, target.id())
	}
	products := catalog.products("")
	var drift []string
	compare := func(what string, have, want []string) {
		for _, item := range want {
			if !slices.Contains(have, item) {
				drift = append(drift, fmt.Sprintf("catalog lacks %s %s that PROD-INT-006 ships", what, item))
			}
		}
		for _, item := range have {
			if !slices.Contains(want, item) {
				drift = append(drift, fmt.Sprintf("catalog has %s %s that PROD-INT-006 does not ship", what, item))
			}
		}
	}
	compare("target", targets, plan.targets)
	compare("product binary", products, plan.products)
	slices.Sort(drift)
	return drift
}

// ARTIFACT-BIN-V0-520, -521: the embedded catalog ships exactly the targets and product
// binaries the accepted plan's PROD-INT-006 names.
func TestCatalogMatchesTheAcceptedPlan(t *testing.T) {
	catalog, err := loadCatalog("")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(repositoryRoot(t), "docs", "specs", "PRODUCT-PLAN-1.0.md"))
	if err != nil {
		t.Fatal(err)
	}
	plan, err := parsePlanLists(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(plan.targets, []string{"darwin/arm64", "linux/amd64", "linux/arm64"}) {
		t.Errorf("PROD-INT-006 targets read as %v", plan.targets)
	}
	if drift := planDrift(catalog, plan); len(drift) > 0 {
		t.Fatalf("binaries.json disagrees with the accepted plan:\n%s", strings.Join(drift, "\n"))
	}
}

// ARTIFACT-BIN-V0-521: each kind of drift is named, and a plan block the test cannot read fails
// instead of passing.
func TestPlanDriftIsNamed(t *testing.T) {
	const plan = "- `PROD-INT-006`: (accepted) A user MUST be able to install each product: archives for\n" +
		"  linux/amd64, linux/arm64 and darwin/arm64 with `SHA256SUMS`.\n" +
		"  - Binaries per product (owner decision R14; read from `tools/release-binary/binaries.json`), each built for the three targets:\n" +
		"    testing `corvint`, `corvint-mcp`; update `corvint-update`; Tasks binaries ship from the Tasks repository. `corvint-cli` is open.\n" +
		"- `PROD-INT-007`: next.\n"
	lists, err := parsePlanLists(plan)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(lists.products, []string{"corvint", "corvint-mcp", "corvint-update"}) {
		t.Fatalf("products %v: the note must stop at the Tasks sentence and skip the catalog path", lists.products)
	}
	target := func(goos, goarch string) Target { return Target{GOOS: goos, GOARCH: goarch} }
	product := func(name string) Binary { return Binary{Name: name, Role: "product"} }
	three := []Target{target("darwin", "arm64"), target("linux", "amd64"), target("linux", "arm64")}
	shipped := []Binary{product("corvint"), product("corvint-mcp"), product("corvint-update")}
	cases := []struct {
		name     string
		targets  []Target
		binaries []Binary
		want     []string
	}{
		{"agrees", three, shipped, nil},
		{"extra target", append(slices.Clone(three), Target{GOOS: "windows", GOARCH: "amd64", Suffix: ".exe"}), shipped,
			[]string{"catalog has target windows/amd64 that PROD-INT-006 does not ship"}},
		{"missing target", three[1:], shipped, []string{"catalog lacks target darwin/arm64 that PROD-INT-006 ships"}},
		{"removed binary still product", three, append(slices.Clone(shipped), product("corvint-analyzer-go")),
			[]string{"catalog has product binary corvint-analyzer-go that PROD-INT-006 does not ship"}},
		{"shipped binary withheld", three, []Binary{product("corvint"), product("corvint-mcp"), {Name: "corvint-update", Role: "withheld"}},
			[]string{"catalog lacks product binary corvint-update that PROD-INT-006 ships"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := planDrift(Catalog{Targets: tc.targets, Binaries: tc.binaries}, lists)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("drift %q, want %q", got, tc.want)
			}
		})
	}
	for name, broken := range map[string]string{
		"no block":       "- `PROD-INT-005`: other.\n",
		"no target list": strings.Replace(plan, "archives for", "packages for", 1),
		"no binary note": strings.Replace(plan, "Binaries per product", "Executables", 1),
		"no tasks end":   strings.Replace(plan, "Tasks binaries ship", "Tasks binaries are", 1),
	} {
		if _, err := parsePlanLists(broken); err == nil {
			t.Errorf("%s: an unreadable plan block must fail", name)
		}
	}
}

// ARTIFACT-BIN-V0-520: a withheld binary stays classified but is never selected or built.
func TestWithheldBinaryIsNeverSelectedOrBuilt(t *testing.T) {
	catalog, err := loadCatalog("")
	if err != nil {
		t.Fatal(err)
	}
	binary, ok := catalog.find("corvint-analyzer-go")
	if !ok || binary.Role != "withheld" || binary.Decision != "0596" || binary.VersionSet != "" || binary.Stamp != "" {
		t.Fatalf("corvint-analyzer-go: want withheld under decision 0596 with no set or stamp, got %+v", binary)
	}
	if _, err := selectTag(catalog, "corvint-analyzer-go/v1.0.0"); err == nil || !strings.HasPrefix(err.Error(), "withheld-binary:") {
		t.Fatalf("select of a withheld binary: %v", err)
	}
	all, err := selectTag(catalog, "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(all.Binaries, "corvint-analyzer-go") {
		t.Fatalf("vX.Y.Z selects a withheld binary: %v", all.Binaries)
	}
	f := newFixture(t)
	withheld := strings.Replace(sprintfCatalog("linux", "amd64", ""), `"role": "product", "decision": "0594"}`, `"role": "withheld", "decision": "0596"}`, 1)
	if err := os.WriteFile(f.catalog, []byte(withheld), 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "release")
	code, _, stderr := runCLI(t, "build", "--root", f.root, "--binaries", f.catalog, "--binary", "corvint-gamma", "--version", "1.0.0", "--output", output)
	requireRefusal(t, code, stderr, "withheld-binary")
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("a refused build wrote %s: %v", output, err)
	}
	for _, bad := range []string{
		`"role": "withheld", "versionSet": "core", "decision": "0596"}`,
		`"role": "withheld", "stamp": "none", "decision": "0596"}`,
	} {
		broken := strings.Replace(sprintfCatalog("linux", "amd64", ""), `"role": "product", "decision": "0594"}`, bad, 1)
		path := filepath.Join(t.TempDir(), "binaries.json")
		if err := os.WriteFile(path, []byte(broken), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := loadCatalog(path); err == nil {
			t.Errorf("a withheld binary with a set or stamp rule must refuse: %s", bad)
		}
	}
}
