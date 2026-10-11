package testvaliditydoc

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

const (
	// EvidenceDirectory is the one worktree-relative retained-evidence
	// location discovery reads (LPCV-V0-053, decision 0202).
	EvidenceDirectory = ".corvint/test-evidence"
	// MaxEvidenceEntries bounds the retained-evidence directory listing.
	MaxEvidenceEntries = 256
	// MaxDiscoveryAttempts bounds how many newest candidates are read.
	MaxDiscoveryAttempts = 16
	evidenceAnchorPrefix = "retained-evidence:"
)

// emptyPackageDigest is what the JavaScript provider binds when neither a
// package.json nor a lockfile was named: sha256("=\n=\n").
var emptyPackageDigest = digestHex([]byte("=\n=\n"))

// ReceiptAge provides a stable age origin rather than a clock-dependent duration.
// The local retained file's mtime is not an authenticated execution timestamp.
type ReceiptAge struct {
	Basis string `json:"basis"`
	Since string `json:"since,omitempty"`
}

// Discovery records which retained document was selected and whether its
// bound identity still matches the worktree. It is emitted only by discovery.
type Discovery struct {
	ReceiptAge ReceiptAge         `json:"receiptAge"`
	Location   string             `json:"location"`
	Evidence   string             `json:"evidence,omitempty"`
	Freshness  *testvalidity.Axis `json:"freshness,omitempty"`
	Skipped    int                `json:"skipped"`
	Abstention *Abstention        `json:"abstention,omitempty"`
}

// Abstention explains why discovery yields no evidence that could project a
// passing test (LPCV-V0-056). Reason and Requires are closed; an unattributable
// cause is UNKNOWN, never a guess. Rediscovery alone cannot change any reason:
// Requires names the change that can.
type Abstention struct {
	Reason               string           `json:"reason"`
	Requires             string           `json:"requires"`
	Observed             *ObservedRuntime `json:"observed,omitempty"`
	Producers            []ProducerHint   `json:"producers,omitempty"`
	ExpectedEvidencePath string           `json:"expectedEvidencePath,omitempty"`
	ReceiptRevision      string           `json:"receiptRevision,omitempty"`
}

// ProducerHint is an inert template, not an instruction to execute a provider.
// Qualification is unknown on this host; retained runtime tuples cannot prove it.
type ProducerHint struct {
	Command       string `json:"command"`
	Qualification string `json:"qualification"`
}

// producerAbstention names the opt-in retention entry points without probing
// executables, inspecting arbitrary host paths or guessing a repository language.
func producerAbstention(reason string) *Abstention {
	return &Abstention{
		Reason: reason, Requires: RequiresRetainedProducerRun,
		ExpectedEvidencePath: EvidenceDirectory,
		Producers: []ProducerHint{
			{Command: "corvint-js-test-provider unit --retain --dir <project-directory> --config <vitest-config> --runner-version <vitest-version> --test-file <test-file>", Qualification: "NOT_OBSERVED"},
			{Command: "corvint-js-test-provider e2e --retain --dir <project-directory> --config <playwright-config> --runner-version <playwright-version> --external-server --app-identity <application-identity> --server-ready-url <http-or-https-url> --test-file <test-file>", Qualification: "NOT_OBSERVED"},
			{Command: "corvint-go-test-provider session --retain --experimental --trusted-local --foreground --authority-bundle <bundle-file>", Qualification: "NOT_OBSERVED"},
		},
	}
}

// ObservedRuntime is the retained receipt's own runtime tuple. Platform and
// architecture are not retained by an unqualified receipt, so they are absent.
type ObservedRuntime struct {
	RunnerVersion string `json:"runnerVersion"`
	NodeVersion   string `json:"nodeVersion"`
}

// Closed abstention vocabulary (LPCV-V0-056).
const (
	AbstentionStaleEvidence            = "stale-evidence"
	AbstentionNoRetainedEvidence       = "no-retained-evidence"
	AbstentionRetainedEvidenceUnusable = "retained-evidence-unusable"
	AbstentionRuntimeTupleUnqualified  = "runtime-tuple-unqualified"
	AbstentionUnknown                  = "UNKNOWN"
	RequiresRetainedProducerRun        = "retained-producer-run"
	RequiresQualifiedRuntimeTuple      = "qualified-runtime-tuple"
	// maxObservedVersion bounds each echoed version; a longer retained value
	// is withheld rather than truncated.
	maxObservedVersion = 64
)

// DiscoveryError is a coded discovery refusal shared by the CLI and MCP tool.
type DiscoveryError struct{ Code, Message string }

