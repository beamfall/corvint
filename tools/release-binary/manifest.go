// SPDX-License-Identifier: Apache-2.0
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	entrySchema    = "corvint.release-entry/1"
	manifestSchema = "corvint.release-manifest/1"
	manifestName   = "release-manifest.json"
	entrySuffix    = ".release-entry.json"
	// maximumJSONBytes bounds every JSON document this tool reads.
	maximumJSONBytes = 4 << 20
)

var (
	commitPattern  = regexp.MustCompile(`^[0-9a-f]{40}$`)
	digestPattern  = regexp.MustCompile(`^[0-9a-f]{64}$`)
	versionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$`)
	buildPattern   = regexp.MustCompile(`^[1-9][0-9]*$`)
)

// Entry is one binary's record: what was built, from which commit, and the archive digests.
// `build` writes one per binary; `manifest` merges them (ARTIFACT-BIN-V0-003).
type Entry struct {
	Schema       string         `json:"schema"`
	Name         string         `json:"name"`
	Version      string         `json:"version"`
	SourceCommit string         `json:"sourceCommit"`
	Build        string         `json:"build"`
	GoVersion    string         `json:"goVersion"`
	VersionSet   string         `json:"versionSet,omitempty"`
	Archives     []ArchiveEntry `json:"archives"`
}

type ArchiveEntry struct {
	GOOS         string `json:"goos"`
	GOARCH       string `json:"goarch"`
	Name         string `json:"name"`
	SHA256       string `json:"sha256"`
	Bytes        int64  `json:"bytes"`
	BinarySHA256 string `json:"binarySha256"`
}

// Manifest is the release manifest: one record per released binary, carried forward across
// releases, plus every skew the releasing owner declared (ARTIFACT-BIN-V0-003, -004).
type Manifest struct {
	Schema       string            `json:"schema"`
	Binaries     map[string]Record `json:"binaries"`
	DeclaredSkew []SkewDeclaration `json:"declaredSkew"`
}

type Record struct {
	Version      string         `json:"version"`
	SourceCommit string         `json:"sourceCommit"`
	Build        string         `json:"build"`
	GoVersion    string         `json:"goVersion"`
	VersionSet   string         `json:"versionSet,omitempty"`
	Archives     []ArchiveEntry `json:"archives"`
}

// SkewDeclaration records that the releasing owner knowingly released one version set with its
// members at different identities. Members is the exact `version@commit` of every recorded
// member when the declaration was made; the declaration covers that snapshot only, so a later
// release that moves any member must declare its own skew (ARTIFACT-BIN-V0-004).
type SkewDeclaration struct {
	VersionSet string            `json:"versionSet"`
	Reason     string            `json:"reason"`
	Members    map[string]string `json:"members"`
}

// identity is the version@commit a member is recorded at.
func (r Record) identity() string { return r.Version + "@" + r.SourceCommit }

// snapshot is the recorded members of set and their identities.
func (m Manifest) snapshot(set string) map[string]string {
	members := map[string]string{}
	for name, record := range m.Binaries {
		if record.VersionSet == set {
			members[name] = record.identity()
		}
	}
	return members
}

// covers reports whether the declaration was made for exactly the set's current snapshot.
func (d SkewDeclaration) covers(m Manifest) bool {
	return len(d.Members) > 0 && maps.Equal(d.Members, m.snapshot(d.VersionSet))
}

func describeSnapshot(members map[string]string) string {
	byIdentity := map[string][]string{}
	for name, identity := range members {
		byIdentity[identity] = append(byIdentity[identity], name)
	}
	var parts []string
	for identity, names := range byIdentity {
		sort.Strings(names)
		parts = append(parts, strings.Join(names, ",")+" at "+identity)
	}
	sort.Strings(parts)
	return strings.Join(parts, "; ")
}

func readBoundedJSON(path string, into any) error {
	raw, err := readBoundedJSONBytes(path)
	if err != nil {
		return err
	}
	if err := decodeStrict(raw, into); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func readBoundedJSONBytes(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > maximumJSONBytes {
		return nil, fmt.Errorf("%s: %d bytes exceeds the %d byte bound", path, info.Size(), maximumJSONBytes)
	}
	return os.ReadFile(path)
}

func (e Entry) validate(catalog Catalog) error {
	if e.Schema != entrySchema {
		return fmt.Errorf("entry schema must be %q", entrySchema)
	}
	binary, known := catalog.find(e.Name)
	if !known || binary.Role != "product" {
		return fmt.Errorf("entry names %q, which is not a product binary in the catalog", e.Name)
	}
	if binary.VersionSet != e.VersionSet {
		return fmt.Errorf("entry %s names version set %q; the catalog says %q", e.Name, e.VersionSet, binary.VersionSet)
	}
	if err := (Record{Version: e.Version, SourceCommit: e.SourceCommit, Build: e.Build, GoVersion: e.GoVersion, Archives: e.Archives}).validate(e.Name, catalog); err != nil {
		return err
	}
	return nil
}

func (r Record) validate(name string, catalog Catalog) error {
	if !versionPattern.MatchString(r.Version) {
		return fmt.Errorf("%s: version %q is not MAJOR.MINOR.PATCH[-pre]", name, r.Version)
	}
	if !commitPattern.MatchString(r.SourceCommit) {
		return fmt.Errorf("%s: sourceCommit must be a 40-hex commit", name)
	}
	if !buildPattern.MatchString(r.Build) {
		return fmt.Errorf("%s: build must be a positive first-parent count", name)
	}
	if !strings.HasPrefix(r.GoVersion, "go1.") {
		return fmt.Errorf("%s: goVersion %q is not a Go release", name, r.GoVersion)
	}
	if len(r.Archives) != len(catalog.Targets) {
		return fmt.Errorf("%s: %d archives recorded; the catalog has %d targets", name, len(r.Archives), len(catalog.Targets))
	}
	for index, archive := range r.Archives {
		target := catalog.Targets[index]
		if archive.GOOS != target.GOOS || archive.GOARCH != target.GOARCH {
			return fmt.Errorf("%s: archive row %d is %s/%s; want %s", name, index, archive.GOOS, archive.GOARCH, target.id())
		}
		if archive.Name != archiveName(name, target) {
			return fmt.Errorf("%s: archive row %d is named %q; want %q", name, index, archive.Name, archiveName(name, target))
		}
		if !digestPattern.MatchString(archive.SHA256) || !digestPattern.MatchString(archive.BinarySHA256) || archive.Bytes <= 0 {
			return fmt.Errorf("%s: archive %s needs lowercase sha256 digests and a positive size", name, archive.Name)
		}
	}
	return nil
}

func (m Manifest) validate(catalog Catalog) error {
	if m.Schema != manifestSchema {
		return fmt.Errorf("manifest schema must be %q", manifestSchema)
	}
	if m.Binaries == nil || m.DeclaredSkew == nil {
		return errors.New("manifest must carry binaries and declaredSkew, empty objects allowed")
	}
	for name, record := range m.Binaries {
		binary, known := catalog.find(name)
		if !known || binary.Role != "product" {
			return fmt.Errorf("manifest records %q, which is not a product binary in the catalog", name)
		}
		if record.VersionSet != binary.VersionSet {
			return fmt.Errorf("manifest records %s in version set %q; the catalog says %q", name, record.VersionSet, binary.VersionSet)
		}
		if err := record.validate(name, catalog); err != nil {
			return err
		}
	}
	seenDeclaration := map[string]bool{}
	for _, declaration := range m.DeclaredSkew {
		if _, declared := catalog.VersionSets[declaration.VersionSet]; !declared || strings.TrimSpace(declaration.Reason) == "" {
			return fmt.Errorf("skew declaration for %q needs a declared version set and a reason", declaration.VersionSet)
		}
		if seenDeclaration[declaration.VersionSet] {
			return fmt.Errorf("skew declaration for %q appears twice", declaration.VersionSet)
		}
		seenDeclaration[declaration.VersionSet] = true
		if !declaration.covers(m) {
			return fmt.Errorf("stale-skew-declaration: set %s is declared for %s; the manifest records %s; declare this release's skew with --declare-skew %s=REASON", declaration.VersionSet, describeSnapshot(declaration.Members), describeSnapshot(m.snapshot(declaration.VersionSet)), declaration.VersionSet)
		}
		if !m.skewed(declaration.VersionSet) {
			return fmt.Errorf("skew declaration for %q names a set whose members agree; a declaration lives only while the set disagrees", declaration.VersionSet)
		}
	}
	return m.checkSkew(catalog)
}

// checkSkew is the ARTIFACT-BIN-V0-004 rule: every recorded member of one version set shares
// one version and one source commit unless the manifest declares that set's skew for exactly
// the members and identities it records now.
func (m Manifest) checkSkew(catalog Catalog) error {
	covered := map[string]bool{}
	for _, declaration := range m.DeclaredSkew {
		if declaration.covers(m) {
			covered[declaration.VersionSet] = true
		}
	}
	var problems []string
	for set := range catalog.VersionSets {
		if !m.skewed(set) || covered[set] {
			continue
		}
		problems = append(problems, fmt.Sprintf("version-skew: set %s members disagree: %s; rebuild the set or pass --declare-skew %s=REASON", set, describeSnapshot(m.snapshot(set)), set))
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return errors.New(strings.Join(problems, "\n"))
	}
	return nil
}

// mergeManifest lays the entries over base. A declaration passed to this release is bound to
// the merged snapshot of its set; a declaration carried from base is kept only while the set
// still records exactly the members and identities it was made for, so a declaration lives
// exactly as long as the disagreement it explains and never covers a later one.
func mergeManifest(catalog Catalog, base Manifest, entries []Entry, declarations []SkewDeclaration) Manifest {
	merged := Manifest{Schema: manifestSchema, Binaries: map[string]Record{}, DeclaredSkew: []SkewDeclaration{}}
	for name, record := range base.Binaries {
		merged.Binaries[name] = record
	}
	for _, entry := range entries {
		merged.Binaries[entry.Name] = Record{Version: entry.Version, SourceCommit: entry.SourceCommit, Build: entry.Build, GoVersion: entry.GoVersion, VersionSet: entry.VersionSet, Archives: entry.Archives}
	}
	bySet := map[string]SkewDeclaration{}
	for _, declaration := range base.DeclaredSkew {
		if declaration.covers(merged) && merged.skewed(declaration.VersionSet) {
			bySet[declaration.VersionSet] = declaration
		}
	}
	for _, declaration := range declarations {
		if merged.skewed(declaration.VersionSet) {
			declaration.Members = merged.snapshot(declaration.VersionSet)
			bySet[declaration.VersionSet] = declaration
		}
	}
	for _, declaration := range bySet {
		merged.DeclaredSkew = append(merged.DeclaredSkew, declaration)
	}
	sort.Slice(merged.DeclaredSkew, func(i, j int) bool { return merged.DeclaredSkew[i].VersionSet < merged.DeclaredSkew[j].VersionSet })
	return merged
}

// skewed reports whether the recorded members of set carry more than one version@commit.
func (m Manifest) skewed(set string) bool {
	identities := map[string]bool{}
	for _, identity := range m.snapshot(set) {
		identities[identity] = true
	}
	return len(identities) > 1
}

// loadEntries reads every NAME.release-entry.json in directory, sorted by file name.
func loadEntries(directory string, catalog Catalog) ([]Entry, error) {
	matches, err := filepath.Glob(filepath.Join(directory, "*"+entrySuffix))
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	var entries []Entry
	for _, path := range matches {
		var entry Entry
		if err := readBoundedJSON(path, &entry); err != nil {
			return nil, err
		}
		if err := entry.validate(catalog); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		// One file per binary: the file name is the binary's, so a second entry for one binary
		// can only arrive under a mismatched name, which refuses here.
		if filepath.Base(path) != entry.Name+entrySuffix {
			return nil, fmt.Errorf("entry-name-mismatch: %s records binary %q; the file name must be %s", path, entry.Name, entry.Name+entrySuffix)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func canonicalJSON(value any) ([]byte, error) {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

func digestOf(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// outputs retains one run's files under directory without ever replacing an existing file
// (ARTIFACT-BIN-V0-007): every path is checked for absence before the first write, each file
// is written to a temporary sibling and renamed into place, and discard removes what the run
// wrote so a failure leaves no partial output.
type outputs struct {
	directory string
	written   []string
}

// reserve refuses `output-exists` for any named output already present.
func (o *outputs) reserve(names ...string) error {
	for _, name := range names {
		if _, err := os.Lstat(filepath.Join(o.directory, name)); err == nil {
			return fmt.Errorf("output-exists: %s", filepath.Join(o.directory, name))
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func (o *outputs) write(name string, raw []byte, mode os.FileMode) error {
	path := filepath.Join(o.directory, name)
	temporary, err := os.CreateTemp(o.directory, "."+name+".tmp-*")
	if err != nil {
		return err
	}
	fail := func(err error) error {
		temporary.Close()
		os.Remove(temporary.Name())
		return err
	}
	if _, err := temporary.Write(raw); err != nil {
		return fail(err)
	}
	if err := temporary.Chmod(mode); err != nil {
		return fail(err)
	}
	if err := temporary.Close(); err != nil {
		return fail(err)
	}
	if _, err := os.Lstat(path); err == nil {
		return fail(fmt.Errorf("output-exists: %s", path))
	}
	if err := os.Rename(temporary.Name(), path); err != nil {
		return fail(err)
	}
	o.written = append(o.written, path)
	return nil
}

// discard removes every file this run wrote, newest first.
func (o *outputs) discard() {
	for index := len(o.written) - 1; index >= 0; index-- {
		os.Remove(o.written[index])
	}
	o.written = nil
}

// refuseOutputInsideRepository is the ARTIFACT-BIN-V0-007 rule that release output lives
// outside the repository: a DIR at or under root refuses `output-inside-repository`.
func refuseOutputInsideRepository(root, output string) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	output, err = filepath.Abs(output)
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(output); err == nil {
		output = resolved
	}
	relative, err := filepath.Rel(root, output)
	if err != nil {
		return nil
	}
	if relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))) {
		return fmt.Errorf("output-inside-repository: %s is under the repository %s; write release output outside it", output, root)
	}
	return nil
}

// verifyOutput rechecks directory against the manifest and repairs nothing: the digest file
// must bind the manifest bytes, every archive present must be recorded, every recorded binary
// with archives present must have all of them with the recorded size and SHA-256 and a
// NAME.SHA256SUMS that lists them, and at least one binary must have been rechecked.
func verifyOutput(manifest Manifest, manifestRaw []byte, directory string) ([]string, error) {
	digest, err := readBoundedFile(filepath.Join(directory, manifestName+".sha256"), int64(len(digestOf(manifestRaw))+len("  "+manifestName+"\n")))
	if err != nil {
		return nil, fmt.Errorf("manifest-digest-mismatch: %s: %w", manifestName+".sha256", err)
	}
	if string(digest) != digestOf(manifestRaw)+"  "+manifestName+"\n" {
		return nil, fmt.Errorf("manifest-digest-mismatch: %s does not bind the verified manifest", manifestName+".sha256")
	}
	recorded := map[string]bool{}
	names := make([]string, 0, len(manifest.Binaries))
	for name, record := range manifest.Binaries {
		names = append(names, name)
		for _, archive := range record.Archives {
			recorded[archive.Name] = true
		}
	}
	sort.Strings(names)
	var problems []string
	for _, pattern := range []string{"*.tar.gz", "*.zip"} {
		matches, err := filepath.Glob(filepath.Join(directory, pattern))
		if err != nil {
			return nil, err
		}
		for _, match := range matches {
			if !recorded[filepath.Base(match)] {
				problems = append(problems, filepath.Base(match)+": not recorded in the manifest")
			}
		}
	}
	var rechecked []string
	for _, name := range names {
		record := manifest.Binaries[name]
		present := 0
		for _, archive := range record.Archives {
			if _, err := os.Lstat(filepath.Join(directory, archive.Name)); err == nil {
				present++
			}
		}
		if present == 0 {
			continue
		}
		if present != len(record.Archives) {
			problems = append(problems, fmt.Sprintf("%s: %d of %d archives present", name, present, len(record.Archives)))
			continue
		}
		var checksums strings.Builder
		for _, archive := range record.Archives {
			checksums.WriteString(archive.SHA256 + "  " + archive.Name + "\n")
			raw, err := readBoundedFile(filepath.Join(directory, archive.Name), archive.Bytes)
			if err != nil {
				problems = append(problems, archive.Name+": "+err.Error())
				continue
			}
			if digestOf(raw) != archive.SHA256 {
				problems = append(problems, archive.Name+": sha256 differs from the manifest")
			}
		}
		sums, err := readBoundedFile(filepath.Join(directory, name+".SHA256SUMS"), int64(checksums.Len()))
		if err != nil {
			problems = append(problems, name+".SHA256SUMS: "+err.Error())
		} else if string(sums) != checksums.String() {
			problems = append(problems, name+".SHA256SUMS: rows differ from the manifest")
		}
		rechecked = append(rechecked, name)
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return nil, errors.New("archive-mismatch: " + strings.Join(problems, "; "))
	}
	if len(rechecked) == 0 {
		return nil, fmt.Errorf("nothing-rechecked: %s holds no archive the manifest records", directory)
	}
	return rechecked, nil
}

func readBoundedFile(path string, limit int64) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() != limit {
		return nil, fmt.Errorf("%d bytes on disk, %d recorded", info.Size(), limit)
	}
	return os.ReadFile(path)
}
