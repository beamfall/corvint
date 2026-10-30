package doccorpus

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// Artifact validation states an info receipt reports (DCR-V1-413).
const (
	ValidationVerified  = "artifact-verified"
	ValidationRederived = "source-rederived"
)

// ReadOptions selects how a reader admits an artifact.
type ReadOptions struct {
	// Rederive runs the complete DCP-V1-011 rederivation instead of the verified
	// read (DCR-V1-413).
	Rederive bool
	// Timings, when set, receives the read's phase durations (DCR-V1-415).
	Timings *ReadTimings
	// Header runs on the artifact's header members (schema, builder, manifest,
	// its digests and tree; no records) while the records are decoded and
	// verified, so a reader's freshness work overlaps them (DCR-V1-411). Its
	// result must not be used unless OpenRead succeeds. Header is not run on a
	// rederived read.
	Header func(*Artifact)
	// Index derives the claim terms of the query index on the decode workers
	// while the records are decoded, for BuildQueryIndex to take
	// (DCR-V1-434). It only moves work: the index, its bounds and refusals are
	// unchanged.
	Index bool
}

// ReadTimings is one read's phase summary. It is diagnostics only: it never
// enters a receipt, and wall time depends on the host.
type ReadTimings struct {
	Validation                                     string
	Parse, Verify, Freshness, Index, Query, Encode time.Duration
	// Delta adds the delta's comparison phase, Compare, to the line (DCR-V1-462).
	Delta   bool
	Compare time.Duration
}

// Write renders the summary as one line for stderr.
func (t *ReadTimings) Write(w io.Writer, total time.Duration) {
	ms := func(d time.Duration) string { return fmt.Sprintf("%.1f", float64(d.Microseconds())/1000) }
	compare := ""
	if t.Delta {
		compare = " compare_ms=" + ms(t.Compare)
	}
	fmt.Fprintf(w, "corvint docs corpus timings: validation=%s parse_ms=%s verify_ms=%s freshness_ms=%s index_ms=%s query_ms=%s encode_ms=%s%s total_ms=%s\n",
		t.Validation, ms(t.Parse), ms(t.Verify), ms(t.Freshness), ms(t.Index), ms(t.Query), ms(t.Encode), compare, ms(total))
}

// OpenRead admits an artifact for a read. By default it is the verified read
// (DCR-V1-410): the artifact must be canonical, match its recorded content
// digest, name this builder, carry the digests of its own manifest and
// profile, pair its schema with the manifest's, and every revision it pins
// must still resolve in the checkout that owns it, the primary one to the
// recorded tree. The records are then decoded, never rederived. Any failed
// check refuses the read; nothing unverified is served. opts.Rederive, or a
// running builder whose label is dirty or unrecorded, runs Open instead.
func OpenRead(ctx context.Context, root string, data []byte, opts ReadOptions) (*Artifact, error) {
	if rederives(opts) {
		return openRederived(ctx, root, data, opts.Timings)
	}
	start := time.Now()
	var digest artifactTail
	var digestWG sync.WaitGroup
	digestWG.Go(func() { digest = hashArtifactTail(data) })
	a, err := openVerified(ctx, root, opts, start, func(header func(*Artifact), claims func(int, []Claim)) (*Artifact, artifactTail, error) {
		a, err := decodeArtifactBytes(data, header, claims)
		digestWG.Wait()
		return a, digest, err
	})
	digestWG.Wait()
	return a, err
}

// OpenReadFile is OpenRead of the artifact at path under root, read as
// ReadCorpusFile reads it. The verified read streams the file: it is hashed,
// scanned and decoded window by window, so the artifact bytes are never held
// whole beside the records they decode to (DCR-V1-431). A rederived read reads
// the whole file, as before.
func OpenReadFile(ctx context.Context, root, path string, opts ReadOptions) (*Artifact, error) {
	t := opts.Timings
	if rederives(opts) {
		start := time.Now()
		data, err := ReadCorpusFile(root, path)
		if t != nil {
			t.Parse += time.Since(start)
		}
		if err != nil {
			return nil, err
		}
		return openRederived(ctx, root, data, t)
	}
	start := time.Now()
	var a *Artifact
	err := streamCorpusFile(root, path, func(r io.Reader) error {
		var err error
		a, err = openVerified(ctx, root, opts, start, func(header func(*Artifact), claims func(int, []Claim)) (*Artifact, artifactTail, error) {
			return decodeArtifactReader(r, header, claims)
		})
		return err
	})
	return a, err
}

