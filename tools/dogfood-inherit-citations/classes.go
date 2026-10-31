// SPDX-License-Identifier: Apache-2.0
package main

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/patch"
)

// A hunk no constituent citation covers can still belong to a class whose
// governing BASE text is known: a byte-identical sealed-CEM archive or rename
// (DCW-V0-829, V1-3370) cites the seal clause, and a hunk a registered
// generator reproduces at TARGET (DCW-V0-828, V1-3369) cites that generator's
// requirements. Every proposed row is a BASE span the cite pre-flight rechecks.

// sealClause is the docs/DOGFOOD.md sentence that makes a seal rename a
// covered mechanical step.
const (
	dogfoodPath = "docs/DOGFOOD.md"
	sealClause  = "only renames its parent's CEM to `.corvint/changes/<parent>.cem.json`, is covered by that parent"
)

// requirement names one requirement entry, `- `ID`:` at a line start, in a
// spec at BASE; its span runs to the end of that list entry.
type requirement struct{ path, id string }

type generator struct {
	name     string
	owns     func(path string) bool
	cites    []requirement
	produces func(in *inheritor, h *patch.Hunk) bool
}

var specIndexFiles = map[string]bool{
	"docs/specs/REQUIREMENTS.tsv": true, "docs/specs/INDEX.json": true, "docs/specs/README.md": true,
}

var generators = []generator{
	{
		// make spec-requirements (SRG-V0-012), run by make batch-regen (ASD-V0-033).
		name: "spec-index",
		owns: func(path string) bool { return specIndexFiles[path] },
		cites: []requirement{
			{"docs/specs/spec-requirement-index-generator-v0.md", "SRG-V0-012"},
			{"docs/specs/corvint-self-development-v0.md", "ASD-V0-033"},
		},
		produces: func(in *inheritor, h *patch.Hunk) bool { return in.specIndexReproduced(h.DisplayPath) },
	},
	{
		// A moved line citation's repinned `path:N@anchor` (DCG-V0-007).
		name:  "line-citation-repin",
		owns:  func(path string) bool { return strings.HasSuffix(path, ".md") },
		cites: []requirement{{"docs/specs/documentation-citation-gate-v0.md", "DCG-V0-007"}},
		produces: func(in *inheritor, h *patch.Hunk) bool {
			return maskedEqual(h, citationToken, "`$1:#`") && in.lineCitationsPass()
		},
	},
	{
		// A use-case receipt or ledger digest repin (UCV0-004, UCV0-005).
		name: "use-case-receipt-repin",
		owns: func(path string) bool {
			return path == "conformance/use-cases-v0/ledger.json" ||
				strings.HasPrefix(path, "conformance/use-cases-v0/receipts/") && strings.HasSuffix(path, ".json")
		},
		cites: []requirement{
			{"docs/specs/use-case-conformance-v0.md", "UCV0-004"},
			{"docs/specs/use-case-conformance-v0.md", "UCV0-005"},
		},
		produces: func(in *inheritor, h *patch.Hunk) bool {
			return maskedEqual(h, hexDigest, "#") && in.digestsPinned(h)
		},
	},
}

var (
	citationToken = regexp.MustCompile("`([^`\\s]+):[0-9]+(?:[-,][0-9]+)*(?:@[0-9a-f]+)?`")
	hexDigest     = regexp.MustCompile(`\b(?:[0-9a-f]{64}|[0-9a-f]{40})\b`)
	digest64      = regexp.MustCompile(`\b[0-9a-f]{64}\b`)
	revision40    = regexp.MustCompile(`"repositoryRevision": "([0-9a-f]{40})"`)
	quotedPath    = regexp.MustCompile(`"([^"\\\s]+)"`)
)

// propose returns the rows and class of a hunk no constituent covers, or no
// rows when it is in no class or its governing text is absent at BASE.
func (in *inheritor) propose(h *patch.Hunk) ([]row, string) {
	if h == nil {
		return nil, ""
	}
	if in.sealRename(h) {
		if r, ok := in.textSpan(dogfoodPath, sealClause); ok {
			return []row{r}, "sealed-cem-rename"
		}
		return nil, ""
	}
	for _, g := range generators {
		if !g.owns(h.DisplayPath) || !g.produces(in, h) {
			continue
		}
		var rows []row
		for _, req := range g.cites {
			r, ok := in.requirementSpan(req)
			if !ok {
				return nil, ""
			}
			rows = append(rows, r)
		}
		return rows, g.name
	}
	return nil, ""
}

func (in *inheritor) textSpan(path, text string) (row, bool) {
	data, ok := in.blob(in.base + ":" + path)
	if !ok {
		return row{}, false
	}
	at := bytes.Index(data, []byte(text))
	if at < 0 {
		return row{}, false
	}
	return row{path: path, start: int64(at), end: int64(at + len(text)), relation: "specification"}, true
}

