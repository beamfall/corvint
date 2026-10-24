package doccorpus

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/contextindex"
)

// Symbol-anchor states (DCP-V1-140). Only live and moved assert that the
// anchored region still exists unchanged; every refusal is unresolved.
const (
	SymbolAnchorLive       = "live"
	SymbolAnchorMoved      = "moved"
	SymbolAnchorChanged    = "changed"
	SymbolAnchorGone       = "gone"
	SymbolAnchorUnresolved = "unresolved"
)

const (
	// symbolFileMaxBytes bounds one source read for re-resolution, matching
	// the flow check's imported-source bound.
	symbolFileMaxBytes = 4 << 20
	// symbolReadMaxBytes bounds every source read by one resolver.
	symbolReadMaxBytes = 64 << 20
	// symbolCandidateMaxPaths bounds the repository-wide search for a symbol
	// that left its anchored path.
	symbolCandidateMaxPaths = 64
)

// SymbolLocation is where a symbol anchor's region lies in the live tree.
// Estimated marks a changed region whose range is only the pinned range
// shifted with its declaration line, not a proven region boundary.
type SymbolLocation struct {
	Revision    string `json:"revision"`
	Path        string `json:"path"`
	Blob        string `json:"blob"`
	SHA256      string `json:"sha256"`
	Start       int    `json:"start_line"`
	End         int    `json:"end_line"`
	Declaration int    `json:"declaration_line,omitempty"`
	SpanSHA256  string `json:"span_sha256"`
	Estimated   bool   `json:"estimated,omitempty"`
}

// SymbolAnchorCheck is the live re-resolution of one anchor that names a
// symbol (`path:START-END#Symbol@REV`). The normalized digests cover the
// region's comment- and whitespace-insensitive code lines.
type SymbolAnchorCheck struct {
	Anchor               string          `json:"anchor"`
	State                string          `json:"state"`
	Reason               string          `json:"reason"`
	Live                 *SymbolLocation `json:"live,omitempty"`
	NormalizedSHA256     string          `json:"normalized_sha256,omitempty"`
	LiveNormalizedSHA256 string          `json:"live_normalized_sha256,omitempty"`
}

// FormatSymbolAnchor renders an anchor in the `path:START-END#Symbol@REV` form.
func FormatSymbolAnchor(a Anchor) string {
	return fmt.Sprintf("%s:%d-%d#%s@%s", a.Path, a.Start, a.End, a.Symbol, a.Revision)
}

// SymbolResolver re-resolves symbol anchors against one live tree: the
// commit Revision, minus the working-tree paths named dirty, which it never
// classifies. It reads through the caller's object session.
type SymbolResolver struct {
	auth       *gitauth.Repository
	index      *contextindex.Index
	repository Repository
	dirty      map[string]bool
	files      map[string]*symbolFile
	byName     map[string][]string
	read       int
	shared     *sharedSymbolFiles
	prefetched *pathPrefetch
}

type symbolFile struct {
	raw     []byte
	extents []contextindex.SymbolExtent
	lines   []contextindex.NormalizedLine
	count   int
	refusal string
}

// NewSymbolResolver binds a resolver to the live tree. index must describe
// repository.Revision; dirty names paths whose working tree differs from it.
func NewSymbolResolver(auth *gitauth.Repository, index *contextindex.Index, repository Repository, dirty []string) *SymbolResolver {
	r := &SymbolResolver{auth: auth, index: index, repository: repository, dirty: map[string]bool{}, files: map[string]*symbolFile{}}
	for _, p := range dirty {
		r.dirty[p] = true
	}
	return r
}

