package testsource

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/rubytestprovider"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

// The `corvint tests example-join` reading (TSA-V0-410..419): each example of one retained
// provider receipt joined to the static test-validity verdict of the same example at the revision
// the receipt ran on, keyed by a line-free example identity.

const (
	// ExampleJoinSchema names the report.
	ExampleJoinSchema = "corvint-example-join/0"
	// MaxJoinExamples bounds the receipt examples one join reads.
	MaxJoinExamples = 2000
	// MaxJoinFiles bounds the distinct committed files one join reads.
	MaxJoinFiles = 256
	// ExampleIdentityRSpec keys an RSpec example by its spec path and scoped id.
	ExampleIdentityRSpec = "rspec-scoped-id"
	// ExampleIdentityJS keys a JS example by its file, project and full title.
	ExampleIdentityJS = "js-title-path"

	joinReasonSourceUnbound     = "example-source-unbound"
	joinReasonIdentityUnbound   = "example-identity-unbound"
	joinReasonIdentityAmbiguous = "example-identity-ambiguous"
	// joinReasonFileAmbiguous: a JS anchor file names more than one retained test-file digest and
	// none of them is the anchor file itself (V1-2085).
	joinReasonFileAmbiguous = "example-file-ambiguous"
	// joinReasonFileOutsideRepository: the one retained digest a JS anchor names lies outside the
	// repository root, so no committed file can bind it (V1-2085).
	joinReasonFileOutsideRepository = "example-file-outside-repository"
)

// ExampleJoinOptions names the revision ("" reads HEAD), exactly one decoded receipt and the
// caller's requirement selections (TSA-V0-417).
type ExampleJoinOptions struct {
	Revision   string
	Ruby       *rubytestprovider.Document
	JS         *jstestprovider.Receipt
	Selections []RequirementSelection
}

// ExampleJoinReport is the corvint-example-join/0 document.
type ExampleJoinReport struct {
	Schema     string                `json:"schema"`
	Revision   string                `json:"revision"`
	Language   string                `json:"language"`
	Repository ExampleJoinRepository `json:"repository"`
	Examples   []ExampleJoinRecord   `json:"examples"`
}

// ExampleJoinRepository is the repository identity the receipt retained around its run.
type ExampleJoinRepository struct {
	RevisionBefore string `json:"revisionBefore"`
	RevisionAfter  string `json:"revisionAfter"`
	DirtyState     string `json:"dirtyState"`
}

// ExampleJoinRecord is one example: its identity, run outcome, verdict and the source both used.
type ExampleJoinRecord struct {
	Example      string               `json:"example"`
	IdentityKind string               `json:"identityKind"`
	File         string               `json:"file"`
	Line         int                  `json:"line"`
	Outcome      string               `json:"outcome"`
	Requirements *ExampleRequirements `json:"requirements,omitempty"`
	Verdict      ExampleJoinVerdict   `json:"verdict"`
	Source       ExampleJoinSource    `json:"source"`
}

// ExampleJoinVerdict is the hygiene and strength axes the static example rules give the example.
type ExampleJoinVerdict struct {
	Hygiene  testvalidity.Axis `json:"hygiene"`
	Strength testvalidity.Axis `json:"strength"`
	// Conditions are the flag values the example forces; its verdict holds only under them
	// (TSA-V0-440).
	Conditions []testvalidity.FlagCondition `json:"conditions,omitempty"`
}

// ExampleJoinSource is the revision and the committed files the verdict read.
type ExampleJoinSource struct {
	Revision string            `json:"revision"`
	Files    []ExampleJoinFile `json:"files"`
}

// ExampleJoinFile is one committed file and its Git blob ID at the revision.
type ExampleJoinFile struct {
	Path string `json:"path"`
	Blob string `json:"blob"`
}

// joinFile is one committed file read at the join revision.
type joinFile struct {
	text, blob string
	found      bool
}

// joiner reads committed files once each, within MaxJoinFiles.
type joiner struct {
	r      Repo
	commit string
	files  map[string]joinFile
}

func (j *joiner) file(name string) (joinFile, error) {
	if f, ok := j.files[name]; ok {
		return f, nil
	}
	if len(j.files) >= MaxJoinFiles {
		return joinFile{}, refuse("tests-bound-exceeded", "more than %d source files", MaxJoinFiles)
	}
	data, blob, found, err := j.r.readOneBlob(j.commit, name, MaxFileBytes)
	if err != nil {
		return joinFile{}, err
	}
	f := joinFile{text: string(data), blob: blob, found: found}
	j.files[name] = f
	return f, nil
}

