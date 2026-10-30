package witness

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/rubytestprovider"
	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

// MaxEvidenceFiles bounds how many --evidence documents one report reads.
const MaxEvidenceFiles = 8

// evidenceSourceName names every caller-supplied test-run source in the report.
const evidenceSourceName = "evidence"

// maxEvidenceBlobBytes bounds the head blobs one evidence document's file
// digests are compared against (AGW-V0-009). A document whose resolved files
// exceed it cannot be verified and is UNBOUND.
const maxEvidenceBlobBytes = 64 << 20

// readEvidence reads one caller-supplied test-run document and binds it to the
// head tree (AGW-V0-008). A file that cannot be read, exceeds the provider
// bound, or decodes as neither a provider document nor a
// corvint-test-validity/0 document is an error, so the report refuses rather
// than reading an unverifiable input as "no evidence". A decodable document
// that does not prove it ran on exactly the head tree, or whose file digests
// or blob IDs disagree with the head blobs (AGW-V0-009, AGW-V0-010), is
// UNBOUND and contributes no witness. It returns the bound document's tests
// with their files resolved to head-tree paths, and the bound Playwright
// receipt with its resolved paths for paired-range citation (AGW-V0-014).
func readEvidence(ctx context.Context, budget *gitrun.Budget, index *contextindex.Index, file string) (Source, []evidenceTest, *receiptBinding, error) {
	source := Source{Name: evidenceSourceName, Path: file}
	if !filepath.IsAbs(file) || filepath.Clean(file) != file {
		return source, nil, nil, fmt.Errorf("--evidence %s: not a clean absolute path", file)
	}
	data, err := readEvidenceFile(file)
	if err != nil {
		return source, nil, nil, fmt.Errorf("--evidence %s: %w", file, err)
	}
	return bindEvidence(ctx, budget, index, source, data, "--evidence "+file)
}

// readRepositoryEvidence reads one test-run document named by a path relative
// to the checkout root (AGW-V0-011). The name must be clean, relative and free
// of ".." components; it is resolved once through every symbolic link, and a
// resolution that leaves the checkout is refused. The resolved file is then
// read below the resolved root without following links, so a link swapped in
// after resolution is refused rather than followed. Every refusal names its
// reason; none is dropped as "no evidence".
func readRepositoryEvidence(ctx context.Context, budget *gitrun.Budget, index *contextindex.Index, name string) (Source, []evidenceTest, *receiptBinding, error) {
	label := "--evidence-in-repo " + name
	source := Source{Name: evidenceSourceName, Path: name}
	if reason := repositoryEvidenceReason(name); reason != "" {
		return source, nil, nil, fmt.Errorf("%s: %s", label, reason)
	}
	root, err := filepath.EvalSymlinks(index.Root)
	if err != nil {
		return source, nil, nil, fmt.Errorf("%s: the checkout root cannot be resolved: %w", label, err)
	}
	target, err := filepath.EvalSymlinks(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		return source, nil, nil, fmt.Errorf("%s: %w", label, err)
	}
	relative, err := filepath.Rel(root, target)
	if err != nil || !filepath.IsLocal(relative) {
		return source, nil, nil, fmt.Errorf("%s: resolves through a symbolic link to %s, outside the checkout", label, target)
	}
	checkout, err := os.OpenRoot(root)
	if err != nil {
		return source, nil, nil, fmt.Errorf("%s: %w", label, err)
	}
	defer checkout.Close()
	data, err := testvaliditydoc.ReadFile(checkout, relative)
	if err != nil {
		return source, nil, nil, fmt.Errorf("%s: %w", label, err)
	}
	return bindEvidence(ctx, budget, index, source, data, label)
}

