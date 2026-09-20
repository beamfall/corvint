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

// Discovery records which retained document was selected and whether its
// bound identity still matches the worktree. It is emitted only by discovery.
type Discovery struct {
	Location  string             `json:"location"`
	Evidence  string             `json:"evidence,omitempty"`
	Freshness *testvalidity.Axis `json:"freshness,omitempty"`
	Skipped   int                `json:"skipped"`
}

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
	discovery := &Discovery{Location: EvidenceDirectory}
	if !found {
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
		binding := bindFreshness(worktree, root, input)
		binding.Anchors = []string{evidenceAnchorPrefix + discovery.Evidence}
		discovery.Freshness = &binding
		return withDiscovery(applyFreshness(Project(input), binding), discovery), nil
	}
	return withDiscovery(Unsupported(), discovery), nil
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

// bindFreshness compares the retained document's bound identity with the
// worktree now. A definite mismatch is STALE; an identity that cannot be
// recomputed is UNKNOWN; only fully matched bound digests are CURRENT.
func bindFreshness(worktree *os.Root, root string, input Input) testvalidity.Axis {
	if input.goSession != nil {
		if input.goSession.State == "stale" {
			return testvalidity.Axis{State: testvalidity.FreshnessStale, Reason: "workspace-execution-identity-mismatch"}
		}
		return testvalidity.Axis{State: testvalidity.FreshnessUnknown, Reason: "retained-session-identity-unverifiable"}
	}
	identity := input.js.Identity
	bound := map[string]string{}
	for path, digest := range identity.ConfigInputDigests {
		bound[path] = digest
	}
	for path, digest := range identity.TestFileDigests {
		bound[path] = digest
	}
	if identity.ConfigFile != "" {
		bound[identity.ConfigFile] = identity.ConfigDigest
	}
	if len(bound) == 0 {
		return testvalidity.Axis{State: testvalidity.FreshnessUnknown, Reason: "retained-identity-unbound"}
	}
	unknown := ""
	paths := make([]string, 0, len(bound))
	for path := range bound {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		reason := compareBound(worktree, root, path, bound[path])
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
	if unknown == "" && (input.js.Profile == jstestprovider.ExternalProfile || input.js.Profile == jstestprovider.AttestedExternalProfile) {
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