// bind refuses tests-join-revision-mismatch unless name is committed at the revision with the
// content the receipt bound (TSA-V0-414).
func (j *joiner) bind(name string, same func(joinFile) bool) error {
	f, err := j.file(name)
	if err != nil {
		return err
	}
	if !f.found || !same(f) {
		return refuse("tests-join-revision-mismatch", "%s at the revision is not the file the receipt bound", name)
	}
	return nil
}

// joined is one record and what selectors match it by.
type joined struct {
	record   ExampleJoinRecord
	path     string
	included []string
	// unbound is the typed reason a JS example whose anchor resolved no file abstains with.
	unbound string
}

// ExampleJoin joins each receipt example to its verdict at the revision (TSA-V0-410..416) and
// echoes the caller's requirement IDs on the examples the selections name (TSA-V0-417..419).
func ExampleJoin(r Repo, options ExampleJoinOptions) (ExampleJoinReport, error) {
	if (options.Ruby == nil) == (options.JS == nil) {
		return ExampleJoinReport{}, refuse("tests-invalid-arguments", "the receipt is not one Ruby or JS provider receipt")
	}
	revision := options.Revision
	if revision == "" {
		revision = "HEAD"
	}
	commit, err := r.Resolve(revision)
	if err != nil {
		return ExampleJoinReport{}, err
	}
	report := ExampleJoinReport{Schema: ExampleJoinSchema, Revision: commit}
	var before, after *jstestprovider.ApplicationRepositoryIdentity
	if options.Ruby != nil {
		report.Language, before, after = "ruby", options.Ruby.RepositoryBefore, options.Ruby.RepositoryAfter
	} else {
		report.Language, before, after = "js", options.JS.TestRepositoryAtStart, options.JS.TestRepositoryAtPublish
	}
	if before == nil || after == nil || before.Revision == "" || after.Revision == "" {
		return ExampleJoinReport{}, refuse("tests-join-revision-unbound", "the receipt retains no repository revision before and after its run")
	}
	report.Repository = ExampleJoinRepository{RevisionBefore: before.Revision, RevisionAfter: after.Revision, DirtyState: after.DirtyState}
	if before.Revision != after.Revision || !strings.EqualFold(after.Revision, commit) {
		return ExampleJoinReport{}, refuse("tests-join-revision-mismatch", "the receipt ran at %s..%s, not at %s", before.Revision, after.Revision, commit)
	}
	dirty := before.DirtyState != "clean" || after.DirtyState != "clean"
	j := &joiner{r: r, commit: commit, files: map[string]joinFile{}}
	var rows []joined
	if options.Ruby != nil {
		rows, err = j.ruby(*options.Ruby, dirty)
	} else {
		rows, err = j.js(*options.JS)
	}
	if err != nil {
		return ExampleJoinReport{}, err
	}
	if err := applySelections(rows, options.Selections); err != nil {
		return ExampleJoinReport{}, err
	}
	report.Examples = make([]ExampleJoinRecord, 0, len(rows))
	for _, row := range rows {
		report.Examples = append(report.Examples, row.record)
	}
	return report, nil
}

// abstain sets a record's verdict to ABSTAINED with reason.
func abstain(record *ExampleJoinRecord, reason string) {
	anchor := []string{fmt.Sprintf("%s:%d", record.File, record.Line)}
	record.Verdict = ExampleJoinVerdict{
		Hygiene:  testvalidity.Axis{State: testvalidity.HygieneAbstained, Reason: reason, Anchors: anchor},
		Strength: testvalidity.Axis{State: testvalidity.StrengthNotMeasured, Reason: reason, Anchors: anchor},
	}
}

// joinVerdict is the projection the apply function gives a source; an unlocated read keeps strength
// NOT_MEASURED with the read reason.
func joinVerdict(apply func(testvalidity.Projection, testvalidity.ExampleSource, string) testvalidity.Projection, source testvalidity.ExampleSource, anchor string) ExampleJoinVerdict {
	p := apply(testvalidity.Projection{Strength: testvalidity.Axis{State: testvalidity.StrengthNotMeasured}}, source, anchor)
	if p.Strength.Reason == "" {
		p.Strength = testvalidity.Axis{State: testvalidity.StrengthNotMeasured, Reason: p.Hygiene.Reason, Anchors: p.Hygiene.Anchors}
	}
	if p.Strength.Anchors == nil {
		p.Strength.Anchors = []string{}
	}
	return ExampleJoinVerdict{Hygiene: p.Hygiene, Strength: p.Strength, Conditions: source.Conditions}
}

