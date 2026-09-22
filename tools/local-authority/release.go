package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"golang.org/x/sys/unix"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/authoritystore"
	"github.com/Beamfall/corvint/internal/localauthority"
)

type releaseFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Mode   string `json:"mode"`
}
type releaseManifest struct {
	Profile               string        `json:"profile"`
	SourceRevision        string        `json:"sourceRevision"`
	ReleaseID             string        `json:"releaseId"`
	AdapterTemplateSHA256 string        `json:"adapterTemplateSHA256"`
	Files                 []releaseFile `json:"files"`
}

func releasePath(path string) bool {
	return filepath.Clean(path) == path && !filepath.IsAbs(path) && !strings.Contains(path, "\\") && (path == "local-authority" || path == "corvint" || path == "authority-hook.json" || path == "bin/git" || strings.HasPrefix(path, "go/")) && !strings.HasPrefix(path, "../")
}
func validateRelease(m releaseManifest) error {
	if m.Profile != "corvint-authority-release/1" || len(m.Files) == 0 || len(m.Files) > 100000 {
		return errors.New("release profile/bound")
	}
	if _, e := decodeHex(m.SourceRevision, 20); e != nil {
		return errors.New("source revision")
	}
	if _, e := decodeHex(m.AdapterTemplateSHA256, 32); e != nil {
		return e
	}
	if m.ReleaseID != sourceReleaseID(m) {
		return errors.New("source release identity")
	}
	previous := ""
	required := map[string]bool{"local-authority": false, "corvint": false, "go/bin/go": false, "bin/git": false, "authority-hook.json": false}
	for _, f := range m.Files {
		if !releasePath(f.Path) || f.Path <= previous {
			return errors.New("release path/order")
		}
		previous = f.Path
		if _, e := decodeHex(f.SHA256, 32); e != nil {
			return e
		}
		if f.Mode != "0444" && f.Mode != "0555" {
			return errors.New("release mode")
		}
		if _, ok := required[f.Path]; ok {
			if (f.Path == "authority-hook.json" && f.Mode != "0444") || (f.Path != "authority-hook.json" && f.Mode != "0555") {
				return errors.New("nonexecutable tool")
			}
			required[f.Path] = true
		}
	}
	for _, ok := range required {
		if !ok {
			return errors.New("missing fixed tool")
		}
	}
	return nil
}
func prepareRelease(output, consumer, goRoot, gitBinary, revision, adapterTemplate string) error {
	if os.Geteuid() == 0 {
		return errors.New("prepare release as unprivileged builder")
	}
	if _, e := decodeHex(revision, 20); e != nil {
		return errors.New("source revision")
	}
	if !filepath.IsAbs(output) || filepath.Clean(output) != output {
		return errors.New("output path")
	}
	// Reject malformed inputs before creating a bundle or durably copying the
	// toolchain. These reads are bounded and do not publish any payload.
	for _, path := range []string{consumer, gitBinary, filepath.Join(goRoot, "bin", "go")} {
		if _, e := readRegular(path, 64<<20); e != nil {
			return e
		}
		info, e := os.Stat(path)
		if e != nil || info.Mode()&0111 == 0 {
			return errors.New("nonexecutable input")
		}
	}
	template, e := readRegular(adapterTemplate, 128<<10)
	if e != nil {
		return e
	}
	if _, e = renderAdapter(template, "/preflight/corvint", strings.Repeat("0", 64)); e != nil {
		return e
	}
	if e = os.Mkdir(output, 0700); e != nil {
		return e
	}
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	manifest := releaseManifest{Profile: "corvint-authority-release/1", SourceRevision: revision, Files: []releaseFile{}}
	sources := map[string]string{"local-authority": exe, "corvint": consumer, "bin/git": gitBinary}
	goRoot, e = filepath.EvalSymlinks(goRoot)
	if e != nil {
		return e
	}
	e = filepath.WalkDir(goRoot, func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return errors.New("Go root symlink")
		}
		relative, e := filepath.Rel(goRoot, path)
		if e != nil {
			return e
		}
		sources["go/"+relative] = path
		return nil
	})
	if e != nil {
		return e
	}
	paths := []string{}
	for p := range sources {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		b, e := readRegular(sources[p], 64<<20)
		if e != nil {
			return e
		}
		info, e := os.Stat(sources[p])
		if e != nil {
			return e
		}
		mode := "0444"
		perm := os.FileMode(0444)
		if info.Mode()&0111 != 0 {
			mode = "0555"
			perm = 0555
		}
		destination := filepath.Join(output, p)
		if e = os.MkdirAll(filepath.Dir(destination), 0755); e != nil {
			return e
		}
		if e = exclusiveFile(destination, b, perm); e != nil {
			return e
		}
		manifest.Files = append(manifest.Files, releaseFile{p, digest(b), mode})
	}
	manifest.AdapterTemplateSHA256 = digest(template)
	manifest.ReleaseID = sourceReleaseID(manifest)
	releaseID := manifest.ReleaseID
	consumerHash := ""
	for _, file := range manifest.Files {
		if file.Path == "corvint" {
			consumerHash = file.SHA256
		}
	}
	prepared, e := renderAdapter(template, filepath.Join(authoritystore.RootPath, "versions", releaseID, "corvint"), consumerHash)
	if e != nil {
		return e
	}
	if e = exclusiveFile(filepath.Join(output, "authority-hook.json"), prepared, 0444); e != nil {
		return e
	}
	manifest.Files = append(manifest.Files, releaseFile{"authority-hook.json", digest(prepared), "0444"})
	sort.Slice(manifest.Files, func(i, j int) bool { return manifest.Files[i].Path < manifest.Files[j].Path })
	if e = validateRelease(manifest); e != nil {
		return e
	}
	raw := mustJSONLine(manifest)
	if e = exclusiveFile(filepath.Join(output, "manifest.json"), raw, 0444); e != nil {
		return e
	}
	// Apple CLT Git is copied without rewriting its signature. Admission reviews
	// this dependency inventory; non-system dynamic libraries block preparation.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	deps, e := exec.CommandContext(ctx, "/usr/bin/otool", "-L", filepath.Join(output, "bin", "git")).Output()
	if e != nil {
		return e
	}
	for _, line := range strings.Split(string(deps), "\n") {
		if !strings.HasPrefix(line, "\t") {
			continue
		}
		dependency := strings.Fields(line)[0]
		if !strings.HasPrefix(dependency, "/usr/lib/") && !strings.HasPrefix(dependency, "/System/Library/") {
			return errors.New("Git has unbundled non-system dependency")
		}
	}
	if e = exclusiveFile(output+".git-dependencies.txt", deps, 0444); e != nil {
		return e
	}
	tree, e := treeDigest(filepath.Join(output, "go"), false)
	if e != nil {
		return e
	}
	root := filepath.Join(authoritystore.RootPath, "versions", releaseID)
	images := map[string]string{}
	for _, f := range manifest.Files {
		images[f.Path] = f.SHA256
	}
	policy := executionPolicy{Profile: "corvint-protected-execution-policy/0", Worker: authoritystore.Image{Path: filepath.Join(root, "local-authority"), SHA256: images["local-authority"]}, Go: authoritystore.Image{Path: filepath.Join(root, "go/bin/go"), SHA256: images["go/bin/go"]}, GoRootSHA256: tree, DriverSHA256: digest(driverSource), RecipeSHA256: digest([]byte(recipe)), RuntimeModule: "github.com/tetratelabs/wazero", RuntimeVersion: "v1.12.0", RuntimeSum: "h1:DuWcpNu/FzgEXgGBDp8J1Spc+CWOvvtvVyjKlaZopYU=", BuildDeadlineMillis: "60000", RunDeadlineMillis: "5000", MemoryPages: "8192", WorkerRSSKiB: "786432", BuildRSSKiB: "2097152"}
	policyBytes, e := localauthority.Canonical(policy)
	if e != nil {
		return e
	}
	if e = exclusiveFile(output+".execution-policy.proposed.json", policyBytes, 0444); e != nil {
		return e
	}
	fmt.Printf("releaseId=%s manifestSHA256=%s\n", releaseID, digest(raw))
	return nil
}
func readRegular(path string, limit int) ([]byte, error) {
	fd, e := unix.Open(path, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if e != nil {
		return nil, e
	}
	f := os.NewFile(uintptr(fd), path)
	defer f.Close()
	info, e := f.Stat()
	if e != nil || !info.Mode().IsRegular() {
		return nil, errors.New("not regular file")
	}
	return readBound(f, limit)
}
func installRelease(source, expected string) error {
	if os.Geteuid() != 0 {
		return errors.New("operator root required")
	}
	if _, e := decodeHex(expected, 32); e != nil {
		return e
	}
	raw, e := readRegular(filepath.Join(source, "manifest.json"), 16<<20)
	if e != nil {
		return e
	}
	if digest(raw) != expected {
		return errors.New("unreviewed manifest")
	}
	var manifest releaseManifest
	if e = strictDecode(raw, &manifest); e != nil {
		return e
	}
	if !bytes.Equal(raw, mustJSONLine(manifest)) {
		return errors.New("manifest framing")
	}
	if e = validateRelease(manifest); e != nil {
		return e
	}
	if e = ensureRootDirectory(authoritystore.RootPath); e != nil {
		return e
	}
	unlock, lockErr := operatorLock()
	if lockErr != nil {
		return lockErr
	}
	defer unlock()
	versions := filepath.Join(authoritystore.RootPath, "versions")
	if e = ensureRootDirectory(versions); e != nil {
		return e
	}
	destination := filepath.Join(versions, manifest.ReleaseID)
	if _, e = os.Lstat(destination); !os.IsNotExist(e) {
		return errors.New("release already installed")
	}
	stage, e := os.MkdirTemp(versions, ".install-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(stage)
	for _, f := range manifest.Files {
		b, e := readRegular(filepath.Join(source, f.Path), 64<<20)
		if e != nil {
			return e
		}
		if digest(b) != f.SHA256 {
			return errors.New("release file changed")
		}
		path := filepath.Join(stage, f.Path)
		if e = os.MkdirAll(filepath.Dir(path), 0755); e != nil {
			return e
		}
		mode := os.FileMode(0444)
		if f.Mode == "0555" {
			mode = 0555
		}
		if e = exclusiveFile(path, b, mode); e != nil {
			return e
		}
	}
	if e = exclusiveFile(filepath.Join(stage, "manifest.json"), raw, 0444); e != nil {
		return e
	}
	if e = os.Chmod(stage, 0755); e != nil {
		return e
	}
	if e = os.Rename(stage, destination); e != nil {
		return e
	}
	// Installing code does not create a key, accepted root, policy or account.
	fmt.Println(destination)
	return nil
}
func removeRelease(releaseID string) error {
	if os.Geteuid() != 0 {
		return errors.New("operator root required")
	}
	if _, e := decodeHex(releaseID, 32); e != nil {
		return e
	}
	unlock, lockErr := operatorLock()
	if lockErr != nil {
		return lockErr
	}
	defer unlock()
	dir := filepath.Join(authoritystore.RootPath, "versions", releaseID)
	rootBytes, e := readRootFile(filepath.Join(authoritystore.RootPath, "accepted-root.json"), 128<<10)
	if e == nil {
		var root authoritystore.RootDocument
		if e = localauthority.Decode(rootBytes, &root); e != nil {
			return e
		}
		if !root.Revoked && strings.HasPrefix(root.Consumer.Path, dir+"/") {
			return errors.New("revoke authority before release removal")
		}
	} else {
		if _, statErr := os.Lstat(filepath.Join(authoritystore.RootPath, "accepted-root.json")); !os.IsNotExist(statErr) {
			return errors.New("cannot establish revocation state")
		}
	}
	raw, e := readRootFile(filepath.Join(dir, "manifest.json"), 16<<20)
	if e != nil {
		return e
	}

	var manifest releaseManifest
	if e = strictDecode(raw, &manifest); e != nil {
		return e
	}
	if e = validateRelease(manifest); e != nil {
		return e
	}
	if manifest.ReleaseID != releaseID {
		return errors.New("installed release identity")
	}
	// Verify every owned file before deleting anything. Unknown files or changed
	// bytes remain visible for operator inspection; no recursive wildcard removal.
	expected := map[string]bool{"manifest.json": true}
	for _, f := range manifest.Files {
		b, e := readRootFile(filepath.Join(dir, f.Path), 64<<20)
		if e != nil || digest(b) != f.SHA256 {
			return errors.New("installed file drift")
		}
		expected[f.Path] = true
	}
	dirs := []string{}
	e = filepath.WalkDir(dir, func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		rel, _ := filepath.Rel(dir, path)
		if d.IsDir() {
			dirs = append(dirs, path)
			return nil
		}
		if !expected[rel] {
			return errors.New("unowned release file; retained")
		}
		return nil
	})
	if e != nil {
		return e
	}
	for path := range expected {
		if e = os.Remove(filepath.Join(dir, path)); e != nil {
			return e
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dirs)))
	for _, path := range dirs {
		if e = os.Remove(path); e != nil {
			return e
		}
	}
	return nil
}

