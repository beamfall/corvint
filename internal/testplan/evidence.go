package testplan

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

// MaxEvidenceFiles bounds --tests and --map each (TCN-V0-003, TCN-V0-004).
const MaxEvidenceFiles = 8

// Request is one consolidation request. Tests and Maps are file paths as the caller resolved them;
// Root is the repository Git is read from. MaxSteps 0 is the default.
type Request struct {
	Root     string
	Input    []byte
	Tests    []string
	Maps     []string
	Revision string
	MaxSteps int
}

// Run reads the named evidence and builds the plan. It reads only the input bytes, the named
// files and Git; it writes nothing (TCN-V0-001).
func Run(ctx context.Context, req Request) (*Plan, error) {
	maxSteps := req.MaxSteps
	if maxSteps == 0 {
		maxSteps = DefaultMaxSteps
	}
	if maxSteps < MinMaxSteps || maxSteps > MaxMaxSteps {
		return nil, invalidArguments("--max-steps must be %d..%d", MinMaxSteps, MaxMaxSteps)
	}
	if len(req.Tests) > MaxEvidenceFiles || len(req.Maps) > MaxEvidenceFiles {
		return nil, invalidArguments("at most %d --tests and %d --map files", MaxEvidenceFiles, MaxEvidenceFiles)
	}
	if req.Revision != "" && len(req.Tests) == 0 && len(req.Maps) == 0 {
		return nil, invalidArguments("--revision needs --map or --tests")
	}
	in, err := DecodeInput(req.Input)
	if err != nil {
		return nil, err
	}
	evidence, err := gather(ctx, req, in)
	if err != nil {
		return nil, err
	}
	return Build(in, evidence, maxSteps)
}

// gather reads the maps, provider documents and Git evidence the request names.
func gather(ctx context.Context, req Request, in *Input) (Evidence, error) {
	evidence := Evidence{Revision: none, TestsDigest: none}
	if len(req.Tests) == 0 && len(req.Maps) == 0 {
		return evidence, nil
	}
	maps := make([]*appmap.Map, 0, len(req.Maps))
	apps := map[string]bool{}
	for _, name := range req.Maps {
		m, err := appmap.LoadMap(name)
		if err != nil {
			return Evidence{}, err
		}
		if apps[m.App] {
			return Evidence{}, &gokernel.Error{Code: "appmap-invalid-map", Message: "two --map files describe app " + m.App}
		}
		apps[m.App] = true
		maps = append(maps, m)
	}
	type providerFile struct {
		digest string
		input  testvaliditydoc.Input
	}
	providers := []providerFile{}
	for _, name := range req.Tests {
		data, err := readProvider(name)
		if err != nil {
			return Evidence{}, err
		}
		input, err := testvaliditydoc.Decode(data)
		if err != nil {
			return Evidence{}, invalidReceipt()
		}
		sum := sha256.Sum256(data)
		providers = append(providers, providerFile{digest: hex.EncodeToString(sum[:]), input: input})
	}
	sort.SliceStable(providers, func(i, j int) bool { return providers[i].digest < providers[j].digest })

	root, err := filepath.Abs(req.Root)
	if err != nil {
		return Evidence{}, invalidArguments("--root is unreadable")
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return Evidence{}, invalidArguments("--root is unreadable")
	}
	r := repo{ctx: ctx, root: resolvedRoot}
	top, err := r.top()
	if err != nil {
		return Evidence{}, invalidArguments("--root is not inside a Git worktree")
	}
	r.root = top
	revision := req.Revision
	if revision == "" {
		revision = "HEAD"
	}
	evaluated, err := r.resolve(revision)
	if err != nil {
		return Evidence{}, invalidArguments("--revision does not name a commit")
	}
	evidence.Revision = evaluated

	if len(req.Maps) > 0 {
		evidence.Maps = []MapView{}
		for _, m := range maps {
			evidence.Maps = append(evidence.Maps, mapView(ctx, top, evaluated, m, in))
		}
	}
	if len(providers) > 0 {
		// Bound paths are compared under the worktree top as Git and the provider name it.
		prefixes := []string{top}
		if root != resolvedRoot && resolvedRoot == top {
			prefixes = append(prefixes, root)
		}
		relative := map[string]string{}
		paths := []string{}
		for _, provider := range providers {
			for _, path := range testvaliditydoc.BoundPaths(provider.input) {
				if rel, ok := worktreePath(prefixes, path); ok && relative[path] == "" {
					relative[path] = rel
					paths = append(paths, rel)
				}
			}
		}
		sort.Strings(paths)
		sources, err := r.boundSources(evaluated, uniqueSorted(paths))
		if err != nil {
			if coded, ok := err.(*gokernel.Error); ok {
				return Evidence{}, coded
			}
			return Evidence{}, invalidArguments("Git could not read the bound sources at the evaluated revision")
		}
		compare := func(path, digest string) string {
			rel, ok := relative[path]
			if !ok {
				return "retained-bound-path-outside-worktree"
			}
			source, ok := sources[rel]
			switch {
			case !ok:
				return "retained-digest-mismatch"
			case source.reason != "":
				return source.reason
			case source.digest != digest:
				return "retained-digest-mismatch"
			}
			return ""
		}
		digests := []string{}
		for _, provider := range providers {
			evidence.Documents = append(evidence.Documents, testvaliditydoc.ProjectBound(provider.input, compare))
			digests = append(digests, provider.digest)
		}
		evidence.TestsDigest = digestList(digests)
	}
	return evidence, nil
}