// ruby joins the examples of one Ruby provider document.
func (j *joiner) ruby(document rubytestprovider.Document, dirty bool) ([]joined, error) {
	if len(document.Examples) > MaxJoinExamples {
		return nil, refuse("tests-bound-exceeded", "more than %d examples", MaxJoinExamples)
	}
	blobIs := func(blob string) func(joinFile) bool {
		return func(f joinFile) bool { return strings.EqualFold(f.blob, blob) }
	}
	helpers := map[string]bool{}
	for _, helper := range document.Helpers {
		if err := j.bind(helper.Path, blobIs(helper.Blob)); err != nil {
			return nil, err
		}
		helpers[helper.Path] = true
	}
	boundSpecs := map[string]bool{}
	for _, example := range document.Examples {
		if example.SpecBlob == "" {
			continue
		}
		if err := j.bind(example.File, blobIs(example.SpecBlob)); err != nil {
			return nil, err
		}
		boundSpecs[example.File] = true
	}
	rows := make([]joined, 0, len(document.Examples))
	counts := map[string]int{}
	for _, example := range document.Examples {
		row := joined{record: ExampleJoinRecord{IdentityKind: ExampleIdentityRSpec, File: example.File, Line: example.Line, Outcome: example.Status,
			Source: ExampleJoinSource{Revision: j.commit, Files: []ExampleJoinFile{}}}}
		if file, _, ok := testvalidity.ParseRSpecScopedID(example.RSpecID); ok {
			row.path = path.Join(document.ProjectDirectory, path.Clean(file))
			row.record.Example = row.path + example.RSpecID[strings.LastIndex(example.RSpecID, "["):]
			counts[row.record.Example]++
		}
		if example.SharedExample != nil {
			row.included = example.SharedExample.IncludedAt
		}
		rows = append(rows, row)
	}
	supports := map[string]joinSupport{}
	sources := map[string]map[int]testvalidity.ExampleSource{}
	reasons := map[string]string{}
	var flags *testvalidity.FlagLookups
	var flagFile []ExampleJoinFile
	for index, example := range document.Examples {
		record := &rows[index].record
		switch {
		case record.Example == "":
			abstain(record, joinReasonIdentityUnbound)
			continue
		case counts[record.Example] > 1:
			abstain(record, joinReasonIdentityAmbiguous)
			continue
		case example.SpecBlob == "":
			abstain(record, joinReasonSourceUnbound)
			continue
		}
		spec := rows[index].path
		set, err := j.rubySupport(spec, supports, helpers, dirty, document.HelpersUnbound)
		if err != nil {
			return nil, err
		}
		support := set.support
		shared, _ := j.file(example.File)
		record.Source.Files = append(record.Source.Files, ExampleJoinFile{Path: example.File, Blob: shared.blob})
		if spec == example.File {
			if flags == nil {
				// The flag lookup declaration classifies stubs (TSA-V0-439); read once per document.
				f, err := j.file(testvalidity.FlagLookupsPath)
				if err != nil {
					return nil, err
				}
				read := testvalidity.BuiltinFlagLookups()
				if f.found {
					if read, err = DecodeFlagLookups([]byte(f.text)); err != nil {
						return nil, err
					}
					flagFile = []ExampleJoinFile{{Path: testvalidity.FlagLookupsPath, Blob: f.blob}}
				}
				flags = &read
			}
			support.Flags = flags
			examples, ok := sources[spec]
			if !ok {
				examples, reasons[spec] = RSpecExampleSourcesWithSupport(spec, shared.text, support)
				sources[spec] = examples
			}
			source := testvalidity.ExampleSource{Read: reasons[spec], Findings: []testvalidity.SourceFinding{}}
			if reasons[spec] == "" {
				if located, found := examples[example.Line]; found {
					source = located
				} else {
					source.Read = testvalidity.HygieneReasonRSpecExampleUnplaced
				}
			}
			record.Verdict = joinVerdict(testvalidity.ApplyExampleSource, source, fmt.Sprintf("%s:%d", example.File, example.Line))
			record.Source.Files = withSupport(withSupport(record.Source.Files, set.files), flagFile)
			continue
		}
		including, err := j.file(spec)
		if err != nil {
			return nil, err
		}
		if !including.found || dirty && !boundSpecs[spec] {
			abstain(record, testvalidity.InclusionSourceUnbound)
			continue
		}
		record.Source.Files = append([]ExampleJoinFile{{Path: spec, Blob: including.blob}}, record.Source.Files...)
		_, ids, _ := testvalidity.ParseRSpecScopedID(example.RSpecID)
		source, ref := RSpecInclusionSource(testvalidity.RSpecInclusion{
			SpecPath: spec, SpecText: including.text, ScopedID: ids, Description: example.Description,
			SharedPath: example.File, SharedText: shared.text, ExampleLine: example.Line, Support: support,
		})
		record.Verdict = joinVerdict(testvalidity.ApplyExampleSource, source, ref)
		record.Source.Files = withSupport(record.Source.Files, set.files)
	}
	return rows, nil
}