// repositoryEvidenceReason names why a caller's repository-relative evidence
// path is not admitted, or "" when it is.
func repositoryEvidenceReason(name string) string {
	slash := filepath.ToSlash(name)
	switch {
	case name == "":
		return "the path is empty"
	case filepath.IsAbs(name) || strings.HasPrefix(slash, "/") || filepath.VolumeName(name) != "":
		return "the path is absolute; give a path relative to the checkout root"
	case slash == ".." || strings.HasPrefix(slash, "../") || strings.Contains(slash, "/../") || strings.HasSuffix(slash, "/.."):
		return "the path leaves the checkout through .."
	case strings.ContainsRune(name, 0) || path.Clean(slash) != slash || slash == ".":
		return "the path is not a clean relative file path"
	case !filepath.IsLocal(filepath.FromSlash(slash)):
		return "the path is not local to the checkout"
	}
	return ""
}

// bindEvidence decodes one read document and binds it to the head tree; label
// prefixes a refusal so it names the caller's input. Only a bound Playwright
// receipt is returned as a receiptBinding; Go and RSpec documents never cite
// across paired ranges (AGW-V0-014).
func bindEvidence(ctx context.Context, budget *gitrun.Budget, index *contextindex.Index, source Source, data []byte, label string) (Source, []evidenceTest, *receiptBinding, error) {
	if run, ok, err := decodeGoRun(data); ok {
		if err != nil {
			return source, nil, nil, fmt.Errorf("%s: %w", label, err)
		}
		source.Status, source.Detail = bindGo(ctx, budget, index, run)
		if source.Status != "BOUND" {
			return source, nil, nil, nil
		}
		source.Hunks = len(run.Tests)
		return source, goTests(run), nil, nil
	}
	receipt, ruby, err := decodeEvidence(data)
	if err != nil {
		return source, nil, nil, fmt.Errorf("%s: %w", label, err)
	}
	if ruby != nil {
		source.Status, source.Detail = bindRuby(ctx, budget, index, ruby)
		if source.Status != "BOUND" {
			return source, nil, nil, nil
		}
		source.Hunks = len(ruby.Examples)
		return source, rubyTests(ruby), nil, nil
	}
	source.Status, source.Detail = bindReceipt(receipt, index.CommitRevision, index.Revision)
	if source.Status != "BOUND" {
		return source, nil, nil, nil
	}
	paths := resolveEvidencePaths(index, receipt)
	if detail := compareDigests(ctx, budget, index, receipt, paths); detail != "" {
		source.Status, source.Detail = "UNBOUND", detail
		return source, nil, nil, nil
	}
	source.Hunks = len(receipt.Tests)
	return source, receiptTests(receipt, paths), &receiptBinding{path: source.Path, receipt: receipt, paths: paths}, nil
}

// readEvidenceFile reads the named document under the provider receipt bound,
// as the test-validity --receipt reader does: the caller-named path is resolved
// once, then the leaf is read below its pinned parent without following links.
func readEvidenceFile(file string) ([]byte, error) {
	resolved, err := filepath.EvalSymlinks(file)
	if err != nil {
		return nil, err
	}
	parent, err := os.OpenRoot(filepath.Dir(resolved))
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	return testvaliditydoc.ReadFile(parent, filepath.Base(resolved))
}

// decodeEvidence accepts a corvint-test-validity/0 document or any provider
// document testvaliditydoc.Decode admits. It returns the Playwright receipt or
// the RSpec provider document the input carries, or neither when the form
// carries no repository identity (a Go session event, or a projection of an
// unprofiled receipt), which binds nothing.
func decodeEvidence(data []byte) (*jstestprovider.Receipt, *rubytestprovider.Document, error) {
	var members map[string]json.RawMessage
	if err := json.Unmarshal(data, &members); err != nil {
		return nil, nil, errors.New("is not a JSON object")
	}
	if _, projected := members["schema"]; projected {
		var document testvaliditydoc.Document
		if err := json.Unmarshal(data, &document); err != nil || document.Schema != testvaliditydoc.Schema {
			return nil, nil, errors.New("is not a " + testvaliditydoc.Schema + " document")
		}
		return document.Playwright, nil, nil
	}
	input, err := testvaliditydoc.Decode(data)
	if err != nil {
		return nil, nil, err
	}
	if ruby, ok := input.RubyDocument(); ok {
		return nil, ruby, nil
	}
	receipt, _ := input.JSReceipt()
	return receipt, nil, nil
}