// Resolve classifies one symbol anchor. It looks for the symbol at the
// anchored path first and then across the repository's same-language sources,
// and reports unresolved, never a guess, whenever the evidence for live,
// moved, changed or gone is incomplete. An error is returned only for a
// cancelled context or a failed tree lookup.
func (r *SymbolResolver) Resolve(ctx context.Context, a Anchor) (SymbolAnchorCheck, error) {
	c := SymbolAnchorCheck{Anchor: FormatSymbolAnchor(a), State: SymbolAnchorUnresolved}
	family := contextindex.SymbolRegionLanguage(a.Path)
	switch {
	case a.Symbol == "":
		c.Reason = "anchor names no symbol"
		return c, nil
	case family == "":
		c.Reason = "symbol re-resolution covers Ruby and TypeScript/JavaScript only; no region normalizer for this language"
		return c, nil
	case a.Repository != r.repository.ID:
		c.Reason = "anchor names another repository; the live tree cannot re-resolve it"
		return c, nil
	}
	old, err := r.file(ctx, a.Blob, a.Path)
	if err != nil {
		return c, err
	}
	if old.refusal != "" {
		c.Reason = "pinned source " + old.refusal
		return c, nil
	}
	declaration := 0
	for _, e := range old.extents {
		if e.Name == a.Symbol && e.Start >= a.Start && e.Start <= a.End {
			if declaration != 0 {
				c.Reason = "symbol is declared more than once in the pinned span"
				return c, nil
			}
			declaration = e.Start
		}
	}
	if declaration == 0 || a.Start < 1 || a.End < a.Start || a.End > old.count {
		c.Reason = "symbol does not resolve in the pinned span"
		return c, nil
	}
	region, position := regionLines(old.lines, a.Start, a.End, declaration)
	if position < 0 {
		c.Reason = "pinned declaration line has no code"
		return c, nil
	}
	c.NormalizedSHA256 = normalizedDigest(region)
	if r.dirty[a.Path] {
		c.Reason = "anchored path has uncommitted working-tree changes; re-resolution reads committed content only"
		return c, nil
	}
	entry, exists, err := r.prefetched.lookup(ctx, r.auth, r.repository.Revision, a.Path)
	if err != nil {
		return c, err
	}
	if exists && entry.Type == "blob" && (entry.Mode == "100644" || entry.Mode == "100755") {
		current, err := r.file(ctx, entry.OID, a.Path)
		if err != nil {
			return c, err
		}
		if current.refusal != "" {
			c.Reason = "live source " + current.refusal
			return c, nil
		}
		var equal, different []SymbolLocation
		var differentHash []string
		for _, e := range current.extents {
			if e.Name != a.Symbol {
				continue
			}
			location, hash, same := r.match(a, entry.OID, current, region, position, declaration, e.Start)
			if same {
				equal = append(equal, location)
			} else {
				different = append(different, location)
				differentHash = append(differentHash, hash)
			}
		}
		switch {
		case len(equal) == 1 && equal[0].Start == a.Start && equal[0].End == a.End:
			c.State, c.Reason = SymbolAnchorLive, "symbol region unchanged at its pinned lines"
		case len(equal) == 1:
			c.State, c.Reason = SymbolAnchorMoved, "symbol region unchanged at new lines in the same path"
		case len(equal) > 1:
			c.Reason = "symbol region matches more than one declaration in the live path"
			return c, nil
		case len(different) == 1:
			c.State, c.Reason = SymbolAnchorChanged, "symbol found in the same path but its normalized region differs"
			c.Live, c.LiveNormalizedSHA256 = &different[0], differentHash[0]
			return c, nil
		case len(different) > 1:
			c.Reason = "symbol is declared more than once in the live path and none matches"
			return c, nil
		}
		if len(equal) == 1 {
			c.Live, c.LiveNormalizedSHA256 = &equal[0], c.NormalizedSHA256
			return c, nil
		}
	} else if exists {
		c.Reason = "anchored path is no longer a regular file"
		return c, nil
	}
	return r.search(ctx, c, a, family, region, position, declaration)
}

