package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appflows"
	"github.com/Beamfall/corvint/internal/extevidence"
	"github.com/Beamfall/corvint/internal/liveverify/affected/typescript"
)

// AFU-V1-024: the strict and coverage receipts stay byte-identical to the output captured before
// the e2e-safe profile existed. Commit IDs, the temporary provider path and the provider digest vary
// per run, so they are replaced by fixed placeholders before the comparison.
func TestAFUV1024StrictAndCoverageBytesUnchanged(t *testing.T) {
	t.Parallel()
	root, base, record := affectedSelectionRepository(t)
	head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	origin := strings.TrimSpace(affectedGit(t, root, "rev-list", "--max-parents=0", "HEAD"))
	body, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	placeholders := strings.NewReplacer(record, "PROVIDER", hex.EncodeToString(sum[:]), "PROVIDER_SHA256", head, "HEAD_COMMIT", base, "BASE_COMMIT", origin, "ORIGIN_COMMIT")
	for _, profile := range []string{"strict", "coverage"} {
		_, got, stderr, code := runAffectedArguments(t, root, "--base", base, "--provider", record, "--selection-profile", profile)
		if code != 0 {
			t.Fatalf("%s: exit %d: %s", profile, code, stderr)
		}
		got = []byte(placeholders.Replace(string(got)))
		golden := filepath.Join("testdata", "affected-selection-"+profile+".golden")
		if os.Getenv("CORVINT_UPDATE_GOLDEN") == "1" {
			if err := os.WriteFile(golden, got, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		want, err := os.ReadFile(golden)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("%s receipt changed:\n%s\nwant:\n%s", profile, got, want)
		}
	}
}

// e2eCorpus is the frozen, labelled, fault-injected selection corpus of AFU-V1-040. Each case
// commits the base files (A), anchors reviews at A (B, also the coverage evidence commit), applies
// optional base changes, then commits one injected fault as HEAD. Needed lists the inventoried
// tests the fault breaks.
type e2eCorpus struct {
	Schema   string                    `json:"schema"`
	Note     string                    `json:"note"`
	Files    map[string]string         `json:"files"`
	Coverage []appflows.CoverageRecord `json:"coverage"`
	Cases    []e2eCase                 `json:"cases"`
}

type e2eCase struct {
	ID                 string                    `json:"id"`
	Fault              string                    `json:"fault"`
	Changes            map[string]*string        `json:"changes"`
	BaseChanges        map[string]*string        `json:"base_changes"`
	Discovery          string                    `json:"discovery"`
	DropCoverage       []string                  `json:"drop_coverage"`
	IncompleteCoverage []string                  `json:"incomplete_coverage"`
	CoverageOverride   []appflows.CoverageRecord `json:"coverage_override"`
	Needed             []string                  `json:"needed"`
	Expect             struct {
		State string   `json:"state"`
		Codes []string `json:"codes"`
	} `json:"expect"`
	extraCoveragePaths int
}

func loadE2ECorpus(t *testing.T) e2eCorpus {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "e2e-safe-corpus.json"))
	if err != nil {
		t.Fatal(err)
	}
	var corpus e2eCorpus
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&corpus); err != nil {
		t.Fatal(err)
	}
	return corpus
}

func (corpus e2eCorpus) find(t *testing.T, id string) e2eCase {
	t.Helper()
	for _, c := range corpus.Cases {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("no corpus case %s", id)
	return e2eCase{}
}

func e2eApply(t *testing.T, root string, changes map[string]*string) {
	t.Helper()
	for name, body := range changes {
		if body == nil {
			if err := os.Remove(filepath.Join(root, name)); err != nil {
				t.Fatal(err)
			}
			continue
		}
		shopWrite(t, root, map[string]string{name: *body})
	}
}

// buildE2ECase returns the repository and base commit of one case; the coverage file and the
// discovery record are ignored runner artifacts written after HEAD.
func buildE2ECase(t *testing.T, corpus e2eCorpus, c e2eCase) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	shopGit(t, root, "init", "-q")
	files := map[string]string{}
	for name, body := range corpus.Files {
		files[name] = strings.ReplaceAll(body, `, "reviewed_at": "@ANCHOR@"`, "")
	}
	shopWrite(t, root, files)
	a := shopCommit(t, root, "A: shop, provider and intents")
	for name, body := range corpus.Files {
		if strings.HasPrefix(name, "flows/") {
			shopWrite(t, root, map[string]string{name: strings.ReplaceAll(body, "@ANCHOR@", a)})
		}
	}
	evidence := shopCommit(t, root, "B: review at A")
	base := evidence
	if len(c.BaseChanges) != 0 {
		e2eApply(t, root, c.BaseChanges)
		base = shopCommit(t, root, "base changes")
	}
	e2eApply(t, root, c.Changes)
	head := shopCommit(t, root, "fault: "+c.ID)
	writeE2ECoverage(t, root, corpus.Coverage, c, evidence)
	switch c.Discovery {
	case "missing":
	case "base":
		writeE2EDiscovery(t, root, base)
	default:
		writeE2EDiscovery(t, root, head)
	}
	return root, base, evidence
}

