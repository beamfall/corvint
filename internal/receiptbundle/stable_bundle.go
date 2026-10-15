package receiptbundle

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/workflow"
)

// Stable bundle layout and refusal codes (CEM-V1-060..064). A stable bundle is
// a directory holding manifest.json, the exact cem/1.0 map bytes as cem.json and
// each declared role artifact at artifacts/<declared path>, so the directory can
// move anywhere and still verify with `cem verify-stable --map <dir>/cem.json
// --artifacts <dir>/artifacts`.
const (
	StableProfile     = "corvint-cem-stable-bundle/1"
	StableMapName     = "cem.json"
	StableArtifactDir = "artifacts"

	CodeStableEntryMissing  = "stable-bundle-entry-missing"  // a declared file is absent
	CodeStableEntryUnused   = "stable-bundle-entry-unused"   // a file or directory nothing declares
	CodeStableEntryEscaping = "stable-bundle-entry-escaping" // a symlink or other non-regular, non-directory entry
	CodeStableEntryDigest   = "stable-bundle-entry-digest"   // bytes differ from the declared digest or length
	CodeStableManifest      = "stable-bundle-manifest"       // manifest not exact, or disagrees with the map
	CodeStableRevision      = "stable-bundle-revision"       // recorded base or target differs from the caller's
)

// maxStableEntries bounds the bundle walk: the map's 1024 artifacts, their
// directories and the fixed entries.
const maxStableEntries = 4096

// StableArtifact is one role artifact a stable bundle carries as exact bytes.
type StableArtifact struct {
	Kind, Path, Sha256 string
	data               []byte
}

// Bytes returns a copy of the artifact's exact bytes.
func (a StableArtifact) Bytes() []byte { return bytes.Clone(a.data) }

// StableBundle is a complete cem/1.0 bundle in memory: the exact map bytes, the
// base and target it was verified against, and every declared artifact in map
// order. Only CollectStable and ReadStable construct one.
type StableBundle struct {
	Base, Target string
	Map          []byte
	Artifacts    []StableArtifact
}

type stableFile struct {
	Kind       string `json:"kind,omitempty"`
	Path       string `json:"path"`
	Sha256     string `json:"sha256"`
	ByteLength int    `json:"byteLength"`
}

type stableManifest struct {
	Profile   string       `json:"profile"`
	Spec      string       `json:"spec"`
	Base      string       `json:"base"`
	Target    string       `json:"target"`
	Map       stableFile   `json:"map"`
	Artifacts []stableFile `json:"artifacts"`
}

// Manifest renders the bundle's canonical manifest: one JSON object and one LF.
// Artifact paths are the map's declared paths, relative to artifacts/.
func (b *StableBundle) Manifest() []byte {
	m := stableManifest{
		Profile: StableProfile, Spec: wire.StableSpec, Base: b.Base, Target: b.Target,
		Map:       stableFile{Path: StableMapName, Sha256: digest(b.Map), ByteLength: len(b.Map)},
		Artifacts: make([]stableFile, 0, len(b.Artifacts)),
	}
	for _, a := range b.Artifacts {
		m.Artifacts = append(m.Artifacts, stableFile{a.Kind, a.Path, a.Sha256, len(a.data)})
	}
	encoded, _ := json.Marshal(m)
	return append(encoded, '\n')
}

func stableRefusal(code, format string, args ...any) error { return cemcode.New(code, format, args...) }

// CollectStable reads every artifact the cem/1.0 map declares from
// artifactRoot, by its declared path inside that root, and refuses one whose
// bytes no longer match the declared digest. It reads nothing else.
func CollectStable(mapBytes []byte, base, target, artifactRoot string) (*StableBundle, error) {
	document, err := wire.ParseStable(mapBytes)
	if err != nil {
		return nil, err
	}
	bundle := &StableBundle{Base: base, Target: target, Map: bytes.Clone(mapBytes)}
	root, err := os.OpenRoot(artifactRoot)
	if err != nil {
		return nil, stableRefusal("artifact-unavailable", "artifact root cannot be opened")
	}
	defer root.Close()
	total := 0
	for _, declared := range document.Artifacts {
		data, err := readRootFile(root, declared.Path, wire.CandidateMaxArtifactBytes)
		if err != nil {
			return nil, stableRefusal("artifact-unavailable", "artifact %s is not a readable regular file", declared.Path)
		}
		if total += len(data); total > wire.CandidateMaxTotalArtifactBytes {
			return nil, stableRefusal("artifact-resource-limit", "artifact aggregate")
		}
		if digest(data) != declared.Sha256 {
			return nil, stableRefusal("artifact-digest-mismatch", "artifact %s changed after verification", declared.Path)
		}
		bundle.Artifacts = append(bundle.Artifacts, StableArtifact{declared.Kind, declared.Path, declared.Sha256, data})
	}
	return bundle, nil
}

