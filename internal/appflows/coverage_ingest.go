package appflows

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/gokernel"
)

// The per-test coverage ingest behind `corvint flows ingest-coverage` (AFU-V1-112..119, GitHub
// #1587). It reads one coverage file (V8 or Istanbul JSON from one Playwright test, Ruby Coverage
// results keyed by RSpec example ID, or a single-test LCOV), pins it and the run evidence by digest,
// binds every covered test to exactly one test-run-evidence/0 record at the named revision, and
// reports `observed` tested_by relations from each executed source file to the test, with the
// executed line spans anchored to the blob at the revision. A declared or supplied link that the
// coverage contradicts is a typed conflict; coverage from another revision or a dirty run is stale
// and is not joined. It reads files and Git objects only and writes nothing.

const (
	CoverageIngestSchema = "corvint-coverage-ingest/1"
	CoverageLinksSchema  = "corvint-coverage-links/1"

	// maxCoverageTests bounds the tests one coverage input may name.
	maxCoverageTests = 2048
	// maxCoverageFiles bounds the (test, file) coverage entries across the input.
	maxCoverageFiles = 4096
	// maxCoverageLines bounds the instrumented line records across the input.
	maxCoverageLines = 1 << 22
	// maxCoverageRanges bounds the V8 block ranges across the input.
	maxCoverageRanges = 1 << 20
	// maxCoverageLinks bounds the declared or supplied links one links file may carry.
	maxCoverageLinks = 4096
	// maxCoverageUnknowns bounds the reported unknowns; the overflow is counted, not listed.
	maxCoverageUnknowns = 4096
	// maxCoverageAnchors is the corpus evidence anchor bound for one relation or conflict.
	maxCoverageAnchors = 64
	// maxCoverageAnchorLines splits a long executed span into anchors of at most this many lines.
	maxCoverageAnchorLines = 256

	coverageReasonStale           = "coverage-stale"
	coverageReasonDirty           = "run-source-dirty"
	coverageReasonNoRun           = "no-run-record"
	coverageReasonAmbiguousRun    = "run-record-ambiguous"
	coverageReasonNotRun          = "test-not-run"
	coverageReasonNotCovered      = "test-not-covered"
	coverageReasonOutside         = "path-outside-repository"
	coverageReasonSourceMap       = "v8-url-needs-source-map"
	coverageReasonSourceMismatch  = "v8-source-mismatch"
	coverageReasonTranspiled      = "v8-transpiled-without-source"
	coverageReasonAbsent          = "source-not-at-revision"
	coverageReasonTooLarge        = "source-too-large"
	coverageReasonOutOfRange      = "coverage-line-out-of-range"
	coverageReasonBlankLine       = "coverage-line-blank"
	coverageReasonPaintBound      = "coverage-bound-exceeded"
	coverageReasonNotInstrumented = "subject-not-instrumented"
	coverageReasonContradiction   = "coverage-contradiction"
	coverageReasonUnbound         = "coverage-not-bound-to-run"
	coverageReasonUnanchorable    = "coverage-unanchorable"
)

// maxCoverageWork bounds the UTF-16 units that V8 painting scans and paints across one ingest:
// each distinct script costs the blob's units plus its painted range widths (AFU-V1-113).
var maxCoverageWork = 1 << 26

var coverageLimitations = []string{
	"coverage is generated, observed evidence: it shows which lines one test executed in one run, not that the test asserts their behavior",
	"line granularity: a line counts as executed when any of its non-blank characters (V8) or any statement starting on it (Istanbul) ran",
	"V8 offsets are applied to the committed blob only when the entry's source equals it or the file is plain JavaScript; source maps are not resolved, so transpiled or bundled scripts are unknown",
	"a test joins only the one run record whose test key equals it exactly; coverage with no such record, a stale or dirty run, or a skipped test is not joined",
}

// CoverageIngestInput names the coverage file, the run evidence and the revision to bind them to.
type CoverageIngestInput struct {
	Root, Format, From string
	Evidence           []string
	Revision, TestKey  string
	Links, StripPrefix string
}

// CoverageReport is the corvint-coverage-ingest/1 document.
type CoverageReport struct {
	Schema      string             `json:"schema"`
	Status      string             `json:"status"`
	Trust       string             `json:"trust"`
	Format      string             `json:"format"`
	Repository  string             `json:"repository"`
	Revision    string             `json:"revision"`
	Tree        string             `json:"tree"`
	Inputs      []CoverageInput    `json:"inputs"`
	Runs        []CoverageRun      `json:"runs"`
	Relations   []CoverageRelation `json:"relations"`
	Conflicts   []CoverageConflict `json:"conflicts"`
	Links       []CoverageLinkPlan `json:"links"`
	Unknowns    []CoverageUnknown  `json:"unknowns"`
	Omitted     int                `json:"unknowns_omitted,omitempty"`
	Limitations []string           `json:"limitations"`
}

// CoverageInput pins one input by role and digest; local file names are never recorded.
type CoverageInput struct {
	Role   string `json:"role"`
	SHA256 string `json:"sha256"`
	Bytes  int    `json:"bytes"`
}

