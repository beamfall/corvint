package releasegate

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const gitDeadline = 30 * time.Second

// Test code enables low-level Git fixtures on Unix; product scans always use
// the platform containment decision below.
var gitContainmentTestOverride bool

func Scan(ctx context.Context, options Options) (Report, error) {
	if options.Root == "" {
		return Report{}, errors.New("release gate root is required")
	}
	if !gitContainmentSupported() {
		return unsupportedGitReport(), nil
	}
	identity, err := resolveGitExecutable(options.Root, options.GitExecutable, options.GitSHA256)
	if err != nil {
		return Report{}, err
	}
	runner := func(call context.Context, root string, limit int, args ...string) ([]byte, error) {
		return runGitAt(call, identity, root, limit, args...)
	}
	format, err := objectFormat(ctx, options.Root, runner)
	if err != nil {
		return Report{}, err
	}
	if !validCommit(options.Commit, format) {
		return Report{}, errors.New("release gate requires one full lowercase commit ID")
	}
	_, tree, err := resolveTree(ctx, options.Root, options.Commit, runner)
	if err != nil {
		return Report{}, err
	}
	entries, err := readTree(ctx, options.Root, tree, runner)
	if err != nil {
		return Report{}, err
	}
	blobs, err := batchReadBlobs(ctx, identity, options.Root, entries)
	if err != nil {
		return Report{}, err
	}
	options.GitExecutable, options.GitSHA256 = identity.path, identity.sha256
	cached := func(call context.Context, root string, limit int, args ...string) ([]byte, error) {
		if len(args) == 3 && args[0] == "cat-file" && args[1] == "blob" {
			data, ok := blobs[args[2]]
			if !ok {
				return nil, errors.New("unbatched Git blob request")
			}
			if len(data) > limit {
				return nil, errors.New("batched Git blob exceeds requested bound")
			}
			return append([]byte(nil), data...), nil
		}
		return runner(call, root, limit, args...)
	}
	return scan(ctx, options, cached)
}

func scan(parent context.Context, options Options, run gitRun) (Report, error) {
	if options.Root == "" || options.ManifestPath == "" {
		return Report{}, errors.New("release gate root and canonical --manifest path are required")
	}
	if !safePath(options.ManifestPath) {
		return Report{}, errors.New("manifest path is unsafe")
	}
	run = boundedGitRun(run)
	format, err := objectFormat(parent, options.Root, run)
	if err != nil {
		return Report{}, err
	}
	if !validCommit(options.Commit, format) {
		return Report{}, errors.New("release gate requires one full lowercase commit ID")
	}
	commit, tree, err := resolveTree(parent, options.Root, options.Commit, run)
	if err != nil {
		return Report{}, err
	}
	entries, err := readTree(parent, options.Root, tree, run)
	if err != nil {
		return Report{}, err
	}
	byPath := make(map[string]treeEntry, len(entries))
	for _, entry := range entries {
		if _, exists := byPath[entry.path]; exists {
			return Report{}, errors.New("release tree has duplicate path")
		}
		byPath[entry.path] = entry
	}
	manifestEntry, ok := byPath[options.ManifestPath]
	if !ok || manifestEntry.mode != "100644" {
		return Report{}, errors.New("canonical manifest is not a regular file in scanned tree")
	}
	manifestBytes, err := readBlob(parent, options.Root, manifestEntry, run, maxManifestBytes)
	if err != nil {
		return Report{}, err
	}
	manifest, err := decodeManifest(manifestBytes)
	if err != nil {
		return Report{}, fmt.Errorf("invalid canonical manifest: %w", err)
	}
	if err := validateManifest(manifest, byPath, options.ManifestPath); err != nil {
		return Report{}, err
	}
	externalPolicy, err := loadPinnedPolicy(options.Root, options.PolicyPath, options.PolicySHA256)
	if err != nil {
		return Report{}, err
	}
	if !samePinnedPolicy(manifest, externalPolicy) {
		return Report{}, errors.New("release manifest differs from externally pinned exhaustive policy")
	}
	inventoryBlobs := make(map[string][]byte, len(manifest.ArtifactInventory))
	for _, name := range manifest.ArtifactInventory {
		entry := byPath[name]
		data, err := readBlob(parent, options.Root, entry, run, maxBlobBytes)
		if err != nil {
			return Report{}, err
		}
		inventoryBlobs[entry.oid] = data
	}
	inventorySHA, err := artifactInventorySHA256(manifest.ArtifactInventory, byPath, inventoryBlobs)
	if err != nil {
		return Report{}, err
	}
	if inventorySHA != manifest.ArtifactSHA256 {
		return Report{}, errors.New("artifact inventory checksum does not bind scanned blobs")
	}
	for _, allowance := range manifest.Allowances {
		data, err := readBlob(parent, options.Root, byPath[allowance.Path], run, maxBlobBytes)
		if err != nil {
			return Report{}, err
		}
		if sha256Hex(data) != allowance.BlobSHA256 {
			return Report{}, errors.New("allowance bytes do not match canonical digest")
		}
	}
	report := Report{Commit: commit, Tree: tree, ManifestPath: options.ManifestPath, ManifestBlobOID: manifestEntry.oid, ManifestBlobSHA256: sha256Hex(manifestBytes), GitExecutable: options.GitExecutable, GitSHA256: options.GitSHA256, Checks: unrunChecks()}
	allowances := allowanceMap(manifest.Allowances)
	for _, name := range manifest.ArtifactInventory {
		entry := byPath[name]
		findings, err := scanEntry(parent, options.Root, entry, allowances, run)
		if err != nil {
			return Report{}, err
		}
		report.Findings = append(report.Findings, findings...)
	}
	findings, err := modularityFindings(parent, options.Root, entries, manifest, run)
	if err != nil {
		return Report{}, err
	}
	report.Findings = append(report.Findings, findings...)
	_, observedTree, err := resolveTree(parent, options.Root, commit, run)
	if err != nil {
		return Report{}, err
	}
	if observedTree != tree {
		return Report{}, errors.New("release tree changed while scanning")
	}
	sortFindings(report.Findings)
	if len(report.Findings) == 0 {
		report.Checks.Safety = Pass
	} else {
		report.Checks.Safety = Fail
	}
	return report, nil
}