// WriteStable creates the bundle as the new directory name under parent and
// writes every file exclusively inside that directory, after checking that the
// bundle is exactly the map's declared closure; a failed write removes only
// what it wrote. The manifest is written last.
func WriteStable(parent *os.Root, name string, b *StableBundle) error {
	out, err := writeStable(parent, name, b)
	if err == nil {
		out.dir.Close()
	}
	return err
}

// stableWriteHook lets tests change the output between the bundle directory's
// creation and its identity check ("mkdir"), before its open ("open"), and
// after each file is written (its path); it is nil outside tests.
var stableWriteHook func(stage string)

func callStableWriteHook(stage string) {
	if stableWriteHook != nil {
		stableWriteHook(stage)
	}
}

// stableOutput is a written bundle: its directory, opened as a root bound to
// the identity Mkdir created, which stays the anchor for any later cleanup.
type stableOutput struct {
	parent  *os.Root
	name    string
	created os.FileInfo
	dir     *os.Root
	bundle  *StableBundle
}

// writeStable is WriteStable that keeps the written directory open for the
// caller, which must call close or remove. Nothing is written unless the
// opened root is the same real directory the name held right after Mkdir, the
// name still holds it, and it is empty; otherwise nothing is removed either.
func writeStable(parent *os.Root, name string, b *StableBundle) (*stableOutput, error) {
	if err := b.checkClosure(); err != nil {
		return nil, err
	}
	if err := parent.Mkdir(name, 0o700); err != nil {
		return nil, cemcode.New(cemcode.PublishFailed, "bundle directory cannot be created")
	}
	callStableWriteHook("mkdir")
	created, err := parent.Lstat(name)
	if err != nil || !created.IsDir() {
		return nil, cemcode.New(cemcode.PublishFailed, "bundle directory changed after it was created")
	}
	callStableWriteHook("open")
	dir, err := parent.OpenRoot(name)
	if err != nil {
		return nil, cemcode.New(cemcode.PublishFailed, "bundle directory cannot be opened")
	}
	out := &stableOutput{parent: parent, name: name, created: created, dir: dir, bundle: b}
	opened, statErr := dir.Stat(".")
	named, lstatErr := parent.Lstat(name)
	if statErr != nil || lstatErr != nil || !named.IsDir() || !os.SameFile(created, opened) || !os.SameFile(created, named) {
		dir.Close()
		return nil, cemcode.New(cemcode.PublishFailed, "bundle directory changed after it was created")
	}
	if !emptyRoot(dir) {
		dir.Close()
		return nil, cemcode.New(cemcode.PublishFailed, "bundle directory is not empty")
	}
	if err := out.write(); err != nil {
		out.remove()
		return nil, err
	}
	return out, nil
}

func (o *stableOutput) write() error {
	if err := o.dir.Mkdir(StableArtifactDir, 0o700); err != nil {
		return cemcode.New(cemcode.PublishFailed, "bundle artifacts directory cannot be created")
	}
	if err := writeFile(o.dir, StableMapName, o.bundle.Map); err != nil {
		return err
	}
	callStableWriteHook(StableMapName)
	for _, a := range o.bundle.Artifacts {
		destination := filepath.Join(StableArtifactDir, filepath.FromSlash(a.Path))
		if err := o.dir.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
			return cemcode.New(cemcode.PublishFailed, "bundle artifact directory cannot be created")
		}
		if err := writeFile(o.dir, destination, a.data); err != nil {
			return err
		}
	}
	return writeFile(o.dir, ManifestName, o.bundle.Manifest())
}

