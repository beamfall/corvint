package breakagemap

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/extevidence"
)

var spanSuffix = regexp.MustCompile(`^([1-9][0-9]{0,5})-([1-9][0-9]{0,5})$`)

// BuildManifest pins the HEAD commit, tree, origin and whole-file blob spans
// of explicitly named sources in explicitly bound checkouts (BKM-V0-010). It
// only reads Git objects; it never discovers checkouts or sources, and the
// result is round-tripped through Decode so it cannot emit a manifest the
// compiler would refuse.
func BuildManifest(ctx context.Context, bindings map[string]string, sources []string) (Manifest, error) {
	return BuildManifestWith(ctx, bindings, sources, false)
}

// BuildManifestWith is BuildManifest with the BKM-V0-094 extensions: a binding value
// DIR@COMMIT pins that commit instead of HEAD, a source REPOSITORY:PATH:START-END pins that line
// span, and providerSkeleton emits one empty external-evidence-provider/3 record whose repository
// pins equal the manifest's, for the operator to add relations to.
func BuildManifestWith(ctx context.Context, bindings map[string]string, sources []string, providerSkeleton bool) (Manifest, error) {
	m := Manifest{Schema: "corvint-breakage-manifest/0", Repositories: []Repository{}, Sources: []Source{}, Providers: []json.RawMessage{}}
	if len(bindings) == 0 || len(bindings) > 8 || len(sources) == 0 || len(sources) > MaxSources {
		return m, errors.New("manifest helper needs 1-8 --repository bindings and 1-256 --source pins")
	}
	roots := map[string]string{}
	repos := map[string]Repository{}
	ids := make([]string, 0, len(bindings))
	for id := range bindings {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if !identifier.MatchString(id) {
			return m, errors.New("invalid repository identifier")
		}
		dir, rev := bindings[id], "HEAD"
		if at := strings.LastIndex(dir, "@"); at > 0 {
			dir, rev = dir[:at], dir[at+1:]
			if !oid.MatchString(rev) {
				return m, errors.New(id + ": @COMMIT must be a full commit id")
			}
		}
		root, err := checkoutTopLevel(ctx, dir)
		if err != nil {
			return m, errors.New(id + ": " + err.Error())
		}
		commit, err := git(ctx, root, "rev-parse", "--verify", rev+"^{commit}")
		if err != nil {
			return m, errors.New(id + ": " + rev + " commit unavailable")
		}
		tree, err := git(ctx, root, "rev-parse", "--verify", rev+"^{tree}")
		if err != nil {
			return m, errors.New(id + ": " + rev + " tree unavailable")
		}
		origins, err := git(ctx, root, "rev-list", "--max-parents=0", rev)
		if err != nil || len(strings.Fields(string(origins))) == 0 {
			return m, errors.New(id + ": origin unavailable")
		}
		candidates := strings.Fields(string(origins))
		sort.Strings(candidates)
		repo := Repository{ID: id, Origin: candidates[0], Commit: strings.TrimSpace(string(commit)), Tree: strings.TrimSpace(string(tree))}
		roots[id] = root
		repos[id] = repo
		m.Repositories = append(m.Repositories, repo)
	}
	for _, value := range sources {
		id, p, ok := strings.Cut(value, ":")
		start, end := 1, 0
		if at := strings.LastIndex(p, ":"); at > 0 {
			if span := spanSuffix.FindStringSubmatch(p[at+1:]); span != nil {
				start, _ = strconv.Atoi(span[1])
				end, _ = strconv.Atoi(span[2])
				p = p[:at]
			}
		}
		repo, bound := repos[id]
		if !ok || !bound || !safePath(p) {
			return m, errors.New("source must be BOUND-REPOSITORY:RELATIVE-PATH[:START-END]")
		}
		pin, err := ReadSpan(ctx, roots[id], repo.Commit, p, start, end)
		if err != nil {
			return m, errors.New(id + ":" + p + ": " + err.Error())
		}
		m.Sources = append(m.Sources, Source{Repository: id, Path: p, Blob: pin.Blob, Start: pin.Start, End: pin.End})
	}
	sort.Slice(m.Sources, func(i, j int) bool {
		return sourceKey(m.Sources[i].Repository, m.Sources[i].Path) < sourceKey(m.Sources[j].Repository, m.Sources[j].Path)
	})
	if providerSkeleton {
		record := extevidence.Record1{Schema: extevidence.Schema3, Provider: extevidence.Identity{ID: "skeleton", Revision: "skeleton-0"},
			Repositories: []extevidence.Repository1{}, Entities: []extevidence.Entity{}, Relations: []extevidence.Relation1{}}
		for _, repo := range m.Repositories {
			record.Repositories = append(record.Repositories, extevidence.Repository1{ID: repo.ID, Origin: repo.Origin, Revision: repo.Commit, Tree: repo.Tree})
		}
		raw, err := json.Marshal(record)
		if err != nil {
			return m, err
		}
		m.Providers = append(m.Providers, raw)
	}
	encoded, err := json.Marshal(m)
	if err != nil {
		return m, err
	}
	if _, err := Decode(encoded); err != nil {
		return m, errors.New("built manifest refused: " + err.Error())
	}
	return m, nil
}

