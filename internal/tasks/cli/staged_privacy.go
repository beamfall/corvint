// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/safeopen"
)

// openStagedPath is only for automatic hint discovery, never an explicit
// operator snapshot. Resolve benign staging links without traversing a
// protected home folder, then open without following replacement links.
// ASD-V0-2421: checking only the final resolved path would already probe
// protected descendants while resolving the path.
func openStagedPath(name string) (*os.File, error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil, fmt.Errorf("home unavailable")
	}
	home, err = filepath.EvalSymlinks(home)
	if err != nil {
		return nil, err
	}
	protected := func(p string) bool {
		for _, dir := range []string{"Desktop", "Documents", "Downloads", "Movies", "Music", "Pictures"} {
			root := filepath.Join(home, dir)
			if runtime.GOOS == "darwin" {
				p, root = strings.ToLower(p), strings.ToLower(root)
			}
			if p == root || strings.HasPrefix(p, root+string(filepath.Separator)) {
				return true
			}
		}
		return false
	}
	path, err := filepath.Abs(name)
	if err != nil {
		return nil, err
	}
	// Pin each parent before inspecting its child. A renamed or replaced
	// ancestor must not redirect even a metadata probe into a media folder.
	resolve := func(path string) (*os.File, string, error) {
		dir, err := safeopen.Root(string(filepath.Separator))
		if err != nil {
			return nil, "", err
		}
		defer func() { _ = dir.Close() }()
		parts := strings.Split(strings.TrimPrefix(filepath.Clean(path), string(filepath.Separator)), string(filepath.Separator))
		current := string(filepath.Separator)
		for i, part := range parts {
			current = filepath.Join(current, part)
			if protected(current) {
				return nil, "", fmt.Errorf("protected home folder")
			}
			fi, err := dir.Lstat(part)
			if err != nil {
				return nil, "", err
			}
			if fi.Mode()&os.ModeSymlink != 0 {
				target, err := dir.Readlink(part)
				if err != nil {
					return nil, "", err
				}
				if !filepath.IsAbs(target) {
					target = filepath.Join(filepath.Dir(current), target)
				}
				return nil, filepath.Join(append([]string{target}, parts[i+1:]...)...), nil
			}
			if i == len(parts)-1 {
				f, err := safeopen.InRoot(dir, part, os.O_RDONLY, 0, false)
				return f, "", err
			}
			next, err := safeopen.SubRoot(dir, part)
			if err != nil {
				return nil, "", err
			}
			_ = dir.Close()
			dir = next
		}
		return nil, "", fmt.Errorf("empty staging path")
	}
	for links := 0; links <= 40; links++ {
		f, next, err := resolve(path)
		if err != nil || next == "" {
			return f, err
		}
		path = next
	}
	return nil, fmt.Errorf("too many staging links")
}