// rederives reports whether a read runs the complete rederivation: on request, or for a running
// builder whose label names no immutable build, which cannot show that an equal recorded label
// means equal extraction (DCR-V1-410).
func rederives(opts ReadOptions) bool {
	return opts.Rederive || !rebuildAttested(currentBuilder())
}

func openRederived(ctx context.Context, root string, data []byte, t *ReadTimings) (*Artifact, error) {
	start := time.Now()
	a, err := Open(ctx, root, data)
	if t != nil {
		t.Verify += time.Since(start)
	}
	if err != nil {
		return nil, err
	}
	a.validation = ValidationRederived
	if t != nil {
		t.Validation = a.validation
	}
	return a, nil
}

// openVerified is the verified read over decode, which decodes and verifies the canonical
// artifact, runs header on its header members and returns its digest tail. The recorded
// identity is verified, and the caller's header work runs, from the header members while the
// records are decoded and compared with their canonical encoding; the first failure in the
// order decode, digest, recorded identity refuses (DCR-V1-414).
func openVerified(ctx context.Context, root string, opts ReadOptions, start time.Time, decode func(header func(*Artifact), claims func(int, []Claim)) (*Artifact, artifactTail, error)) (*Artifact, error) {
	t := opts.Timings
	var recorded error
	var verified time.Duration
	var claims func(int, []Claim)
	var blocksMu sync.Mutex
	var blocks []*claimTermBlock
	if opts.Index {
		claims = func(lo int, chunk []Claim) {
			b := deriveClaimTerms(chunk, lo, nil)
			blocksMu.Lock()
			blocks = append(blocks, b)
			blocksMu.Unlock()
		}
	}
	a, digest, err := decode(func(h *Artifact) {
		step := time.Now()
		// The header work runs Git in the checkouts the manifest names, so only once the manifest
		// is valid and every additional repository root is admitted.
		if recorded = validateManifest(h.Manifest); recorded != nil {
			verified = time.Since(step)
			return
		}
		var wg sync.WaitGroup
		if opts.Header != nil {
			wg.Go(func() { opts.Header(h) })
		}
		recorded = verifyRecorded(ctx, root, h)
		verified = time.Since(step)
		wg.Wait()
	}, claims)
	if t != nil {
		t.Parse += time.Since(start)
		t.Verify += verified
	}
	if err != nil {
		return nil, err
	}
	if err := digest.verify(a); err != nil {
		return nil, err
	}
	if recorded != nil {
		return nil, recorded
	}
	a.validation = ValidationVerified
	if opts.Index {
		sort.Slice(blocks, func(i, j int) bool { return blocks[i].lo < blocks[j].lo })
		a.claimTerms = &claimTermHandoff{blocks: blocks, of: a.Claims}
	}
	if t != nil {
		t.Validation = a.validation
	}
	return a, nil
}

// attestedBuilder reports whether b names one immutable build: a recorded
// source revision without uncommitted changes. Binaries built from different
// uncommitted changes share one `<revision>+dirty` label, and binaries built
// without version-control metadata share `unrecorded`, so equal labels of
// either kind say nothing about equal extraction behavior. No executable
// content identity is recorded in the artifact to tell such builds apart.
// Only a full lowercase Git object ID is a recorded revision, so an empty or
// malformed label is no attestation either (DCX-V1-471).
func attestedBuilder(b Builder) bool {
	return wire.IsGitOid(b.Revision)
}

// unattestedBuilderTrusted admits the verified read for an unattested
// running builder. Test binaries never record version-control metadata, so
// only tests set it, through TrustUnattestedBuilderForTest.
var unattestedBuilderTrusted atomic.Bool

func rebuildAttested(b Builder) bool {
	return attestedBuilder(b) || unattestedBuilderTrusted.Load()
}