// CoverageRun is one covered test's binding to its run record.
type CoverageRun struct {
	TestKey      string    `json:"test_key"`
	RunID        string    `json:"run_id,omitempty"`
	Project      string    `json:"project,omitempty"`
	Runner       RunRunner `json:"runner"`
	Source       RunSource `json:"source"`
	RecordSHA256 string    `json:"record_sha256,omitempty"`
	State        string    `json:"state"`
	Reason       string    `json:"reason,omitempty"`
}

// CoverageRelation is one observed tested_by relation from an executed source file to the test.
type CoverageRelation struct {
	ID            string             `json:"id"`
	Type          string             `json:"type"`
	From          string             `json:"from"`
	To            string             `json:"to"`
	Path          string             `json:"path"`
	TestKey       string             `json:"test_key"`
	RunID         string             `json:"run_id"`
	Instrumented  int                `json:"instrumented_lines"`
	ExecutedLines int                `json:"executed_lines"`
	ExecutedSpans [][2]int           `json:"executed_spans"`
	Evidence      doccorpus.Evidence `json:"evidence"`
}

// CoverageConflict is a declared or supplied link that the test ran without executing.
type CoverageConflict struct {
	ID           string             `json:"id"`
	Type         string             `json:"type"`
	Subject      string             `json:"subject"`
	TestKey      string             `json:"test_key"`
	LinkSource   string             `json:"link_source"`
	Reference    string             `json:"reference,omitempty"`
	Reason       string             `json:"reason"`
	RunID        string             `json:"run_id"`
	Instrumented int                `json:"instrumented_lines"`
	Evidence     doccorpus.Evidence `json:"evidence"`
}

// CoverageLinkPlan is the outcome of checking one link against the coverage.
type CoverageLinkPlan struct {
	Subject   string `json:"subject"`
	TestKey   string `json:"test_key"`
	Source    string `json:"source"`
	Reference string `json:"reference,omitempty"`
	State     string `json:"state"`
	Reason    string `json:"reason,omitempty"`
}

// CoverageUnknown is coverage that could not be classified or joined, with its reason.
type CoverageUnknown struct {
	TestKey string `json:"test_key,omitempty"`
	Path    string `json:"path,omitempty"`
	Reason  string `json:"reason"`
	Detail  string `json:"detail"`
}

// CoverageLinks is the corvint-coverage-links/1 input of declared or supplied test-to-source links.
type CoverageLinks struct {
	Schema string         `json:"schema"`
	Links  []CoverageLink `json:"links"`
}

type CoverageLink struct {
	Subject   string `json:"subject"`
	Test      string `json:"test"`
	Source    string `json:"source"`
	Reference string `json:"reference,omitempty"`
}

// fileCoverage is one file's coverage in one test: per instrumented line, its largest count, or
// the V8 scripts whose offsets need the committed blob.
type fileCoverage struct {
	lines map[int]int64
	v8    []v8Script
}

// v8Script is one V8 entry for a file. key identifies its source and ranges, so identical
// entries are painted once; sourceKey identifies its source alone.
type v8Script struct {
	source         *string
	ranges         []v8Range
	key, sourceKey string
}

type v8Range struct {
	StartOffset int   `json:"startOffset"`
	EndOffset   int   `json:"endOffset"`
	Count       int64 `json:"count"`
}

// coverageSet is the parsed input: test key ("" for a single-test format) → raw path → coverage.
type coverageSet struct {
	tests            map[string]map[string]*fileCoverage
	files, lines, rs int
}

func coverageFail(format string, args ...any) error {
	return &gokernel.Error{Code: "coverage-refused", Message: fmt.Sprintf(format, args...)}
}

func coverageBound(what string) error {
	return &gokernel.Error{Code: "coverage-bound-exceeded", Message: "coverage input exceeds the " + what + " bound"}
}

func (s *coverageSet) file(test, name string) (*fileCoverage, error) {
	files := s.tests[test]
	if files == nil {
		if len(s.tests) >= maxCoverageTests {
			return nil, coverageBound("test")
		}
		files = map[string]*fileCoverage{}
		s.tests[test] = files
	}
	f := files[name]
	if f == nil {
		if s.files++; s.files > maxCoverageFiles {
			return nil, coverageBound("file")
		}
		f = &fileCoverage{lines: map[int]int64{}}
		files[name] = f
	}
	return f, nil
}

func (s *coverageSet) line(f *fileCoverage, line int, count int64) error {
	if line < 1 || count < 0 {
		return coverageFail("coverage line %d or count %d is invalid", line, count)
	}
	if old, ok := f.lines[line]; ok {
		f.lines[line] = max(old, count)
		return nil
	}
	if s.lines++; s.lines > maxCoverageLines {
		return coverageBound("line")
	}
	f.lines[line] = count
	return nil
}

// decodeLoose parses JSON that wire.Parse first admits (no duplicate keys, bounded depth), keeping
// only the fields the ingest reads.
func decodeLoose(raw []byte, out any) error {
	if _, err := wire.Parse(raw); err != nil {
		return coverageFail("coverage input is not admissible JSON")
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return coverageFail("coverage input does not match the %T shape", out)
	}
	return nil
}

