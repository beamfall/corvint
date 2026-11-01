package affected_test

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/shell"
)

// TestRepositoryShellForeignCallersComplete_TSP_V0_005 verifies this
// repository's .corvint/shell-foreign-callers.json against the tracked tree,
// so the declaration is checked rather than trusted. It uses the shell
// plugin's own candidate rule (shell.CallerCandidate) over every tracked file,
// .github included, so the two cannot disagree (V1-4178): each candidate that
// names a declared script's file name must be listed as a caller or a mention.
// The plugin, run over the tracked files only so an untracked scratch file
// cannot break the test, must then accept the declaration (which also proves
// that every listed file names the script, literally or by a name a shell
// file builds) and drop the foreign-invocation frontier from the script.
func TestRepositoryShellForeignCallersComplete_TSP_V0_005(t *testing.T) {
	root := repositoryRoot(t)
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(affected.ShellForeignCallersPath)))
	if err != nil {
		t.Fatal(err)
	}
	var declared struct {
		Scripts map[string]struct {
			Callers  []string `json:"callers"`
			Mentions []string `json:"mentions"`
		} `json:"scripts"`
	}
	if err := json.Unmarshal(body, &declared); err != nil {
		t.Fatal(err)
	}
	if len(declared.Scripts) == 0 {
		t.Fatalf("no script is declared: %s", body)
	}

	// The declaration covers tracked files: an ignored build product in the
	// checkout, such as the ./corvint binary `go build ./cmd/corvint` writes,
	// embeds every script name and is no caller (V1-1819).
	listing, err := exec.Command("git", "-C", root, "ls-files", "-z").Output()
	if err != nil {
		t.Fatal(err)
	}
	tracked := map[string]bool{}
	for _, name := range strings.Split(string(listing), "\x00") {
		if name != "" {
			tracked[name] = true
		}
	}
	for script, entry := range declared.Scripts {
		listed := append(slices.Clone(entry.Callers), entry.Mentions...)
		for name := range tracked {
			if name == script || !shell.CallerCandidate(path.Base(name)) {
				continue
			}
			text, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if bytes.Contains(text, []byte(path.Base(script))) && !slices.Contains(listed, name) {
				t.Errorf("%s names %s but is not a declared caller or mention in %s", name, script, affected.ShellForeignCallersPath)
			}
		}
	}

	directories := map[string]bool{".": true}
	for name := range tracked {
		for directory := path.Dir(name); directory != "."; directory = path.Dir(directory) {
			directories[directory] = true
		}
	}
	result, err := shell.New().UnitsSource(affected.FSSource(trackedFS{os.DirFS(root), tracked, directories}))
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(result.Frontier, shell.FrontierForeignCallersInvalid) {
		t.Fatalf("the shell plugin refused %s: the listing is incomplete or a listed file no longer names its script", affected.ShellForeignCallersPath)
	}
	for _, unit := range result.Units {
		if entry, ok := declared.Scripts[strings.TrimPrefix(unit.ID, "shell:")]; ok {
			if !slices.Equal(unit.Callers, entry.Callers) || slices.Contains(unit.Frontier, shell.FrontierForeignInvocation) {
				t.Errorf("%s: callers %v, frontier %v; want declared callers and no foreign frontier", unit.ID, unit.Callers, unit.Frontier)
			}
		}
	}
}

// trackedFS shows only the tracked files of a checkout and the directories
// that hold one.
type trackedFS struct {
	fsys        fs.FS
	tracked     map[string]bool
	directories map[string]bool
}

func (t trackedFS) visible(name string) bool { return t.tracked[name] || t.directories[name] }

func (t trackedFS) Open(name string) (fs.File, error) {
	if !t.visible(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return t.fsys.Open(name)
}

func (t trackedFS) ReadDir(name string) ([]fs.DirEntry, error) {
	entries, err := fs.ReadDir(t.fsys, name)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(entries, func(entry fs.DirEntry) bool { return !t.visible(path.Join(name, entry.Name())) }), nil
}

// OpenBounded is the bounded open an immutable Source requires.
func (t trackedFS) OpenBounded(name string, limit int) (fs.File, error) {
	file, err := t.Open(name)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err == nil && (!info.Mode().IsRegular() || info.Size() > int64(limit)) {
		err = affected.ErrWalkLimit
	}
	if err != nil {
		file.Close()
		return nil, err
	}
	return file, nil
}
