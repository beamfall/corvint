package flowcoverage

import (
	"bytes"
	"context"
	"fmt"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/flowdocs"
	"github.com/Beamfall/corvint/internal/gitstatus"
	"github.com/Beamfall/corvint/internal/gokernel"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

type source struct {
	indexChanged              map[string]bool
	ctx                       context.Context
	auth                      *gitauth.Repository
	root, current, repository string
	sourceRevision            string
	checked                   map[string]bool
}

func safe(p string) bool { return p != "." && fs.ValidPath(p) && !strings.ContainsAny(p, "\\\x00\r\n") }
func (s *source) blob(rev, p string, limit int) ([]byte, string, error) {
	if !safe(p) {
		return nil, "", fmt.Errorf("invalid immutable path")
	}
	en, ok, e := s.auth.LookupTreeEntry(s.ctx, rev, p)
	if e != nil {
		return nil, "", e
	}
	if !ok || en.Type != "blob" || (en.Mode != "100644" && en.Mode != "100755") {
		return nil, "", fmt.Errorf("immutable regular file missing: %s", p)
	}
	b, e := s.auth.BlobBytes(s.ctx, en.OID)
	if len(b) > limit {
		return nil, "", fmt.Errorf("source byte bound exceeded")
	}
	return b, en.OID, e
}
func (s *source) ref(r Ref, limit int) ([]byte, error) {
	b, _, e := s.blob(r.Revision, r.Path, limit)
	if e == nil && digest(b) != r.SHA256 {
		e = fmt.Errorf("immutable reference digest mismatch: %s", r.Path)
	}
	return b, e
}
func (s *source) unchanged(rev, p string) bool {
	key := rev + ":" + p
	if v, ok := s.checked[key]; ok {
		return v
	}
	old, _, e := s.blob(rev, p, flowdocs.MaxManifestBytes)
	cur, _, e2 := s.blob(s.current, p, flowdocs.MaxManifestBytes)
	actual, e3 := flowdocs.ReadFile(path.Join(s.root, p))
	v := !s.indexChanged[p] && e == nil && e2 == nil && e3 == nil && digest(old) == digest(cur) && digest(cur) == digest(actual)
	s.checked[key] = v
	return v
}
func (s *source) anchor(a doccorpus.Anchor) (bool, error) {
	if a.Repository != s.repository || a.Reason == "" || !strings.Contains(" syntax source-document external-provider ", " "+a.Authority+" ") || a.Authority == "" || !strings.Contains(" source declared observed review imported ", " "+a.Kind+" ") || a.Kind == "" {
		return false, fmt.Errorf("anchor repository/reason mismatch")
	}
	b, oid, e := s.blob(a.Revision, a.Path, 4<<20)
	if e != nil {
		return false, e
	}
	lines := bytes.SplitAfter(b, []byte{'\n'})
	if a.Start < 1 || a.End < a.Start || a.End > len(lines) || a.Blob != oid || a.SHA256 != digest(b) || a.SpanSHA256 != digest(bytes.Join(lines[a.Start-1:a.End], nil)) {
		return false, fmt.Errorf("anchor binding mismatch: %s", a.Path)
	}
	span := bytes.Join(lines[a.Start-1:a.End], nil)
	if len(span) > 64<<10 || len(bytes.TrimSpace(span)) == 0 {
		return false, fmt.Errorf("vacuous or oversized anchor span")
	}
	return s.unchanged(a.Revision, a.Path), nil
}
func (s *source) files(rev, dir string) ([]string, error) {
	if !safe(dir) {
		return nil, fmt.Errorf("invalid directory")
	}
	names, e := s.auth.TreePaths(s.ctx, rev, dir)
	if e != nil {
		return nil, e
	}
	out := []string{}
	for _, p := range names {
		en, ok, e := s.auth.LookupTreeEntry(s.ctx, rev, p)
		if e != nil {
			return nil, e
		}
		if !ok {
			return nil, fmt.Errorf("missing tree entry")
		}
		if en.Type == "tree" {
			continue
		}
		if en.Type != "blob" || (en.Mode != "100644" && en.Mode != "100755") {
			return nil, fmt.Errorf("nonregular entry in bound directory")
		}
		out = append(out, p)
	}
	sort.Strings(out)
	if len(out) > 4096 {
		return nil, fmt.Errorf("directory bound exceeded")
	}
	return out, nil
}
func (s *source) build(rev, dir string) (string, bool, error) {
	names, e := s.files(rev, dir)
	if e != nil {
		return "", false, e
	}
	if len(names) == 0 {
		return "", false, nil
	}
	var v strings.Builder
	fresh := true
	total := 0
	for _, p := range names {
		b, _, e := s.blob(rev, p, 4<<20)
		if e != nil {
			return "", false, e
		}
		total += len(b)
		if total > 128<<20 {
			return "", false, fmt.Errorf("build bound exceeded")
		}
		fmt.Fprintf(&v, "%s=%s\n", strings.TrimPrefix(p, dir+"/"), digest(b))
		fresh = fresh && s.unchanged(rev, p)
	}
	current, e := s.files(s.current, dir)
	fresh = fresh && e == nil && strings.Join(names, "\n") == strings.Join(current, "\n")
	actual := []string{}
	walkErr := filepath.WalkDir(filepath.Join(s.root, dir), func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("build symlink")
		}
		rel, e := filepath.Rel(s.root, p)
		if e != nil {
			return e
		}
		actual = append(actual, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(actual)
	fresh = fresh && walkErr == nil && strings.Join(actual, "\n") == strings.Join(names, "\n")
	return digest([]byte(v.String())), fresh, nil
}

// directoryFresh checks the whole declared inventory, including dirty additions.
func (s *source) directoryFresh(rev, dir string) bool {
	before, e := s.files(rev, dir)
	if e != nil {
		return false
	}
	after, e := s.files(s.current, dir)
	if e != nil || strings.Join(before, "\n") != strings.Join(after, "\n") {
		return false
	}
	actual := []string{}
	e = filepath.WalkDir(filepath.Join(s.root, dir), func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink in inventory")
		}
		rel, e := filepath.Rel(s.root, p)
		if e != nil {
			return e
		}
		actual = append(actual, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(actual)
	if e != nil || strings.Join(before, "\n") != strings.Join(actual, "\n") {
		return false
	}
	for _, p := range before {
		if !s.unchanged(rev, p) {
			return false
		}
	}
	return true
}

func (s *source) indexChanges() (map[string]bool, error) {
	b, e := gitrun.Run(s.ctx, gitrun.NewDefaultBudget(), gitrun.Options{Binary: gitstatus.Executable(), Dir: s.root, Env: gokernel.SanitizedGitEnvironment(), StdoutLimit: 4 << 20}, "--no-optional-locks", "-c", "core.fsmonitor=false", "-c", "credential.helper=", "diff", "--cached", "--name-only", "--no-renames", "-z", s.current, "--")
	if e != nil {
		return nil, e
	}
	out := map[string]bool{}
	for _, p := range strings.Split(string(b), "\x00") {
		if p != "" {
			out[p] = true
		}
	}
	return out, nil
}
