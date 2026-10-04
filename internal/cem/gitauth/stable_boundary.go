package gitauth

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
)

// StableFault lets a test fail one metadata inspection with a host error such
// as EACCES, ELOOP or EIO. It is consulted with the inspected path before the
// path is examined; nil, or a nil result, leaves the real filesystem in charge.
type StableFault func(path string) error

// StableBoundary is the admitted named boundary of a Stable verification:
// every directory chain the admission walked, each retained by descriptor and
// by the identity of every named ancestor. Validate re-observes all of it.
type StableBoundary struct {
	chains []*stableChain
	files  []stableFile
}

type stableChain struct {
	names  []string
	infos  []fs.FileInfo
	handle *os.File
}

type stableFile struct {
	path string
	info fs.FileInfo // nil records an admitted absence
}

// OpenStable admits root under the canonical-repository-bounded/1 envelope
// and returns the repository with its retained boundary. The legacy Open
// grammar is unchanged; only the Stable verifier calls this.
//
// The order is fixed and stops at the first refusal: root; marker;
// administrative directory; back-pointer and reciprocity; optional commondir
// and common directory; objects; optional objects/info; optional
// objects/pack; ambient alternates; objects/info/alternates;
// objects/info/http-alternates; common info/attributes; distinct
// per-worktree info/attributes.
func OpenStable(root string, budget *gitrun.Budget, fault StableFault) (*Repository, *StableBoundary, error) {
	a := &stableAdmission{fault: fault, boundary: &StableBoundary{}}
	repository, err := a.admit(root)
	if err != nil {
		a.boundary.Close()
		return nil, nil, err
	}
	repository.budget = budget
	return repository, a.boundary, nil
}

type stableAdmission struct {
	fault    StableFault
	boundary *StableBoundary
}

func (a *stableAdmission) lstat(path string) (fs.FileInfo, error) {
	if a.fault != nil {
		if err := a.fault(path); err != nil {
			return nil, err
		}
	}
	return os.Lstat(path)
}

// directory admits one absolute, lexically clean directory: every named
// ancestor and the leaf are ordinary directories, and the leaf is retained by
// a descriptor opened without following it.
func (a *stableAdmission) directory(path, what string) (fs.FileInfo, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, unavailable("%s is not an absolute lexically clean path", what)
	}
	chain := &stableChain{}
	for name := path; ; name = filepath.Dir(name) {
		chain.names = append(chain.names, name)
		if name == filepath.Dir(name) {
			break
		}
	}
	for index := len(chain.names) - 1; index >= 0; index-- {
		info, err := a.lstat(chain.names[index])
		if err != nil || !info.IsDir() {
			return nil, unavailable("%s is not reached through ordinary directories", what)
		}
		chain.infos = append(chain.infos, info)
	}
	handle, err := openDirNoFollow(path)
	if err != nil {
		return nil, unavailable("%s cannot be opened without following links", what)
	}
	chain.handle = handle
	a.boundary.chains = append(a.boundary.chains, chain)
	leaf := chain.infos[len(chain.infos)-1]
	if opened, err := handle.Stat(); err != nil || !os.SameFile(opened, leaf) {
		return nil, unavailable("%s changed while being admitted", what)
	}
	return leaf, nil
}

// optional inspects a metadata leaf that may be absent. Only an actual
// not-found result is absence; every other failure refuses.
func (a *stableAdmission) optional(path string) (fs.FileInfo, error) {
	info, err := a.lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		a.boundary.files = append(a.boundary.files, stableFile{path: path})
		return nil, nil
	}
	if err != nil {
		return nil, unavailable("existence of %s cannot be established", filepath.Base(path))
	}
	a.boundary.files = append(a.boundary.files, stableFile{path: path, info: info})
	return info, nil
}

// pointer reads one bounded single-line metadata file.
func (a *stableAdmission) pointer(path string) (string, error) {
	data, err := readBoundedMetadata(path, maxGitfileBytes)
	if err != nil {
		return "", err
	}
	return stableMetadataLine(data)
}

// stableMetadataLine applies the interoperable metadata grammar: valid UTF-8,
// no NUL or CR, one nonempty line with at most one final LF, nothing trimmed.
func stableMetadataLine(data []byte) (string, error) {
	line := strings.TrimSuffix(string(data), "\n")
	if !utf8.Valid(data) || line == "" || strings.ContainsAny(line, "\x00\r\n") {
		return "", unavailable("repository metadata text is malformed")
	}
	return line, nil
}