// builds counts compiler builds, rederivations included, so a test can show that a read path
// rederived nothing (DCR-V1-461).
var builds atomic.Int64

// BuildsForTest returns how many builds this process has run. Production code never calls it.
func BuildsForTest() int64 { return builds.Load() }

// pins counts input pins, so a test can show that a retried compile reads the inputs its
// refused compile pinned instead of pinning them again (DCS-V1-572).
var pins atomic.Int64

// PinsForTest returns how many input pins this process has run. Production code never calls it.
func PinsForTest() int64 { return pins.Load() }

// TrustUnattestedBuilderForTest lets a test binary, whose builder label is
// always `unrecorded`, exercise the verified read; the returned function
// restores the refusal. Production code never calls it.
func TrustUnattestedBuilderForTest() (restore func()) {
	old := unattestedBuilderTrusted.Swap(true)
	return func() { unattestedBuilderTrusted.Store(old) }
}

// errNotRederivable is the refusal of a verified read whose recorded identity
// this builder could never rederive (DCR-V1-410).
func errNotRederivable(reason string) error {
	return fail("artifact cannot be rederived from current builder and pinned inputs: " + reason)
}

// verifyRecorded checks what a verified read trusts instead of rederiving. Its caller has
// already validated a.Manifest, which validateManifest only reads, so it is not validated again
// (DCR-V1-558).
func verifyRecorded(ctx context.Context, root string, a *Artifact) error {
	if err := sameDerivation(a.Builder, currentBuilder()); err != nil {
		return err
	}
	if (a.Schema == SchemaV2) != (a.Manifest.Schema == ManifestSchemaV2) {
		return errNotRederivable("artifact and manifest schemas differ")
	}
	manifest, err := hashValue(a.Manifest)
	if err != nil || manifest != a.ManifestSHA256 {
		return errNotRederivable("manifest digest differs")
	}
	profile, err := hashValue(a.Manifest.Profile)
	if err != nil || profile != a.ProfileSHA256 {
		return errNotRederivable("profile digest differs")
	}
	return verifyPinnedRevisions(ctx, root, a)
}

// verifyPinnedRevisions resolves every pinned revision in the checkout that
// owns it, one batch per checkout and the checkouts in parallel; the primary
// revision must resolve to the recorded tree.
func verifyPinnedRevisions(ctx context.Context, root string, a *Artifact) (err error) {
	defer func() { err = gitCapacity(err, "while verifying pinned revisions") }()
	m := a.Manifest
	byDir := map[string]map[string]bool{}
	pin := func(revision string) {
		dir := m.RevisionRoot(root, revision)
		if byDir[dir] == nil {
			byDir[dir] = map[string]bool{}
		}
		byDir[dir][revision] = true
	}
	pin(m.Repository.Revision)
	for _, s := range m.Scopes {
		pin(s.Revision)
	}
	for _, in := range m.Inputs {
		pin(in.Revision)
	}
	for _, p := range m.Providers {
		if p.Revision != "" {
			pin(p.Revision)
		}
	}
	for _, r := range m.AdditionalRepositories {
		for _, revision := range r.pinned() {
			pin(revision)
		}
	}
	dirs := make([]string, 0, len(byDir))
	for dir := range byDir {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	errs := make([]error, len(dirs))
	var wg sync.WaitGroup
	for i, dir := range dirs {
		wg.Go(func() {
			auth, err := gitauth.Open(dir, newGitBudget(ctx))
			if err != nil {
				errs[i] = err
				return
			}
			revisions := make([]string, 0, len(byDir[dir]))
			for revision := range byDir[dir] {
				revisions = append(revisions, revision)
			}
			sort.Strings(revisions)
			for _, revision := range revisions {
				tree, err := auth.CommitTree(ctx, revision)
				if err != nil {
					errs[i] = err
					return
				}
				if revision == m.Repository.Revision && tree != a.Tree {
					errs[i] = errNotRederivable("recorded tree differs from the pinned revision")
					return
				}
			}
		})
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// validationOf is the info receipt's validation state.
func validationOf(a *Artifact) string {
	if strings.TrimSpace(a.validation) == "" {
		return ValidationRederived
	}
	return a.validation
}
