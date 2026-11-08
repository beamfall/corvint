// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/gateselect"
)

// inputDecision is shared by direct doc-gates and the batch's Make target. Only
// compilation and the replay driver's reads concern this checkout: corvint and
// the selector read detached, fixed historical trees, not the current worktree.
// The driver's declared scope is checked by Corvint's own reader index. Its
// absence, an unbounded driver or an unreadable compilation closure runs fully.
func inputDecision(ctx context.Context, root, base string) (bool, string) {
	if base == "" {
		var err error
		base, err = eventBase(os.Getenv("GITHUB_EVENT_PATH"), os.Getenv("GITHUB_EVENT_NAME"))
		if err != nil {
			return true, "unknown read set: " + err.Error()
		}
	}
	if !fullOID(base) {
		return true, "unknown read set: no full base commit"
	}
	if _, err := run(ctx, root, "git", "cat-file", "-e", base+"^{commit}"); err != nil {
		return true, "unknown read set: base commit unavailable"
	}
	diff, err := run(ctx, root, "git", "diff", "--name-only", "--no-renames", "-z", base, "--")
	if err != nil {
		return true, "unknown read set: diff unavailable"
	}
	untracked, err := run(ctx, root, "git", "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return true, "unknown read set: untracked paths unavailable"
	}
	paths := strings.Split(diff+untracked, "\x00")
	ignored, err := run(ctx, root, "git", "ls-files", "--others", "--ignored", "--exclude-standard", "-z")
	if err != nil {
		return true, "unknown read set: ignored inputs unavailable"
	}
	paths = append(paths, strings.Split(ignored, "\x00")...)
	module, err := moduleOf(filepath.Join(root, "go.mod"))
	if err != nil {
		return true, "unknown read set: module unavailable"
	}
	// go list supplies the transitive production compilation closure, including
	// platform-selected and embedded inputs. Matching entire package subtrees is
	// conservative for new/deleted files, embed globs and native sources too.
	if _, err := os.Stat(filepath.Join(root, "vendor", "modules.txt")); err == nil || !os.IsNotExist(err) {
		return true, "unknown read set: vendored compilation"
	}
	workspace, err := run(ctx, root, "go", "env", "GOWORK")
	if err != nil {
		return true, "unknown read set: workspace configuration unavailable"
	}
	if workspace = strings.TrimSpace(workspace); workspace != "" && workspace != "off" {
		return true, "unknown read set: workspace or local-module compilation read set is unbounded"
	}
	modules, err := run(ctx, root, "go", "list", "-mod=readonly", "-m", "-json", "all")
	if err != nil {
		return true, "unknown read set: module graph unavailable"
	}
	if err := boundModuleGraph(root, strings.NewReader(modules)); err != nil {
		return true, "unknown read set: " + err.Error()
	}
	listed, err := run(ctx, root, "go", "list", "-mod=readonly", "-deps", "-json", "./cmd/corvint", "./tools/gate-affected-select", "./tools/selection-baseline")
	if err != nil {
		return true, "unknown read set: compilation closure unavailable"
	}
	dirs, err := compilationDirs(root, strings.NewReader(listed))
	if err != nil {
		return true, "unknown read set: " + err.Error()
	}
	bounds, err := gateselect.Bounds(module, root, append(append([]string{}, paths...), "tools/selection-baseline/main.go"))
	if err != nil {
		return true, "unknown read set: reader index unavailable"
	}
	return decideInputs(paths, dirs, bounds, module+"/tools/selection-baseline", base)
}

// Even a module with no imported package can affect version selection through
// its requirements (or, in a workspace, replacements). Check the module graph,
// rather than relying only on modules appearing in the package closure.
func boundModuleGraph(root string, input io.Reader) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("module root unavailable")
	}
	physical, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("module root unavailable")
	}
	dec := json.NewDecoder(input)
	count := 0
	for {
		var module struct {
			Main    bool
			Dir     string
			Replace *struct{ Version string }
		}
		if err := dec.Decode(&module); err == io.EOF {
			break
		} else if err != nil {
			return fmt.Errorf("module graph malformed")
		}
		count++
		if (module.Main && module.Dir != root && module.Dir != physical) || (module.Replace != nil && module.Replace.Version == "") {
			return fmt.Errorf("workspace or local-module compilation read set is unbounded")
		}
	}
	if count == 0 {
		return fmt.Errorf("module graph empty")
	}
	return nil
}

// eventBase uses the tested event's immutable base, never HEAD^ (which could
// omit earlier commits in a PR). Unknown events and shallow history run fully.
func eventBase(path, event string) (string, error) {
	var data struct {
		Before      string `json:"before"`
		PullRequest struct {
			Base struct {
				SHA string `json:"sha"`
			} `json:"base"`
		} `json:"pull_request"`
		MergeGroup struct {
			BaseSHA string `json:"base_sha"`
		} `json:"merge_group"`
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("event base unavailable")
	}
	if json.Unmarshal(content, &data) != nil {
		return "", fmt.Errorf("event base malformed")
	}
	switch event {
	case "pull_request":
		return data.PullRequest.Base.SHA, nil
	case "merge_group":
		return data.MergeGroup.BaseSHA, nil
	case "push":
		return data.Before, nil
	default:
		return "", fmt.Errorf("event base unsupported")
	}
}