func parseCoverage(format string, raw []byte) (*coverageSet, error) {
	s := &coverageSet{tests: map[string]map[string]*fileCoverage{}}
	var err error
	switch format {
	case "v8":
		err = s.parseV8(raw)
	case "istanbul":
		err = s.parseIstanbul(raw)
	case "lcov":
		err = s.parseLCOV(raw)
	case "rspec":
		err = s.parseRSpec(raw)
	default:
		err = coverageFail("coverage format must be v8, istanbul, rspec or lcov")
	}
	return s, err
}

// parseV8 reads a Playwright stopJSCoverage array or a NODE_V8_COVERAGE {"result": [...]} file.
func (s *coverageSet) parseV8(raw []byte) error {
	type script struct {
		URL       *string `json:"url"`
		Source    *string `json:"source"`
		Functions []struct {
			Ranges []v8Range `json:"ranges"`
		} `json:"functions"`
	}
	var entries []script
	if trimmed := bytes.TrimLeft(raw, " \t\r\n"); len(trimmed) > 0 && trimmed[0] == '{' {
		var wrapped struct {
			Result *[]script `json:"result"`
		}
		if err := decodeLoose(raw, &wrapped); err != nil {
			return err
		}
		if wrapped.Result == nil {
			return coverageFail("V8 coverage object has no result array")
		}
		entries = *wrapped.Result
	} else if err := decodeLoose(raw, &entries); err != nil {
		return err
	}
	type v8Seen struct {
		f   *fileCoverage
		key string
	}
	seen := map[v8Seen]bool{}
	for _, e := range entries {
		if e.URL == nil {
			return coverageFail("V8 coverage entry has no url")
		}
		f, err := s.file("", *e.URL)
		if err != nil {
			return err
		}
		sc := v8Script{source: e.Source, sourceKey: "none"}
		if e.Source != nil {
			sc.sourceKey = Digest([]byte(*e.Source))
		}
		for _, fn := range e.Functions {
			if s.rs += len(fn.Ranges); s.rs > maxCoverageRanges {
				return coverageBound("V8 range")
			}
			sc.ranges = append(sc.ranges, fn.Ranges...)
		}
		h := sha256.New()
		h.Write([]byte(sc.sourceKey))
		for _, r := range sc.ranges {
			fmt.Fprintf(h, ";%d,%d,%d", r.StartOffset, r.EndOffset, r.Count)
		}
		sc.key = hex.EncodeToString(h.Sum(nil))
		if !seen[v8Seen{f, sc.key}] {
			seen[v8Seen{f, sc.key}] = true
			f.v8 = append(f.v8, sc)
		}
	}
	return nil
}

// parseIstanbul reads an Istanbul coverage map (window.__coverage__ or coverage-final.json).
func (s *coverageSet) parseIstanbul(raw []byte) error {
	type position struct {
		Line int `json:"line"`
	}
	var files map[string]struct {
		Path         string `json:"path"`
		StatementMap map[string]struct {
			Start position `json:"start"`
		} `json:"statementMap"`
		S map[string]int64 `json:"s"`
	}
	if err := decodeLoose(raw, &files); err != nil {
		return err
	}
	for _, key := range sortedKeys(files) {
		fc := files[key]
		name := fc.Path
		if name == "" {
			name = key
		}
		if fc.StatementMap == nil || fc.S == nil {
			return coverageFail("Istanbul entry %q lacks statementMap or s", key)
		}
		f, err := s.file("", name)
		if err != nil {
			return err
		}
		for _, id := range sortedKeys(fc.StatementMap) {
			count, ok := fc.S[id]
			if !ok {
				return coverageFail("Istanbul statement %s of %q has no count", id, key)
			}
			if err := s.line(f, fc.StatementMap[id].Start.Line, count); err != nil {
				return err
			}
		}
	}
	return nil
}

// parseLCOV reads SF/DA records of a single-test LCOV trace; other record types are ignored. A
// trace whose TN records name more than one test is refused, because its per-test lines cannot be
// told apart once bound to one test key.
func (s *coverageSet) parseLCOV(raw []byte) error {
	var f *fileCoverage
	testName, named := "", false
	for n, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimRight(line, "\r")
		switch {
		case strings.HasPrefix(line, "TN:"):
			name := strings.TrimSpace(strings.TrimPrefix(line, "TN:"))
			if named && name != testName {
				return coverageFail("LCOV line %d starts a second test (TN) in a single-test trace", n+1)
			}
			testName, named = name, true
		case strings.HasPrefix(line, "SF:"):
			var err error
			if f, err = s.file("", strings.TrimPrefix(line, "SF:")); err != nil {
				return err
			}
		case strings.HasPrefix(line, "DA:"):
			fields := strings.Split(strings.TrimPrefix(line, "DA:"), ",")
			if f == nil || len(fields) < 2 {
				return coverageFail("LCOV line %d is a DA record outside an SF record", n+1)
			}
			at, err1 := strconv.Atoi(fields[0])
			count, err2 := strconv.ParseInt(fields[1], 10, 64)
			if err1 != nil || err2 != nil {
				return coverageFail("LCOV line %d is a malformed DA record", n+1)
			}
			if err := s.line(f, at, count); err != nil {
				return err
			}
		case line == "end_of_record":
			f = nil
		}
	}
	if len(s.tests) == 0 {
		return coverageFail("LCOV input holds no SF record")
	}
	return nil
}