// mapView validates the input's screens of one map's app at the evaluated revision.
func mapView(ctx context.Context, top, revision string, m *appmap.Map, in *Input) MapView {
	view := MapView{App: m.App, Digest: m.Digest, Screens: map[string]MapScreen{}}
	ids := []string{}
	for i := range in.Variations {
		v := &in.Variations[i]
		if v.App != nil && *v.App == m.App && v.Screen != nil {
			ids = append(ids, *v.Screen)
		}
	}
	lineage := appmap.LineageFreshness(ctx, top, revision, m, uniqueSorted(sortedCopy(ids)))
	for _, s := range m.Screens {
		state, ok := lineage[s.ID]
		if !ok {
			continue
		}
		view.Screens[s.ID] = MapScreen{Resolved: s.Status == appmap.StatusResolved && !s.Abstract, Template: s.Template, Lineage: state}
	}
	return view
}

// worktreePath maps an absolute bound path to a local repository-relative path that does not name
// .git; anything else cannot be compared (retained-bound-path-outside-worktree).
func worktreePath(prefixes []string, path string) (string, bool) {
	if !filepath.IsAbs(path) {
		return "", false
	}
	for _, prefix := range prefixes {
		rel, err := filepath.Rel(prefix, path)
		if err != nil || rel == "." || !filepath.IsLocal(rel) {
			continue
		}
		rel = filepath.ToSlash(rel)
		for _, part := range strings.Split(rel, "/") {
			if strings.EqualFold(part, ".git") {
				return "", false
			}
		}
		return rel, true
	}
	return "", false
}

func uniqueSorted(sorted []string) []string {
	out := []string{}
	for i, s := range sorted {
		if i == 0 || s != sorted[i-1] {
			out = append(out, s)
		}
	}
	return out
}

func invalidReceipt() error {
	return &gokernel.Error{Code: "invalid-test-validity-receipt", Message: "--tests must name one bounded provider document test-validity accepts"}
}

// readProvider reads one --tests file through the test-validity safe reader and bound.
func readProvider(name string) ([]byte, error) {
	absolute, err := filepath.Abs(name)
	if err != nil {
		return nil, invalidReceipt()
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return nil, invalidReceipt()
	}
	root, err := os.OpenRoot(filepath.Dir(resolved))
	if err != nil {
		return nil, invalidReceipt()
	}
	defer root.Close()
	data, err := testvaliditydoc.ReadFile(root, filepath.Base(resolved))
	if err != nil {
		return nil, invalidReceipt()
	}
	return data, nil
}