// remove deletes the bundle's declared files and then its declared
// directories, deepest first, through the root opened on the created
// directory wherever that directory now is, never recursively; it then
// removes name, non-recursively, only while name still holds that directory.
// An entry nothing declares keeps the directory in place.
func (o *stableOutput) remove() {
	files := []string{ManifestName, StableMapName}
	dirs := map[string]bool{StableArtifactDir: true}
	for _, a := range o.bundle.Artifacts {
		destination := filepath.Join(StableArtifactDir, filepath.FromSlash(a.Path))
		files = append(files, destination)
		for d := filepath.Dir(destination); d != StableArtifactDir && d != "."; d = filepath.Dir(d) {
			dirs[d] = true
		}
	}
	for _, f := range files {
		o.dir.Remove(f)
	}
	ordered := slices.SortedFunc(maps.Keys(dirs), func(a, b string) int {
		return strings.Count(b, string(filepath.Separator)) - strings.Count(a, string(filepath.Separator))
	})
	for _, d := range ordered {
		o.dir.Remove(d)
	}
	o.dir.Close()
	if named, err := o.parent.Lstat(o.name); err == nil && named.IsDir() && os.SameFile(o.created, named) {
		o.parent.Remove(o.name)
	}
}

// emptyRoot reports whether the root directory holds no entry.
func emptyRoot(dir *os.Root) bool {
	top, err := dir.Open(".")
	if err != nil {
		return false
	}
	defer top.Close()
	names, err := top.Readdirnames(1)
	return len(names) == 0 && errors.Is(err, io.EOF)
}

// checkClosure refuses a bundle whose base, artifacts or bytes are not exactly
// what its map declares, so no altered path or role is ever written.
func (b *StableBundle) checkClosure() error {
	document, err := wire.ParseStable(b.Map)
	if err != nil {
		return err
	}
	if b.Base != document.Change.BaseRevision || len(b.Artifacts) != len(document.Artifacts) {
		return stableRefusal(CodeStableManifest, "bundle disagrees with its map")
	}
	total := 0
	for i, declared := range document.Artifacts {
		a := b.Artifacts[i]
		if a.Kind != declared.Kind || a.Path != declared.Path || a.Sha256 != declared.Sha256 {
			return stableRefusal(CodeStableManifest, "bundle artifact %d disagrees with its map", i)
		}
		if total += len(a.data); len(a.data) > wire.CandidateMaxArtifactBytes || total > wire.CandidateMaxTotalArtifactBytes {
			return stableRefusal("artifact-resource-limit", "artifact aggregate")
		}
		if digest(a.data) != declared.Sha256 {
			return stableRefusal(CodeStableEntryDigest, "bundle artifact %s does not match its declared digest", declared.Path)
		}
	}
	return nil
}