func writeE2ECoverage(t *testing.T, root string, records []appflows.CoverageRecord, c e2eCase, evidence string) {
	t.Helper()
	out := []appflows.CoverageRecord{}
	for _, record := range records {
		if slices.Contains(c.DropCoverage, record.TestKey) {
			continue
		}
		if i := slices.IndexFunc(c.CoverageOverride, func(o appflows.CoverageRecord) bool { return o.TestKey == record.TestKey }); i >= 0 {
			record = c.CoverageOverride[i]
		}
		record.Commit = evidence
		record.Tiers = slices.Clone(record.Tiers)
		for i := range record.Tiers {
			record.Tiers[i].Complete = i == 0 || !slices.Contains(c.IncompleteCoverage, record.TestKey)
			record.Tiers[i].Paths = slices.Clone(record.Tiers[i].Paths)
		}
		for i := 0; i < c.extraCoveragePaths; i++ {
			record.Tiers[0].Paths = append(record.Tiers[0].Paths, fmt.Sprintf("generated/%05d.ts", i))
		}
		out = append(out, record)
	}
	raw, err := json.Marshal(appflows.CoverageFile{Schema: appflows.CoverageSchema, Records: out})
	if err != nil {
		t.Fatal(err)
	}
	shopWrite(t, root, map[string]string{"coverage.json": string(raw)})
}

// writeE2EDiscovery records what `playwright test --list` reports: every spec file in the one project.
func writeE2EDiscovery(t *testing.T, root, revision string) {
	t.Helper()
	config, err := os.ReadFile(filepath.Join(root, "playwright.config.ts"))
	if err != nil {
		t.Fatal(err)
	}
	digest, err := typescript.ObservePlaywrightSources(root, "playwright.config.ts")
	if err != nil {
		t.Fatal(err)
	}
	specs, err := filepath.Glob(filepath.Join(root, "e2e", "*.spec.ts"))
	if err != nil {
		t.Fatal(err)
	}
	units := []typescript.PlaywrightDiscoveryUnit{}
	for _, spec := range specs {
		units = append(units, typescript.PlaywrightDiscoveryUnit{Project: "chromium", Test: "e2e/" + filepath.Base(spec)})
	}
	sum := sha256.Sum256(config)
	receipt := typescript.PlaywrightDiscovery{Config: typescript.PlaywrightConfigIdentity{Path: "playwright.config.ts", SHA256: hex.EncodeToString(sum[:])},
		Profile: "playwright-discovery/0", Revision: revision, SourceDigest: digest, Units: units}
	raw, err := jsonv2.Marshal(receipt, jsonv2.Deterministic(true))
	if err != nil {
		t.Fatal(err)
	}
	shopWrite(t, root, map[string]string{"discovery.json": string(raw)})
}

func runE2ECase(t *testing.T, corpus e2eCorpus, c e2eCase) (appflows.E2ESelection, string, string) {
	t.Helper()
	root, base, evidence := buildE2ECase(t, corpus, c)
	return runE2ESelection(t, root, base), base, evidence
}