// requirementSpan is the BASE span of one requirement entry: from its `- `ID`:`
// line to the line before the next list item, heading or blank line.
func (in *inheritor) requirementSpan(req requirement) (row, bool) {
	data, ok := in.blob(in.base + ":" + req.path)
	if !ok {
		return row{}, false
	}
	marker := []byte("- `" + req.id + "`:")
	start := -1
	if bytes.HasPrefix(data, marker) {
		start = 0
	} else if at := bytes.Index(data, append([]byte{'\n'}, marker...)); at >= 0 {
		start = at + 1
	}
	if start < 0 {
		return row{}, false
	}
	end := start
	for end < len(data) {
		next := bytes.IndexByte(data[end:], '\n')
		if next < 0 {
			end = len(data)
			break
		}
		end += next + 1
		rest := data[end:]
		if len(rest) == 0 || rest[0] == '\n' || rest[0] == '#' || bytes.HasPrefix(rest, []byte("- ")) {
			end--
			break
		}
	}
	return row{path: req.path, start: int64(start), end: int64(end), relation: "specification"}, true
}

// sealRename reports a whole-file addition or deletion of a sealed CEM path
// whose blob the range also removes or adds under a CEM path, or which is the
// sidecar CEM its name's commit holds: the same bytes moved, nothing edited.
func (in *inheritor) sealRename(h *patch.Hunk) bool {
	name, ok := strings.CutPrefix(h.DisplayPath, sealedDirectory)
	if !ok || !strings.HasSuffix(name, ".cem.json") || strings.Contains(name, "/") {
		return false
	}
	in.loadSealedTrees()
	baseOid, atBase := in.sealedAt[0][h.DisplayPath]
	targetOid, atTarget := in.sealedAt[1][h.DisplayPath]
	switch {
	case !atBase && atTarget && wholeFile(h, '+'):
		if in.removedCEMs[targetOid] {
			return true
		}
		oid, err := in.git(nil, "rev-parse", "--verify", "--quiet", "--end-of-options",
			strings.TrimSuffix(name, ".cem.json")+":"+changeMapPath)
		return err == nil && oid == targetOid
	case atBase && !atTarget && wholeFile(h, '-'):
		return in.addedCEMs[baseOid]
	}
	return false
}

func wholeFile(h *patch.Hunk, prefix byte) bool {
	if len(h.Body) == 0 {
		return false
	}
	for _, line := range h.Body {
		if line.Prefix != prefix && line.Prefix != '\\' {
			return false
		}
	}
	return true
}

// loadSealedTrees reads the sealed CEM blobs at BASE and TARGET once, and the
// sidecar CEMs the range's commits replace or remove.
func (in *inheritor) loadSealedTrees() {
	if in.sealedAt[0] != nil {
		return
	}
	in.removedCEMs, in.addedCEMs = map[string]bool{}, map[string]bool{}
	for i, rev := range []string{in.base, in.target} {
		in.sealedAt[i] = map[string]string{}
		listing, _ := in.git(nil, "ls-tree", "-r", "--end-of-options", rev, "--", sealedDirectory, changeMapPath)
		for _, line := range strings.Split(listing, "\n") {
			meta, path, ok := strings.Cut(line, "\t")
			if fields := strings.Fields(meta); ok && len(fields) == 3 {
				in.sealedAt[i][path] = fields[2]
			}
		}
	}
	for path, oid := range in.sealedAt[0] {
		if in.sealedAt[1][path] != oid {
			in.removedCEMs[oid] = true
		}
	}
	for path, oid := range in.sealedAt[1] {
		if _, ok := in.sealedAt[0][path]; !ok {
			in.addedCEMs[oid] = true
		}
	}
	// A sidecar a range commit wrote and a later commit moved is removed in between.
	commits, _ := in.git(nil, "log", "--format=%H", "--end-of-options", in.base+".."+in.target, "--", changeMapPath)
	for _, commit := range strings.Fields(commits) {
		if oid, err := in.git(nil, "rev-parse", "--verify", "--quiet", commit+":"+changeMapPath); err == nil {
			in.removedCEMs[oid] = true
		}
	}
}

// maskedEqual reports a hunk whose removed and added lines are equal, line for
// line and nonempty, once every pattern match is replaced: only generated
// tokens changed.
func maskedEqual(h *patch.Hunk, pattern *regexp.Regexp, replacement string) bool {
	var removed, added []string
	for _, line := range h.Body {
		switch line.Prefix {
		case '-':
			removed = append(removed, pattern.ReplaceAllString(string(line.Payload), replacement))
		case '+':
			added = append(added, pattern.ReplaceAllString(string(line.Payload), replacement))
		}
	}
	if len(removed) == 0 || len(removed) != len(added) {
		return false
	}
	for i := range removed {
		if removed[i] != added[i] {
			return false
		}
	}
	return true
}