// search looks for a symbol that left its anchored path among the
// same-language sources the live index names. Gone requires a complete
// inventory: an unparsed, truncated, excluded or dirty source of that
// language could still declare it.
func (r *SymbolResolver) search(ctx context.Context, c SymbolAnchorCheck, a Anchor, family string, region []contextindex.NormalizedLine, position, declaration int) (SymbolAnchorCheck, error) {
	paths := r.candidates(a, family)
	if len(paths) > symbolCandidateMaxPaths {
		c.Reason = fmt.Sprintf("symbol name is declared in more than %d live paths; search withheld", symbolCandidateMaxPaths)
		return c, nil
	}
	var equal []SymbolLocation
	candidates, incomplete := 0, ""
	for _, p := range paths {
		if r.dirty[p] {
			incomplete = "a same-name candidate has uncommitted working-tree changes"
			continue
		}
		entry, exists, err := r.prefetched.lookup(ctx, r.auth, r.repository.Revision, p)
		if err != nil {
			return c, err
		}
		if !exists || entry.Type != "blob" {
			continue
		}
		current, err := r.file(ctx, entry.OID, p)
		if err != nil {
			return c, err
		}
		if current.refusal != "" {
			incomplete = "a same-name candidate " + current.refusal
			continue
		}
		for _, e := range current.extents {
			if e.Name != a.Symbol {
				continue
			}
			candidates++
			if location, _, same := r.match(a, entry.OID, current, region, position, declaration, e.Start); same {
				location.Path = p
				equal = append(equal, location)
			}
		}
	}
	switch {
	case len(equal) == 1:
		c.State, c.Reason = SymbolAnchorMoved, "symbol region unchanged in another live path"
		c.Live, c.LiveNormalizedSHA256 = &equal[0], c.NormalizedSHA256
		return c, nil
	case len(equal) > 1:
		c.Reason = "symbol region matches declarations in more than one live path"
		return c, nil
	case candidates > 0:
		c.Reason = "symbol name exists only in other live paths with a different region; identity across paths is unproven"
		return c, nil
	case incomplete != "":
		c.Reason = incomplete
		return c, nil
	}
	if reason := r.inventoryGap(family); reason != "" {
		c.Reason = "symbol absent from the indexed sources, but " + reason
		return c, nil
	}
	c.State, c.Reason = SymbolAnchorGone, "symbol absent from the anchored path and from every same-language live source"
	return c, nil
}

// inventoryGap names why the live index could miss a declaration of the
// family, or "" when every same-language source was walked to its end.
func (r *SymbolResolver) inventoryGap(family string) string {
	for _, u := range r.index.Unparsed {
		if contextindex.SymbolRegionLanguage(u.Path) == family {
			return "a same-language source was not parsed: " + u.Path
		}
	}
	for _, n := range r.index.ExtractionNotes {
		if contextindex.SymbolRegionLanguage(n.Path) == family {
			return "a same-language source was only partly walked: " + n.Path
		}
	}
	for _, x := range r.index.Exclusions {
		if contextindex.SymbolRegionLanguage(x.Path) == family {
			return "a same-language source was excluded from the index: " + x.Path
		}
	}
	for p := range r.dirty {
		if contextindex.SymbolRegionLanguage(p) == family {
			return "a same-language source has uncommitted working-tree changes: " + p
		}
	}
	return ""
}

// match compares the pinned region against the region around one live
// declaration. The live region is the run of normalized lines at the same
// offsets from the declaration; its range keeps the pinned range's margins
// outside the first and last code line. A region that does not fit, or whose
// lines differ, yields the pinned range shifted with the declaration, marked
// estimated.
func (r *SymbolResolver) match(a Anchor, blob string, current *symbolFile, region []contextindex.NormalizedLine, position, declaration, line int) (SymbolLocation, string, bool) {
	location := SymbolLocation{Revision: r.repository.Revision, Path: a.Path, Blob: blob, SHA256: Digest(current.raw), Declaration: line}
	at := -1
	for i, l := range current.lines {
		if l.Line == line {
			at = i
			break
		}
	}
	low, high := at-position, at-position+len(region)
	same := at >= 0 && low >= 0 && high <= len(current.lines)
	if same {
		for i, l := range region {
			if current.lines[low+i].Text != l.Text {
				same = false
				break
			}
		}
	}
	if same {
		location.Start = current.lines[low].Line - (region[0].Line - a.Start)
		location.End = current.lines[high-1].Line + (a.End - region[len(region)-1].Line)
	} else {
		location.Start = line - (declaration - a.Start)
		location.End = line + (a.End - declaration)
		location.Estimated = true
	}
	location.Start = max(location.Start, 1)
	location.End = min(location.End, current.count)
	location.SpanSHA256 = spanDigest(current.raw, location.Start, location.End)
	if same {
		return location, normalizedDigest(region), true
	}
	live, _ := regionLines(current.lines, location.Start, location.End, line)
	return location, normalizedDigest(live), false
}