func compilationDirs(root string, input io.Reader) ([]string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	lexicalRoot := root
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("compilation root unavailable")
	}
	var dirs []string
	dec := json.NewDecoder(input)
	for {
		var pkg struct {
			Dir                                        string
			Standard                                   bool
			Incomplete                                 bool
			Error                                      *json.RawMessage
			GoFiles, CgoFiles, EmbedFiles              []string
			CFiles, CXXFiles, MFiles, HFiles, FFiles   []string
			SFiles, SwigFiles, SwigCXXFiles, SysoFiles []string
			Module                                     *struct {
				Main    bool
				Dir     string
				Version string
				Replace *struct{ Version string }
			}
		}
		if err := dec.Decode(&pkg); err == io.EOF {
			break
		} else if err != nil {
			return nil, fmt.Errorf("compilation closure malformed")
		}
		if pkg.Incomplete || pkg.Error != nil || pkg.Dir == "" {
			return nil, fmt.Errorf("compilation closure incomplete")
		}
		if pkg.Standard {
			continue
		}
		// Native include/link directives can read outside package subtrees.
		// go list is not a complete bound for those compiler reads.
		if len(pkg.CgoFiles)+len(pkg.CFiles)+len(pkg.CXXFiles)+len(pkg.MFiles)+len(pkg.HFiles)+len(pkg.FFiles)+len(pkg.SFiles)+len(pkg.SwigFiles)+len(pkg.SwigCXXFiles)+len(pkg.SysoFiles) > 0 {
			return nil, fmt.Errorf("native compilation read set is unbounded")
		}
		// Preserve a caller's spelling of the root (notably /var on macOS)
		// while still refusing symlinks below it in compilation inputs.
		if suffix, ok := strings.CutPrefix(pkg.Dir, lexicalRoot+string(filepath.Separator)); ok {
			pkg.Dir = filepath.Join(root, suffix)
		}
		rel, err := filepath.Rel(root, pkg.Dir)
		if err != nil {
			return nil, fmt.Errorf("compilation path unavailable")
		}
		// External module sources are pinned by go.mod/go.sum, both explicit inputs.
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			if pkg.Module == nil || pkg.Module.Main || pkg.Module.Version == "" || (pkg.Module.Replace != nil && pkg.Module.Replace.Version == "") {
				return nil, fmt.Errorf("compilation reads an unpinned external directory")
			}
			continue
		}
		// Workspace modules and local replacements have their own manifests,
		// sums and replacement graphs outside the package subtrees below. Their
		// resolution is not bounded by the root manifests: run conservatively.
		if pkg.Module != nil && ((pkg.Module.Main && pkg.Module.Dir != "" && filepath.Clean(pkg.Module.Dir) != lexicalRoot && filepath.Clean(pkg.Module.Dir) != root) || (pkg.Module.Replace != nil && pkg.Module.Replace.Version == "")) {
			return nil, fmt.Errorf("workspace or local-module compilation read set is unbounded")
		}
		if rel == "." {
			return nil, fmt.Errorf("compilation reads the root package")
		}
		physical, err := filepath.EvalSymlinks(pkg.Dir)
		if err != nil || physical != pkg.Dir {
			return nil, fmt.Errorf("compilation directory is unavailable or symbolic")
		}
		for _, files := range [][]string{pkg.GoFiles, pkg.CgoFiles, pkg.EmbedFiles, pkg.CFiles, pkg.CXXFiles, pkg.MFiles, pkg.HFiles, pkg.FFiles, pkg.SFiles, pkg.SwigFiles, pkg.SwigCXXFiles, pkg.SysoFiles} {
			for _, name := range files {
				path := filepath.Join(pkg.Dir, name)
				physical, err := filepath.EvalSymlinks(path)
				if err != nil || physical != path {
					return nil, fmt.Errorf("compilation input is unavailable or symbolic")
				}
			}
		}
		dirs = append(dirs, filepath.ToSlash(rel)+"/")
	}
	if len(dirs) == 0 {
		return nil, fmt.Errorf("compilation closure empty")
	}
	return dirs, nil
}

func decideInputs(paths, dirs, bounds []string, driver, base string) (bool, string) {
	if len(dirs) == 0 || len(bounds) == 0 {
		return true, "unknown read set: empty compilation or reader bound"
	}
	known := false
	for _, line := range bounds {
		if strings.HasPrefix(line, "unresolved "+driver+":") {
			return true, "unknown read set: " + strings.TrimSpace(line)
		}
		if strings.HasPrefix(line, "bound "+driver+" ") {
			known = true
		}
	}
	if !known {
		return true, "unknown read set: driver reader bound absent"
	}
	// These inputs bind the recipe, compiler environment and declared runtime
	// reads. Reader bounds below also admit any future declared driver input.
	explicit := []string{"Makefile", "go.mod", "go.sum", "go.work", "go.work.sum", "VERSION", ".corvint/test-read-scopes.json", ".corvint/unbounded-readers.json"}
	prefixes := append([]string{"benchmarks/selection-baseline/", ".github/cishards/"}, dirs...)
	sort.Strings(paths)
	for _, p := range paths {
		if p == "" {
			continue
		}
		for _, name := range explicit {
			if p == name {
				return true, "matching input " + p
			}
		}
		for _, prefix := range prefixes {
			if strings.HasPrefix(p, prefix) {
				return true, "matching input " + p
			}
		}
		for _, line := range bounds {
			if strings.HasPrefix(line, "bound "+driver+" ") {
				_, path, ok := strings.Cut(strings.TrimPrefix(strings.TrimSpace(line), "bound "+driver+" "), " ")
				if !ok {
					return true, "unknown read set: malformed reader bound"
				}
				if path == p {
					return true, "matching reader input " + p
				}
			}
		}
	}
	return false, "no selection-baseline input changed since " + base + ": diff is disjoint from compilation and reader bounds"
}