// joinSupport is the support set of one spec prefix and the committed files it was read from.
type joinSupport struct {
	support testvalidity.RubySupport
	files   []ExampleJoinFile
}

// rubySupport is the committed support set of a spec, read once per prefix through the join's own
// file and size bounds (TSA-V0-416). A receipt that did not bind its helper set leaves it unbound
// whatever its dirty state; a dirty receipt binds it only when every committed support file is a
// helper it retained with a matching blob (TSA-V0-414).
func (j *joiner) rubySupport(spec string, cache map[string]joinSupport, helpers map[string]bool, dirty bool, helpersUnbound string) (joinSupport, error) {
	prefix := specPrefix(spec)
	if set, ok := cache[prefix]; ok {
		return set, nil
	}
	set, err := j.readSupport(prefix, helpers, dirty, helpersUnbound)
	if err != nil {
		return joinSupport{}, err
	}
	cache[prefix] = set
	return set, nil
}

func (j *joiner) readSupport(prefix string, helpers map[string]bool, dirty bool, helpersUnbound string) (joinSupport, error) {
	if helpersUnbound != "" {
		return joinSupport{support: testvalidity.RubySupport{Unbound: helpersUnbound}}, nil
	}
	entries, err := j.r.tree(j.commit, RubySupportPaths(prefix), MaxFiles)
	var refusal *Error
	if errors.As(err, &refusal) && refusal.Code == "tests-bound-exceeded" {
		// A support set past the listing budget is past the join's own file bound too, so it
		// refuses with the join's message (V1-2088).
		return joinSupport{}, refuse("tests-bound-exceeded", "more than %d source files", MaxJoinFiles)
	}
	if err != nil {
		return joinSupport{}, err
	}
	set := joinSupport{support: testvalidity.RubySupport{Files: map[string]string{}}}
	for _, e := range entries {
		if dirty && !helpers[e.path] {
			return joinSupport{support: testvalidity.RubySupport{Unbound: testvalidity.HygieneReasonRSpecSourceUnbound}}, nil
		}
		f, err := j.file(e.path)
		if err != nil {
			return joinSupport{}, err
		}
		set.support.Files[e.path] = f.text
		set.files = append(set.files, ExampleJoinFile{Path: e.path, Blob: f.blob})
	}
	slices.SortFunc(set.files, func(a, b ExampleJoinFile) int { return strings.Compare(a.Path, b.Path) })
	return set, nil
}

// withSupport appends the support files a verdict was read with to its source record, each once
// (TSA-V0-412).
func withSupport(files, support []ExampleJoinFile) []ExampleJoinFile {
	for _, f := range support {
		if !slices.ContainsFunc(files, func(g ExampleJoinFile) bool { return g.Path == f.Path }) {
			files = append(files, f)
		}
	}
	return files
}