// parseRSpec reads Ruby Coverage line results keyed by RSpec example ID: each example maps a file
// to its lines array (index 0 is line 1, null is not relevant) or to {"lines": [...]}.
func (s *coverageSet) parseRSpec(raw []byte) error {
	var examples map[string]map[string]json.RawMessage
	if err := decodeLoose(raw, &examples); err != nil {
		return err
	}
	for _, id := range sortedKeys(examples) {
		test := strings.TrimPrefix(id, "./")
		if test == "" {
			return coverageFail("RSpec coverage has an empty example ID")
		}
		for _, name := range sortedKeys(examples[id]) {
			var lines []*int64
			value := examples[id][name]
			if err := json.Unmarshal(value, &lines); err != nil {
				var nested struct {
					Lines []*int64 `json:"lines"`
				}
				if json.Unmarshal(value, &nested) != nil || nested.Lines == nil {
					return coverageFail("RSpec coverage for %q in %q is not a lines array", name, id)
				}
				lines = nested.Lines
			}
			f, err := s.file(test, name)
			if err != nil {
				return err
			}
			for i, count := range lines {
				if count == nil {
					continue
				}
				if err := s.line(f, i+1, *count); err != nil {
					return err
				}
			}
		}
	}
	if len(s.tests) == 0 {
		return coverageFail("RSpec coverage names no example")
	}
	return nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// coverageIngest carries one ingest's resolved state.
type coverageIngest struct {
	ctx         context.Context
	in          CoverageIngestInput
	roots       []string
	report      *CoverageReport
	blobs       map[string]*doccorpus.LineIndex
	blobReasons map[string]CoverageUnknown
	work        int // V8 painting work spent, against maxCoverageWork
}

func (c *coverageIngest) unknown(u CoverageUnknown) {
	if len(c.report.Unknowns) >= maxCoverageUnknowns {
		c.report.Omitted++
		return
	}
	c.report.Unknowns = append(c.report.Unknowns, u)
}

// IngestCoverage builds the encoded corvint-coverage-ingest/1 report (AFU-V1-112).
func IngestCoverage(ctx context.Context, in CoverageIngestInput) ([]byte, error) {
	if !slices.Contains([]string{"v8", "istanbul", "rspec", "lcov"}, in.Format) {
		return nil, coverageFail("coverage format must be v8, istanbul, rspec or lcov")
	}
	if len(in.Evidence) == 0 {
		return nil, coverageFail("coverage ingest requires run evidence")
	}
	if !wire.IsGitOid(in.Revision) {
		return nil, coverageFail("coverage ingest requires a full commit revision")
	}
	commit, err := ResolveRevision(ctx, in.Root, in.Revision)
	if err != nil || commit != in.Revision {
		return nil, &gokernel.Error{Code: "coverage-invalid-revision", Message: "coverage revision is not a commit"}
	}
	treeOut, err := git(ctx, in.Root, "rev-parse", "--verify", "--end-of-options", commit+"^{tree}")
	if err != nil {
		return nil, &gokernel.Error{Code: "coverage-invalid-revision", Message: "coverage revision tree unavailable"}
	}
	repository, err := contextindex.CorpusRepositoryID(ctx, in.Root, commit)
	if err != nil {
		return nil, err
	}
	report := &CoverageReport{Schema: CoverageIngestSchema, Status: "generated", Trust: "generated", Format: in.Format, Repository: repository,
		Revision: commit, Tree: strings.TrimSpace(string(treeOut)), Inputs: []CoverageInput{}, Runs: []CoverageRun{}, Relations: []CoverageRelation{},
		Conflicts: []CoverageConflict{}, Links: []CoverageLinkPlan{}, Unknowns: []CoverageUnknown{}, Limitations: coverageLimitations}
	raw, err := readCoverageInput("coverage input", in.From)
	if err != nil {
		return nil, err
	}
	report.Inputs = append(report.Inputs, CoverageInput{Role: "coverage", SHA256: Digest(raw), Bytes: len(raw)})
	set, err := parseCoverage(in.Format, raw)
	if err != nil {
		return nil, err
	}
	records, err := readCoverageRuns(in.Evidence, report)
	if err != nil {
		return nil, err
	}
	var links []CoverageLink
	if in.Links != "" {
		if links, err = readCoverageLinks(in.Links, report); err != nil {
			return nil, err
		}
	}
	if err = bindSingleTest(set, in, records); err != nil {
		return nil, err
	}
	c := &coverageIngest{ctx: ctx, in: in, roots: coverageRoots(in.Root), report: report, blobs: map[string]*doccorpus.LineIndex{}, blobReasons: map[string]CoverageUnknown{}}
	joined := map[string]map[string]joinedFile{} // test → repository path, for joined tests only
	runs := map[string]CoverageRun{}
	for _, test := range sortedKeys(set.tests) {
		run := c.bind(test, records)
		runs[test] = run
		report.Runs = append(report.Runs, run)
		if run.State != "joined" {
			c.unknown(CoverageUnknown{TestKey: test, Reason: run.Reason, Detail: "the test's coverage is not joined"})
			continue
		}
		joined[test] = map[string]joinedFile{}
		byPath := map[string]*fileCoverage{}
		for _, raw := range sortedKeys(set.tests[test]) {
			name, reason := c.repositoryPath(raw)
			if reason != "" {
				c.unknown(CoverageUnknown{TestKey: test, Path: boundCoveragePath(raw), Reason: reason, Detail: "the covered path does not name a committed repository file"})
				continue
			}
			f := set.tests[test][raw]
			if merged := byPath[name]; merged != nil {
				for line, count := range f.lines {
					merged.lines[line] = max(merged.lines[line], count)
				}
				merged.v8 = append(merged.v8, f.v8...)
				continue
			}
			byPath[name] = &fileCoverage{lines: maps.Clone(f.lines), v8: slices.Clone(f.v8)}
		}
		for _, name := range sortedKeys(byPath) {
			if err := c.joinFile(test, run, name, byPath[name], joined[test]); err != nil {
				return nil, err
			}
		}
	}
	for _, link := range links {
		c.checkLink(link, set, runs, joined)
	}
	slices.SortFunc(report.Relations, func(a, b CoverageRelation) int { return strings.Compare(a.ID, b.ID) })
	slices.SortFunc(report.Conflicts, func(a, b CoverageConflict) int { return strings.Compare(a.ID, b.ID) })
	slices.SortStableFunc(report.Links, func(a, b CoverageLinkPlan) int {
		return strings.Compare(a.Subject+"\x00"+a.TestKey+"\x00"+a.Source, b.Subject+"\x00"+b.TestKey+"\x00"+b.Source)
	})
	slices.SortStableFunc(report.Unknowns, func(a, b CoverageUnknown) int {
		return strings.Compare(a.TestKey+"\x00"+a.Path+"\x00"+a.Reason+"\x00"+a.Detail, b.TestKey+"\x00"+b.Path+"\x00"+b.Reason+"\x00"+b.Detail)
	})
	return doccorpus.Encode(report)
}

// readCoverageInput reads one input; an input over the byte bound is coverage-bound-exceeded.
func readCoverageInput(role, name string) ([]byte, error) {
	raw, err := ReadFile(name)
	if errors.Is(err, errInputBytes) {
		return nil, coverageBound(role + " byte")
	}
	if err != nil {
		return nil, coverageFail("%s: %v", role, err)
	}
	return raw, nil
}

// readCoverageRuns reads every run-evidence file once, pinning each by digest before decoding.
func readCoverageRuns(files []string, report *CoverageReport) ([]TestRunEvidence, error) {
	records := []TestRunEvidence{}
	for _, name := range files {
		raw, err := readCoverageInput("run evidence", name)
		if err != nil {
			return nil, err
		}
		report.Inputs = append(report.Inputs, CoverageInput{Role: "run-evidence", SHA256: Digest(raw), Bytes: len(raw)})
		for line := range bytes.Lines(raw) {
			if len(records) >= maxRunRecords {
				return nil, coverageBound("run record")
			}
			r, err := DecodeRunEvidence(line)
			if err != nil {
				return nil, coverageFail("run evidence: %v", err)
			}
			records = append(records, r)
		}
	}
	return records, nil
}

func readCoverageLinks(name string, report *CoverageReport) ([]CoverageLink, error) {
	raw, err := readCoverageInput("coverage links", name)
	if err != nil {
		return nil, err
	}
	report.Inputs = append(report.Inputs, CoverageInput{Role: "links", SHA256: Digest(raw), Bytes: len(raw)})
	var doc CoverageLinks
	if err := Decode(raw, &doc); err != nil {
		return nil, coverageFail("coverage links: %v", err)
	}
	if doc.Schema != CoverageLinksSchema {
		return nil, coverageFail("coverage links schema must be %s", CoverageLinksSchema)
	}
	if len(doc.Links) > maxCoverageLinks {
		return nil, coverageBound("link")
	}
	for _, l := range doc.Links {
		if !safePath(l.Subject) || !flowText(l.Test) || (l.Source != "declared" && l.Source != "supplied") || (l.Reference != "" && !flowText(l.Reference)) {
			return nil, coverageFail("coverage link needs a repository-relative subject, a test key and source declared or supplied")
		}
	}
	return doc.Links, nil
}

// bindSingleTest names the one test of a single-test format: --test-key, or the only run record.
func bindSingleTest(set *coverageSet, in CoverageIngestInput, records []TestRunEvidence) error {
	if in.Format == "rspec" {
		if in.TestKey != "" {
			return coverageFail("--test-key does not apply to rspec coverage, which is keyed by example ID")
		}
		return nil
	}
	key := in.TestKey
	if key == "" {
		if len(records) != 1 {
			return coverageFail("--test-key is required when the run evidence holds %d records", len(records))
		}
		key = records[0].TestKey
	}
	if !flowText(key) {
		return coverageFail("--test-key is not a valid test key")
	}
	if files, ok := set.tests[""]; ok {
		delete(set.tests, "")
		set.tests[key] = files
	}
	return nil
}

// bind finds the covered test's one run record and decides whether its coverage may join.
func (c *coverageIngest) bind(test string, records []TestRunEvidence) CoverageRun {
	matches := []TestRunEvidence{}
	for _, r := range records {
		if r.TestKey == test {
			matches = append(matches, r)
		}
	}
	run := CoverageRun{TestKey: test, State: "unjoined"}
	switch len(matches) {
	case 0:
		run.Reason = coverageReasonNoRun
		return run
	case 1:
	default:
		run.Reason = coverageReasonAmbiguousRun
		return run
	}
	r := matches[0]
	line, err := EncodeRunEvidence(r)
	if err == nil {
		run.RecordSHA256 = Digest(line)
	}
	run.RunID, run.Project, run.Runner, run.Source = r.RunID, r.Project, r.Runner, r.Source
	switch {
	case r.Source.Commit != c.report.Revision || r.Source.Tree != c.report.Tree:
		run.State, run.Reason = "stale", coverageReasonStale
	case !r.Source.Clean:
		run.State, run.Reason = "stale", coverageReasonDirty
	case !slices.ContainsFunc(r.CoverageArtifacts, func(a RunDigestRef) bool { return a.Digest == c.report.Inputs[0].SHA256 }):
		run.Reason = coverageReasonUnbound
	case !slices.ContainsFunc(r.Attempts, func(a RunAttempt) bool { return a.Outcome != "skipped" }):
		run.Reason = coverageReasonNotRun
	default:
		run.State = "joined"
	}
	return run
}

// coverageRoots are the absolute checkout prefixes an absolute coverage path may carry.
func coverageRoots(root string) []string {
	roots := []string{filepath.ToSlash(filepath.Clean(root))}
	if real, err := filepath.EvalSymlinks(root); err == nil && filepath.ToSlash(real) != roots[0] {
		roots = append(roots, filepath.ToSlash(real))
	}
	return roots
}

// repositoryPath maps a coverage path or URL to a repository-relative path, or names why not.
func (c *coverageIngest) repositoryPath(name string) (string, string) {
	if strings.HasPrefix(name, "http://") || strings.HasPrefix(name, "https://") {
		return "", coverageReasonSourceMap
	}
	if strings.HasPrefix(name, "file://") {
		u, err := url.Parse(name)
		if err != nil {
			return "", coverageReasonOutside
		}
		name = u.Path
	} else if scheme, _, ok := strings.Cut(name, ":"); ok && scheme != "" && !strings.Contains(scheme, "/") {
		return "", coverageReasonOutside // node:internal, webpack://, data: and other non-file scripts
	}
	if c.in.StripPrefix != "" && strings.HasPrefix(name, c.in.StripPrefix) {
		name = strings.TrimLeft(strings.TrimPrefix(name, c.in.StripPrefix), "/")
	} else if strings.HasPrefix(name, "/") {
		trimmed := ""
		for _, root := range c.roots {
			if strings.HasPrefix(name, root+"/") {
				trimmed = strings.TrimPrefix(name, root+"/")
				break
			}
		}
		if trimmed == "" {
			return "", coverageReasonOutside
		}
		name = trimmed
	}
	name = strings.TrimPrefix(name, "./")
	if !safePath(name) || name == "." {
		return "", coverageReasonOutside
	}
	return name, ""
}

// blob reads the committed file at the revision once, or records why it cannot be read.
func (c *coverageIngest) blob(name string) (*doccorpus.LineIndex, *CoverageUnknown) {
	if x, ok := c.blobs[name]; ok {
		return x, nil
	}
	if u, ok := c.blobReasons[name]; ok {
		return nil, &u
	}
	fail := func(reason, detail string) (*doccorpus.LineIndex, *CoverageUnknown) {
		u := CoverageUnknown{Path: name, Reason: reason, Detail: detail}
		c.blobReasons[name] = u
		return nil, &u
	}
	out, err := git(c.ctx, c.in.Root, "--literal-pathspecs", "ls-tree", "-z", "-l", "--full-tree", c.report.Revision, "--", name)
	if err != nil {
		return fail(coverageReasonAbsent, "the path is unreadable at the revision")
	}
	meta, got, _ := strings.Cut(string(bytes.TrimSuffix(out, []byte{0})), "\t")
	fields := strings.Fields(meta)
	if got != name || len(fields) != 4 {
		return fail(coverageReasonAbsent, "the path is not a committed file at the revision")
	}
	e := treeEntry{name: name, mode: fields[0], kind: fields[1], oid: fields[2], size: fields[3]}
	if size, err := strconv.Atoi(e.size); err == nil && size > MaxBytes {
		return fail(coverageReasonTooLarge, fmt.Sprintf("the blob exceeds %d bytes", MaxBytes))
	}
	data, err := committedBlob(c.ctx, c.in.Root, e)
	if err != nil {
		return fail(coverageReasonAbsent, "the committed blob is not a readable regular file")
	}
	x := doccorpus.NewLineIndex(c.report.Repository, c.report.Revision, name, e.oid, data)
	c.blobs[name] = x
	return x, nil
}

// joinedFile is one repository file's line classification in one joined test.
type joinedFile struct {
	x            *doccorpus.LineIndex
	instrumented []int
	executed     int
	unanchored   bool // executed lines exist but none could be anchored
}

// joinFile turns one covered file of a joined test into a tested_by relation or an unknown.
func (c *coverageIngest) joinFile(test string, run CoverageRun, name string, f *fileCoverage, seen map[string]joinedFile) error {
	x, u := c.blob(name)
	if u != nil {
		u.TestKey = test
		c.unknown(*u)
		return nil
	}
	lines := f.lines
	if len(f.v8) > 0 {
		var reason, detail string
		if lines, reason, detail = v8Lines(name, x, f.v8, &c.work); reason != "" {
			c.unknown(CoverageUnknown{TestKey: test, Path: name, Reason: reason, Detail: detail})
			return nil
		}
	}
	instrumented, executed := []int{}, []int{}
	for _, line := range slices.Sorted(mapKeys(lines)) {
		if line > x.Lines() {
			c.unknown(CoverageUnknown{TestKey: test, Path: name, Reason: coverageReasonOutOfRange, Detail: fmt.Sprintf("line %d is past the blob's %d lines", line, x.Lines())})
			return nil
		}
		if strings.TrimSpace(x.Line(line)) == "" {
			c.unknown(CoverageUnknown{TestKey: test, Path: name, Reason: coverageReasonBlankLine, Detail: fmt.Sprintf("instrumented line %d is blank at the revision", line)})
			return nil
		}
		instrumented = append(instrumented, line)
		if lines[line] > 0 {
			executed = append(executed, line)
		}
	}
	if len(executed) == 0 {
		seen[name] = joinedFile{x: x, instrumented: instrumented}
		return nil
	}
	spans := lineSpans(executed)
	evidence, err := c.evidence(x, spans, "supported", "line executed by test "+test+" in run "+run.RunID)
	if err != nil {
		return err
	}
	if len(evidence.Anchors) == 0 {
		seen[name] = joinedFile{x: x, instrumented: instrumented, executed: len(executed), unanchored: true}
		c.unknown(CoverageUnknown{TestKey: test, Path: name, Reason: coverageReasonUnanchorable, Detail: "no executed line could be anchored to the committed blob"})
		return nil
	}
	seen[name] = joinedFile{x: x, instrumented: instrumented, executed: len(executed)}
	c.report.Relations = append(c.report.Relations, CoverageRelation{ID: "coverage:" + name + " -> " + test, Type: "tested_by", From: "source:" + name,
		To: "test:" + test, Path: name, TestKey: test, RunID: run.RunID, Instrumented: len(instrumented), ExecutedLines: len(executed), ExecutedSpans: spans, Evidence: evidence})
	return nil
}

func mapKeys[V any](m map[int]V) func(func(int) bool) {
	return func(yield func(int) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}

func boundCoveragePath(s string) string {
	if len(s) > 512 {
		s = s[:512]
	}
	return strings.ToValidUTF8(strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return '?'
		}
		return r
	}, s), "?")
}

