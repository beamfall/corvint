package localcompletion

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// exportProfile names an enrollment export bundle (LCP-V0-883).
const exportProfile = "corvint-dogfood-enrollment-export/0"

// Export bounds: at most maxExportFiles files of at most maxArtifactBytes
// each, and maxExportBytes in all.
const (
	maxExportFiles = 512
	maxExportBytes = 64 << 20
)

// exportFile is one exported file: under the session directory ("session"),
// the private Git directory ("gitdir") or the worktree ("root"), by its
// slash-separated relative path, with its SHA-256 and bytes.
type exportFile struct {
	Anchor string `json:"anchor"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Data   string `json:"data"`
}

// exportBundle is the export: the enrollment's identity, the immutable commit
// and tree it is bound to, the source locations its saved paths name, and its
// files. Digest is the SHA-256 of the bundle with Digest empty.
type exportBundle struct {
	Profile    string       `json:"profile"`
	Session    string       `json:"session"`
	Lifecycle  string       `json:"lifecycle"`
	PlanDigest string       `json:"planDigest"`
	Base       string       `json:"base"`
	Head       string       `json:"head"`
	Tree       string       `json:"tree"`
	Root       string       `json:"root"`
	GitDir     string       `json:"gitDir"`
	Files      []exportFile `json:"files"`
	Digest     string       `json:"digest"`
}

func (bundle exportBundle) digest() string {
	bundle.Digest = ""
	return valueDigest(bundle)
}

// Export writes, to a new file at output, the enrollment of key with every
// file of its session directory and every worktree or Git-directory artifact
// its state names, bound to HEAD's commit and tree (LCP-V0-883). It changes no
// repository or enrollment state. An aggregate-outcome or sealed-continuation
// enrollment is refused: its ledgers name paths this bundle does not relocate.
func Export(ctx context.Context, root, key, output string) (Evaluation, error) {
	repo, err := open(root, key)
	if err != nil {
		return Evaluation{}, err
	}
	unlock, err := repo.lock()
	if err != nil {
		return Evaluation{}, err
	}
	defer unlock()
	saved, err := repo.load()
	if os.IsNotExist(err) {
		return Evaluation{}, errors.New("enrollment-absent")
	}
	if err != nil {
		return Evaluation{}, err
	}
	if saved.AggregateOutcome != nil || saved.Sealed != nil {
		return Evaluation{}, errors.New("enrollment-export-unsupported")
	}
	if _, err = os.Lstat(output); err == nil {
		return Evaluation{}, errors.New("export-output-exists")
	}
	snap, err := repo.snapshot(ctx)
	if err != nil {
		return Evaluation{}, err
	}
	bundle := exportBundle{Profile: exportProfile, Session: key, Lifecycle: saved.Lifecycle, PlanDigest: saved.PlanDigest, Base: saved.Plan.Base,
		Head: snap.target, Tree: snap.tree, Root: repo.auth.Root, GitDir: repo.auth.GitDir}
	total := 0
	add := func(anchor, base, name string) error {
		relative, err := filepath.Rel(base, name)
		if err != nil || !validPath(filepath.ToSlash(relative)) {
			return errors.New("export-path-invalid")
		}
		raw, err := readFile(name, maxArtifactBytes)
		if err != nil {
			return err
		}
		total += len(raw)
		if len(bundle.Files) >= maxExportFiles || total > maxExportBytes {
			return errors.New("export-bound-exceeded")
		}
		bundle.Files = append(bundle.Files, exportFile{Anchor: anchor, Path: filepath.ToSlash(relative), SHA256: digest(raw), Data: base64.StdEncoding.EncodeToString(raw)})
		return nil
	}
	session := filepath.Join(repo.directory, key)
	err = filepath.WalkDir(session, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return errors.New("local-state-symlink")
		}
		if entry.Type().IsRegular() {
			return add("session", session, name)
		}
		return nil
	})
	if err != nil {
		return Evaluation{}, err
	}
	// The preservation hold and its release records live beside the session
	// directory; a restore without them would admit a cancel the hold refuses
	// (LCP-V0-880, LCP-V0-883).
	holds, err := repo.holdFiles()
	if err != nil {
		return Evaluation{}, err
	}
	for _, name := range holds {
		if err = add("gitdir", repo.auth.GitDir, name); err != nil {
			return Evaluation{}, err
		}
	}
	seen := map[string]bool{}
	for _, item := range repo.referencedArtifacts(saved) {
		if seen[item.Path] || strings.HasPrefix(item.Path, session+string(filepath.Separator)) {
			continue
		}
		seen[item.Path] = true
		anchor, base := "root", repo.auth.Root
		if relative, relErr := filepath.Rel(repo.auth.GitDir, item.Path); relErr == nil && validPath(filepath.ToSlash(relative)) {
			anchor, base = "gitdir", repo.auth.GitDir
		}
		if err = add(anchor, base, item.Path); err != nil {
			return Evaluation{}, err
		}
	}
	bundle.Digest = bundle.digest()
	raw, err := json.Marshal(bundle)
	if err != nil {
		return Evaluation{}, err
	}
	file, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return Evaluation{}, errors.New("export-output-unwritable")
	}
	_, err = file.Write(append(raw, '\n'))
	if err = errors.Join(err, file.Close()); err != nil {
		return Evaluation{}, errors.New("export-output-unwritable")
	}
	return repo.evaluate(ctx, saved)
}

// holdFiles are the session's hold and release records that exist.
func (repo *repository) holdFiles() ([]string, error) {
	var names []string
	if _, err := os.Lstat(repo.holdPath()); err == nil {
		names = append(names, repo.holdPath())
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	entries, err := os.ReadDir(repo.releasesPath())
	if os.IsNotExist(err) {
		return names, nil
	}
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		names = append(names, filepath.Join(repo.releasesPath(), entry.Name()))
	}
	return names, nil
}

// referencedArtifacts are the artifacts the saved state names by absolute
// path: report-set bindings and reports, terminal artifacts and coordination.
func (repo *repository) referencedArtifacts(saved *state) []artifact {
	var items []artifact
	if saved.ReportSet != nil {
		items = append(append(items, saved.ReportSet.Bindings...), saved.ReportSet.Reports...)
	}
	if saved.Terminal != nil {
		items = append(items, saved.Terminal.Artifacts...)
	}
	return append(items, saved.Coordination...)
}

// readBundle reads and verifies an export bundle for key. As ReadPlan does,
// it reads through the resolved parent directory, so a symlinked ancestor
// such as macOS /tmp is admitted while a symlinked bundle is not (LCP-V0-884).
func readBundle(input, key string) (*exportBundle, error) {
	parent, err := filepath.EvalSymlinks(filepath.Dir(input))
	if err != nil {
		return nil, errors.New("import-bundle-unreadable")
	}
	raw, err := readFile(filepath.Join(parent, filepath.Base(input)), maxExportBytes*2)
	if err != nil {
		return nil, errors.New("import-bundle-unreadable")
	}
	var bundle exportBundle
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&bundle) != nil || bundle.Profile != exportProfile || bundle.Digest != bundle.digest() {
		return nil, errors.New("import-bundle-invalid")
	}
	if bundle.Session != key {
		return nil, errors.New("import-session-mismatch")
	}
	if len(bundle.Files) > maxExportFiles {
		return nil, errors.New("import-bundle-invalid")
	}
	for _, file := range bundle.Files {
		data, decodeErr := base64.StdEncoding.DecodeString(file.Data)
		if decodeErr != nil || digest(data) != file.SHA256 || !validPath(file.Path) || (file.Anchor != "session" && file.Anchor != "gitdir" && file.Anchor != "root") {
			return nil, errors.New("import-bundle-invalid")
		}
	}
	return &bundle, nil
}

// Import restores an exported enrollment into this worktree (LCP-V0-884): HEAD
// must be the bundle's exact commit and tree, the session must have no state
// here and no other session may own the worktree. Session and Git-directory
// files are written; a worktree artifact must already hold the exported bytes
// or be absent and ignored by Git. Saved paths are relocated from the source
// root and Git directory to this one, and the report-set digest, and the
// review and terminal acknowledgments of exactly that report set, are
// recomputed over the relocated paths; nothing else changes, so the restored
// enrollment evaluates as it did at its source, never more satisfied.
func Import(ctx context.Context, root, key, input string) (Evaluation, error) {
	bundle, err := readBundle(input, key)
	if err != nil {
		return Evaluation{}, err
	}
	repo, err := open(root, key)
	if err != nil {
		return Evaluation{}, err
	}
	unlock, err := repo.lock()
	if err != nil {
		return Evaluation{}, err
	}
	defer unlock()
	if _, err = os.Lstat(filepath.Join(repo.directory, key)); err == nil {
		return Evaluation{}, errors.New("import-would-replace")
	}
	if owner, ownerErr := repo.owner(); ownerErr != nil {
		return Evaluation{}, ownerErr
	} else if owner != "" && owner != key {
		return Evaluation{}, errors.New("worktree-already-enrolled")
	}
	snap, err := repo.snapshot(ctx)
	if err != nil {
		return Evaluation{}, err
	}
	if snap.target != bundle.Head || snap.tree != bundle.Tree {
		return Evaluation{}, errors.New("import-target-mismatch")
	}
	if base, resolveErr := repo.auth.Resolve(ctx, bundle.Base); resolveErr != nil || base != bundle.Base {
		return Evaluation{}, errors.New("base-unavailable")
	}
	session := filepath.Join(repo.directory, key)
	destination := func(file exportFile) string {
		switch file.Anchor {
		case "session":
			return filepath.Join(session, filepath.FromSlash(file.Path))
		case "gitdir":
			return filepath.Join(repo.auth.GitDir, filepath.FromSlash(file.Path))
		}
		return filepath.Join(repo.auth.Root, filepath.FromSlash(file.Path))
	}
	// Every worktree artifact is checked before anything is written.
	pending := []exportFile{}
	for _, file := range bundle.Files {
		name := destination(file)
		current, readErr := readFile(name, maxArtifactBytes)
		switch {
		case readErr == nil && digest(current) == file.SHA256:
			continue
		case readErr == nil:
			return Evaluation{}, errors.New("import-artifact-conflict")
		case !os.IsNotExist(readErr):
			return Evaluation{}, readErr
		case file.Anchor == "root":
			if _, ignoredErr := repo.git(ctx, "check-ignore", "-q", "--no-index", "--", file.Path); ignoredErr != nil {
				return Evaluation{}, errors.New("import-artifact-not-ignored")
			}
		}
		pending = append(pending, file)
	}
	sort.SliceStable(pending, func(i, j int) bool { return pending[i].Path != "state.json" && pending[j].Path == "state.json" })
	// The state is relocated, and may refuse, before anything is written.
	contents := make([][]byte, len(pending))
	for index, file := range pending {
		contents[index], _ = base64.StdEncoding.DecodeString(file.Data)
		if file.Anchor == "session" && file.Path == "state.json" {
			if contents[index], err = relocateState(contents[index], bundle, repo); err != nil {
				return Evaluation{}, err
			}
		}
	}
	for index, file := range pending {
		if err = writeFile(destination(file), contents[index]); err != nil {
			return Evaluation{}, err
		}
	}
	if err = writeFile(filepath.Join(repo.directory, "owner"), []byte(key+"\n")); err != nil {
		return Evaluation{}, err
	}
	saved, err := repo.load()
	if err != nil {
		return Evaluation{}, err
	}
	return repo.evaluate(ctx, saved)
}

// relocateState rewrites every saved path under the bundle's source Git
// directory or root to this repository's, then recomputes the report-set
// digest, when it held at the source, and carries the review and terminal
// acknowledgments only when they named that exact report set. The frozen plan
// is carried unchanged, since its digest binds it; one that names the source
// root or Git directory cannot move to another location (LCP-V0-884,
// LCP-V0-885).
func relocateState(raw []byte, bundle *exportBundle, repo *repository) ([]byte, error) {
	var value map[string]any
	var source state
	if json.Unmarshal(raw, &value) != nil || json.Unmarshal(raw, &source) != nil {
		return nil, errors.New("import-bundle-invalid")
	}
	if (bundle.Root != repo.auth.Root || bundle.GitDir != repo.auth.GitDir) && namesAny(value["plan"], bundle.Root, bundle.GitDir) {
		return nil, errors.New("import-plan-not-relocatable")
	}
	// Only a report set whose digest held at its source is re-digested, so a
	// stale one stays stale after the move.
	sourceValid := false
	if set := source.ReportSet; set != nil {
		copy := *set
		copy.Digest = ""
		sourceValid = valueDigest(copy) == set.Digest
	}
	prefixes := [][2]string{{bundle.GitDir + string(filepath.Separator), repo.auth.GitDir + string(filepath.Separator)}, {bundle.Root + string(filepath.Separator), repo.auth.Root + string(filepath.Separator)}}
	var walk func(any) any
	walk = func(node any) any {
		switch typed := node.(type) {
		case map[string]any:
			for name, member := range typed {
				typed[name] = walk(member)
			}
		case []any:
			for index, member := range typed {
				typed[index] = walk(member)
			}
		case string:
			for _, prefix := range prefixes {
				if strings.HasPrefix(typed, prefix[0]) {
					return prefix[1] + strings.TrimPrefix(typed, prefix[0])
				}
			}
		}
		return node
	}
	plan := value["plan"]
	delete(value, "plan")
	walk(value)
	value["plan"] = plan
	relocated, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var saved state
	if err = strictJSON(relocated, &saved, maxStateBytes); err != nil {
		return nil, err
	}
	if set := saved.ReportSet; set != nil && sourceValid {
		previous := set.Digest
		set.Digest = ""
		set.Digest = valueDigest(*set)
		if saved.Review == previous {
			saved.Review = set.Digest
		}
		if saved.Terminal != nil && saved.Terminal.ReportSet == previous {
			saved.Terminal.ReportSet = set.Digest
		}
	}
	out, err := json.Marshal(saved)
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

// namesAny reports whether any string in node names one of dirs: holds it
// followed by a path separator or the end of the string.
func namesAny(node any, dirs ...string) bool {
	switch typed := node.(type) {
	case map[string]any:
		for _, member := range typed {
			if namesAny(member, dirs...) {
				return true
			}
		}
	case []any:
		for _, member := range typed {
			if namesAny(member, dirs...) {
				return true
			}
		}
	case string:
		for _, dir := range dirs {
			for rest := typed; dir != ""; {
				at := strings.Index(rest, dir)
				if at < 0 {
					break
				}
				rest = rest[at+len(dir):]
				if rest == "" || rest[0] == filepath.Separator {
					return true
				}
			}
		}
	}
	return false
}
