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
// that directory. The manifest is written last.
func WriteStable(parent *os.Root, name string, b *StableBundle) (err error) {
	if err := b.checkClosure(); err != nil {
		return err
	}
	if err := parent.Mkdir(name, 0o700); err != nil {
		return cemcode.New(cemcode.PublishFailed, "bundle directory cannot be created")
	}
	defer func() {
		if err != nil {
			parent.RemoveAll(name)
		}
	}()
	dir, err := parent.OpenRoot(name)
	if err != nil {
		return cemcode.New(cemcode.PublishFailed, "bundle directory cannot be opened")
	}
	defer dir.Close()
	if err := dir.Mkdir(StableArtifactDir, 0o700); err != nil {
		return cemcode.New(cemcode.PublishFailed, "bundle artifacts directory cannot be created")
	}
	if err := writeFile(dir, StableMapName, b.Map); err != nil {
		return err
	}
	for _, a := range b.Artifacts {
		destination := filepath.Join(StableArtifactDir, filepath.FromSlash(a.Path))
		if err := dir.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
			return cemcode.New(cemcode.PublishFailed, "bundle artifact directory cannot be created")
		}
		if err := writeFile(dir, destination, a.data); err != nil {
			return err
		}
	}
	return writeFile(dir, ManifestName, b.Manifest())
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
	if info, err := os.Lstat(dir); err != nil || !info.IsDir() {
		return nil, stableRefusal(CodeStableEntryMissing, "bundle directory is not a directory")
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, stableRefusal(CodeStableEntryMissing, "bundle directory cannot be opened")
	}
	defer root.Close()
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
		bundle.Artifacts = append(bundle.Artifacts, StableArtifact{a.Kind, a.Path, a.Sha256, data})
	}
	if !bytes.Equal(bundle.Manifest(), raw) {
		return nil, stableRefusal(CodeStableManifest, "manifest is not in canonical form")
	}
	if err := unusedEntries(files, dirs, declared); err != nil {
		return nil, err
	}
	// A second walk after every read refuses a tree that changed meanwhile,
	// such as an entry swapped for a symlink to an undeclared sibling.
	filesAfter, dirsAfter, err := walkStable(root)
	if err != nil {
		return nil, err
	}
	if !maps.Equal(files, filesAfter) || !maps.Equal(dirs, dirsAfter) {
		return nil, stableRefusal(CodeStableEntryEscaping, "bundle changed while it was read")
	}
	return bundle, nil
}

// walkStable lists every regular file and directory under the bundle root by
// slash path, reading each directory incrementally so an oversized tree is
// refused before it is held in memory, and refusing any other entry kind.
func walkStable(root *os.Root) (map[string]bool, map[string]bool, error) {
	files, dirs := map[string]bool{}, map[string]bool{".": true}
	pending := []string{"."}
	for len(pending) > 0 {
		dir := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		entries, err := readStableDir(root, dir, maxStableEntries-len(files)-len(dirs))
		if err != nil {
			return nil, nil, err
		}
		for _, entry := range entries {
			name := path.Join(dir, entry.Name())
			switch {
			case entry.Type().IsRegular():
				files[name] = true
			case entry.IsDir():
				dirs[name] = true
				pending = append(pending, name)
			default:
				return nil, nil, stableRefusal(CodeStableEntryEscaping, "bundle entry %s is neither a regular file nor a directory", name)
			}
		}
	}
	return files, dirs, nil
}

// readStableDir reads one bundle directory in bounded batches, refusing more
// than room entries and a directory replaced by any other entry kind.
func readStableDir(root *os.Root, dir string, room int) ([]fs.DirEntry, error) {
	name := filepath.FromSlash(dir)
	before, err := root.Lstat(name)
	if err != nil || !before.IsDir() {
		return nil, stableRefusal(CodeStableEntryEscaping, "bundle entry %s is not a directory", dir)
	}
	file, err := openStableEntry(root, name)
	if err != nil {
		return nil, stableRefusal(CodeStableEntryEscaping, "bundle directory %s cannot be opened", dir)
	}
	defer file.Close()
	if opened, err := file.Stat(); err != nil || !opened.IsDir() || !os.SameFile(before, opened) {
		return nil, stableRefusal(CodeStableEntryEscaping, "bundle directory %s changed while opened", dir)
	}
	var entries []fs.DirEntry
	for {
		batch, err := file.ReadDir(64)
		if entries = append(entries, batch...); len(entries) > room {
			return nil, stableRefusal(CodeStableEntryUnused, "bundle holds more than %d entries", maxStableEntries)
		}
		if errors.Is(err, io.EOF) {
			return entries, nil
		}
		if err != nil {
			return nil, stableRefusal(CodeStableEntryEscaping, "bundle directory %s cannot be read", dir)
		}
	}
}

// readDeclared reads one declared file the walk listed; a declared file the
// walk did not list as a regular file is missing.
func readDeclared(root *os.Root, files map[string]bool, name string, bound int) ([]byte, error) {
	if !files[name] {
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
func unusedEntries(files, dirs, declared map[string]bool) error {
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
	if !dirs[StableArtifactDir] {
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
	info, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > int64(bound) {
		return nil, errors.New("not a bounded regular file")
	}
	if stableReadHook != nil {
		stableReadHook(name)
	}
	file, err := openStableEntry(root, name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return nil, errors.New("file changed while opened")
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
	return data, err
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
	return result, result.Refuse("arguments", "invalid-arguments", false)
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
	if err := WriteStable(parent, name, bundle); err != nil {
		return nil, err
	}
	dir := filepath.Join(parent.Name(), name)
	result, exit, _, err := VerifyStable(ctx, dir, options.Repository, bundle.Base, bundle.Target)
	if err != nil || exit != 0 || !reflect.DeepEqual(result, source) {
		parent.RemoveAll(name)
		return nil, cemcode.New(cemcode.PublishFailed, "the written stable bundle does not reproduce the source verification")
	}
	return map[string]any{
		"ok": true, "mutates": false, "tool": "cem-export", "profile": StableProfile, "bundle": dir,
		"manifestSha256":     digest(bundle.Manifest()),
		"stableVerification": workflow.StableVerification{Result: result, Exit: exit},
	}, nil
}