func sourceReleaseID(m releaseManifest) string {
	files := []releaseFile{}
	for _, f := range m.Files {
		if f.Path != "authority-hook.json" {
			files = append(files, f)
		}
	}
	original := struct {
		Profile               string        `json:"profile"`
		SourceRevision        string        `json:"sourceRevision"`
		AdapterTemplateSHA256 string        `json:"adapterTemplateSHA256"`
		Files                 []releaseFile `json:"files"`
	}{"corvint-authority-source-release/1", m.SourceRevision, m.AdapterTemplateSHA256, files}
	return digest(mustJSONLine(original))
}

type nativeAdapterDeclaration struct {
	Profile        string  `json:"profile"`
	Consumer       *string `json:"consumer"`
	ConsumerSHA256 *string `json:"consumerSha256"`
}

func renderAdapter(template []byte, consumer, hash string) ([]byte, error) {
	var declaration nativeAdapterDeclaration
	if strictDecode(template, &declaration) != nil || declaration.Profile != "corvint-native-authority-hook/0" || declaration.Consumer != nil || declaration.ConsumerSHA256 != nil || !bytes.Equal(template, mustJSONLine(declaration)) {
		return nil, errors.New("unrecognized native adapter declaration")
	}
	declaration.Consumer = &consumer
	declaration.ConsumerSHA256 = &hash
	return mustJSONLine(declaration), nil
}