func (a *stableAdmission) admit(root string) (*Repository, error) {
	rootInfo, err := a.directory(root, "repository root")
	if err != nil {
		return nil, err
	}
	marker := filepath.Join(root, ".git")
	markerInfo, err := a.lstat(marker)
	if err != nil {
		return nil, unavailable("root has no admissible .git marker")
	}
	admin, common := marker, marker
	switch {
	case markerInfo.IsDir():
		if _, err := a.directory(marker, "administrative directory"); err != nil {
			return nil, err
		}
	case markerInfo.Mode().IsRegular():
		a.boundary.files = append(a.boundary.files, stableFile{path: marker, info: markerInfo})
		if admin, common, err = a.linked(root, rootInfo, marker, markerInfo); err != nil {
			return nil, err
		}
	default:
		return nil, unavailable(".git marker is neither a directory nor a regular file")
	}
	objects := filepath.Join(common, "objects")
	if _, err := a.directory(objects, "objects directory"); err != nil {
		return nil, err
	}
	var objectsInfo fs.FileInfo
	for _, name := range []string{"info", "pack"} {
		info, err := a.optional(filepath.Join(objects, name))
		if err != nil {
			return nil, err
		}
		if info != nil {
			if _, err := a.directory(filepath.Join(objects, name), "objects/"+name); err != nil {
				return nil, err
			}
		}
		if name == "info" {
			objectsInfo = info
		}
	}
	if os.Getenv("GIT_ALTERNATE_OBJECT_DIRECTORIES") != "" {
		return nil, cemcode.New(cemcode.UnsupportedObjectAlternates,
			"ambient GIT_ALTERNATE_OBJECT_DIRECTORIES is not supported")
	}
	if objectsInfo != nil {
		for _, name := range []string{"alternates", "http-alternates"} {
			info, err := a.optional(filepath.Join(objects, "info", name))
			if err != nil {
				return nil, err
			}
			if info != nil {
				return nil, cemcode.New(cemcode.UnsupportedObjectAlternates,
					"repository objects/info/%s is not supported", name)
			}
		}
	}
	for _, directory := range attributeDirs(admin, common) {
		if err := a.attributes(directory); err != nil {
			return nil, err
		}
	}
	return &Repository{Root: root, GitDir: admin, CommonDir: common}, nil
}

// linked admits a regular .git marker: the per-worktree directory it selects,
// the mandatory reciprocal back-pointer, and the optional relative commondir.
func (a *stableAdmission) linked(root string, rootInfo fs.FileInfo, marker string, markerInfo fs.FileInfo) (string, string, error) {
	line, err := a.pointer(marker)
	if err != nil {
		return "", "", err
	}
	target, hasPrefix := strings.CutPrefix(line, "gitdir: ")
	if !hasPrefix || target == "" {
		return "", "", unavailable(".git gitfile is malformed")
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(root, target)
	}
	admin := filepath.Clean(target)
	if _, err := a.directory(admin, "administrative directory"); err != nil {
		return "", "", err
	}
	back := filepath.Join(admin, "gitdir")
	backInfo, err := a.lstat(back)
	if err != nil {
		return "", "", unavailable("per-worktree gitdir back-pointer is unavailable")
	}
	a.boundary.files = append(a.boundary.files, stableFile{path: back, info: backInfo})
	recorded, err := a.pointer(back)
	if err != nil {
		return "", "", err
	}
	if !filepath.IsAbs(recorded) {
		recorded = filepath.Join(admin, recorded)
	}
	recorded = filepath.Clean(recorded)
	recordedInfo, err := a.lstat(recorded)
	if err != nil || !os.SameFile(recordedInfo, markerInfo) {
		return "", "", unavailable("worktree metadata does not reciprocally identify this root")
	}
	parentInfo, err := a.lstat(filepath.Dir(recorded))
	if err != nil || !os.SameFile(parentInfo, rootInfo) {
		return "", "", unavailable("worktree metadata does not reciprocally identify this root")
	}
	common := admin
	pointer := filepath.Join(admin, "commondir")
	pointerInfo, err := a.optional(pointer)
	if err != nil {
		return "", "", err
	}
	if pointerInfo != nil {
		relative, err := a.pointer(pointer)
		if err != nil {
			return "", "", err
		}
		if strings.HasPrefix(relative, "/") {
			return "", "", unavailable("commondir is not a relative path")
		}
		common = filepath.Clean(filepath.Join(admin, relative))
		if _, err := a.directory(common, "common directory"); err != nil {
			return "", "", err
		}
	}
	return admin, common, nil
}

func (a *stableAdmission) attributes(directory string) error {
	info, err := a.optional(filepath.Join(directory, "info"))
	if err != nil || info == nil {
		return err
	}
	if _, err := a.directory(filepath.Join(directory, "info"), "info directory"); err != nil {
		return err
	}
	path := filepath.Join(directory, "info", "attributes")
	if info, err = a.optional(path); err != nil || info == nil {
		return err
	}
	content, err := readBoundedMetadata(path, maxAttributesBytes)
	if err != nil {
		return err
	}
	if hasEffectiveAttributeLine(string(content)) {
		return cemcode.New(cemcode.UnsupportedRepositoryAttributes,
			"repository info/attributes carries effective attribute lines; canonical derivation cannot isolate it")
	}
	return nil
}

// Validate re-observes the whole retained boundary: every named ancestor,
// every retained descriptor, and every inspected metadata leaf must still be
// the same object or the same absence. A changed directory mtime or entry
// count alone is not a changed identity.
func (b *StableBoundary) Validate() error {
	for _, chain := range b.chains {
		for index, info := range chain.infos {
			name := chain.names[len(chain.names)-1-index]
			current, err := os.Lstat(name)
			if err != nil || !current.IsDir() || !os.SameFile(current, info) {
				return unavailable("repository boundary changed during verification")
			}
		}
		opened, err := chain.handle.Stat()
		if err != nil || !os.SameFile(opened, chain.infos[len(chain.infos)-1]) {
			return unavailable("repository boundary changed during verification")
		}
	}
	for _, file := range b.files {
		current, err := os.Lstat(file.path)
		switch {
		case file.info == nil && errors.Is(err, fs.ErrNotExist):
		case file.info != nil && err == nil && os.SameFile(current, file.info) && current.Mode().Type() == file.info.Mode().Type():
		default:
			return unavailable("repository metadata changed during verification")
		}
	}
	return nil
}

// Close releases the retained descriptors.
func (b *StableBoundary) Close() {
	for _, chain := range b.chains {
		if chain.handle != nil {
			chain.handle.Close()
		}
	}
	b.chains = nil
}