func (failure *DiscoveryError) Error() string { return failure.Message }

type candidate struct {
	name string
	info fs.FileInfo
}

// Discover projects the newest completed provider document retained under
// EvidenceDirectory of the absolute worktree root. It lists only that one
// directory, reads beyond it only the confined files the document binds,
// never follows a symlink, and writes nothing.
func Discover(root string) (Document, error) {
	worktree, err := os.OpenRoot(root)
	if err != nil {
		return Document{}, locationError("worktree root cannot be opened")
	}
	defer worktree.Close()
	evidence, found, err := openEvidenceDirectory(worktree)
	if err != nil {
		return Document{}, err
	}
	discovery := &Discovery{Location: EvidenceDirectory, ReceiptAge: ReceiptAge{Basis: "NOT_OBSERVED"}}
	if !found {
		discovery.Abstention = producerAbstention(AbstentionNoRetainedEvidence)
		return withDiscovery(Unsupported(), discovery), nil
	}
	defer evidence.Close()
	candidates, skipped, err := listCandidates(evidence)
	if err != nil {
		return Document{}, err
	}
	discovery.Skipped = skipped
	for index, item := range candidates {
		if index == MaxDiscoveryAttempts {
			discovery.Skipped += len(candidates) - index
			break
		}
		input, ok := readCandidate(evidence, item)
		if !ok {
			discovery.Skipped++
			continue
		}
		discovery.Evidence = EvidenceDirectory + "/" + item.name
		binding := bindFreshness(input, func(path, digest string) string {
			return compareBound(worktree, root, path, digest)
		})
		binding.Anchors = []string{evidenceAnchorPrefix + discovery.Evidence}
		discovery.Freshness = &binding
		discovery.ReceiptAge = ReceiptAge{Basis: "retained-file-mtime", Since: item.info.ModTime().UTC().Format(time.RFC3339Nano)}
		discovery.Abstention = runtimeAbstention(input)
		document := JoinStepNegation(applyFreshness(Project(input), binding), input, root)
		revision := receiptRevision(input)
		if head := headRevision(worktree); document.Run.Freshness.State == testvalidity.FreshnessStale ||
			(revision != AbstentionUnknown && head != "" && head != revision) {
			abstention := producerAbstention(AbstentionStaleEvidence)
			abstention.ReceiptRevision = revision
			if discovery.Abstention != nil {
				abstention.Observed = discovery.Abstention.Observed
			}
			discovery.Abstention = abstention
		}
		return withDiscovery(document, discovery), nil
	}
	reason := AbstentionNoRetainedEvidence
	if discovery.Skipped != 0 {
		reason = AbstentionRetainedEvidenceUnusable
	}
	discovery.Abstention = producerAbstention(reason)
	return withDiscovery(Unsupported(), discovery), nil
}

// headRevision resolves HEAD to an object ID by reading .git files inside the
// worktree root, starting no process. A linked worktree (.git is a file), a
// symlink, an oversized file or any unresolved ref yields "" (unknown), which
// never produces a staleness claim.
func headRevision(worktree *os.Root) string {
	head := readSmall(worktree, ".git/HEAD")
	if object := strings.TrimSpace(head); isObjectID(object) {
		return object
	}
	name, ok := strings.CutPrefix(strings.TrimSpace(head), "ref: ")
	if !ok || !strings.HasPrefix(name, "refs/") || strings.Contains(name, "..") || strings.ContainsAny(name, "\\\x00") {
		return ""
	}
	if object := strings.TrimSpace(readSmall(worktree, ".git/"+name)); isObjectID(object) {
		return object
	}
	for _, line := range strings.Split(readSmall(worktree, ".git/packed-refs"), "\n") {
		if object, ref, ok := strings.Cut(line, " "); ok && ref == name && isObjectID(object) {
			return object
		}
	}
	return ""
}