// bindReceipt admits a receipt only when it records the same clean test
// repository identity at run start and at publish, and that identity is the
// report's head commit and head tree. Every other case names why it is UNBOUND.
func bindReceipt(receipt *jstestprovider.Receipt, head, headTree string) (string, string) {
	if receipt == nil {
		return "UNBOUND", "the document carries no Playwright receipt or RSpec provider document with a test-repository identity, so no test in it can be bound to the head tree"
	}
	start, publish := receipt.TestRepositoryAtStart, receipt.TestRepositoryAtPublish
	switch {
	case start == nil || publish == nil:
		return "UNBOUND", "the receipt does not record the test repository identity at both run start and publish"
	case *start != *publish:
		return "UNBOUND", "the test repository identity changed between run start and publish"
	case start.DirtyState != "clean":
		return "UNBOUND", "the test repository was not clean during the run (dirtyState=" + start.DirtyState + ")"
	case start.Revision != head:
		return "UNBOUND", "the receipt ran at revision " + start.Revision + ", not the report head " + head
	case start.Tree != headTree:
		return "UNBOUND", "the receipt ran at tree " + start.Tree + ", not the report head tree " + headTree
	}
	return "BOUND", "caller-reported test run at the head tree; CALLER_REPORTED never closes an obligation (CF-V0-014)"
}

// evidenceTest is one test outcome a bound document reports, its files
// already resolved to head-tree paths.
type evidenceTest struct {
	name, state, anchor string
	// files are the sorted head-tree paths the test declares itself in or
	// reports reading.
	files []string
}