// lineSpans folds sorted line numbers into inclusive contiguous spans.
func lineSpans(lines []int) [][2]int {
	spans := [][2]int{}
	for _, l := range lines {
		if n := len(spans); n > 0 && spans[n-1][1] == l-1 {
			spans[n-1][1] = l
			continue
		}
		spans = append(spans, [2]int{l, l})
	}
	return spans
}

// evidence anchors spans as observed evidence, splitting long spans and keeping the corpus bound.
func (c *coverageIngest) evidence(x *doccorpus.LineIndex, spans [][2]int, state, reason string) (doccorpus.Evidence, error) {
	e := doccorpus.Evidence{Derivation: "observed", Trust: "generated", State: state, Freshness: "fresh", Anchors: []doccorpus.Anchor{}, Limitations: []string{}}
	var add func(start, end int) error
	add = func(start, end int) error {
		if len(e.Anchors) >= maxCoverageAnchors {
			return errAnchorsFull
		}
		if end-start+1 > maxCoverageAnchorLines {
			mid := start + maxCoverageAnchorLines - 1
			if err := add(start, mid); err != nil {
				return err
			}
			return add(mid+1, end)
		}
		a, err := x.Anchor(start, end, "observed", reason)
		if err == nil {
			e.Anchors = append(e.Anchors, a)
			return nil
		}
		if start == end {
			e.Limitations = append(e.Limitations, fmt.Sprintf("line %d is too long to anchor", start))
			return nil
		}
		mid := start + (end-start)/2
		if err := add(start, mid); err != nil {
			return err
		}
		return add(mid+1, end)
	}
	for _, s := range spans {
		if err := add(s[0], s[1]); errors.Is(err, errAnchorsFull) {
			e.Limitations = append(e.Limitations, fmt.Sprintf("only the first %d anchors are kept; the spans list every line", maxCoverageAnchors))
			break
		} else if err != nil {
			return e, err
		}
	}
	if len(e.Anchors) == 0 {
		e.State, e.Unknown = "unknown", "no span could be anchored"
	}
	return e, nil
}