// SpanPin is one immutable Git blob span and the SHA-256 of its
// newline-joined physical lines, the same digest breakage anchors carry.
type SpanPin struct {
	Commit string `json:"commit"`
	Path   string `json:"path"`
	Blob   string `json:"blob"`
	Start  int    `json:"start"`
	End    int    `json:"end"`
	SHA256 string `json:"sha256"`
}

// ReadSpan reads one regular UTF-8 blob at an immutable commit and digests the
// 1-based inclusive line span. end 0 selects through the last physical line.
func ReadSpan(ctx context.Context, root, commit, p string, start, end int) (SpanPin, error) {
	return NewSpanReader(ctx, root).Read(commit, p, start, end)
}

// SpanReader reads span digests from one checkout and caches each blob's
// lines by commit and path, so a profile that cites many spans of one file
// reads that blob once. A cached failure is returned again unchanged.
type SpanReader struct {
	ctx   context.Context
	root  string
	blobs map[string]spanBlob
}

type spanBlob struct {
	blob  string
	lines []string
	err   error
}

func NewSpanReader(ctx context.Context, root string) *SpanReader {
	return &SpanReader{ctx: ctx, root: root, blobs: map[string]spanBlob{}}
}

// Read returns the span pin of lines start..end of p at commit; end 0 means
// the last line.
func (r *SpanReader) Read(commit, p string, start, end int) (SpanPin, error) {
	if !oid.MatchString(commit) || !safePath(p) {
		return SpanPin{}, errors.New("span needs a full commit id and a clean relative path")
	}
	key := commit + "\x00" + p
	cached, ok := r.blobs[key]
	if !ok {
		cached = r.load(commit, p)
		r.blobs[key] = cached
	}
	if cached.err != nil {
		return SpanPin{}, cached.err
	}
	lines := cached.lines
	if end == 0 {
		end = len(lines)
	}
	if start < 1 || end < start || end > len(lines) {
		return SpanPin{}, errors.New("source span outside blob")
	}
	return SpanPin{Commit: commit, Path: p, Blob: cached.blob, Start: start, End: end, SHA256: digest([]byte(strings.Join(lines[start-1:end], "\n")))}, nil
}

func (r *SpanReader) load(commit, p string) spanBlob {
	row, err := git(r.ctx, r.root, "ls-tree", "-z", commit, "--", p)
	if err != nil {
		return spanBlob{err: err}
	}
	fields := strings.SplitN(strings.TrimSuffix(string(row), "\x00"), "\t", 2)
	meta := []string{}
	if len(fields) == 2 {
		meta = strings.Fields(fields[0])
	}
	if len(meta) != 3 {
		return spanBlob{err: errors.New("source missing")}
	}
	text, err := readBlob(r.ctx, r.root, commit, p, meta[2])
	if err != nil {
		return spanBlob{err: err}
	}
	return spanBlob{blob: meta[2], lines: strings.Split(string(text), "\n")}
}

func checkoutTopLevel(ctx context.Context, dir string) (string, error) {
	canonical, err := filepath.EvalSymlinks(dir)
	if err == nil {
		canonical, err = filepath.Abs(canonical)
	}
	if err != nil {
		return "", errors.New("checkout unavailable")
	}
	top, err := git(ctx, canonical, "rev-parse", "--show-toplevel")
	if err != nil || !sameTopLevel(string(top), canonical) {
		return "", errors.New("checkout must name its Git top level")
	}
	return canonical, nil
}