// candidates returns, sorted, the other same-language live paths the index
// says declare the anchor's symbol.
func (r *SymbolResolver) candidates(a Anchor, family string) []string {
	if r.byName == nil {
		r.byName = map[string][]string{}
		seen := map[string]bool{}
		for _, s := range r.index.Symbols {
			key := s.Name + "\x00" + s.Path
			if !seen[key] {
				seen[key] = true
				r.byName[s.Name] = append(r.byName[s.Name], s.Path)
			}
		}
	}
	var paths []string
	for _, p := range r.byName[a.Symbol] {
		if p != a.Path && contextindex.SymbolRegionLanguage(p) == family {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	return paths
}

// file reads and parses one blob once per resolver.
func (r *SymbolResolver) file(ctx context.Context, oid, name string) (*symbolFile, error) {
	key := oid + "\x00" + name
	if f, ok := r.files[key]; ok {
		return f, nil
	}
	f := &symbolFile{}
	r.files[key] = f
	if r.read > symbolReadMaxBytes {
		f.refusal = "was not read: re-resolution read budget exhausted"
		return f, nil
	}
	if r.shared != nil {
		if kept, ok := r.shared.get(key); ok {
			*f = *kept.file
			r.read += kept.cost
			return f, nil
		}
	}
	data, over, err := r.prefetched.blobWithin(ctx, r.auth, oid, symbolFileMaxBytes)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if code := cemcode.CodeOf(err); code == cemcode.GitBudgetExceeded || code == cemcode.GitOutputExceeded {
			// A capacity failure is the request's, not the blob's: it
			// withholds every symbol state with its diagnostic (DCP-V1-216).
			delete(r.files, key)
			return nil, err
		}
		f.refusal = "blob is unavailable"
		return f, nil
	}
	if over {
		f.refusal = "exceeds the 4 MiB re-resolution bound"
		r.keep(key, f, 0)
		return f, nil
	}
	r.read += len(data)
	extents, ok := contextindex.SymbolExtents(name, data)
	lines, normalized := contextindex.NormalizedCodeLines(name, data)
	if !ok || !normalized {
		f.refusal = "did not parse completely"
		r.keep(key, f, len(data))
		return f, nil
	}
	f.raw, f.extents, f.lines = data, extents, lines
	f.count = strings.Count(string(data), "\n")
	if len(data) > 0 && data[len(data)-1] != '\n' {
		f.count++
	}
	r.keep(key, f, len(data))
	return f, nil
}

// keep offers a file whose outcome the blob alone fixes to the shared files.
func (r *SymbolResolver) keep(key string, f *symbolFile, cost int) {
	if r.shared != nil {
		r.shared.put(key, f, cost)
	}
}

// regionLines returns the normalized lines inside [start, end] and the index
// of the declaration line among them, -1 when it has no code.
func regionLines(lines []contextindex.NormalizedLine, start, end, declaration int) ([]contextindex.NormalizedLine, int) {
	var region []contextindex.NormalizedLine
	position := -1
	for _, l := range lines {
		if l.Line < start || l.Line > end {
			continue
		}
		if l.Line == declaration {
			position = len(region)
		}
		region = append(region, l)
	}
	return region, position
}

// normalizedDigest hashes length-prefixed line texts, so line boundaries are
// part of the digest and no two different regions share an encoding.
func normalizedDigest(lines []contextindex.NormalizedLine) string {
	h := sha256.New()
	for _, l := range lines {
		fmt.Fprintf(h, "%d:%s\n", len(l.Text), l.Text)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// spanDigest is the exact-span digest an anchor's SpanSHA256 records.
func spanDigest(raw []byte, start, end int) string {
	lines := strings.SplitAfter(string(raw), "\n")
	if start < 1 || end > len(lines) || end < start {
		return ""
	}
	return Digest([]byte(strings.Join(lines[start-1:end], "")))
}

// SymbolAnchorKey identifies one symbol anchor in an overlay.
func SymbolAnchorKey(a Anchor) string {
	return FormatSymbolAnchor(a) + ":" + a.Blob + ":" + a.SpanSHA256
}

// symbolOverlay is the live tree a reader re-resolves cited symbol anchors
// against. Requests under one overlay share its parsed source files; each
// request still resolves under its own budget (symbolChecker).
type symbolOverlay struct {
	root, repository string
	// live is the observed primary checkout: its commit and dirty paths decide
	// every anchor whose pinned blob is unchanged without an index.
	live *contextindex.TreeObservation
	// observation is live's ObservationKey; the full index a re-resolution
	// needs is admitted only when it was built from that same state.
	observation string
	files       *sharedSymbolFiles
	mu          sync.Mutex
	index       *contextindex.Index
}

// liveIndex returns the full live index a symbol re-resolution reads, building
// it on the first anchor whose pinned blob changed and keeping it for later
// requests under this overlay (DCR-V1-412). An index built from another state
// than the overlay observed is refused, so a re-resolution never mixes two
// checkout states; a failure is not kept.
func (o *symbolOverlay) liveIndex(ctx context.Context) (*contextindex.Index, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.index != nil {
		return o.index, nil
	}
	index, err := contextindex.BuildContext(ctx, o.root, "")
	if err != nil {
		return nil, err
	}
	key := ObservationKey(contextindex.Observation{ObjectFormat: index.ObjectFormat, CommitRevision: index.CommitRevision, Revision: index.Revision, StatusSHA256: index.StatusSHA256, DirtyPaths: index.DirtyPaths})
	if key != o.observation {
		return nil, fail("live checkout changed after its freshness observation")
	}
	o.index = index
	return index, nil
}

// sharedSymbolFiles keeps parsed source files across requests. A request that
// reuses a file is charged its bytes as if it had read them, so the shared
// files never change a check (DCP-V1-144). Only outcomes fixed by the blob
// itself are kept; a budget refusal or an unavailable blob is not.
type sharedSymbolFiles struct {
	mu    sync.Mutex
	files map[string]sharedSymbolFile
	bytes int
}

type sharedSymbolFile struct {
	file *symbolFile
	cost int
}

// sharedSymbolFilesMaxBytes bounds the bytes the shared files retain.
const sharedSymbolFilesMaxBytes = 128 << 20

func (s *sharedSymbolFiles) get(key string) (sharedSymbolFile, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.files[key]
	return f, ok
}

func (s *sharedSymbolFiles) put(key string, f *symbolFile, cost int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.files[key]; ok || s.bytes+len(f.raw) > sharedSymbolFilesMaxBytes {
		return
	}
	s.files[key] = sharedSymbolFile{file: f, cost: cost}
	s.bytes += len(f.raw)
}

// symbolChecker answers one request's symbol-anchor checks. It resolves an
// anchor only when a receipt cites it, under one resolver and Git budget per
// request. A nil overlay evaluates nothing; a failed re-resolution withholds
// every symbol state and names the failure as a limitation rather than
// refusing the read; only a cancelled context is an error.
type symbolChecker struct {
	root     string // primary checkout of a live reader (DCP-V1-332)
	overlay  *symbolOverlay
	resolver *SymbolResolver
	release  func()
	blobs    map[string]string
	checks   map[string]SymbolAnchorCheck
	withheld string
	anchors  *anchorStates
	indexErr error
}

func (c *symbolChecker) close() {
	if c.release != nil {
		c.release()
	}
}

func (c *symbolChecker) check(ctx context.Context, anchor Anchor) (SymbolAnchorCheck, error) {
	key := SymbolAnchorKey(anchor)
	if check, ok := c.checks[key]; ok {
		return check, nil
	}
	if c.overlay == nil || c.withheld != "" {
		return unevaluatedSymbolAnchor(anchor), nil
	}
	check, err := c.resolve(ctx, anchor)
	if err != nil {
		if ctx.Err() != nil {
			return SymbolAnchorCheck{}, ctx.Err()
		}
		c.withheld = "symbol-anchor re-resolution withheld: " + err.Error()
		return unevaluatedSymbolAnchor(anchor), nil
	}
	c.checks[key] = check
	return check, nil
}

// resolve checks one anchor. An anchor whose pinned blob is still the
// committed, clean content of its path is live without a parse.
func (c *symbolChecker) resolve(ctx context.Context, anchor Anchor) (SymbolAnchorCheck, error) {
	live := c.overlay.live
	if err := c.open(ctx); err != nil {
		return SymbolAnchorCheck{}, err
	}
	blob, seen := c.blobs[anchor.Path]
	if !seen {
		entry, exists, err := c.resolver.prefetched.lookup(ctx, c.resolver.auth, live.CommitRevision, anchor.Path)
		if err != nil {
			return SymbolAnchorCheck{}, err
		}
		if exists && entry.Type == "blob" {
			blob = entry.OID
		}
		c.blobs[anchor.Path] = blob
	}
	if blob == anchor.Blob && !c.resolver.dirty[anchor.Path] && anchor.Repository == c.resolver.repository.ID && contextindex.SymbolRegionLanguage(anchor.Path) != "" {
		return SymbolAnchorCheck{Anchor: FormatSymbolAnchor(anchor), State: SymbolAnchorLive, Reason: "pinned blob is unchanged at the live path",
			Live: &SymbolLocation{Revision: live.CommitRevision, Path: anchor.Path, Blob: blob, SHA256: anchor.SHA256, Start: anchor.Start, End: anchor.End, SpanSHA256: anchor.SpanSHA256}}, nil
	}
	// Resolve answers an anchor it cannot re-resolve without reading the index.
	if anchor.Symbol != "" && contextindex.SymbolRegionLanguage(anchor.Path) != "" && anchor.Repository == c.resolver.repository.ID {
		if err := c.bindIndex(ctx); err != nil {
			return SymbolAnchorCheck{}, err
		}
	}
	return c.resolver.Resolve(ctx, anchor)
}

// bindIndex gives the request's resolver the overlay's full live index. A
// failed build is kept for the request, so it is attempted once.
func (c *symbolChecker) bindIndex(ctx context.Context) error {
	if c.resolver.index != nil || c.indexErr != nil {
		return c.indexErr
	}
	index, err := c.overlay.liveIndex(ctx)
	if err != nil {
		if ctx.Err() == nil {
			c.indexErr = err
		}
		return err
	}
	c.resolver.index = index
	return nil
}

// open binds the request's resolver and Git budget on first use.
func (c *symbolChecker) open(ctx context.Context) error {
	if c.resolver != nil {
		return nil
	}
	live := c.overlay.live
	auth, err := gitauth.Open(c.overlay.root, newGitBudget(ctx))
	if err != nil {
		return err
	}
	c.release = auth.BeginObjectSession()
	c.resolver = NewSymbolResolver(auth, nil, Repository{ID: c.overlay.repository, Revision: live.CommitRevision}, live.DirtyPaths)
	c.resolver.shared = c.overlay.files
	c.resolver.prefetched = newPathPrefetch(live.CommitRevision)
	c.blobs = map[string]string{}
	return nil
}

// symbolPrefetchMaxBytes bounds the blob bytes one prefetch reads ahead, so
// bodies a resolution never consumes cannot exhaust the request's Git blob
// budget (DCP-V1-216).
const symbolPrefetchMaxBytes = symbolReadMaxBytes / 2

// prefetch batches the tree lookups and blob reads the cited anchors not yet
// checked will need (DCP-V1-216): one walk for their paths, one for the
// same-name candidates of those that need a re-resolution, and one header
// and one body batch for the pinned, live and candidate blobs, pinned and
// live first. It only warms the resolver's cache; check still decides every
// anchor, and a batch that fails leaves the per-anchor reads to decide.
// Only a cancelled context is an error.
func (c *symbolChecker) prefetch(ctx context.Context, citations []Anchor) error {
	if c.overlay == nil || c.withheld != "" {
		return nil
	}
	var anchors []Anchor
	paths := []string{}
	for _, anchor := range citations {
		if _, done := c.checks[SymbolAnchorKey(anchor)]; anchor.Symbol != "" && !done {
			anchors = append(anchors, anchor)
			paths = append(paths, anchor.Path)
		}
	}
	if len(anchors) == 0 || c.open(ctx) != nil {
		return nil // check reports a failed open
	}
	r, p := c.resolver, c.resolver.prefetched
	if err := p.resolve(ctx, r.auth, paths); err != nil {
		return err
	}
	oids, candidates := []string{}, []string{}
	indexed := true
	for _, anchor := range anchors {
		family := contextindex.SymbolRegionLanguage(anchor.Path)
		entry, exists := p.entries[anchor.Path]
		live := exists && entry.Type == "blob" && entry.OID == anchor.Blob && !r.dirty[anchor.Path]
		if family == "" || anchor.Repository != r.repository.ID || live {
			continue
		}
		oids = append(oids, anchor.Blob)
		if exists && regularBlob(entry) && !r.dirty[anchor.Path] {
			oids = append(oids, entry.OID)
		}
		if indexed && c.bindIndex(ctx) != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			indexed = false // check reports the failed index build
		}
		if !indexed {
			continue
		}
		if found := r.candidates(anchor, family); len(found) <= symbolCandidateMaxPaths {
			candidates = append(candidates, found...)
		}
	}
	if err := p.resolve(ctx, r.auth, candidates); err != nil {
		return err
	}
	for _, path := range candidates {
		if !r.dirty[path] {
			oids = append(oids, p.regularOIDs([]string{path})...)
		}
	}
	return p.read(ctx, r.auth, oids, symbolFileMaxBytes, symbolPrefetchMaxBytes)
}

func unevaluatedSymbolAnchor(anchor Anchor) SymbolAnchorCheck {
	return SymbolAnchorCheck{Anchor: FormatSymbolAnchor(anchor), State: SymbolAnchorUnresolved, Reason: "cited symbol anchor was not evaluated against the live tree"}
}

// attachSymbolAnchors adds the check for each cited symbol anchor. A nil
// checker means the reader has no live tree and adds nothing; a withheld
// re-resolution reports every cited anchor unevaluated.
func attachSymbolAnchors(ctx context.Context, r *Receipt, c *symbolChecker) error {
	r.SymbolAnchors = nil
	if c == nil {
		return nil
	}
	if err := c.prefetch(ctx, r.Citations); err != nil {
		return err
	}
	for _, anchor := range r.Citations {
		if anchor.Symbol == "" {
			continue
		}
		check, err := c.check(ctx, anchor)
		if err != nil {
			return err
		}
		r.SymbolAnchors = append(r.SymbolAnchors, check)
	}
	if c.withheld != "" {
		for i, check := range r.SymbolAnchors {
			r.SymbolAnchors[i] = SymbolAnchorCheck{Anchor: check.Anchor, State: SymbolAnchorUnresolved, Reason: "cited symbol anchor was not evaluated against the live tree"}
		}
		r.Limitations = append(r.Limitations, c.withheld)
	}
	return nil
}