func isObjectID(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	for _, c := range value {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// readSmall reads at most 1 MiB of a regular file confined to the root; any
// failure yields "".
func readSmall(worktree *os.Root, name string) string {
	file, err := worktree.Open(name)
	if err != nil {
		return ""
	}
	defer file.Close()
	if info, err := file.Stat(); err != nil || !info.Mode().IsRegular() {
		return ""
	}
	raw, _ := io.ReadAll(io.LimitReader(file, 1<<20))
	return string(raw)
}

// receiptRevision echoes only the retained test-repository identity at run start.
// The application may belong to another repository. Missing or malformed object
// IDs are unknown; arbitrary receipt strings must not become revision guidance.
func receiptRevision(input Input) string {
	if input.js == nil || input.js.TestRepositoryAtStart == nil {
		return AbstentionUnknown
	}
	revision := input.js.TestRepositoryAtStart.Revision
	if len(revision) != 40 && len(revision) != 64 {
		return AbstentionUnknown
	}
	for _, c := range revision {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return AbstentionUnknown
		}
	}
	return revision
}

// runtimeAbstention reuses the provider's own tuple classification of the
// selected Playwright receipt. A qualified-candidate or non-Playwright input
// carries no abstention: its projections already explain themselves.
func runtimeAbstention(input Input) *Abstention {
	if input.js == nil {
		return nil
	}
	var observed *ObservedRuntime
	if identity := input.js.Identity; observableVersion(identity.RunnerVersion) && observableVersion(identity.NodeVersion) {
		observed = &ObservedRuntime{RunnerVersion: identity.RunnerVersion, NodeVersion: identity.NodeVersion}
	}
	switch jstestprovider.ReceiptRuntimeTuple(*input.js) {
	case jstestprovider.RuntimeTupleUnqualified:
		return &Abstention{Reason: AbstentionRuntimeTupleUnqualified, Requires: RequiresQualifiedRuntimeTuple, Observed: observed}
	case jstestprovider.RuntimeTupleUnobserved:
		return &Abstention{Reason: AbstentionUnknown, Requires: AbstentionUnknown, Observed: observed}
	}
	return nil
}

// observableVersion bounds an echoed version to a short release token; anything
// else is withheld rather than reflected into the document.
func observableVersion(value string) bool {
	if len(value) > maxObservedVersion {
		return false
	}
	for _, c := range value {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '.' || c == '-' || c == '+') {
			return false
		}
	}
	return true
}

func openEvidenceDirectory(worktree *os.Root) (*os.Root, bool, error) {
	parent := worktree
	for _, component := range strings.Split(EvidenceDirectory, "/") {
		if _, err := parent.Lstat(component); errors.Is(err, fs.ErrNotExist) {
			closeUnlessWorktree(parent, worktree)
			return nil, false, nil
		}
		next, err := openReceiptDirectory(parent, component)
		closeUnlessWorktree(parent, worktree)
		if err != nil {
			return nil, false, locationError("retained-evidence location is not a directory inside the worktree")
		}
		parent = next
	}
	return parent, true, nil
}

func closeUnlessWorktree(directory, worktree *os.Root) {
	if directory != worktree {
		directory.Close()
	}
}

// listCandidates returns regular *.json entries newest first (modification
// time, then name). A non-regular *.json entry is counted as skipped.
func listCandidates(evidence *os.Root) ([]candidate, int, error) {
	listing, err := evidence.Open(".")
	if err != nil {
		return nil, 0, locationError("retained-evidence location cannot be listed")
	}
	defer listing.Close()
	entries, err := listing.ReadDir(MaxEvidenceEntries + 1)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, 0, locationError("retained-evidence location cannot be listed")
	}
	if len(entries) > MaxEvidenceEntries {
		return nil, 0, &DiscoveryError{Code: "test-evidence-limit-exceeded", Message: "retained-evidence location holds more than 256 entries"}
	}
	candidates, skipped := []candidate{}, 0
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil || !info.Mode().IsRegular() {
			skipped++
			continue
		}
		candidates = append(candidates, candidate{name: entry.Name(), info: info})
	}
	sort.Slice(candidates, func(left, right int) bool {
		leftTime, rightTime := candidates[left].info.ModTime(), candidates[right].info.ModTime()
		if !leftTime.Equal(rightTime) {
			return leftTime.After(rightTime)
		}
		return candidates[left].name > candidates[right].name
	})
	return candidates, skipped, nil
}

func readCandidate(evidence *os.Root, item candidate) (Input, bool) {
	data, err := readRegular(evidence, item.name, item.info)
	if err != nil {
		return Input{}, false
	}
	input, err := Decode(data)
	return input, err == nil
}

// ProjectBound projects input through the shared builder and states the
// LPCV-V0-053 binding on every freshness axis, comparing each bound path's
// retained digest through compare. compare returns "" for a match,
// retained-digest-mismatch for changed or removed content, and an
// unverifiable reason otherwise. TCN-V0-004 compares against a Git revision
// instead of the worktree.
func ProjectBound(input Input, compare func(path, digest string) string) Document {
	return applyFreshness(Project(input), bindFreshness(input, compare))
}

