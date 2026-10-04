package postmergeworkflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/delta"
	"github.com/Beamfall/corvint/internal/intake"
	connector "github.com/Beamfall/corvint/internal/postmergeconnector"
)

// FixtureProfile is only a bounded dispatch hint. Each route still performs its
// full strict decode; malformed hints fall through to the unchanged /0 refusal.
func FixtureProfile(name string) string {
	b, err := readBounded(name)
	if err != nil {
		return ""
	}
	var fields map[string]json.RawMessage
	if Decode(b, &fields) != nil {
		return ""
	}
	var profile string
	if json.Unmarshal(fields["profile"], &profile) != nil {
		return ""
	}
	return profile
}

func newNativeReport() NativeReport {
	r := NativeReport{Profile: NativeProfile, Status: "BLOCKED", Reasons: []string{}, Stages: []NativeStage{}, ComparisonStatus: "NOT_RUN", GeneratedMismatches: []Mismatch{}, HumanVerifiedMismatches: []Mismatch{}, WorkflowQualification: "NOT_OBSERVED"}
	for _, name := range []string{"trigger", "connector-intake", "intake", "delta", "follow-up", "docs-author", "docs-scope", "docs-validation", "tests-author", "tests-scope", "tests-validation", "draft-requests", "findings", "metrics", "approved-republish", "recording"} {
		r.Stages = append(r.Stages, NativeStage{Name: name, Disposition: "not-run", Inputs: []ArtifactRef{}, Reasons: []string{"prior-stage-blocked"}})
	}
	r.Stages[0].Reasons = []string{"host-trigger-not-executed"}
	return r
}