func runE2ESelection(t *testing.T, root, base string) appflows.E2ESelection {
	t.Helper()
	receipt, _, stderr, code := runAffectedArguments(t, root, "--base", base, "--provider", "e2e-provider.json", "--selection-profile", "e2e-safe")
	if code != 0 {
		t.Fatalf("affected exited %d: %s", code, stderr)
	}
	raw, err := json.Marshal(testSelection(t, receipt))
	if err != nil {
		t.Fatal(err)
	}
	var selection appflows.E2ESelection
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&selection); err != nil {
		t.Fatalf("test_selection is not e2e-safe-selection/0: %v\n%s", err, raw)
	}
	return selection
}

func e2eCodes(selection appflows.E2ESelection) []string {
	codes := []string{}
	for _, f := range selection.Fallback {
		if !slices.Contains(codes, f.Code) {
			codes = append(codes, f.Code)
		}
	}
	slices.Sort(codes)
	return codes
}

// e2eBasisReport is one basis row of the corpus report.
type e2eBasisReport struct {
	Omitted        int     `json:"omitted"`
	Unsafe         int     `json:"unsafe"`
	UnsafeRate     float64 `json:"unsafe_narrowing_rate"`
	ReductionRatio float64 `json:"reduction_ratio"`
}

type e2eCorpusReport struct {
	Schema        string                    `json:"schema"`
	Cases         int                       `json:"cases"`
	TestsPerCase  int                       `json:"tests_per_case"`
	NarrowedCases int                       `json:"narrowed_cases"`
	Bases         map[string]e2eBasisReport `json:"bases"`
	FallbackCodes map[string]int            `json:"fallback_codes"`
	Withdrawn     []string                  `json:"withdrawn"`
}