// js joins the tests of one JS provider receipt. A test file is bound by its retained SHA-256
// when its absolute path lies under the repository root.
func (j *joiner) js(receipt jstestprovider.Receipt) ([]joined, error) {
	if len(receipt.Tests) > MaxJoinExamples {
		return nil, refuse("tests-bound-exceeded", "more than %d examples", MaxJoinExamples)
	}
	relative := map[string]string{}
	for key, digest := range receipt.Identity.TestFileDigests {
		name, ok := j.underRoot(key)
		if !ok {
			continue
		}
		if err := j.bind(name, func(f joinFile) bool { return strings.EqualFold(sha256Hex([]byte(f.text)), digest) }); err != nil {
			return nil, err
		}
		relative[key] = name
	}
	rows := make([]joined, 0, len(receipt.Tests))
	counts := map[string]int{}
	for _, test := range receipt.Tests {
		row := joined{record: ExampleJoinRecord{IdentityKind: ExampleIdentityJS, Outcome: string(test.State),
			Source: ExampleJoinSource{Revision: j.commit, Files: []ExampleJoinFile{}}}}
		if test.Anchor != nil {
			row.record.File, row.record.Line = filepath.ToSlash(test.Anchor.File), test.Anchor.Line
			key, unbound := anchorKey(receipt.Identity.TestFileDigests, test.Anchor.File)
			switch {
			case unbound != "":
				row.unbound = unbound
			case relative[key] == "":
				row.unbound = joinReasonFileOutsideRepository
			default:
				project := ""
				if test.Project != nil {
					project = test.Project.Name
				}
				row.path, row.record.File = relative[key], relative[key]
				row.record.Example = row.path + " > " + project + " > " + test.FullName
				counts[row.record.Example]++
			}
		}
		rows = append(rows, row)
	}
	sources := map[string]map[int]testvalidity.ExampleSource{}
	reasons := map[string]string{}
	for index := range rows {
		record := &rows[index].record
		switch {
		case rows[index].unbound != "":
			abstain(record, rows[index].unbound)
			continue
		case record.Example == "":
			abstain(record, joinReasonIdentityUnbound)
			continue
		case counts[record.Example] > 1:
			abstain(record, joinReasonIdentityAmbiguous)
			continue
		}
		name := rows[index].path
		f, _ := j.file(name)
		record.Source.Files = append(record.Source.Files, ExampleJoinFile{Path: name, Blob: f.blob})
		examples, ok := sources[name]
		if !ok {
			examples, reasons[name] = JSExampleSources(name, f.text)
			sources[name] = examples
		}
		source := testvalidity.ExampleSource{Read: reasons[name], Findings: []testvalidity.SourceFinding{}}
		line := record.Line
		if reasons[name] == "" {
			if placed, found := placeExample(examples, line); found {
				source, line = examples[placed], placed
			} else {
				source.Read = testvalidity.HygieneReasonJSExampleUnplaced
			}
		}
		record.Verdict = joinVerdict(testvalidity.ApplyJSExampleSource, source, fmt.Sprintf("%s:%d", name, line))
	}
	return rows, nil
}

// underRoot is the repository-relative slash path of an absolute receipt path under the root.
func (j *joiner) underRoot(key string) (string, bool) {
	if !filepath.IsAbs(key) {
		return "", false
	}
	roots := []string{j.r.root}
	if abs, err := filepath.Abs(j.r.root); err == nil {
		roots = append(roots, abs)
		if resolved, err := filepath.EvalSymlinks(abs); err == nil {
			roots = append(roots, resolved)
		}
	}
	for _, root := range roots {
		if rel, err := filepath.Rel(root, key); err == nil && filepath.IsLocal(rel) {
			name := filepath.ToSlash(rel)
			if name == ".git" || strings.HasPrefix(name, ".git/") {
				return "", false
			}
			return name, true
		}
	}
	return "", false
}

// anchorKey is the one digest key that equals, or ends with "/" plus, the anchor file. When
// several keys match, only a key equal to the anchor file itself binds: a relative anchor is
// relative to the reporter's root, which the receipt does not retain, so a key whose
// repository-relative path equals it is no better a match than any other and the example
// abstains `example-file-ambiguous` (V1-2085). No match returns joinReasonIdentityUnbound.
func anchorKey(digests map[string]string, file string) (string, string) {
	file = filepath.ToSlash(file)
	match, count := "", 0
	for key := range digests {
		slashed := filepath.ToSlash(key)
		if slashed == file {
			return key, ""
		}
		if strings.HasSuffix(slashed, "/"+file) {
			match, count = key, count+1
		}
	}
	switch count {
	case 0:
		return "", joinReasonIdentityUnbound
	case 1:
		return match, ""
	}
	return "", joinReasonFileAmbiguous
}

// placeExample is the example declared at line, else the innermost example whose lines hold it.
func placeExample(examples map[int]testvalidity.ExampleSource, line int) (int, bool) {
	if _, ok := examples[line]; ok {
		return line, true
	}
	best := 0
	for start, example := range examples {
		if start < line && example.EndLine >= line && start > best {
			best = start
		}
	}
	return best, best != 0
}