// ReplayNative observes the real connector, intake and delta only. Expected
// labels never enter author input; a failed delta forbids every downstream
// operation, and the unintegrated follow-up stage blocks the rest.
func ReplayNative(ctx context.Context, root, fixtureFile, policyFile, change string) (NativeReport, error) {
	r := newNativeReport()
	var retained *nativeRetention
	block := func(stage int, code string, original error) (NativeReport, error) {
		if original != nil && retained != nil {
			if _, err := retained.save(r.Stages[stage].Name+"-error.txt", []byte(original.Error())); err != nil {
				code = "runtime-retention-failed"
			}
		}
		if retained != nil && retained.verify() != nil {
			code = "runtime-retention-failed"
			// A native call occurred, but changed evidence cannot remain an
			// observed item whose path/hash no longer describes retained bytes.
			for i := range r.Stages {
				if r.Stages[i].Disposition == "observed" {
					r.Stages[i].Disposition = "blocked"
					r.Stages[i].NativeProfile = ""
					r.Stages[i].Output = nil
					r.Stages[i].Reasons = []string{code}
				}
			}
		}
		r.Stages[stage].Disposition = "blocked"
		r.Stages[stage].Reasons = []string{code}
		r.Reasons = append(r.Reasons, code)
		return r, fmt.Errorf("%s", code)
	}
	if ctx.Err() != nil {
		return block(1, "runtime-cancelled", ctx.Err())
	}
	fb, err := nativeRead(fixtureFile, MaxBytes)
	if err != nil {
		return block(1, "runtime-input-invalid", err)
	}
	pb, err := nativeRead(policyFile, MaxBytes)
	if err != nil {
		return block(1, "runtime-input-invalid", err)
	}
	r.FixtureSHA256 = SHA256(fb)
	r.PolicySHA256 = SHA256(pb)
	var f Fixture
	var p NativePolicy
	if Decode(fb, &f) != nil || !present(fb, "profile", "connector", "source_item", "expected") || !nestedPresent(fb, "expected", "affected_flows", "followup", "test_gaps", "defects", "labels") {
		return block(1, "runtime-fixture-invalid", nil)
	}
	if Decode(pb, &p) != nil || !present(pb, "profile", "connector", "manifest", "manifest_sha256") || !nestedPresent(pb, "connector", "profile", "expected", "hierarchy_level", "allowed_classes", "max_findings", "url_origins") || !present(jsonMember(pb, "connector", "expected"), "forge", "repository", "change", "base", "merge") {
		return block(1, "runtime-policy-invalid", nil)
	}
	binding := f.Connector.Forge.Change.Binding
	if f.Profile != NativeProfile || p.Profile != NativeProfile || binding != p.Connector.Expected || change != binding.Change || !identifier.MatchString(f.SourceItem) || !identifier.MatchString(binding.Forge) || !identifier.MatchString(binding.Repository) || !identifier.MatchString(binding.Change) || !wire.IsGitOid(binding.Base) || !wire.IsGitOid(binding.Merge) || binding.Base == binding.Merge {
		return block(1, "fixture-policy-binding-invalid", nil)
	}
	if err = connector.ValidatePolicy(p.Connector); err != nil {
		return block(1, "runtime-policy-invalid", err)
	}
	// Only admitted bounded identity values may enter the public report.
	r.Binding = binding
	if !stringsValid(f.Expected.AffectedFlows, false) || !stringsValid(f.Expected.TestGaps, false) || !findingsValid(f.Expected.Defects) || len(f.Expected.Labels) != 4 {
		return block(1, "expectations-invalid", nil)
	}
	for field := range expectedValues(f.Expected) {
		label, ok := f.Expected.Labels[field]
		if !ok || !hash.MatchString(label.EvidenceSHA256) || !present(jsonMember(fb, "expected", "labels", field), "basis", "evidence_sha256", "human", "approval") {
			return block(1, "label-invalid", nil)
		}
		if label.Basis == "human-verified" {
			return block(1, "label-qualification-unavailable", nil)
		}
		if label.Basis != "generated" {
			return block(1, "label-basis-invalid", nil)
		}
		if label.Human != "" || label.Approval != "" {
			return block(1, "generated-label-promoted", nil)
		}
	}
	if !hash.MatchString(p.ManifestSHA256) {
		return block(1, "runtime-manifest-invalid", nil)
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return block(1, "product-root-invalid", err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return block(1, "product-root-invalid", err)
	}
	for _, directory := range []string{root, filepath.Dir(fixtureFile), filepath.Dir(policyFile)} {
		if nativeWithin(directory, p.Manifest) {
			return block(1, "runtime-manifest-invalid", nil)
		}
	}
	mb, err := nativeRead(p.Manifest, MaxBytes)
	if err != nil {
		return block(1, "runtime-manifest-invalid", err)
	}
	if SHA256(mb) != p.ManifestSHA256 {
		return block(1, "runtime-manifest-pin-mismatch", nil)
	}
	r.ManifestSHA256 = p.ManifestSHA256
	var m NativeManifest
	if Decode(mb, &m) != nil || !present(mb, "profile", "mode", "implementation", "product", "fixture_sha256", "reader_candidate", "retained_output_root") || !nestedPresent(mb, "implementation", "source_commit", "source_tree", "executable_sha256") || !nestedPresent(mb, "product", "base", "merge", "tree") || !nestedPresent(mb, "reader_candidate", "path", "sha256") {
		return block(1, "runtime-manifest-invalid", nil)
	}
	if m.Profile != NativeManifestProfile || !wire.IsGitOid(m.Implementation.SourceCommit) || !wire.IsGitOid(m.Implementation.SourceTree) || !hash.MatchString(m.Implementation.ExecutableSHA256) || !hash.MatchString(m.ReaderCandidate.SHA256) || m.FixtureSHA256 != r.FixtureSHA256 || m.Product.Base != r.Binding.Base || m.Product.Merge != r.Binding.Merge || !wire.IsGitOid(m.Product.Tree) {
		return block(1, "runtime-manifest-binding-invalid", nil)
	}
	r.Implementation = m.Implementation
	if m.Mode == "operational" {
		return block(1, "operational-qualification-unavailable", nil)
	}
	if m.Mode != "candidate-qualification" {
		return block(1, "runtime-manifest-invalid", nil)
	}
	exe, err := os.Executable()
	if err != nil {
		return block(1, "executable-invalid", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return block(1, "executable-invalid", err)
	}
	executable, err := nativeRead(exe, 128<<20)
	if err != nil {
		return block(1, "executable-invalid", err)
	}
	if SHA256(executable) != m.Implementation.ExecutableSHA256 {
		return block(1, "executable-pin-mismatch", nil)
	}
	candidate, err := nativeRead(m.ReaderCandidate.Path, MaxBytes)
	if err != nil {
		return block(1, "runtime-candidate-invalid", err)
	}
	if SHA256(candidate) != m.ReaderCandidate.SHA256 {
		return block(1, "runtime-candidate-pin-mismatch", nil)
	}
	repo, err := gitauth.Open(root, gitrun.NewBudget(128, 30*time.Second))
	if err != nil {
		return block(1, "runtime-product-pin-invalid", err)
	}
	for _, revision := range []string{m.Product.Base, m.Product.Merge} {
		actual, e := repo.Resolve(ctx, revision)
		if e != nil || actual != revision {
			return block(1, "runtime-product-pin-invalid", e)
		}
	}
	tree, err := repo.CommitTree(ctx, m.Product.Merge)
	if err != nil || tree != m.Product.Tree {
		return block(1, "runtime-product-pin-invalid", err)
	}
	protected := []string{root, repo.GitDir, repo.CommonDir, filepath.Dir(fixtureFile), filepath.Dir(policyFile), filepath.Dir(p.Manifest), filepath.Dir(m.ReaderCandidate.Path), exe}
	retained, err = newNativeRetention(m.RetainedOutputRoot, protected)
	if err != nil {
		return block(1, "runtime-output-invalid", err)
	}
	defer retained.root.Close()
	for _, snapshot := range []struct {
		name  string
		bytes []byte
	}{{"fixture.json", fb}, {"policy.json", pb}, {"manifest.json", mb}} {
		if _, err = retained.save(snapshot.name, snapshot.bytes); err != nil {
			return block(1, "runtime-retention-failed", err)
		}
	}
	ci := struct {
		Root    string            `json:"root"`
		Fixture connector.Fixture `json:"fixture"`
		Policy  connector.Policy  `json:"policy"`
		Source  string            `json:"source_item"`
	}{root, f.Connector, p.Connector, f.SourceItem}
	cib, _ := json.Marshal(ci)
	ref, err := retained.save("connector-input.json", cib)
	if err != nil {
		return block(1, "runtime-retention-failed", err)
	}
	r.Stages[1].Inputs = append(r.Stages[1].Inputs, ref)
	contextBytes, _ := json.Marshal(struct {
		Root string `json:"root"`
		Base string `json:"base"`
		Head string `json:"head"`
	}{root, m.Product.Base, m.Product.Merge})
	// The closed candidate's exact bytes are captured separately, not re-rendered
	// or replaced by connector raw prose or fixture expectations.
	candidateRef, err := retained.save("reader-candidate.json", candidate)
	if err != nil {
		return block(1, "runtime-retention-failed", err)
	}
	contextRef, err := retained.save("intake-context.json", contextBytes)
	if err != nil {
		return block(1, "runtime-retention-failed", err)
	}
	if ctx.Err() != nil {
		return block(1, "runtime-cancelled", ctx.Err())
	}
	observed, err := connector.Read(ctx, root, f.Connector, p.Connector, f.SourceItem)
	if err != nil {
		return block(1, nativeCallCode(ctx, "native-connector-failed"), err)
	}
	ob, err := json.Marshal(observed)
	if err != nil {
		return block(1, "runtime-retention-failed", err)
	}
	output, err := retained.save("connector-output.json", ob)
	if err != nil {
		return block(1, "runtime-retention-failed", err)
	}
	r.Stages[1].Disposition = "observed"
	r.Stages[1].NativeProfile = connector.Profile
	connectorOutput := output
	r.Stages[1].Output = &connectorOutput
	r.Stages[1].Reasons = []string{}
	r.Stages[2].Inputs = []ArtifactRef{contextRef, candidateRef}
	authorInput, err := intake.BuildAuthorInput(ctx, root, m.Product.Base, m.Product.Merge, candidate)
	if err != nil {
		return block(2, nativeCallCode(ctx, "native-intake-failed"), err)
	}
	output, err = retained.save("intake-output.json", authorInput)
	if err != nil {
		return block(2, "runtime-retention-failed", err)
	}
	r.Stages[2].Disposition = "observed"
	r.Stages[2].NativeProfile = intake.Profile
	intakeOutput := output
	r.Stages[2].Output = &intakeOutput
	r.Stages[2].Reasons = []string{}
	for i := 4; i < len(r.Stages); i++ {
		r.Stages[i].Reasons = []string{"dependency-delta-blocked"}
	}
	// The actual immutable delta compiler (#389) runs with only code-fixed
	// options: no providers, documentation baseline or work-key pattern, so its
	// record keeps the resulting unknowns and full-suite obligations exactly.
	// The build label is the operator-pinned source commit, not an attestation.
	options := delta.Options{Base: m.Product.Base, Head: m.Product.Merge, Build: m.Implementation.SourceCommit}
	deltaInput, _ := json.Marshal(struct {
		Root               string   `json:"root"`
		Base               string   `json:"base"`
		Head               string   `json:"head"`
		Build              string   `json:"build"`
		PreviousGeneration string   `json:"previous_generation"`
		WorkKeyPattern     string   `json:"work_key_pattern"`
		Providers          []string `json:"providers"`
		Checkouts          []string `json:"checkouts"`
	}{root, options.Base, options.Head, options.Build, "", "", []string{}, []string{}})
	deltaRef, err := retained.save("delta-input.json", deltaInput)
	if err != nil {
		return block(3, "runtime-retention-failed", err)
	}
	r.Stages[3].Inputs = []ArtifactRef{deltaRef}
	if ctx.Err() != nil {
		return block(3, "runtime-cancelled", ctx.Err())
	}
	record, err := delta.Compile(ctx, root, options)
	if err != nil {
		return block(3, nativeCallCode(ctx, "native-delta-failed"), err)
	}
	if record.Base != m.Product.Base || record.Head != m.Product.Merge || record.Tree != m.Product.Tree {
		return block(3, "native-delta-failed", fmt.Errorf("delta-binding-mismatch"))
	}
	deltaBytes, err := record.Canonical()
	if err != nil {
		return block(3, "native-delta-failed", err)
	}
	output, err = retained.save("delta-output.json", deltaBytes)
	if err != nil {
		return block(3, "runtime-retention-failed", err)
	}
	r.Stages[3].Disposition = "observed"
	r.Stages[3].NativeProfile = delta.Schema
	deltaOutput := output
	r.Stages[3].Output = &deltaOutput
	r.Stages[3].Reasons = []string{}
	for i := 5; i < len(r.Stages); i++ {
		r.Stages[i].Reasons = []string{"prior-stage-blocked"}
	}
	// No follow-up, author, draft or recording stage is integrated by this
	// source; connector Build/ValidatePlan/Record stay unreachable from /1.
	return block(4, "follow-up-not-integrated", nil)
}

// nativeWithin compares actual protected and ancestor identities, not path
// spelling: alternate case and mount aliases can name the same directory.
// Any unavailable identity refuses admission. All opens retain no-follow rules.
func nativeWithin(root, name string) bool {
	protected, err := nativeOpenFile(root)
	if err != nil {
		return true
	}
	identity, err := protected.Stat()
	protected.Close()
	if err != nil {
		return true
	}
	// Also protect an exact existing destination, including the executable.
	entry, err := nativeOpenFile(name)
	if err == nil {
		actual, statErr := entry.Stat()
		entry.Close()
		if statErr != nil || os.SameFile(identity, actual) {
			return true
		}
	} else if !os.IsNotExist(err) {
		return true
	}
	if !identity.IsDir() {
		return false
	}
	for directory := filepath.Dir(name); ; directory = filepath.Dir(directory) {
		ancestor, err := nativeOpenRoot(directory)
		if err != nil {
			return true
		}
		actual, err := ancestor.Stat(".")
		ancestor.Close()
		if err != nil || os.SameFile(identity, actual) {
			return true
		}
		if filepath.Dir(directory) == directory {
			return false
		}
	}
}

func nativeCallCode(ctx context.Context, code string) string {
	if ctx.Err() != nil {
		return "runtime-cancelled"
	}
	return code
}
func present(b []byte, keys ...string) bool {
	var fields map[string]json.RawMessage
	if json.Unmarshal(b, &fields) != nil || len(fields) != len(keys) {
		return false
	}
	for _, key := range keys {
		if value, ok := fields[key]; !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return false
		}
	}
	return true
}
func nestedPresent(b []byte, name string, keys ...string) bool {
	var f map[string]json.RawMessage
	if json.Unmarshal(b, &f) != nil {
		return false
	}
	return present(f[name], keys...)
}

// jsonMember selects only code-fixed members after the full strict decode.
func jsonMember(b []byte, names ...string) []byte {
	for _, name := range names {
		var fields map[string]json.RawMessage
		if json.Unmarshal(b, &fields) != nil {
			return nil
		}
		b = fields[name]
	}
	return b
}
func nativeRead(name string, limit int64) ([]byte, error) {
	if !filepath.IsAbs(name) || filepath.Clean(name) != name || len(name) > 512 || strings.ContainsAny(name, "\x00\r\n") {
		return nil, fmt.Errorf("runtime-input-invalid")
	}
	f, err := nativeOpenFile(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > limit {
		return nil, fmt.Errorf("runtime-input-invalid")
	}
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || int64(len(b)) > limit {
		return nil, fmt.Errorf("runtime-input-invalid")
	}
	return b, nil
}

type nativeRetention struct {
	path     string
	root     *os.Root
	identity os.FileInfo
	bytes    int64
	records  int
	refs     []ArtifactRef
}

func newNativeRetention(name string, protected []string) (*nativeRetention, error) {
	if !filepath.IsAbs(name) || filepath.Clean(name) != name || len(name) > 512 || strings.ContainsAny(name, "\x00\r\n") {
		return nil, fmt.Errorf("runtime-output-invalid")
	}
	for _, p := range protected {
		if nativeWithin(p, name) {
			return nil, fmt.Errorf("runtime-output-invalid")
		}
	}
	// Exclusive creation and a descriptor anchor protect existing evidence and
	// avoid following an output directory swapped after its path was checked.
	parent, err := nativeOpenRoot(filepath.Dir(name))
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	if err = parent.Mkdir(filepath.Base(name), 0700); err != nil {
		return nil, err
	}
	anchor, err := nativeOpenSubRoot(parent, filepath.Base(name))
	if err != nil {
		return nil, err
	}
	info, err := anchor.Stat(".")
	if err != nil {
		anchor.Close()
		return nil, err
	}
	return &nativeRetention{path: name, root: anchor, identity: info}, nil
}
func (r *nativeRetention) save(name string, b []byte) (ArtifactRef, error) {
	ref := ArtifactRef{Path: filepath.Join(r.path, name), SHA256: SHA256(b), Bytes: int64(len(b))}
	if r.records >= 32 || r.bytes+ref.Bytes > 16<<20 || ref.Bytes > MaxBytes {
		return ArtifactRef{}, fmt.Errorf("runtime-retention-failed")
	}
	current, err := os.Lstat(r.path)
	if err != nil || !os.SameFile(current, r.identity) || current.Mode()&os.ModeSymlink != 0 {
		return ArtifactRef{}, fmt.Errorf("runtime-retention-failed")
	}
	f, err := nativeOpenInRoot(r.root, name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600, false)
	if err != nil {
		return ArtifactRef{}, err
	}
	n, writeErr := f.Write(b)
	closeErr := f.Close()
	if writeErr != nil || n != len(b) || closeErr != nil {
		return ArtifactRef{}, fmt.Errorf("runtime-retention-failed")
	}
	stored, err := nativeRead(ref.Path, MaxBytes)
	if err != nil || int64(len(stored)) != ref.Bytes || SHA256(stored) != ref.SHA256 {
		return ArtifactRef{}, fmt.Errorf("runtime-retention-failed")
	}
	r.records++
	r.bytes += ref.Bytes
	r.refs = append(r.refs, ref)
	return ref, nil
}

// verify checks the complete retained bundle again at report construction; a
// later native call cannot silently invalidate an earlier observation's bytes.
func (r *nativeRetention) verify() error {
	current, err := os.Lstat(r.path)
	if err != nil || !os.SameFile(current, r.identity) || current.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("runtime-retention-failed")
	}
	for _, ref := range r.refs {
		b, err := nativeRead(ref.Path, MaxBytes)
		if err != nil || int64(len(b)) != ref.Bytes || SHA256(b) != ref.SHA256 {
			return fmt.Errorf("runtime-retention-failed")
		}
	}
	return nil
}
