package doccorpus

import (
	json "encoding/json/v2"
	"path"
	"slices"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

// PlaywrightWitnessImportSchema names the importer's closed output bundle.
const PlaywrightWitnessImportSchema = "corvint-behavior-witness-import/1"

// Playwright annotation types a test publishes for the importer. Each value is the annotation's
// description; an event description is one closed corvint BehaviorEvent JSON object.
const (
	PlaywrightEventAnnotation   = "corvint-behavior-event"
	PlaywrightCleanupAnnotation = "corvint-behavior-cleanup"
	PlaywrightFixtureAnnotation = "corvint-behavior-fixture"
	PlaywrightRoleAnnotation    = "corvint-behavior-role"
)

// PlaywrightWitnessImport binds one Playwright JSON report and one retained qualified receipt to
// the behavior-adapter request's registered tests (DCP-V1-049..050). Witnesses carry canonical
// corvint-behavior-run/1 bytes and the observation link a caller retains and wires; unwitnessed
// tests carry a named reason and are never credited.
type PlaywrightWitnessImport struct {
	Schema         string                      `json:"schema"`
	ContractID     string                      `json:"contract_id"`
	ContractSHA256 string                      `json:"contract_sha256"`
	SourceRevision string                      `json:"source_revision"`
	Receipt        PlaywrightWitnessReceipt    `json:"receipt"`
	ReportSHA256   string                      `json:"report_sha256"`
	Witnesses      []PlaywrightWitness         `json:"witnesses"`
	Unwitnessed    []PlaywrightUnwitnessedTest `json:"unwitnessed"`
	Fallback       string                      `json:"fallback"`
	Limitations    []string                    `json:"limitations"`
}

type PlaywrightWitnessReceipt struct {
	Input    string `json:"input"`
	Revision string `json:"revision"`
	Path     string `json:"path"`
	SHA256   string `json:"sha256"`
}

type PlaywrightWitness struct {
	TestID       string          `json:"test_id"`
	Project      string          `json:"project"`
	Title        string          `json:"title"`
	Retry        int             `json:"retry"`
	ReportTestID string          `json:"report_test_id"`
	ReportSHA256 string          `json:"report_sha256"`
	RunSHA256    string          `json:"run_sha256"`
	Document     string          `json:"document"`
	SHA256       string          `json:"sha256"`
	Observation  ObservationLink `json:"observation"`
}

type PlaywrightUnwitnessedTest struct {
	TestID  string `json:"test_id"`
	Project string `json:"project"`
	Reason  string `json:"reason"`
	Detail  string `json:"detail"`
}

type playwrightReport struct {
	Config struct {
		ConfigFile string `json:"configFile"`
		RootDir    string `json:"rootDir"`
		Version    string `json:"version"`
	} `json:"config"`
	Suites []playwrightReportSuite `json:"suites"`
}

type playwrightReportSuite struct {
	Title  string                  `json:"title"`
	File   string                  `json:"file"`
	Specs  []playwrightReportSpec  `json:"specs"`
	Suites []playwrightReportSuite `json:"suites"`
}

type playwrightReportSpec struct {
	ID    string                 `json:"id"`
	Title string                 `json:"title"`
	File  string                 `json:"file"`
	Line  int                    `json:"line"`
	Tests []playwrightReportTest `json:"tests"`
}

type playwrightReportTest struct {
	ProjectName string                   `json:"projectName"`
	Status      string                   `json:"status"`
	Results     []playwrightReportResult `json:"results"`
}

type playwrightReportResult struct {
	// Retry is a pointer so an absent or null retry is not read as a first attempt.
	Retry       *int                         `json:"retry"`
	Status      string                       `json:"status"`
	Annotations []playwrightReportAnnotation `json:"annotations"`
}

type playwrightReportAnnotation struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type playwrightReportEntry struct {
	id, path, title, project, status string
	// titlePath joins the file and describe titles and the test title with " > ", as the qualified
	// receipt's fullName does after its root and project segments.
	titlePath string
	line      int
	results   []playwrightReportResult
}

type playwrightWitnessContext struct {
	request  BehaviorAdapterRequest
	registry BehaviorRegistry
	receipt  testvaliditydoc.Document
	prefix   string
	digest   string
	input    BehaviorAdapterInput
	report   []playwrightReportEntry
	reportID string
	config   string
}

// ImportPlaywrightWitnesses binds a caller-run Playwright JSON report and the request's retained
// qualified receipt input into per-test behavior runs. Only a test whose receipt and report both
// show a clean first-attempt pass, with observed ordered events, one passed cleanup and the
// declared fixtures and roles, yields a witness. It never runs Playwright, never copies events
// from the declared flow, and never credits a failed, skipped, flaky, missing or foreign test.
func ImportPlaywrightWitnesses(requestRaw []byte, receiptInputID string, reportRaw []byte) ([]byte, error) {
	result, err := BuildBehaviorAdapter(requestRaw, nil)
	if err != nil {
		return nil, err
	}
	var request BehaviorAdapterRequest
	if err := decode(requestRaw, &request); err != nil {
		return nil, err
	}
	registry := *result.Provider.BehaviorContracts
	input, ok := behaviorAdapterInputByID(request, receiptInputID)
	if !ok || receiptInputID == request.MigrationInput || receiptInputID == request.DiscoveryInput {
		return nil, fail("receipt input is not a retained request input other than the migration or discovery")
	}
	var discovery BehaviorDiscovery
	if discoveryInput, _ := behaviorAdapterInputByID(request, request.DiscoveryInput); decode([]byte(discoveryInput.Document), &discovery) != nil {
		return nil, fail("discovery input is not a closed discovery record")
	}
	if len(reportRaw) > PlaywrightFileMaxBytes {
		return nil, fail("playwright report exceeds the input bound")
	}
	var report playwrightReport
	if err := json.Unmarshal(reportRaw, &report); err != nil || report.Config.ConfigFile == "" || report.Config.RootDir == "" {
		return nil, fail("playwright report is not a JSON reporter document with its config identity")
	}
	out := PlaywrightWitnessImport{
		Schema: PlaywrightWitnessImportSchema, ContractID: registry.ContractID, ContractSHA256: registry.ContractSHA256, SourceRevision: registry.SourceRevision,
		Receipt:      PlaywrightWitnessReceipt{Input: input.ID, Revision: input.Anchor.Revision, Path: input.Anchor.Path, SHA256: Digest([]byte(input.Document))},
		ReportSHA256: Digest(reportRaw), Witnesses: []PlaywrightWitness{}, Unwitnessed: []PlaywrightUnwitnessedTest{},
		Fallback: "full-relevant-suite", Limitations: playwrightWitnessLimitations(),
	}
	c := playwrightWitnessContext{request: request, registry: registry, input: input, digest: out.Receipt.SHA256, reportID: out.ReportSHA256, config: discovery.Config.Path}
	shared := ""
	receipt, prefix, err := qualifiedPlaywrightReceipt([]byte(input.Document), discovery.Config.Path)
	switch {
	case err != nil:
		shared = "receipt-unqualified"
	case receipt.TestsOmitted != 0 || receipt.Run.Execution.State == "INCOMPLETE" || receipt.Run.Freshness.State == "STALE":
		shared = "receipt-incomplete"
	case receipt.Playwright.Identity.ConfigDigest != discovery.Config.SHA256:
		shared = "config-mismatch"
	case report.Config.ConfigFile != receipt.Playwright.Identity.ConfigFile || report.Config.Version != receipt.Playwright.Identity.RunnerVersion:
		shared = "report-foreign-config"
	}
	if shared == "" {
		c.receipt, c.prefix = receipt, prefix
		if err := collectPlaywrightReport(report.Config.RootDir, "", nil, report.Suites, 0, &c.report); err != nil {
			return nil, err
		}
	}
	tests := slices.Clone(registry.Tests)
	sort.Slice(tests, func(i, j int) bool { return tests[i].ID < tests[j].ID })
	for _, test := range tests {
		if shared != "" {
			out.Unwitnessed = append(out.Unwitnessed, PlaywrightUnwitnessedTest{TestID: test.ID, Project: test.Project, Reason: shared, Detail: playwrightSharedDetail(shared)})
			continue
		}
		witness, reason, detail := c.witness(test)
		if reason != "" {
			out.Unwitnessed = append(out.Unwitnessed, PlaywrightUnwitnessedTest{TestID: test.ID, Project: test.Project, Reason: reason, Detail: detail})
			continue
		}
		out.Witnesses = append(out.Witnesses, witness)
	}
	return Encode(out)
}

func playwrightSharedDetail(reason string) string {
	return map[string]string{
		"receipt-unqualified":   "receipt input is not a canonical qualified Playwright receipt for the discovery config",
		"receipt-incomplete":    "qualified receipt omitted tests, is incomplete or is stale",
		"config-mismatch":       "qualified receipt ran a config other than the discovery config bytes",
		"report-foreign-config": "playwright report config file or runner version differs from the qualified receipt",
	}[reason]
}

func playwrightWitnessLimitations() []string {
	return []string{
		"the report and qualified receipt are caller-run; Corvint never runs Playwright",
		"the report digest is retained in this bundle, not in the closed corvint-behavior-run/1 record, so the corpus compiler does not re-verify the report",
		"only a first-attempt pass in both receipt and report is credited; retried passes stay unwitnessed",
	}
}

func behaviorAdapterInputByID(request BehaviorAdapterRequest, id string) (BehaviorAdapterInput, bool) {
	for _, input := range request.Inputs {
		if input.ID == id {
			return input, true
		}
	}
	return BehaviorAdapterInput{}, false
}

func collectPlaywrightReport(rootDir, file string, titles []string, suites []playwrightReportSuite, depth int, entries *[]playwrightReportEntry) error {
	if depth > 64 {
		return fail("playwright report suites nest too deeply")
	}
	for _, suite := range suites {
		suiteFile, suiteTitles := file, titles
		if suite.File != "" && depth == 0 {
			suiteFile = suite.File
		}
		if suite.Title != "" {
			suiteTitles = append(slices.Clone(titles), suite.Title)
		}
		for _, spec := range suite.Specs {
			specFile := suiteFile
			if spec.File != "" {
				specFile = spec.File
			}
			absolute := path.Join(rootDir, specFile)
			for _, test := range spec.Tests {
				if len(*entries) >= MaxRecords {
					return fail("playwright report exceeds the execution bound")
				}
				titlePath := strings.Join(append(slices.Clone(suiteTitles), spec.Title), " > ")
				*entries = append(*entries, playwrightReportEntry{id: spec.ID, path: absolute, title: spec.Title, project: test.ProjectName, status: test.Status, titlePath: titlePath, line: spec.Line, results: test.Results})
			}
		}
		if err := collectPlaywrightReport(rootDir, suiteFile, suiteTitles, suite.Suites, depth+1, entries); err != nil {
			return err
		}
	}
	return nil
}

func (c playwrightWitnessContext) repositoryPath(absolute string) (string, bool) {
	relative, ok := strings.CutPrefix(absolute, c.prefix)
	return relative, ok && validPath(relative)
}

// witness returns the test's witness or the named reason it stays unwitnessed.
func (c playwrightWitnessContext) witness(test BehaviorTest) (PlaywrightWitness, string, string) {
	identity := c.receipt.Playwright.Identity
	var native *jstestprovider.Anchor
	fullName := ""
	for _, outcome := range c.receipt.Playwright.Tests {
		if outcome.ID != test.ID || outcome.Anchor == nil {
			continue
		}
		if outcome.Project == nil || outcome.Project.Name != test.Project {
			return PlaywrightWitness{}, "foreign-project", "qualified receipt ran this test under another project"
		}
		mapped, ok := c.repositoryPath(outcome.Anchor.File)
		if !ok || mapped != test.Evidence.Path || identity.TestFileDigests[outcome.Anchor.File] != test.Evidence.SHA256 || outcome.Anchor.Line < test.Evidence.Start || outcome.Anchor.Line > test.Evidence.End || outcome.Name != test.Title {
			return PlaywrightWitness{}, "source-mismatch", "qualified receipt test file, bytes, line or title differ from the registered test"
		}
		if native != nil {
			return PlaywrightWitness{}, "report-ambiguous", "qualified receipt has more than one outcome with this test identity"
		}
		native, fullName = outcome.Anchor, outcome.FullName
	}
	if native == nil {
		return PlaywrightWitness{}, "receipt-test-missing", "qualified receipt has no outcome with this test identity"
	}
	if c.sharedReceiptTuple(native, test.Project, fullName) {
		return PlaywrightWitness{}, "report-ambiguous", "another qualified receipt outcome has this test's file, line, project and full title"
	}
	observed, ok := c.observedTest(test)
	if !ok {
		return PlaywrightWitness{}, "receipt-test-missing", "qualified receipt has no projected result for this test and project"
	}
	if reason, detail := playwrightReceiptState(observed); reason != "" {
		return PlaywrightWitness{}, reason, detail
	}
	entry, reason, detail := c.reportEntry(test, native.File, native.Line, fullName)
	if reason != "" {
		return PlaywrightWitness{}, reason, detail
	}
	if reason, detail := playwrightReportState(entry); reason != "" {
		return PlaywrightWitness{}, reason, detail
	}
	result := entry.results[0]
	events, fixtures, roles, reason, detail := playwrightAnnotations(result.Annotations)
	if reason != "" {
		return PlaywrightWitness{}, reason, detail
	}
	if !slices.Equal(fixtures, test.Fixtures) || !slices.Equal(roles, test.Roles) {
		return PlaywrightWitness{}, "fixture-role-mismatch", "observed fixture or role annotations differ from the registered test"
	}
	run := BehaviorRun{Fixtures: fixtures, Roles: roles, Revisions: c.registry.Revisions, Schema: "corvint-behavior-run/1", ContractID: c.registry.ContractID, ContractSHA256: c.registry.ContractSHA256, SourceRevision: c.registry.SourceRevision, DocumentationRevision: c.registry.DocumentationRevision, RunSHA256: c.digest, TestID: test.ID, Project: test.Project, Retry: *result.Retry, Cleanup: "passed", Events: events}
	document, err := Encode(run)
	if err != nil {
		return PlaywrightWitness{}, "event-malformed", "behavior run exceeds its bound"
	}
	link := ObservationLink{
		ID: c.request.ProviderID + ":run:" + test.ID, Subject: c.request.ProviderID + ":test:" + test.ID,
		TestID: test.ID, Project: test.Project, Test: test.Title, Input: c.input.Anchor.Path, InputRevision: c.input.Anchor.Revision,
		SourceRevision: c.request.SourceRevision, RunID: c.digest,
		SourcePaths: map[string]string{identity.ConfigFile: c.config, native.File: test.Evidence.Path},
	}
	return PlaywrightWitness{TestID: test.ID, Project: test.Project, Title: test.Title, Retry: *result.Retry, ReportTestID: entry.id, ReportSHA256: c.reportID, RunSHA256: c.digest, Document: string(document), SHA256: Digest(document), Observation: link}, "", ""
}

func (c playwrightWitnessContext) observedTest(test BehaviorTest) (testvaliditydoc.Test, bool) {
	for _, observed := range c.receipt.Tests {
		if observed.ID == test.ID && observed.Project != nil && observed.Project.Name == test.Project {
			return observed, true
		}
	}
	return testvaliditydoc.Test{}, false
}

func playwrightReceiptState(observed testvaliditydoc.Test) (string, string) {
	switch observed.State {
	case "passed":
	case "skipped":
		return "skipped", "qualified receipt skipped this test"
	case "flaky":
		return "flaky-after-retry", "qualified receipt passed this test only after a retry"
	default:
		return "failed", "qualified receipt state is " + observed.State
	}
	if len(observed.Attempts) != 1 || observed.Attempts[0].Retry != 0 || observed.Attempts[0].State != "passed" {
		return "flaky-after-retry", "qualified receipt records more than one attempt or a non-first passing attempt"
	}
	if observed.Projection.Execution.State != "PASSED" {
		return "failed", "qualified receipt projection is not PASSED"
	}
	return "", ""
}

// sharedReceiptTuple reports whether more than one qualified receipt outcome has this file, line,
// project and full title, so no report result could be attributed to exactly one of them.
func (c playwrightWitnessContext) sharedReceiptTuple(anchor *jstestprovider.Anchor, project, fullName string) bool {
	count := 0
	for _, outcome := range c.receipt.Playwright.Tests {
		if outcome.Anchor != nil && outcome.Project != nil && outcome.Anchor.File == anchor.File && outcome.Anchor.Line == anchor.Line && outcome.Project.Name == project && outcome.FullName == fullName {
			count++
		}
	}
	return count > 1
}

// reportEntry finds the one report result with the receipt test's file, line, project and full
// title path; a leaf-title match alone never attributes a result.
func (c playwrightWitnessContext) reportEntry(test BehaviorTest, file string, line int, fullName string) (playwrightReportEntry, string, string) {
	matches := []playwrightReportEntry{}
	foreign := false
	titlePath, ok := strings.CutPrefix(fullName, " > "+test.Project+" > ")
	for _, entry := range c.report {
		if !ok || entry.path != file || entry.line != line || entry.title != test.Title || entry.titlePath != titlePath {
			continue
		}
		if entry.project != test.Project {
			foreign = true
			continue
		}
		matches = append(matches, entry)
	}
	switch {
	case len(matches) == 1:
		return matches[0], "", ""
	case len(matches) > 1:
		return playwrightReportEntry{}, "report-ambiguous", "playwright report has more than one result for this test's full title and project"
	case foreign:
		return playwrightReportEntry{}, "foreign-project", "playwright report ran this test only under another project"
	}
	return playwrightReportEntry{}, "report-test-missing", "playwright report has no result for this test file, line, full title and project"
}

func playwrightReportState(entry playwrightReportEntry) (string, string) {
	switch entry.status {
	case "expected":
	case "skipped":
		return "skipped", "playwright report skipped this test"
	case "flaky":
		return "flaky-after-retry", "playwright report passed this test only after a retry"
	default:
		return "report-receipt-disagree", "playwright report status " + entry.status + " contradicts the receipt pass"
	}
	if len(entry.results) != 1 {
		return "flaky-after-retry", "playwright report records more than one attempt"
	}
	if entry.results[0].Retry == nil {
		return "report-test-missing", "playwright report result has no retry, so it is not evidence of a first attempt"
	}
	if *entry.results[0].Retry != 0 {
		return "flaky-after-retry", "playwright report records a non-first attempt"
	}
	if entry.results[0].Status != "passed" {
		return "report-receipt-disagree", "playwright report attempt status " + entry.results[0].Status + " contradicts the receipt pass"
	}
	return "", ""
}

// playwrightAnnotations reads the observed events, cleanup, fixtures and roles of one attempt in
// the order the test published them.
func playwrightAnnotations(annotations []playwrightReportAnnotation) ([]BehaviorEvent, []string, []string, string, string) {
	events := []BehaviorEvent{}
	seen := map[string]bool{}
	var fixtures, roles []string
	cleanups := []string{}
	for _, annotation := range annotations {
		switch annotation.Type {
		case PlaywrightEventAnnotation:
			if len(events) >= MaxRecords {
				return nil, nil, nil, "event-malformed", "observed events exceed the bound"
			}
			var event BehaviorEvent
			if decode([]byte(annotation.Description), &event) != nil {
				return nil, nil, nil, "event-malformed", "an event annotation is not one closed behavior event"
			}
			if !behaviorEventShapeOK(event, len(events)+1) || seen[event.Kind+":"+event.ID] {
				return nil, nil, nil, "event-malformed", "an event is out of sequence, duplicate, not passing or lacks its kind, identity or browser context"
			}
			seen[event.Kind+":"+event.ID] = true
			events = append(events, event)
		case PlaywrightCleanupAnnotation:
			cleanups = append(cleanups, annotation.Description)
		case PlaywrightFixtureAnnotation:
			fixtures = append(fixtures, annotation.Description)
		case PlaywrightRoleAnnotation:
			roles = append(roles, annotation.Description)
		}
	}
	if len(events) == 0 {
		return nil, nil, nil, "events-unobserved", "the passing attempt published no behavior events"
	}
	if len(cleanups) == 0 {
		return nil, nil, nil, "cleanup-unobserved", "the passing attempt published no cleanup outcome"
	}
	if len(cleanups) != 1 || cleanups[0] != "passed" {
		return nil, nil, nil, "cleanup-failed", "the passing attempt did not publish exactly one passed cleanup"
	}
	return events, fixtures, roles, "", ""
}