// ReadStable reads the bundle directory dir and admits it only when its
// manifest is exact, the map is a valid cem/1.0 document whose base and
// artifact declarations the manifest repeats in order, every declared file is
// a regular file with the declared bytes, and no other entry exists. A missing,
// unused or escaping entry is refused; nothing is followed out of dir.
func ReadStable(dir string) (*StableBundle, error) {
	if !filepath.IsAbs(dir) || filepath.Clean(dir) != dir {
		return nil, stableRefusal(cemcode.InvalidArguments, "bundle directory must be an absolute clean path")
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() {
		return nil, stableRefusal(CodeStableEntryMissing, "bundle directory is not a directory")
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, stableRefusal(CodeStableEntryMissing, "bundle directory cannot be opened")
	}
	defer root.Close()
	if opened, err := root.Stat("."); err != nil || !os.SameFile(info, opened) {
		return nil, stableRefusal(CodeStableEntryEscaping, "bundle directory changed while opened")
	}
	files, dirs, err := walkStable(root)
	if err != nil {
		return nil, err
	}
	raw, err := readDeclared(root, files, ManifestName, MaxReceiptBytes)
	if err != nil {
		return nil, err
	}
	var manifest stableManifest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&manifest) != nil || manifest.Profile != StableProfile || manifest.Spec != wire.StableSpec ||
		manifest.Map.Path != StableMapName || manifest.Map.Kind != "" {
		return nil, stableRefusal(CodeStableManifest, "manifest is not a %s manifest", StableProfile)
	}
	mapBytes, err := readDeclared(root, files, StableMapName, wire.MaxMapBytes)
	if err != nil {
		return nil, err
	}
	if digest(mapBytes) != manifest.Map.Sha256 || len(mapBytes) != manifest.Map.ByteLength {
		return nil, stableRefusal(CodeStableEntryDigest, "%s does not match the manifest", StableMapName)
	}
	document, err := wire.ParseStable(mapBytes)
	if err != nil {
		return nil, err
	}
	if manifest.Base != document.Change.BaseRevision || len(manifest.Artifacts) != len(document.Artifacts) {
		return nil, stableRefusal(CodeStableManifest, "manifest disagrees with the map")
	}
	bundle := &StableBundle{Base: manifest.Base, Target: manifest.Target, Map: mapBytes}
	declared := map[string]bool{ManifestName: true, StableMapName: true}
	type readFile struct {
		sha256 string
		bound  int
	}
	readFiles := map[string]readFile{ManifestName: {digest(raw), MaxReceiptBytes}, StableMapName: {manifest.Map.Sha256, wire.MaxMapBytes}}
	total := 0
	for i, a := range document.Artifacts {
		entry := manifest.Artifacts[i]
		if entry.Kind != a.Kind || entry.Path != a.Path || entry.Sha256 != a.Sha256 {
			return nil, stableRefusal(CodeStableManifest, "manifest artifact %d disagrees with the map", i)
		}
		name := StableArtifactDir + "/" + a.Path
		data, err := readDeclared(root, files, name, wire.CandidateMaxArtifactBytes)
		if err != nil {
			return nil, err
		}
		if total += len(data); total > wire.CandidateMaxTotalArtifactBytes {
			return nil, stableRefusal("artifact-resource-limit", "artifact aggregate")
		}
		if digest(data) != a.Sha256 || len(data) != entry.ByteLength {
			return nil, stableRefusal(CodeStableEntryDigest, "%s does not match its declared digest", name)
		}
		declared[name] = true
		readFiles[name] = readFile{a.Sha256, wire.CandidateMaxArtifactBytes}
		bundle.Artifacts = append(bundle.Artifacts, StableArtifact{a.Kind, a.Path, a.Sha256, data})
	}
	if !bytes.Equal(bundle.Manifest(), raw) {
		return nil, stableRefusal(CodeStableManifest, "manifest is not in canonical form")
	}
	if err := unusedEntries(files, dirs, declared); err != nil {
		return nil, err
	}
	// Every declared file is read again, so one rewritten after its own read
	// is refused too; the state each re-read saw is kept for the final walk.
	reread := map[string]os.FileInfo{}
	for name, want := range readFiles {
		data, state, err := readRootFileState(root, filepath.FromSlash(name), want.bound)
		if err != nil || digest(data) != want.sha256 {
			return nil, stableRefusal(CodeStableEntryEscaping, "bundle changed while it was read")
		}
		reread[name] = state
	}
	// A second walk after every read, including the re-reads, refuses a tree
	// that changed meanwhile: an entry or an ancestor directory swapped for a
	// symlink, an undeclared entry, or a directory replaced by another one.
	filesAfter, dirsAfter, err := walkStable(root)
	if err != nil {
		return nil, err
	}
	if len(files) != len(filesAfter) || !maps.EqualFunc(dirs, dirsAfter, os.SameFile) {
		return nil, stableRefusal(CodeStableEntryEscaping, "bundle changed while it was read")
	}
	// Each file must still be the one its re-read saw, with the same size,
	// modification time and inode change time, so a file rewritten in place
	// after its re-read is refused even when its modification time is restored.
	for name, after := range filesAfter {
		if seen, ok := reread[name]; !ok || !sameFileState(seen, after) {
			return nil, stableRefusal(CodeStableEntryEscaping, "bundle changed while it was read")
		}
	}
	// The caller's path must still name the opened directory itself, not a
	// symlink to it or to a moved copy.
	if named, err := os.Lstat(dir); err != nil || !named.IsDir() || !os.SameFile(info, named) {
		return nil, stableRefusal(CodeStableEntryEscaping, "bundle changed while it was read")
	}
	return bundle, nil
}

// sameFileState reports whether two states of a file have the same identity,
// size, modification time and change time; a missing change time is unequal.
func sameFileState(a, b os.FileInfo) bool {
	aChange, aOK := changeTime(a)
	bChange, bOK := changeTime(b)
	return os.SameFile(a, b) && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime()) &&
		aOK && bOK && aChange.Equal(bChange)
}

// readStableProbe reads the map only to select the export route, with the
// same bound as readRegular but without blocking on a substituted FIFO.
func readStableProbe(dir, name string) ([]byte, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return readRootFile(root, name, MaxReceiptBytes)
}