func boundedGitRun(run gitRun) gitRun {
	used := 0
	blobs := map[string][]byte{}
	return func(ctx context.Context, root string, limit int, args ...string) ([]byte, error) {
		if len(args) == 3 && args[0] == "cat-file" && args[1] == "blob" {
			if data, ok := blobs[args[2]]; ok {
				return append([]byte(nil), data...), nil
			}
		}
		data, err := run(ctx, root, limit, args...)
		if err != nil {
			return nil, err
		}
		if len(data) > maxScanBytes-used {
			return nil, errors.New("aggregate Git scan output exceeds bound")
		}
		used += len(data)
		if len(args) == 3 && args[0] == "cat-file" && args[1] == "blob" {
			blobs[args[2]] = append([]byte(nil), data...)
		}
		return data, nil
	}
}

func unrunChecks() Checks {
	return Checks{Parity: NotRun, Safety: NotRun, Performance: NotRun, Packaging: NotRun, CorvintDogfood: NotRun, BeamfallDogfood: NotRun}
}
func objectFormat(ctx context.Context, root string, run gitRun) (string, error) {
	raw, err := run(ctx, root, 128, "rev-parse", "--show-object-format")
	if err != nil {
		return "", err
	}
	f := strings.TrimSpace(string(raw))
	if f != "sha1" && f != "sha256" {
		return "", errors.New("unsupported Git object format")
	}
	return f, nil
}
func validCommit(c, f string) bool {
	n := 40
	if f == "sha256" {
		n = 64
	}
	return regexp.MustCompile(fmt.Sprintf("^[0-9a-f]{%d}$", n)).MatchString(c)
}
func resolveTree(ctx context.Context, root, commit string, run gitRun) (string, string, error) {
	raw, err := run(ctx, root, 256, "rev-parse", commit+"^{commit}", commit+"^{tree}")
	if err != nil {
		return "", "", err
	}
	p := strings.Fields(string(raw))
	if len(p) != 2 || p[0] != commit {
		return "", "", errors.New("Git did not resolve requested exact commit")
	}
	return p[0], p[1], nil
}
func readTree(ctx context.Context, root, tree string, run gitRun) ([]treeEntry, error) {
	raw, err := run(ctx, root, maxTreeBytes, "ls-tree", "-r", "-l", "-z", "--full-tree", tree)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, nil
	}
	if raw[len(raw)-1] != 0 {
		return nil, errors.New("malformed Git tree output")
	}
	raw = raw[:len(raw)-1]
	fields := bytes.Split(raw, []byte{0})
	out := make([]treeEntry, 0, len(fields))
	for _, field := range fields {
		entry, err := parseTreeEntry(field)
		if err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	return out, nil
}
func parseTreeEntry(field []byte) (treeEntry, error) {
	metadata, p, ok := bytes.Cut(field, []byte{'\t'})
	if !ok || !utf8.Valid(p) || !safePath(string(p)) {
		return treeEntry{}, errors.New("Git tree contains unsafe path")
	}
	parts := strings.Fields(string(metadata))
	if len(parts) != 4 || parts[1] != "blob" || !validOID(parts[2]) {
		return treeEntry{}, errors.New("Git tree entry is not a regular blob")
	}
	size, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil || size < 0 {
		return treeEntry{}, errors.New("Git tree entry has invalid size")
	}
	return treeEntry{parts[0], parts[2], size, string(p)}, nil
}
func validOID(v string) bool {
	return regexp.MustCompile("^[0-9a-f]{40}([0-9a-f]{24})?$").MatchString(v)
}
func safePath(v string) bool {
	return v != "" && utf8.ValidString(v) && !strings.ContainsRune(v, 0) && !strings.Contains(v, "\\") && !strings.HasPrefix(v, "/") && !strings.HasSuffix(v, "/") && pathClean(v) == v
}
func pathClean(v string) string {
	parts := strings.Split(v, "/")
	for _, p := range parts {
		if p == "" || p == "." || p == ".." || strings.Contains(p, ":") {
			return ""
		}
	}
	return v
}
func readBlob(ctx context.Context, root string, entry treeEntry, run gitRun, limit int) ([]byte, error) {
	if entry.mode != "100644" && entry.mode != "100755" {
		return nil, errors.New("release tree entry is not regular")
	}
	if entry.size > int64(limit) {
		return nil, errors.New("release tree blob exceeds bound")
	}
	data, err := run(ctx, root, limit, "cat-file", "blob", entry.oid)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) != entry.size {
		return nil, errors.New("Git blob size mismatch")
	}
	return data, nil
}
func scanEntry(ctx context.Context, root string, entry treeEntry, allowances map[string]string, run gitRun) ([]Finding, error) {
	if entry.mode != "100644" && entry.mode != "100755" {
		return []Finding{wholeFinding("unsafe-tree-entry", "release tree entry is not a regular file", Evidence{Path: entry.path, BlobOID: entry.oid})}, nil
	}
	// An oversized blob never reaches this call: every tree entry is the
	// manifest, an inventory member, or an allowance (manifest.go's
	// declared-entry accounting), and each of those is already read
	// through readBlob with this same maxBlobBytes bound before the scan
	// loop calls scanEntry, so an oversized blob has already fail-closed
	// the whole gate with a hard error by this point.
	content, err := readBlob(ctx, root, entry, run, maxBlobBytes)
	if err != nil {
		return nil, err
	}
	e := Evidence{Path: entry.path, BlobOID: entry.oid, BlobSHA256: sha256Hex(content)}
	if digest, ok := allowances[entry.path]; ok && digest == e.BlobSHA256 {
		return nil, nil
	}
	findings := detect(entry.path, content, e)
	if archiveKind(entry.path) != "" || archiveCandidate(content) {
		a, err := scanArchive(entry.path, e, content)
		if err != nil {
			return nil, err
		}
		findings = append(findings, a...)
	}
	return findings, nil
}
func allowanceMap(items []Allowance) map[string]string {
	out := make(map[string]string, len(items))
	for _, a := range items {
		out[a.Path] = a.BlobSHA256
	}
	return out
}
func sha256Hex(v []byte) string { h := sha256.Sum256(v); return hex.EncodeToString(h[:]) }
func sortFindings(v []Finding) {
	sort.Slice(v, func(i, j int) bool {
		a, b := v[i], v[j]
		if a.Evidence.Path != b.Evidence.Path {
			return a.Evidence.Path < b.Evidence.Path
		}
		if a.Evidence.MemberPath != b.Evidence.MemberPath {
			return a.Evidence.MemberPath < b.Evidence.MemberPath
		}
		if a.Evidence.SpanStart != b.Evidence.SpanStart {
			return a.Evidence.SpanStart < b.Evidence.SpanStart
		}
		return a.Kind < b.Kind
	})
}