var errAnchorsFull = errors.New("anchor bound reached")

// v8Lines paints V8 block ranges over the committed blob's UTF-16 units and returns each non-blank
// line's largest count. Nested ranges are narrower and later in (start, -end) order, so they win.
func v8Lines(name string, x *doccorpus.LineIndex, scripts []v8Script, work *int) (map[int]int64, string, string) {
	text := string(x.Bytes())
	textKey := Digest(x.Bytes())
	lines := map[int]int64{}
	painted := map[string]bool{}
	var units int
	var lineOf []int32
	var blank []bool
	var counts []int64
	for _, sc := range scripts {
		if painted[sc.key] {
			continue
		}
		painted[sc.key] = true
		if sc.source != nil {
			if sc.sourceKey != textKey {
				return nil, coverageReasonSourceMismatch, "the entry's source differs from the committed blob"
			}
		} else if !slices.Contains([]string{".js", ".mjs", ".cjs"}, path.Ext(name)) {
			return nil, coverageReasonTranspiled, "the entry carries no source and the file is not plain JavaScript"
		}
		if lineOf == nil {
			if *work += len(text); *work > maxCoverageWork {
				return nil, coverageReasonPaintBound, "the V8 coverage exceeds the painting work bound"
			}
			units, lineOf, blank = utf16Units(text)
			counts = make([]int64, units)
		}
		if *work += units; *work > maxCoverageWork {
			return nil, coverageReasonPaintBound, "the V8 coverage exceeds the painting work bound"
		}
		for i := range counts {
			counts[i] = -1
		}
		ranges := slices.Clone(sc.ranges)
		slices.SortStableFunc(ranges, func(a, b v8Range) int {
			if a.StartOffset != b.StartOffset {
				return a.StartOffset - b.StartOffset
			}
			return b.EndOffset - a.EndOffset
		})
		for _, r := range ranges {
			if r.StartOffset < 0 || r.EndOffset > units || r.StartOffset > r.EndOffset || r.Count < 0 {
				return nil, coverageReasonOutOfRange, fmt.Sprintf("range %d..%d is outside the blob's %d UTF-16 units", r.StartOffset, r.EndOffset, units)
			}
			if *work += r.EndOffset - r.StartOffset; *work > maxCoverageWork {
				return nil, coverageReasonPaintBound, "the V8 coverage exceeds the painting work bound"
			}
			for i := r.StartOffset; i < r.EndOffset; i++ {
				counts[i] = r.Count
			}
		}
		for i, count := range counts {
			if count < 0 || blank[i] {
				continue
			}
			line := int(lineOf[i])
			if old, ok := lines[line]; !ok || count > old {
				lines[line] = count
			}
		}
	}
	return lines, "", ""
}