// AFU-V1-040: the corpus report gives, per basis, the unsafe-narrowing rate (omitted tests the
// injected fault breaks) and the reduction ratio (omitted tests over all tests considered), and the
// cases per fallback code. A basis with any unsafe omission is withdrawn; none may be.
func TestAFUV1040SelectionCorpusReport(t *testing.T) {
	t.Parallel()
	corpus := loadE2ECorpus(t)
	inventory := 5
	report := e2eCorpusReport{Schema: "e2e-safe-selection-corpus-report/0", Cases: len(corpus.Cases), TestsPerCase: inventory,
		Bases: map[string]e2eBasisReport{}, FallbackCodes: map[string]int{}, Withdrawn: []string{}}
	for _, code := range []string{appflows.CodeUnmappedChange, appflows.CodeInventoryIncomplete, appflows.CodeExclusionUnproven, appflows.CodeGlobalPathChanged,
		appflows.CodeMapStale, appflows.CodeInferredLinkOnly, appflows.CodeBoundExceeded} {
		report.FallbackCodes[code] = 0
	}
	omitted, unsafe := map[string]int{appflows.BasisCoverage: 0, appflows.BasisReviewedLinks: 0}, map[string]int{appflows.BasisCoverage: 0, appflows.BasisReviewedLinks: 0}
	for _, c := range corpus.Cases {
		selection, _, _ := runE2ECase(t, corpus, c)
		if selection.State != c.Expect.State || !slices.Equal(e2eCodes(selection), c.Expect.Codes) {
			t.Errorf("%s: state %s codes %v, want %s %v (fallback %v)", c.ID, selection.State, e2eCodes(selection), c.Expect.State, c.Expect.Codes, selection.Fallback)
		}
		if selection.State == "narrow-selection-allowed" {
			report.NarrowedCases++
		}
		if len(selection.Selected)+len(selection.OmittedTests) < inventory {
			t.Errorf("%s: %d selected and %d omitted do not cover the inventory", c.ID, len(selection.Selected), len(selection.OmittedTests))
		}
		for _, code := range e2eCodes(selection) {
			report.FallbackCodes[code]++
		}
		for _, o := range selection.OmittedTests {
			omitted[o.Basis]++
			if slices.Contains(c.Needed, o.TestKey) {
				unsafe[o.Basis]++
				t.Errorf("%s: unsafe omission of %s on %s", c.ID, o.TestKey, o.Basis)
			}
		}
	}
	considered := float64(len(corpus.Cases) * inventory)
	for basis, count := range omitted {
		row := e2eBasisReport{Omitted: count, Unsafe: unsafe[basis], ReductionRatio: float64(count) / considered}
		if count != 0 {
			row.UnsafeRate = float64(unsafe[basis]) / float64(count)
		}
		if row.Unsafe != 0 {
			report.Withdrawn = append(report.Withdrawn, basis)
		}
		report.Bases[basis] = row
	}
	slices.Sort(report.Withdrawn)
	got, err := json.MarshalIndent(report, "", " ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	golden := filepath.Join("testdata", "e2e-safe-corpus.report.json")
	if os.Getenv("CORVINT_UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("corpus report changed:\n%s\nwant:\n%s", got, want)
	}
	if len(report.Withdrawn) != 0 {
		t.Fatalf("bases withdrawn by unsafe narrowing: %v", report.Withdrawn)
	}
}

func e2eFullSuite(t *testing.T, id string, selection appflows.E2ESelection) {
	t.Helper()
	if selection.State != "full-relevant-suite-required" || selection.StateReason != "e2e-fallback" || len(selection.OmittedTests) != 0 || len(selection.Selected) < 5 {
		t.Fatalf("%s: a fallback code must yield the full relevant suite: %+v", id, selection)
	}
}

// AFU-V1-022: each closed fallback code yields the full relevant suite and never narrow-selection-allowed.
func TestAFUV1022EveryFallbackCodeYieldsFullSuite(t *testing.T) {
	t.Parallel()
	corpus := loadE2ECorpus(t)
	bound := corpus.find(t, "search-source")
	bound.extraCoveragePaths = 4097
	cases := map[string]e2eCase{
		appflows.CodeUnmappedChange:      corpus.find(t, "format-helper"),
		appflows.CodeInventoryIncomplete: corpus.find(t, "missing-discovery"),
		appflows.CodeExclusionUnproven:   corpus.find(t, "declared-without-anchor"),
		appflows.CodeGlobalPathChanged:   corpus.find(t, "lockfile"),
		appflows.CodeMapStale:            corpus.find(t, "stale-review"),
		appflows.CodeInferredLinkOnly:    corpus.find(t, "inferred-only"),
		appflows.CodeBoundExceeded:       bound,
	}
	for code, c := range cases {
		t.Run(code, func(t *testing.T) {
			t.Parallel()
			selection, _, _ := runE2ECase(t, corpus, c)
			e2eFullSuite(t, c.ID, selection)
			if !slices.Contains(e2eCodes(selection), code) {
				t.Fatalf("%s: codes %v lack %s", c.ID, e2eCodes(selection), code)
			}
		})
	}
}

// AFU-V1-019: a discovered test missing from the inventory forbids narrowing and is itself selected.
func TestAFUV1019UndiscoveredTestForbidsNarrowing(t *testing.T) {
	t.Parallel()
	corpus := loadE2ECorpus(t)
	selection, _, _ := runE2ECase(t, corpus, corpus.find(t, "undiscovered-test"))
	e2eFullSuite(t, "undiscovered-test", selection)
	want := appflows.E2EFallback{Code: appflows.CodeInventoryIncomplete, Subject: "not-inventoried:chromium:e2e/wishlist.spec.ts"}
	if !slices.Contains(selection.Fallback, want) || selection.Discovery.State != "MATCHED" {
		t.Fatalf("undiscovered test not reported: %+v %+v", selection.Fallback, selection.Discovery)
	}
	if !slices.ContainsFunc(selection.Selected, func(s appflows.E2ETest) bool { return s.Path == "e2e/wishlist.spec.ts" && s.TestKey == "" }) {
		t.Fatalf("uninventoried discovered test is not in the full suite: %+v", selection.Selected)
	}
}

// AFU-V1-020, AFU-V1-021, AFU-V1-023: every omission names its basis and proof, the result counts
// omissions per basis and keeps the ETS note. A declared link without a review anchor and an
// inferred link cannot support reviewed-links, and coverage from before a covered change is stale.
func TestAFUV1020ExclusionProofsPerBasis(t *testing.T) {
	t.Parallel()
	corpus := loadE2ECorpus(t)
	selection, _, evidence := runE2ECase(t, corpus, corpus.find(t, "search-source"))
	if selection.State != "narrow-selection-allowed" || selection.OmissionsPerBasis["coverage"] != 3 || selection.OmissionsPerBasis["reviewed-links"] != 1 {
		t.Fatalf("search fault did not narrow on both bases: %+v", selection)
	}
	if !strings.HasPrefix(selection.Note, extevidence.SelectionNote+"; ") || !strings.Contains(selection.Note, "named basis") {
		t.Fatalf("note does not keep the ETS note plus the e2e-safe note: %q", selection.Note)
	}
	for _, o := range selection.OmittedTests {
		if !slices.Equal(o.Proof.DisjointFrom, []string{"src/search.ts"}) {
			t.Fatalf("%s proof does not name the changed paths: %+v", o.TestKey, o.Proof)
		}
		switch o.Basis {
		case "reviewed-links":
			if o.TestKey != "checkout.spec.ts > pays" || len(o.Proof.Links) != 2 || o.Proof.Attestation != "link completeness is an author attestation" || o.Proof.Coverage != nil {
				t.Fatalf("reviewed-links proof: %+v", o)
			}
		case "coverage":
			if o.Proof.Coverage == nil || o.Proof.Coverage.Commit != evidence || o.Proof.Links != nil {
				t.Fatalf("coverage proof: %+v", o)
			}
		default:
			t.Fatalf("unknown basis %q", o.Basis)
		}
	}
	for id, want := range map[string]appflows.E2EFallback{
		"declared-without-anchor": {Code: appflows.CodeExclusionUnproven, Subject: "profile.spec.ts > views"},
		"inferred-only":           {Code: appflows.CodeInferredLinkOnly, Subject: "cart.spec.ts > adds"},
		"stale-coverage":          {Code: appflows.CodeMapStale, Subject: "search.spec.ts > finds"},
	} {
		got, _, _ := runE2ECase(t, corpus, corpus.find(t, id))
		e2eFullSuite(t, id, got)
		if !slices.Contains(got.Fallback, want) {
			t.Fatalf("%s: fallback %+v lacks %+v", id, got.Fallback, want)
		}
	}
}

// AFU-V1-021: coverage evidence holds at base only under the Verified carry-forward rule. The search
// spec changed after its coverage run to visit the profile page, and its client coverage lists no
// test file, so a profile fault must not omit it on the coverage basis.
func TestAFUV1021CoverageStaleWhenStaticReachChanged(t *testing.T) {
	t.Parallel()
	corpus := loadE2ECorpus(t)
	selection, _, _ := runE2ECase(t, corpus, corpus.find(t, "spec-after-coverage"))
	e2eFullSuite(t, "spec-after-coverage", selection)
	if want := (appflows.E2EFallback{Code: appflows.CodeMapStale, Subject: "search.spec.ts > finds"}); !slices.Contains(selection.Fallback, want) {
		t.Fatalf("fallback %+v lacks %+v", selection.Fallback, want)
	}
}

// AFU-V1-019 AFU-V1-024: malformed selection input fails closed, and the profile takes exactly one
// repository-relative provider file.
func TestAFUV1024E2ESafeRefusesMalformedInput(t *testing.T) {
	t.Parallel()
	corpus := loadE2ECorpus(t)
	root, base, _ := buildE2ECase(t, corpus, corpus.find(t, "search-source"))
	shopWrite(t, root, map[string]string{"coverage.json": `{"schema":"application-flow-coverage/0","records":[],"extra":1}`})
	selection := runE2ESelection(t, root, base)
	if selection.State != "blocked" || selection.StateReason != "provider-unavailable" || len(selection.OmittedTests) != 0 {
		t.Fatalf("malformed coverage did not block: %+v", selection)
	}
	for _, arguments := range [][]string{
		{"--provider", "e2e-provider.json", "--provider", "e2e-provider.json", "--selection-profile", "e2e-safe"},
		{"--provider", "/abs/e2e-provider.json", "--selection-profile", "e2e-safe"},
		{"--provider", "e2e-provider.json", "--repository", "e2e=../e2e", "--selection-profile", "e2e-safe"},
	} {
		if _, _, stderr, code := runAffectedArguments(t, root, arguments...); code != 2 || !strings.Contains(stderr, "e2e-safe requires exactly one repository-relative --provider") {
			t.Fatalf("%v: exit %d %s", arguments, code, stderr)
		}
	}
}