// digestsPinned reports that every 64-hex digest an added line writes is the
// SHA-256 of a TARGET file the same file names, and every new
// repositoryRevision is a commit: the repin script's output, re-derived.
func (in *inheritor) digestsPinned(h *patch.Hunk) bool {
	data, ok := in.blob(in.target + ":" + h.DisplayPath)
	if !ok {
		return false
	}
	pinned := map[string]bool{}
	for _, match := range quotedPath.FindAllSubmatch(data, -1) {
		path := string(match[1])
		if !strings.Contains(path, "/") {
			continue
		}
		if content, ok := in.blob(in.target + ":" + path); ok {
			sum := sha256.Sum256(content)
			pinned[hex.EncodeToString(sum[:])] = true
		}
	}
	for _, line := range h.Body {
		if line.Prefix != '+' {
			continue
		}
		for _, digest := range digest64.FindAllString(string(line.Payload), -1) {
			if !pinned[digest] {
				return false
			}
		}
		for _, match := range revision40.FindAllStringSubmatch(string(line.Payload), -1) {
			if _, err := in.git(nil, "cat-file", "-e", match[1]+"^{commit}"); err != nil {
				return false
			}
		}
	}
	return true
}

// specIndexReproduced runs TARGET's generator over TARGET's specs and reports
// whether it writes the file byte for byte as TARGET holds it.
func (in *inheritor) specIndexReproduced(path string) bool {
	if in.specIndex == nil {
		in.specIndex = map[string]bool{}
		out, err := in.atTarget("gen-spec-requirements", func(root string) error {
			dir := filepath.Join(root, ".generated")
			if err := os.Mkdir(dir, 0o700); err != nil {
				return err
			}
			return in.runScript(root, "script/gen-spec-requirements.sh", "--all", dir)
		})
		for file := range specIndexFiles {
			generated, ok := out[filepath.Join(".generated", strings.TrimPrefix(file, "docs/specs/"))]
			committed, found := in.blob(in.target + ":" + file)
			in.specIndex[file] = err == nil && ok && found && bytes.Equal(generated, committed)
		}
	}
	return in.specIndex[path]
}

// lineCitationsPass runs TARGET's line-citation gate over TARGET's index once.
func (in *inheritor) lineCitationsPass() bool {
	if in.citations == 0 {
		in.citations = -1
		if _, err := in.atTarget("check-line-citations", func(root string) error {
			return in.runScript(root, "script/check-line-citations.sh")
		}); err == nil {
			in.citations = 1
		}
	}
	return in.citations == 1
}

// atTarget materializes TARGET's script/ directory in a private root and runs
// work there against a private index read from TARGET; scripts that read the
// Git index see TARGET exactly, and the repository is never written. It
// returns the root's regular files after work, keyed by relative path.
func (in *inheritor) atTarget(label string, work func(root string) error) (map[string][]byte, error) {
	root, err := os.MkdirTemp("", "dogfood-inherit-"+label+"-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(root)
	archive, err := in.gitBytes(nil, "archive", "--format=tar", "--end-of-options", in.target, "script")
	if err != nil {
		return nil, err
	}
	if err := untar(archive, root); err != nil {
		return nil, err
	}
	if in.gitDir == "" {
		if in.gitDir, err = in.git(nil, "rev-parse", "--absolute-git-dir"); err != nil {
			return nil, err
		}
	}
	index := filepath.Join(root, ".index")
	read := exec.Command("git", "--git-dir="+in.gitDir, "read-tree", in.target)
	read.Env = append(os.Environ(), "GIT_INDEX_FILE="+index)
	read.Stdin = bytes.NewReader(nil)
	if err := read.Run(); err != nil {
		return nil, err
	}
	in.privateIndex = index
	if err := work(root); err != nil {
		return nil, err
	}
	out := map[string][]byte{}
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if strings.HasPrefix(rel, ".generated") {
			out[rel], err = os.ReadFile(path)
		}
		return err
	})
	return out, err
}

func (in *inheritor) runScript(root, script string, args ...string) error {
	cmd := exec.Command("sh", append([]string{filepath.Join(root, script)}, args...)...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GIT_DIR="+in.gitDir, "GIT_WORK_TREE="+root, "GIT_INDEX_FILE="+in.privateIndex,
		"GIT_OPTIONAL_LOCKS=0", "LC_ALL=C")
	cmd.Stdin = bytes.NewReader(nil)
	return cmd.Run()
}

func untar(archive []byte, root string) error {
	reader := tar.NewReader(bytes.NewReader(archive))
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(root, filepath.FromSlash(header.Name))
		if !strings.HasPrefix(target, root+string(filepath.Separator)) {
			continue
		}
		switch header.Typeflag {
		case tar.TypeDir:
			err = os.MkdirAll(target, 0o700)
		case tar.TypeReg:
			if err = os.MkdirAll(filepath.Dir(target), 0o700); err == nil {
				err = writeFrom(target, reader, header.FileInfo().Mode().Perm())
			}
		}
		if err != nil {
			return err
		}
	}
}

func writeFrom(path string, reader io.Reader, mode os.FileMode) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode|0o600)
	if err != nil {
		return err
	}
	_, err = io.Copy(file, reader)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	return err
}