// BoundPaths lists, sorted, the paths whose digests the input binds.
func BoundPaths(input Input) []string {
	bound := boundDigests(input)
	paths := make([]string, 0, len(bound))
	for path := range bound {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func boundDigests(input Input) map[string]string {
	bound := map[string]string{}
	if input.js == nil {
		return bound
	}
	identity := input.js.Identity
	for path, digest := range identity.ConfigInputDigests {
		bound[path] = digest
	}
	for path, digest := range identity.TestFileDigests {
		bound[path] = digest
	}
	if identity.ConfigFile != "" {
		bound[identity.ConfigFile] = identity.ConfigDigest
	}
	return bound
}

// bindFreshness compares the retained document's bound identity through
// compare. A definite mismatch is STALE; an identity that cannot be
// recomputed is UNKNOWN; only fully matched bound digests are CURRENT.
func bindFreshness(input Input, compare func(path, digest string) string) testvalidity.Axis {
	if input.goSession != nil {
		if input.goSession.State == "stale" {
			return testvalidity.Axis{State: testvalidity.FreshnessStale, Reason: "workspace-execution-identity-mismatch"}
		}
		return testvalidity.Axis{State: testvalidity.FreshnessUnknown, Reason: "retained-session-identity-unverifiable"}
	}
	identity := input.js.Identity
	bound := boundDigests(input)
	if len(bound) == 0 {
		return testvalidity.Axis{State: testvalidity.FreshnessUnknown, Reason: "retained-identity-unbound"}
	}
	unknown := ""
	for _, path := range BoundPaths(input) {
		reason := compare(path, bound[path])
		if reason == "retained-digest-mismatch" {
			return testvalidity.Axis{State: testvalidity.FreshnessStale, Reason: reason}
		}
		if reason != "" && unknown == "" {
			unknown = reason
		}
	}
	if unknown == "" && identity.PackageDigest != "" && identity.PackageDigest != emptyPackageDigest {
		unknown = "retained-package-identity-unverifiable"
	}
	if unknown == "" && input.js.AppBuildAtPublish.Digest != "" {
		unknown = "retained-app-build-identity-unverifiable"
	}
	if unknown == "" && (input.js.Profile == jstestprovider.ExternalProfile || input.js.Profile == jstestprovider.AttestedExternalProfile || input.js.Profile == jstestprovider.SensitiveExternalProfile || input.js.Profile == jstestprovider.AttemptExternalProfile) {
		unknown = "retained-external-app-lifecycle-unverifiable"
	}
	if unknown != "" {
		return testvalidity.Axis{State: testvalidity.FreshnessUnknown, Reason: unknown}
	}
	return testvalidity.Axis{State: testvalidity.FreshnessCurrent}
}

// compareBound returns "" for a matching digest, retained-digest-mismatch for
// changed or removed content, and an unverifiable reason otherwise.
func compareBound(worktree *os.Root, root, path, digest string) string {
	if !filepath.IsAbs(path) {
		return "retained-bound-path-outside-worktree"
	}
	relative, err := filepath.Rel(root, path)
	if err != nil || !filepath.IsLocal(relative) || namesGit(relative) {
		return "retained-bound-path-outside-worktree"
	}
	data, err := ReadFile(worktree, relative)
	if errors.Is(err, fs.ErrNotExist) {
		return "retained-digest-mismatch"
	}
	if err != nil {
		return "retained-bound-source-unreadable"
	}
	if digestHex(data) != digest {
		return "retained-digest-mismatch"
	}
	return ""
}

func namesGit(relative string) bool {
	for _, part := range strings.Split(filepath.ToSlash(relative), "/") {
		if strings.EqualFold(part, ".git") {
			return true
		}
	}
	return false
}

// applyFreshness states the binding on every projection's freshness axis. A
// projection already STALE from its own observation stays STALE.
func applyFreshness(document Document, binding testvalidity.Axis) Document {
	document.Run = withFreshness(document.Run, binding)
	for index := range document.Tests {
		document.Tests[index].Projection = withFreshness(document.Tests[index].Projection, binding)
	}
	return document
}

func withFreshness(projection testvalidity.Projection, binding testvalidity.Axis) testvalidity.Projection {
	if projection.Freshness.State != testvalidity.FreshnessStale {
		projection.Freshness = binding
	}
	return projection
}

func withDiscovery(document Document, discovery *Discovery) Document {
	document.Discovery = discovery
	return document
}

func locationError(message string) error {
	return &DiscoveryError{Code: "invalid-test-evidence-location", Message: message}
}

func digestHex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