// utf16Units maps each UTF-16 code unit of text to its 1-based line and whether it is blank.
func utf16Units(text string) (int, []int32, []bool) {
	lineOf, blank := []int32{}, []bool{}
	line := int32(1)
	for _, r := range text {
		n := 1
		if utf16.RuneLen(r) == 2 {
			n = 2
		}
		for range n {
			lineOf = append(lineOf, line)
			blank = append(blank, unicode.IsSpace(r))
		}
		if r == '\n' {
			line++
		}
	}
	return len(lineOf), lineOf, blank
}

// checkLink classifies one declared or supplied link against the joined coverage (AFU-V1-116).
func (c *coverageIngest) checkLink(l CoverageLink, set *coverageSet, runs map[string]CoverageRun, joined map[string]map[string]joinedFile) {
	plan := CoverageLinkPlan{Subject: l.Subject, TestKey: l.Test, Source: l.Source, Reference: l.Reference, State: "unknown"}
	defer func() { c.report.Links = append(c.report.Links, plan) }()
	run, covered := runs[l.Test]
	if _, ok := set.tests[l.Test]; !ok || !covered {
		plan.Reason = coverageReasonNotCovered
		return
	}
	if run.State != "joined" {
		plan.Reason = run.Reason
		return
	}
	file, ok := joined[l.Test][l.Subject]
	if !ok || len(file.instrumented) == 0 {
		plan.Reason = coverageReasonNotInstrumented
		return
	}
	if file.unanchored {
		plan.Reason = coverageReasonUnanchorable
		return
	}
	if file.executed > 0 {
		plan.State = "corroborated"
		return
	}
	evidence, err := c.evidence(file.x, lineSpans(file.instrumented), "conflicted", "instrumented line not executed by test "+l.Test+" in run "+run.RunID)
	if err != nil || len(evidence.Anchors) == 0 {
		plan.Reason = coverageReasonUnanchorable
		return
	}
	plan.State, plan.Reason = "conflicted", coverageReasonContradiction
	c.report.Conflicts = append(c.report.Conflicts, CoverageConflict{ID: "coverage-conflict:" + l.Subject + " -> " + l.Test + " (" + l.Source + ")", Type: "conflicts_with",
		Subject: "source:" + l.Subject, TestKey: l.Test, LinkSource: l.Source, Reference: l.Reference, Reason: coverageReasonContradiction, RunID: run.RunID,
		Instrumented: len(file.instrumented), Evidence: evidence})
}