// walkStable lists every regular file and directory under the bundle root by
// slash path, with each entry's Lstat state, reading each directory
// incrementally so an oversized tree is refused before it is held in memory,
// and refusing any other entry kind.
func walkStable(root *os.Root) (map[string]os.FileInfo, map[string]os.FileInfo, error) {
	files, dirs := map[string]os.FileInfo{}, map[string]os.FileInfo{".": nil}
	pending := []string{"."}
	for len(pending) > 0 {
		dir := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		entries, info, err := readStableDir(root, dir, maxStableEntries-len(files)-len(dirs))
		if err != nil {
			return nil, nil, err
		}
		dirs[dir] = info
		for _, entry := range entries {
			name := path.Join(dir, entry.Name())
			switch {
			case entry.Type().IsRegular():
				info, err := entry.Info()
				if err != nil || !info.Mode().IsRegular() {
					return nil, nil, stableRefusal(CodeStableEntryEscaping, "bundle entry %s changed while it was listed", name)
				}
				files[name] = info
			case entry.IsDir():
				dirs[name] = nil
				pending = append(pending, name)
			default:
				return nil, nil, stableRefusal(CodeStableEntryEscaping, "bundle entry %s is neither a regular file nor a directory", name)
			}
		}
	}
	return files, dirs, nil
}

// readStableDir reads one bundle directory in bounded batches, refusing more
// than room entries and a directory replaced by any other entry kind, and
// returns the directory's identity with its entries.
func readStableDir(root *os.Root, dir string, room int) ([]fs.DirEntry, os.FileInfo, error) {
	name := filepath.FromSlash(dir)
	before, err := root.Lstat(name)
	if err != nil || !before.IsDir() {
		return nil, nil, stableRefusal(CodeStableEntryEscaping, "bundle entry %s is not a directory", dir)
	}
	file, err := openStableEntry(root, name)
	if err != nil {
		return nil, nil, stableRefusal(CodeStableEntryEscaping, "bundle directory %s cannot be opened", dir)
	}
	defer file.Close()
	if opened, err := file.Stat(); err != nil || !opened.IsDir() || !os.SameFile(before, opened) {
		return nil, nil, stableRefusal(CodeStableEntryEscaping, "bundle directory %s changed while opened", dir)
	}
	var entries []fs.DirEntry
	for {
		batch, err := file.ReadDir(64)
		if entries = append(entries, batch...); len(entries) > room {
			return nil, nil, stableRefusal(CodeStableEntryUnused, "bundle holds more than %d entries", maxStableEntries)
		}
		if errors.Is(err, io.EOF) {
			return entries, before, nil
		}
		if err != nil {
			return nil, nil, stableRefusal(CodeStableEntryEscaping, "bundle directory %s cannot be read", dir)
		}
	}
}

// readDeclared reads one declared file the walk listed; a declared file the
// walk did not list as a regular file is missing.
func readDeclared(root *os.Root, files map[string]os.FileInfo, name string, bound int) ([]byte, error) {
	if _, ok := files[name]; !ok {
		return nil, stableRefusal(CodeStableEntryMissing, "bundle has no regular file %s", name)
	}
	data, err := readRootFile(root, filepath.FromSlash(name), bound)
	if err != nil {
		return nil, stableRefusal(CodeStableEntryEscaping, "bundle file %s is not a bounded regular file", name)
	}
	return data, nil
}

// unusedEntries refuses every file nothing declares and every directory that
// is not "." or an ancestor of a declared file.
func unusedEntries(files, dirs map[string]os.FileInfo, declared map[string]bool) error {
	needed := map[string]bool{".": true, StableArtifactDir: true}
	for name := range declared {
		for dir := path.Dir(name); dir != "."; dir = path.Dir(dir) {
			needed[dir] = true
		}
	}
	for name := range files {
		if !declared[name] {
			return stableRefusal(CodeStableEntryUnused, "bundle file %s is not declared", name)
		}
	}
	for name := range dirs {
		if !needed[name] {
			return stableRefusal(CodeStableEntryUnused, "bundle directory %s is not declared", name)
		}
	}
	if _, ok := dirs[StableArtifactDir]; !ok {
		return stableRefusal(CodeStableEntryMissing, "bundle has no %s directory", StableArtifactDir)
	}
	return nil
}

// stableReadHook lets tests change a bundle between an entry's check and its
// open; it is nil outside tests.
var stableReadHook func(name string)

// readRootFile reads name inside root without following a final symlink or
// blocking on a substituted FIFO, and refuses anything but the same regular
// file of at most bound bytes.
func readRootFile(root *os.Root, name string, bound int) ([]byte, error) {
	data, _, err := readRootFileState(root, name, bound)
	return data, err
}