// receiptTests resolves each Playwright test's anchor and imported files to
// head-tree paths (AGW-V0-009).
func receiptTests(receipt *jstestprovider.Receipt, paths evidencePaths) []evidenceTest {
	tests := make([]evidenceTest, 0, len(receipt.Tests))
	for _, test := range receipt.Tests {
		anchor := ""
		if test.Anchor != nil {
			anchor = paths.resolve(test.Anchor.File)
		}
		candidates := map[string]bool{}
		if anchor != "" {
			candidates[anchor] = true
		}
		for file := range test.ImportedFiles {
			if relative := paths.resolve(file); relative != "" {
				candidates[relative] = true
			}
		}
		tests = append(tests, evidenceTest{name: test.FullName, state: string(test.State), anchor: anchor, files: sortedKeys(candidates)})
	}
	return tests
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// attachEvidence adds one non-closing CALLER_REPORTED witness per (test,
// obligation) pair whose obligation path is a file the test declares itself
// in or reports importing, each path resolved to the head tree (AGW-V0-009).
// It returns how many witnesses it attached and how
// many tests attached none. It never changes a verdict: decide runs after it
// and keeps NOT_RUN sticky.
func attachEvidence(obligations []Obligation, tests []evidenceTest) (int, int) {
	return attachEvidenceAs(obligations, tests, evidenceSourceName)
}

// attachEvidenceAs is attachEvidence under a named source: evidence for a
// document bound to this range, crossRangeSourceName for a paired tests-range
// document cited on an application obligation (AGW-V0-014).
func attachEvidenceAs(obligations []Obligation, tests []evidenceTest, source string) (int, int) {
	byPath := map[string][]int{}
	for position := range obligations {
		byPath[obligations[position].Path] = append(byPath[obligations[position].Path], position)
	}
	attached, unattached := 0, 0
	for _, test := range tests {
		before := attached
		for _, file := range test.files {
			for _, position := range byPath[file] {
				obligations[position].Witnesses = append(obligations[position].Witnesses, Witness{
					Source: source, Relation: "test-run:" + test.state,
					Authority: "CALLER_REPORTED", EvidencePath: test.anchor,
					Closing: closes("CALLER_REPORTED"), Test: test.name,
				})
				attached++
			}
		}
		if attached == before {
			unattached++
		}
	}
	return attached, unattached
}

// repositoryPath keeps only a clean repository-relative path; an absolute or
// escaping path names nothing in the head tree and attaches nothing.
func repositoryPath(file string) string {
	file = filepath.ToSlash(file)
	if file == "" || strings.HasPrefix(file, "/") || path.Clean(file) != file || file == ".." || strings.HasPrefix(file, "../") {
		return ""
	}
	return file
}

// evidencePaths maps a receipt's file names onto head-tree paths (AGW-V0-009).
// The qualified reporter writes absolute paths under the checkout it ran in
// and Playwright names an anchor relative to its test directory, so neither
// is a repository path as written. The zero value keeps only clean relative
// paths, as AGW-V0-008 did.
type evidencePaths struct {
	tracked map[string]struct{}
	// root is the inferred checkout directory, in slash form, that absolute
	// receipt paths are under; hasRoot separates the filesystem root "".
	root    string
	hasRoot bool
	// testDir is the inferred tree directory relative test files are under.
	testDir string
	// absoluteOnly resolves only absolute names, for a paired tests-range
	// receipt read against the application tree (AGW-V0-014).
	absoluteOnly bool
}

// resolve returns the head-tree path a receipt file names, or "" when it
// names none. An absolute path resolves only under the inferred root; a clean
// relative path resolves as written when tracked, then under the inferred
// test directory, and otherwise stays as written (it then matches no
// obligation unless the obligation path is that string).
func (paths evidencePaths) resolve(file string) string {
	slash := filepath.ToSlash(file)
	if evidenceAbsolute(file) {
		if !paths.hasRoot || !strings.HasPrefix(slash, paths.root+"/") {
			return ""
		}
		return repositoryPath(slash[len(paths.root)+1:])
	}
	if paths.absoluteOnly {
		return ""
	}
	relative := repositoryPath(slash)
	if relative == "" || paths.tracks(relative) || paths.testDir == "" {
		return relative
	}
	if joined := path.Join(paths.testDir, relative); paths.tracks(joined) {
		return joined
	}
	return relative
}

func (paths evidencePaths) tracks(file string) bool {
	_, ok := paths.tracked[file]
	return ok
}

func evidenceAbsolute(file string) bool {
	return filepath.IsAbs(file) || strings.HasPrefix(filepath.ToSlash(file), "/")
}

// resolveEvidencePaths infers the receipt's checkout root and test directory
// from the files the receipt says are tests: its digested test files and test
// anchors, which the run read from its own checkout of the head tree. The
// root is the directory under which the most absolute test files name a
// tracked path; the test directory is the tree directory under which the
// most untracked relative test files name a tracked path. A tie that the
// local checkout root does not break leaves the root unknown, and any tie
// leaves the test directory unknown, so an ambiguous receipt attaches less
// rather than to the wrong file.
func resolveEvidencePaths(index *contextindex.Index, receipt *jstestprovider.Receipt) evidencePaths {
	paths := evidencePaths{tracked: index.Tracked}
	tests := map[string]bool{}
	for file := range receipt.Identity.TestFileDigests {
		tests[file] = true
	}
	for _, test := range receipt.Tests {
		if test.Anchor != nil && test.Anchor.File != "" {
			tests[test.Anchor.File] = true
		}
	}
	roots, directories, relative := map[string]int{}, map[string]int{}, map[string]bool{}
	for file := range tests {
		slash := filepath.ToSlash(file)
		if !evidenceAbsolute(file) {
			if clean := repositoryPath(slash); clean != "" && !paths.tracks(clean) {
				relative[clean] = true
			}
			continue
		}
		for position := 0; position < len(slash); position++ {
			if slash[position] == '/' && paths.tracks(slash[position+1:]) {
				roots[slash[:position]]++
			}
		}
	}
	local := filepath.ToSlash(index.Root)
	if resolved, err := filepath.EvalSymlinks(index.Root); err == nil {
		local = filepath.ToSlash(resolved)
	}
	paths.root, paths.hasRoot = mostVoted(roots, local)
	if len(relative) != 0 {
		for tracked := range index.Tracked {
			for position := 0; position < len(tracked); position++ {
				if tracked[position] == '/' && relative[tracked[position+1:]] {
					directories[tracked[:position]]++
				}
			}
		}
		paths.testDir, _ = mostVoted(directories, "")
	}
	return paths
}

// mostVoted returns the key with the most votes; a tie is broken only by
// prefer, and otherwise reports no winner.
func mostVoted(votes map[string]int, prefer string) (string, bool) {
	best, winners := 0, []string{}
	for key, count := range votes {
		switch {
		case count > best:
			best, winners = count, []string{key}
		case count == best:
			winners = append(winners, key)
		}
	}
	if len(winners) == 1 {
		return winners[0], true
	}
	for _, key := range winners {
		if key == prefer && prefer != "" {
			return key, true
		}
	}
	return "", false
}

var lowerSHA256 = regexp.MustCompile(`^[0-9a-f]{64}$`)

// compareDigests checks every file digest the receipt records -- each test's
// imported files and the identity's test-file digests -- whose path resolves
// to a tracked head-tree path against the sha256 of that head blob
// (AGW-V0-009). It returns "" when every such digest matches, and otherwise
// names the first file that is malformed, contradictory, unreadable or
// different, which makes the receipt UNBOUND. A path that resolves to no
// tracked file is not compared: it attaches nothing either.
func compareDigests(ctx context.Context, budget *gitrun.Budget, index *contextindex.Index, receipt *jstestprovider.Receipt, paths evidencePaths) string {
	expected := map[string]string{}
	record := func(file, digest string) string {
		resolved := paths.resolve(file)
		if resolved == "" || !paths.tracks(resolved) {
			return ""
		}
		if !lowerSHA256.MatchString(digest) {
			return "the receipt records digest " + strconv.Quote(digest) + " for head file " + resolved + ", which is not a lowercase hex sha256"
		}
		if prior, seen := expected[resolved]; seen && prior != digest {
			return "the receipt records two different digests for head file " + resolved
		}
		if strings.ContainsAny(resolved, "\n\r") {
			return "head file " + strconv.Quote(resolved) + " cannot be named to git cat-file --batch for comparison"
		}
		expected[resolved] = digest
		return ""
	}
	files := make([]string, 0, len(receipt.Identity.TestFileDigests))
	for file := range receipt.Identity.TestFileDigests {
		files = append(files, file)
	}
	sort.Strings(files)
	for _, file := range files {
		if detail := record(file, receipt.Identity.TestFileDigests[file]); detail != "" {
			return detail
		}
	}
	for _, test := range receipt.Tests {
		imported := make([]string, 0, len(test.ImportedFiles))
		for file := range test.ImportedFiles {
			imported = append(imported, file)
		}
		sort.Strings(imported)
		for _, file := range imported {
			if detail := record(file, test.ImportedFiles[file]); detail != "" {
				return detail
			}
		}
	}
	if len(expected) == 0 {
		return ""
	}
	ordered := make([]string, 0, len(expected))
	var request bytes.Buffer
	for file := range expected {
		ordered = append(ordered, file)
	}
	sort.Strings(ordered)
	for _, file := range ordered {
		request.WriteString(index.Revision + ":" + file + "\n")
	}
	output, err := gitrun.Run(ctx, budget, gitrun.Options{Dir: index.Root, Env: gitEnvironment(), Stdin: request.Bytes(), StdoutLimit: maxEvidenceBlobBytes},
		"-c", "advice.graftFileDeprecated=false", "cat-file", "--batch")
	if err != nil {
		return "the head blobs the receipt's digests name could not be read for comparison: " + err.Error()
	}
	for _, file := range ordered {
		newline := bytes.IndexByte(output, '\n')
		if newline < 0 {
			return "git cat-file --batch returned no header for head file " + file
		}
		fields := strings.Fields(string(output[:newline]))
		output = output[newline+1:]
		if len(fields) != 3 || fields[1] != "blob" {
			return "head file " + file + " is not a readable blob"
		}
		size, err := strconv.Atoi(fields[2])
		if err != nil || size < 0 || size+1 > len(output) {
			return "git cat-file --batch returned a truncated blob for head file " + file
		}
		sum := sha256.Sum256(output[:size])
		output = output[size+1:]
		if hex.EncodeToString(sum[:]) != expected[file] {
			return "the receipt's digest for " + file + " differs from the head blob, so the run did not read the head tree's bytes"
		}
	}
	return ""
}
