// Package receiptbundle exports one verified cem/1.0 change map and its
// declared artifacts as exact byte copies into a new directory with a manifest
// (CEM-V1-060..064). The cem/0.x receipt bundle (corvint-receipt-bundle/0,
// RCB-V0) is retired with cem/0.x: a 0.x map refuses (CEM-V1-210).
//
// The export reads repository and Git-directory state and writes only the
// bundle directory the caller names outside the worktree. It never signs or
// publishes.
package receiptbundle

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// Frozen names and bounds (RCB-V0-002, RCB-V0-004).
const (
	ManifestName    = "manifest.json"
	MaxReceiptBytes = wire.MaxMapBytes
)

// Options names the one change to export and where to put it.
type Options struct {
	MapPath      string // repository-relative CEM path
	ExpectedBase string // independent base the map must name (CEM-CB-010)
	Target       string // the commit whose canonical patch the map binds
	Output       string // absolute path of a directory that must not exist
	Witness      string // optional saved `corvint witness --json` report

	// The cem/1.0 route only (CEM-V1-060): the repository root as the caller
	// named it, the explicit artifact root, and which optional flags were given.
	Repository     string
	Artifacts      string
	ArtifactsGiven bool
	WitnessGiven   bool
}

// Axis is one NOT_RUN or NOT_PRODUCED value inside a receipt.
type Axis struct {
	Pointer string `json:"pointer"`
	Value   string `json:"value"`
}

// Export writes the bundle and returns the command envelope. Only a map whose
// bytes declare exactly cem/1.0 is exported (CEM-V1-060); a cem/0.x map is
// retired and refuses with the retired-spec fix (CEM-V1-210).
func Export(ctx context.Context, root string, options Options) (map[string]any, error) {
	raw, err := readStableProbe(root, options.MapPath)
	if err != nil {
		return nil, cemcode.New(cemcode.MapUnavailable, "map %s is not a readable regular file", options.MapPath)
	}
	if !wire.DeclaresStable(raw) {
		if _, err := wire.ParseStable(raw); err != nil {
			return nil, err
		}
	}
	return exportStable(ctx, root, raw, options)
}

// outputParent refuses a relative, existing, or in-repository output
// (RCB-V0-005) and returns its opened parent and its name, so every later
// write goes through the directory that was checked.
func outputParent(repo *gitauth.Repository, output string) (*os.Root, string, error) {
	if !filepath.IsAbs(output) {
		return nil, "", refused("--output must be an absolute path")
	}
	if _, err := os.Lstat(output); !errors.Is(err, os.ErrNotExist) {
		return nil, "", refused("--output must name a path that does not exist")
	}
	parentPath, err := filepath.EvalSymlinks(filepath.Dir(output))
	if err != nil {
		return nil, "", refused("--output parent directory does not resolve")
	}
	parent, err := os.OpenRoot(parentPath)
	if err != nil {
		return nil, "", refused("--output parent directory cannot be opened")
	}
	if err := outsideRepository(repo, parent, parentPath); err != nil {
		parent.Close()
		return nil, "", err
	}
	return parent, filepath.Base(output), nil
}

func refused(message string) error {
	return cemcode.New(cemcode.BundleOutputRefused, "%s", message)
}

// outsideRepository compares the opened parent and each of its ancestors by
// file identity, never by path text, with every protected directory: case
// variants and volume aliases name the same file.
func outsideRepository(repo *gitauth.Repository, parent *os.Root, parentPath string) error {
	opened, err := parent.Stat(".")
	if err != nil {
		return refused("--output parent directory cannot be examined")
	}
	chain, err := ancestors(parentPath)
	if err != nil || !os.SameFile(opened, chain[0]) {
		return refused("--output parent directory cannot be examined")
	}
	protected := protectedDirs(repo)
	for _, dir := range chain {
		if slices.ContainsFunc(protected, func(owned os.FileInfo) bool { return os.SameFile(dir, owned) }) {
			return refused("--output must lie outside every worktree and Git directory")
		}
	}
	return nil
}

// ancestors stats dir and every directory above it.
func ancestors(dir string) ([]os.FileInfo, error) {
	chain := []os.FileInfo{}
	for {
		info, err := os.Stat(dir)
		if err != nil {
			return nil, err
		}
		chain = append(chain, info)
		up := filepath.Dir(dir)
		if up == dir {
			return chain, nil
		}
		dir = up
	}
}

// protectedDirs are this worktree, its Git directories, the primary worktree,
// and every linked worktree the common directory records.
func protectedDirs(repo *gitauth.Repository) []os.FileInfo {
	paths := append([]string{repo.Root, repo.GitDir, repo.CommonDir}, primaryWorktree(repo.CommonDir)...)
	paths = append(paths, configuredWorktree(repo.CommonDir)...)
	paths = append(paths, linkedWorktrees(repo.CommonDir)...)
	infos := []os.FileInfo{}
	for _, path := range paths {
		if info, err := os.Stat(path); err == nil {
			infos = append(infos, info)
		}
	}
	return infos
}

// primaryWorktree is the common directory's parent when that parent's .git
// is the common directory itself; a bare common directory has none.
func primaryWorktree(common string) []string {
	parent := filepath.Dir(common)
	marker, markerErr := os.Stat(filepath.Join(parent, ".git"))
	own, ownErr := os.Stat(common)
	if markerErr != nil || ownErr != nil || !os.SameFile(marker, own) {
		return nil
	}
	return []string{parent}
}

// configuredWorktree is core.worktree from the common config: the primary
// worktree of a repository whose Git directory lives elsewhere. A relative
// value is relative to the Git directory.
func configuredWorktree(common string) []string {
	data, err := readRegular(common, "config")
	if err != nil {
		return nil
	}
	section := ""
	dirs := []string{}
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "[") {
			section = strings.ToLower(strings.Trim(line, "[]"))
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found || section != "core" || !strings.EqualFold(strings.TrimSpace(key), "worktree") {
			continue
		}
		dirs = append(dirs, relativeTo(common, strings.Trim(strings.TrimSpace(value), `"`)))
	}
	return dirs
}

func relativeTo(dir, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(dir, path)
}

// linkedWorktrees reads each $COMMON/worktrees/*/gitdir back-pointer, which
// names a linked worktree's .git file, absolute or relative to itself.
func linkedWorktrees(common string) []string {
	admin := filepath.Join(common, "worktrees")
	entries, _ := os.ReadDir(admin)
	dirs := []string{}
	for _, entry := range entries {
		data, err := readRegular(filepath.Join(admin, entry.Name()), "gitdir")
		if err != nil {
			continue
		}
		marker := relativeTo(filepath.Join(admin, entry.Name()), strings.TrimSuffix(string(data), "\n"))
		dirs = append(dirs, filepath.Dir(marker))
	}
	return dirs
}

// readRegular reads name under dir without following a final symlink and
// refuses anything but a regular file of at most MaxReceiptBytes.
func readRegular(dir, name string) ([]byte, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	info, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("not a regular file")
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxReceiptBytes+1))
	if err == nil && len(data) > MaxReceiptBytes {
		err = errors.New("receipt exceeds the byte bound")
	}
	return data, err
}

func writeFile(parent *os.Root, path string, data []byte) error {
	file, err := parent.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return cemcode.New(cemcode.PublishFailed, "bundle file cannot be created")
	}
	_, err = file.Write(data)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return cemcode.New(cemcode.PublishFailed, "bundle file cannot be written")
	}
	return nil
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