// readRootFileState is readRootFile that also returns the opened file's state
// (identity, size, modification time) as it was before its bytes were read.
func readRootFileState(root *os.Root, name string, bound int) ([]byte, os.FileInfo, error) {
	info, err := root.Lstat(name)
	if err != nil {
		return nil, nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > int64(bound) {
		return nil, nil, errors.New("not a bounded regular file")
	}
	if stableReadHook != nil {
		stableReadHook(name)
	}
	file, err := openStableEntry(root, name)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return nil, nil, errors.New("file changed while opened")
	}
	data, err := io.ReadAll(io.LimitReader(file, int64(bound)+1))
	if err == nil && len(data) > bound {
		err = errors.New("file exceeds its bound")
	}
	// os.Root resolves an in-root symlink even under O_NOFOLLOW, so the name is
	// checked again after the read: it must still be this same regular file.
	if named, lstatErr := root.Lstat(name); err == nil && (lstatErr != nil || !named.Mode().IsRegular() || !os.SameFile(info, named)) {
		err = errors.New("file changed while read")
	}
	return data, opened, err
}

// VerifyStable reads the bundle at dir (ReadStable) and runs the stable
// verifier over its map and its artifacts directory against repository and the
// independently supplied expected base and target, which must equal the ones
// the bundle records. A bundle that cannot be read is an error; every verifier
// outcome is returned with the verifier's exit code.
func VerifyStable(ctx context.Context, dir, repository, expectedBase, target string) (verify.StableResult, int, *StableBundle, error) {
	bundle, err := ReadStable(dir)
	if err != nil {
		return verify.StableResult{}, 2, nil, err
	}
	if bundle.Base != expectedBase || bundle.Target != target {
		return verify.StableResult{}, 2, nil, stableRefusal(CodeStableRevision, "bundle records another base or target")
	}
	result, exit := verify.Stable(ctx, bundle.Map, verify.StableOptions{
		Repository: repository, ExpectedBase: expectedBase, Target: target,
		ArtifactRoot: filepath.Join(dir, StableArtifactDir),
	})
	return result, exit, bundle, nil
}

// stableVerifySource runs the stable verifier over the caller-named root exactly
// as `cem verify-stable --repository` would: a relative or empty root is
// refused at stage arguments before the map is read, and a root beginning with
// "--" keeps no other value (CEM-V1-017).
func stableVerifySource(ctx context.Context, raw []byte, options Options) (verify.StableResult, int) {
	stableOptions := verify.StableOptions{
		Repository: options.Repository, ExpectedBase: options.ExpectedBase,
		Target: options.Target, ArtifactRoot: options.Artifacts,
	}
	if filepath.IsAbs(options.Repository) {
		return verify.Stable(ctx, raw, stableOptions)
	}
	if strings.HasPrefix(options.Repository, "--") {
		stableOptions = verify.StableOptions{Repository: options.Repository}
	}
	result := verify.NewStableResult(stableOptions)
	return result, result.Refuse("arguments", "invalid-arguments", true)
}

// exportStable is `cem export` over an exact cem/1.0 map (CEM-V1-060..063). It
// verifies the source first and publishes nothing unless the verifier accepts;
// it then copies the map and every declared artifact as exact bytes into a new
// bundle outside the repository and accepts the bundle only when verifying it
// from its own artifacts directory reproduces the source result exactly.
func exportStable(ctx context.Context, root string, raw []byte, options Options) (map[string]any, error) {
	if options.WitnessGiven {
		return nil, stableRefusal(cemcode.InvalidArguments, "%s export does not accept --witness", wire.StableSpec)
	}
	source, exit := stableVerifySource(ctx, raw, options)
	if exit != 0 {
		return map[string]any{
			"ok": false, "mutates": false, "tool": "cem-export",
			"stableVerification": workflow.StableVerification{Result: source, Exit: exit},
		}, nil
	}
	bundle, err := CollectStable(raw, *source.ExpectedBase, *source.TargetRevision, options.Artifacts)
	if err != nil {
		return nil, err
	}
	repo, err := gitauth.Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		return nil, err
	}
	parent, name, err := outputParent(repo, options.Output)
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	out, err := writeStable(parent, name, bundle)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(parent.Name(), name)
	result, exit, _, err := VerifyStable(ctx, dir, options.Repository, bundle.Base, bundle.Target)
	if err != nil || exit != 0 || !reflect.DeepEqual(result, source) {
		out.remove()
		return nil, cemcode.New(cemcode.PublishFailed, "the written stable bundle does not reproduce the source verification")
	}
	out.dir.Close()
	return map[string]any{
		"ok": true, "mutates": false, "tool": "cem-export", "profile": StableProfile, "bundle": dir,
		"manifestSha256":     digest(bundle.Manifest()),
		"stableVerification": workflow.StableVerification{Result: result, Exit: exit},
	}, nil
}
