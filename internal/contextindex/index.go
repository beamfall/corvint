package contextindex

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"
	"unsafe"

	"github.com/Beamfall/corvint/internal/projectprofile"
)

const (
	// entriesPerWorker is how much clean-file verification has to be waiting
	// before a second worker, and its own scratch buffer, earns its allocation.
	entriesPerWorker = 256

	maxEvidence         = 10
	maxExclusionSamples = 10
	maxImpactPaths      = 100
	maxPathChars        = 1_024
	maxLimit            = 50
	lfsPointerPrefix    = "version https://git-lfs.github.com/spec/v1"
	lfsPointerExclusion = "git-lfs pointer, content not in the tree"
)

// entriesPerWorkerOverride is entriesPerWorker, indirected only so a test can
// force the single-worker path and compare it against the concurrent one.
var entriesPerWorkerOverride = entriesPerWorker

type cleanFileVerificationScratch struct {
	buffer [32 * 1024]byte
	prefix [4096]byte
}

var (
	// textSuffixes is the admission allowlist: a file whose extension is absent
	// here is invisible to the index entirely, not merely unanalysed.
	//
	// The second row is the set admitted for symbol extraction by
	// langsymbols.go. The third is admitted as searchable text only: SQL has no
	// extractor here, and `.m` is shared by Objective-C, MATLAB and Mathematica,
	// so any symbol rule written for it would be right for at most one of the
	// three. Admitting them still moves them from invisible to searchable.
	textSuffixes = map[string]bool{
		".go": true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true,
		".mjs": true, ".cjs": true, ".py": true, ".sh": true, ".md": true,
		".yaml": true, ".yml": true, ".json": true, ".toml": true, ".mod": true,

		".rs": true, ".cs": true, ".swift": true, ".kt": true, ".kts": true,
		".rb": true,

		".sql": true, ".m": true, ".rst": true, ".mdx": true, ".txt": true,
	}
	// A path is excluded when any of its components is one of these. ".claude"
	// is here because an agent worktree copy under it is not first-party
	// source: indexing it doubles a repository's apparent symbol population
	// with stale duplicates of its own files, which is worse than missing them.
	// Only tracked paths reach this screen, so a repository that gitignores
	// ".claude" -- as the Beamfall workspace does, where 1,054 of the 3,148 Go
	// files on disk sit under it and none are tracked -- is unaffected today;
	// the exclusion holds for a repository that commits one.
	forbiddenParts = map[string]bool{
		".git": true, "vendor": true, "node_modules": true, "app-dist": true,
		"dist": true, "build": true, "coverage": true, ".next": true,
		".cache": true, "target": true, ".claude": true,
	}
	generatedPath = regexp.MustCompile(`(?i)(?:^|/)(?:generated|gen)(?:/|$)|(?:\.gen\.|_generated\.)|(?:^|/)docs/api/(?:openapi\.json|reference\.md|llms(?:-full)?\.txt)$`)
	// The marker must start a line, behind at most whitespace and a comment
	// leader, so a file that merely quotes the marker in a string literal (a
	// test fixture, a generator's own source) is not itself generated.
	generatedHeader = regexp.MustCompile(`(?im)^[ \t]*(?://|#|/\*|\*|--|<!--)?[ \t]*Code generated .* DO NOT EDIT\.`)
	markerPattern   = regexp.MustCompile(`(feature|scenario):([a-z0-9][a-z0-9-]*)`)
	goModulePattern = regexp.MustCompile(`(?m)^module[\t-\r \x1c-\x1f\x{0085}\p{Z}]+([^\t-\r \x1c-\x1f\x{0085}\p{Z}]+)`)
)

// ImpactPathAdmitted reports whether the immutable index admits the path's
// suffix. It does not imply that impact has a reverse-import rule for it.
func ImpactPathAdmitted(value string) bool {
	return textSuffixes[strings.ToLower(path.Ext(value))]
}

type Source struct {
	Path, BlobHash string
	Data           []byte
	Mode           string
	// Checked and Valid record one textDecodable pass over Data, made when the
	// source was pinned. Data is never written after that, so Text can alias
	// it instead of validating and copying the body on every call: on the
	// Beamfall corpus that copy and its garbage were a fifth of a build's CPU.
	Checked, Valid bool
	// body is a context-load pack body not yet verified; Data stays nil and
	// Text verifies it on first read (packBody, IDX-SNAP-V0-015).
	body packBody
}

// Text reports separately whether the source bytes were loaded and whether
// those loaded bytes are valid text. An unloaded source is never an empty one.
func (source Source) Text() (text string, valid, loaded bool) {
	if source.body.section != nil {
		return source.body.text(source)
	}
	if !source.Checked {
		if source.Data == nil {
			return "", false, false
		}
		if !textDecodable(source.Data) {
			return "", false, true
		}
		return string(source.Data), true, true
	}
	if !source.Valid {
		return "", false, true
	}
	if len(source.Data) == 0 {
		return "", true, true
	}
	return unsafe.String(unsafe.SliceData(source.Data), len(source.Data)), true, true
}

// textDecodable is the text test: no NUL in the first 8 KiB and valid UTF-8
// throughout.
func textDecodable(data []byte) bool {
	prefix := data
	if len(prefix) > 8192 {
		prefix = prefix[:8192]
	}
	return bytes.IndexByte(prefix, 0) < 0 && utf8.Valid(data)
}

type Exclusion struct{ Path, Reason string }

// Unparsed names a source the index admitted and counted, but extracted no
// facts from. An Exclusion is a path the index never took in; an Unparsed path
// is one it did take in and then dropped the contents of. Without this table
// the two failure modes a caller most needs to separate -- a file that defines
// nothing, and a file whose definitions were discarded -- are the same
// observation: zero symbols against a source that is present and counted.
//
// It exists because of the defect class this index is most exposed to:
// extraction that stops, emits nothing, and leaves the source counted as
// indexed, so the index reports success over partial data and no counter
// anywhere can observe the omission. A refusal that is recorded is recoverable;
// one that is swallowed is invisible until someone re-derives the truth set by
// hand.
//
// Facts names which table went short -- "symbols" or "imports" -- because a
// source can be refused for one and complete for the other. Reason is the
// stable code a caller branches on. Detail carries the extractor's own refusal
// verbatim, which distinguishes a source the grammar rejected from one that
// merely exceeded a nesting limit. BlobHash binds the loss to exact bytes, so a
// caller can tell a recurring refusal from the same file changing underneath it.
type Unparsed struct{ Path, BlobHash, Facts, Reason, Detail string }

const (
	// UnparsedPythonGrammar marks a .py source the closed 3.12 grammar refused.
	// Every definition and import in the file is absent from the index.
	UnparsedPythonGrammar = "PYTHON_SOURCE_UNPARSED"
	// UnparsedNotText marks a source whose bytes are not decodable text, so no
	// line-oriented extractor ran over it at all.
	UnparsedNotText = "SOURCE_NOT_TEXT"
	// UnparsedGoGrammar marks a .go source go/parser refused. Symbols fall back
	// to the line scanner, so that table is approximate rather than absent;
	// imports have no fallback and are absent.
	UnparsedGoGrammar = "GO_SOURCE_UNPARSED"
	// UnparsedWebLexical marks a JavaScript or TypeScript source whose import
	// lexer reached end of file still inside a block comment or a template
	// literal. Everything after that opener was consumed as literal text, so
	// any import among those lines is missing from the edge set. The file is
	// still indexed and searchable; only its import row is short, which is
	// exactly the loss that would otherwise be indistinguishable from a file
	// that imports nothing.
	UnparsedWebLexical = "WEB_SOURCE_UNPARSED"
)

type Record struct {
	Kind, ID, Path, BlobHash string
	Line                     int
	Fields                   map[string]any
}

type Symbol struct {
	Kind, Name, Path, BlobHash string
	Line                       int
	// EndLine is the declaration's last line, 1-based and inclusive, or 0 when
	// the extractor that produced the symbol names no extent. Only the Python
	// grammar reports one and only the Python context window reads it, because
	// the oracle clamps that window to the declaration end and clamps no other
	// (DR-0005, decision 0007 D4 as amended).
	EndLine int
}

type Marker struct {
	Path, BlobHash string
	Line, Column   int
}

type Index struct {
	Root, ObjectFormat, CommitRevision, Revision, ProfileID, Module string
	StatusSHA256                                                    string
	Sources                                                         map[string]Source
	// Tracked is every path in the tree at Revision, including the kinds the
	// index never reads; a caller naming a subject checks it here (TCP-V0-002).
	Tracked map[string]struct{}
	// Skipped records non-blob tree entries, including gitlinks (SBQ-V0-008).
	Skipped    map[string]struct{}
	Exclusions []Exclusion
	// UnsupportedSuffixCount counts the tracked blobs admittedEntries never
	// read because no allow-listed suffix admits them. They carry no
	// Exclusion row, so the receipt adds this count to its exclusion count
	// (GPK-V0-063) instead of asserting the index read everything else.
	UnsupportedSuffixCount         int
	DirtyPaths                     []string
	Features, Scenarios, Documents map[string]Record
	Markers                        map[string][]Marker
	Symbols                        []Symbol
	Imports                        map[string]map[string]struct{}
	// Unparsed lists, in path order, every admitted source whose facts were
	// dropped. It is the denominator that makes a silent extraction loss
	// countable.
	Unparsed []Unparsed
	// ApproximateImports counts the sources whose import edges no grammar
	// produced: the web languages, whose lexer tells code from comment and
	// literal but recognises no `require()` edge and resolves no specifier,
	// and every other non-Go language, whose scanner has no lexical state at
	// all. It is an aggregate rather than a per-source list because the
	// approximation is a property of the language support, not of any one
	// file: recording it per file would name every JavaScript source in the
	// repository and train its reader to ignore the field.
	ApproximateImports int
	// ExtractionNotes records sources that were indexed as text but whose
	// symbol walk did not complete. It is deliberately separate from
	// Exclusions: an excluded path is absent from Sources, while a noted path
	// is present, searchable, and only partly understood. It is also narrower
	// than Unparsed: Unparsed names a source a grammar refused outright, this
	// names one a scanner walked but could not finish.
	ExtractionNotes []ExtractionNote
	// Vocabulary is the packet's term table (termtable.go), built with the
	// tables on the build and packet paths and carried in the snapshot.
	Vocabulary *TermTable
	blobFacts  map[string]*blobFacts
	// bodies is the deferred pack bodies section of a deferred load, whose
	// first failed body read withholds the result (SnapshotRefusal).
	bodies *packSection
}

// vocabulary returns the term table, building it once for an index compiled
// without one (a query index, or a fixture) so the packet has one source of
// truth for its terms.
func (index *Index) vocabulary() *TermTable {
	if index.Vocabulary == nil {
		index.Vocabulary = index.buildVocabulary()
	}
	return index.Vocabulary
}

// UnparsedPaths returns the set of paths whose facts were dropped, for callers
// that need membership rather than the ordered table.
func (index *Index) UnparsedPaths() map[string]string {
	paths := make(map[string]string, len(index.Unparsed))
	for _, item := range index.Unparsed {
		paths[item.Path] = item.Reason
	}
	return paths
}

func Build(ctx context.Context, root string) (*Index, error) {
	return buildStable(ctx, root, (*Index).compile)
}

// BuildContext compiles the same evidence as Build. Test-relation anchors
// consume imports beyond the explicit subject; omitting them changes cold
// packet ranks and coverage relative to a snapshot hit.
func BuildContext(ctx context.Context, root, subject string) (*Index, error) {
	return buildStable(ctx, root, contextCompile(subject))
}

// importPolicy says whether a source's text is worth extracting imports from.
// nil extracts every source's imports.
type importPolicy func(text string) bool

func noImports(string) bool { return false }

// BuildEval builds the index the harness query event evaluates over. EvalQuery
// ranks exclusively out of Features, Scenarios, Documents, Symbols and Markers,
// and reaches Imports only through featureImplementationCandidates, which reads
// index.Imports[markerPath] for the Go marker test paths of a competitive
// record. Extracting imports for those paths alone is the only table narrowing
// the query profile admits: every other table it reads is compiled in full.
//
// BuildQuery stays narrow and unchanged -- it serves the authority-start query
// verb, which never hands its index to EvalQuery.
func BuildEval(ctx context.Context, root string) (*Index, error) {
	return buildStable(ctx, root, (*Index).compileEval)
}

func buildStableFrom(ctx context.Context, root string, opening *repositoryObservation, compile func(*Index)) (*Index, error) {
	ctx, cancel := context.WithTimeout(ctx, gitDeadline)
	defer cancel()
	for attempt := 0; attempt < 3; attempt++ {
		before := openingObservation(ctx, root, attempt, opening)
		if before.identityErr != nil {
			return nil, before.identityErr
		}
		if before.statusErr != nil {
			return nil, before.statusErr
		}
		candidate, err := buildEvidence(ctx, root, before.identity, before.dirty, before.statusSHA256)
		if err != nil {
			return nil, err
		}
		// The repository is not read again after buildEvidence returns: compile
		// derives every table from candidate.Sources, which are already pinned
		// bytes. So the closing observation runs beside it, and the window it
		// closes is exactly the window in which the evidence was read -- a
		// tighter claim than one that also spans a pure in-memory pass.
		closing := make(chan repositoryObservation, 1)
		go func() { closing <- observeRepository(ctx, root) }()
		compile(candidate)
		after := <-closing
		if after.statusErr != nil {
			return nil, after.statusErr
		}
		if after.identityErr != nil {
			return nil, after.identityErr
		}
		if before.identity != after.identity {
			continue
		}
		if before.statusSHA256 != after.statusSHA256 || !slicesEqual(before.dirty, after.dirty) {
			adoptStatus(candidate, after)
		}
		return candidate, nil
	}
	return nil, &Error{Message: "repository HEAD changed while building context index"}
}

// adoptStatus takes the closing observation's worktree status when only the
// status moved inside the window. Every pinned byte came from the tree
// (DIRTY-CACHE-001), so a status change cannot have reached the evidence; it
// changes only which paths are reported dirty. Rebuilding for it cost a whole
// second evidence pass on 1 of 11 clean-worktree runs (a racy `git status`
// refresh between the two observations), about 400 ms at p95. The dirty set
// becomes the closing status exactly: Git status is the freshness authority,
// including when attributes make clean worktree bytes differ from tree blobs.
func adoptStatus(index *Index, closing repositoryObservation) {
	dirty := make(map[string]struct{}, len(closing.dirty))
	for _, item := range closing.dirty {
		dirty[item] = struct{}{}
	}
	index.DirtyPaths, index.StatusSHA256 = keys(dirty), closing.statusSHA256
}

// repositoryObservation is one paired read of the repository's identity and
// its working-tree status.
type repositoryObservation struct {
	identity     repositoryIdentity
	identityErr  error
	dirty        []string
	statusSHA256 string
	statusErr    error
}

// observeRepository reads the identity and the status snapshot concurrently.
// They are two independent Git invocations, and a build takes four of them --
// one pair to open the stability window and one to close it -- so run
// sequentially each pair spent a whole process spawn and scan waiting for the
// other. Both errors are carried out rather than returned, so each caller
// still reports whichever failure its sequential form reported first.
func observeRepository(ctx context.Context, root string) repositoryObservation {
	var observation repositoryObservation
	var pending sync.WaitGroup
	pending.Add(1)
	go func() {
		defer pending.Done()
		observation.dirty, observation.statusSHA256, observation.statusErr = readStatusSnapshot(ctx, root)
	}()
	observation.identity, observation.identityErr = readIdentity(ctx, root)
	pending.Wait()
	return observation
}

// BuildQuery captures the bounded immutable evidence required by the narrow
// authority-start query profile. Impact continues to use Build so its complete
// source index remains unchanged.
func BuildQuery(ctx context.Context, root, text string) (*Index, error) {
	ctx, cancel := context.WithTimeout(ctx, gitDeadline)
	defer cancel()
	queryText := pythonLower(TrimPythonSpace(text))
	queryTerms := terms(queryText)
	for attempt := 0; attempt < 3; attempt++ {
		before := observeRepository(ctx, root)
		if before.identityErr != nil {
			return nil, before.identityErr
		}
		if before.statusErr != nil {
			return nil, before.statusErr
		}
		candidate, err := buildQueryAttempt(ctx, root, before.identity, before.dirty, before.statusSHA256, queryText, queryTerms)
		if err != nil {
			return nil, err
		}
		// The query build reads the repository throughout, so its closing
		// observation cannot overlap anything; only the pair itself is paired.
		after := observeRepository(ctx, root)
		if after.statusErr != nil {
			return nil, after.statusErr
		}
		if after.identityErr != nil {
			return nil, after.identityErr
		}
		if before.identity != after.identity {
			continue
		}
		if before.statusSHA256 != after.statusSHA256 || !slicesEqual(before.dirty, after.dirty) {
			adoptStatus(candidate, after)
		}
		return candidate, nil
	}
	return nil, &Error{Message: "repository HEAD changed while building context index"}
}

func buildAttempt(ctx context.Context, root string, identity repositoryIdentity, status []string, statusSHA256 string, compile func(*Index)) (*Index, error) {
	index, err := buildEvidence(ctx, root, identity, status, statusSHA256)
	if err != nil {
		return nil, err
	}
	compile(index)
	return index, nil
}

// buildEvidence is buildAttempt's reading half: everything up to and including
// the pinned sources, with no table compiled. It is the only half that touches
// the repository, which is what lets buildStable close its stability window
// around it alone.
func buildEvidence(ctx context.Context, root string, identity repositoryIdentity, status []string, statusSHA256 string) (*Index, error) {
	skipped := make(map[string]struct{})
	entries, err := readTreeEntries(ctx, root, identity, skipped)
	if err != nil {
		return nil, err
	}
	exclusions, candidates, unsupported := admittedEntries(entries)
	dirty := make(map[string]struct{}, len(status))
	for _, item := range status {
		dirty[item] = struct{}{}
	}
	pinnedSources, err := pinCandidates(ctx, root, identity, candidates, dirty)
	if err != nil {
		return nil, err
	}
	sources := make(map[string]Source, len(candidates))
	for position, pinned := range pinnedSources {
		entry := candidates[position]
		if reason := pinned.exclusionReason(); reason != "" {
			exclusions = append(exclusions, Exclusion{entry.path, reason})
			continue
		}
		sources[entry.path] = pinned.source
	}
	sort.Slice(exclusions, func(left, right int) bool { return exclusions[left].Path < exclusions[right].Path })
	dirtyPaths := keys(dirty)
	tracked := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		tracked[entry.path] = struct{}{}
	}
	index := &Index{
		Root: root, ObjectFormat: identity.objectFormat, CommitRevision: identity.commitRevision,
		Revision: identity.treeRevision, StatusSHA256: statusSHA256,
		Sources: sources, Exclusions: exclusions, DirtyPaths: dirtyPaths, Tracked: tracked, Skipped: skipped,
		Features: map[string]Record{}, Scenarios: map[string]Record{}, Documents: map[string]Record{},
		Markers: map[string][]Marker{}, Imports: map[string]map[string]struct{}{},
		ProfileID: projectprofile.Fallback().ID, UnsupportedSuffixCount: unsupported,
	}
	return index, nil
}

func admittedEntries(entries []treeEntry) ([]Exclusion, []treeEntry, int) {
	exclusions := make([]Exclusion, 0)
	candidates := make([]treeEntry, 0)
	unsupported := 0
	for _, entry := range entries {
		if reason := forbiddenPath(entry.path); reason != "" {
			exclusions = append(exclusions, Exclusion{entry.path, reason})
			continue
		}
		base := path.Base(entry.path)
		if !ImpactPathAdmitted(entry.path) && base != "Makefile" && base != "go.mod" {
			unsupported++
			continue
		}
		if entry.size > maxSourceBytes {
			exclusions = append(exclusions, Exclusion{entry.path, "source exceeds size bound"})
			continue
		}
		candidates = append(candidates, entry)
	}
	return exclusions, candidates, unsupported
}

type pinnedEntry struct {
	source     Source
	generated  bool
	lfsPointer bool
}

// pinSources materialises every candidate's pinned bytes: the committed blob,
// or the worktree copy when it still hashes to that blob. Entries arrive in
// tree order, so one opener held across the whole pass resolves each parent
// directory once per run of files beneath it, instead of rebuilding the chain
// from the repository root for every file -- which is what a per-file
// readCleanFile did. Fanning the reads across the CPUs was measured and does
// not pay: the openat is contended, so twelve workers spend three times the CPU
// for a wall time inside the noise of one.
func pinSources(root string, identity repositoryIdentity, entries []treeEntry, blobOf func(treeEntry) []byte, dirty map[string]struct{}) []pinnedEntry {
	pinned := make([]pinnedEntry, len(entries))
	opener, opened := newCleanFileOpener(root)
	if opened {
		defer opener.close()
	}
	for position, entry := range entries {
		disk, worktreePinned := worktreePin(opener, entry, identity, dirty)
		if worktreePinned {
			pinned[position] = pinnedFrom(entry, disk)
			continue
		}
		pinned[position] = pinnedFrom(entry, blobOf(entry))
	}
	return pinned
}

// buildEagerBlobs restores the construction that read every candidate's
// committed blob before pinning any of them. Nothing in the product sets it;
// the pin parity test flips it so one binary can build an index both ways and
// compare the result.
var buildEagerBlobs = false

// pinCandidates is the pin pass under the parity switch. Both branches choose
// the same bytes for the same entry -- see pinSourcesDeferringBlobs -- so they
// are the same index at different cost.
func pinCandidates(ctx context.Context, root string, identity repositoryIdentity, candidates []treeEntry, dirty map[string]struct{}) ([]pinnedEntry, error) {
	if _, qualified := ctx.Value(gitExecutionKey{}).(gitExecution); qualified {
		return pinGitObjects(ctx, root, candidates)
	}
	if buildEagerBlobs {
		blobs, err := readBlobs(ctx, root, candidates)
		if err != nil {
			return nil, err
		}
		return pinSources(root, identity, candidates, func(entry treeEntry) []byte { return blobs[entry.oid] }, dirty), nil
	}
	return pinSourcesDeferringBlobs(ctx, root, identity, candidates, dirty)
}

// pinSourcesDeferringBlobs is pinSources over a whole-repository candidate set,
// reading the committed blob of only the entries the clean worktree could not
// pin by itself. A worktree copy that hashes to its recorded oid IS that blob,
// which is why pinSources already prefers it -- so on a clean repository every
// fetched body was decompressed to be discarded. It was the largest single term
// in a build: 152ms of 470ms over Beamfall's 2,617 admitted files.
//
// The blob read is deferred rather than dropped. A dirty path, a symlink or
// submodule entry, and a worktree copy that has drifted from its oid all still
// need the committed bytes, and they are fetched in one batch exactly as before.
func pinSourcesDeferringBlobs(ctx context.Context, root string, identity repositoryIdentity, entries []treeEntry, dirty map[string]struct{}) ([]pinnedEntry, error) {
	pinned := make([]pinnedEntry, len(entries))
	foreseen := make(chan residualBlobs, 1)
	go func() { foreseen <- fetchResidualBlobs(ctx, root, entries, foreseenResidualEntries(entries, dirty)) }()
	fromWorktree := pinWorktree(root, identity, entries, dirty, pinned)
	first := <-foreseen
	if first.err != nil {
		return nil, first.err
	}
	second := fetchResidualBlobs(ctx, root, entries, driftedResidualEntries(entries, dirty, fromWorktree))
	if second.err != nil {
		return nil, second.err
	}
	for position, entry := range entries {
		if fromWorktree[position] {
			continue
		}
		pinned[position] = pinnedFrom(entry, residualBlob(first.blobs, second.blobs, entry.oid))
	}
	return pinned, nil
}

// pinWorktree fills every position the clean worktree can pin and reports
// which those are, leaving the rest zero for the caller's blob pass. Reading
// and hashing tens of megabytes is now the whole cost of a build, and the
// files are independent, so the pass is split across the CPUs on the same
// terms as verifyCleanEntries: contiguous windows over tree order so a worker
// keeps hitting one cached parent directory, an opener per worker because a
// chain cannot be shared, and a fan-out scaled to the work rather than to the
// machine. Each worker writes only its own window's positions.
func pinWorktree(root string, identity repositoryIdentity, entries []treeEntry, dirty map[string]struct{}, pinned []pinnedEntry) []bool {
	fromWorktree := make([]bool, len(entries))
	eligible := 0
	for _, entry := range entries {
		if eligibleForWorktreePin(entry, dirty) {
			eligible++
		}
	}
	if eligible == 0 {
		return fromWorktree
	}
	window := func(low, high int) {
		opener, opened := newCleanFileOpener(root)
		if !opened {
			return
		}
		defer opener.close()
		for position := low; position < high; position++ {
			entry := entries[position]
			disk, ok := worktreePin(opener, entry, identity, dirty)
			if !ok {
				continue
			}
			pinned[position] = pinnedFrom(entry, disk)
			fromWorktree[position] = true
		}
	}
	workers := min(runtime.NumCPU(), max(eligible/entriesPerWorkerOverride, 1))
	if workers == 1 {
		window(0, len(entries))
		return fromWorktree
	}
	var pending sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		pending.Add(1)
		low := worker * len(entries) / workers
		high := (worker + 1) * len(entries) / workers
		go func(low, high int) {
			defer pending.Done()
			window(low, high)
		}(low, high)
	}
	pending.Wait()
	return fromWorktree
}

// worktreePin returns the worktree bytes that pin entry, or reports that the
// committed blob is still required. dirty is the read-only Git status set;
// only a path's own bit affects its eligibility.
func worktreePin(opener *cleanFileOpener, entry treeEntry, identity repositoryIdentity, dirty map[string]struct{}) ([]byte, bool) {
	if !eligibleForWorktreePin(entry, dirty) {
		return nil, false
	}
	disk, ok := readCleanFileUsing(opener, entry.path)
	if !ok || gitBlobHash(disk, identity.objectFormat) != entry.oid {
		return nil, false
	}
	return disk, true
}

func eligibleForWorktreePin(entry treeEntry, dirty map[string]struct{}) bool {
	if entry.mode != "100644" && entry.mode != "100755" {
		return false
	}
	return !contains(dirty, entry.path)
}

func pinnedFrom(entry treeEntry, data []byte) pinnedEntry {
	prefix := data
	if len(prefix) > 4096 {
		prefix = prefix[:4096]
	}
	return pinnedEntry{
		source:     Source{Path: entry.path, BlobHash: entry.oid, Data: data, Mode: entry.mode, Checked: true, Valid: textDecodable(data)},
		generated:  isGeneratedHeader(prefix),
		lfsPointer: bytes.HasPrefix(prefix, []byte(lfsPointerPrefix)),
	}
}

func (pinned pinnedEntry) exclusionReason() string {
	if pinned.lfsPointer {
		return lfsPointerExclusion
	}
	if pinned.generated {
		return "generated-file header excluded"
	}
	return ""
}

// isGeneratedHeader reports generatedHeader.Match(prefix) without walking the
// NFA over every source. The pattern is the line-anchored
// `(?im)^...Code generated .* DO NOT EDIT\.`, which has no literal prefix, so Go's regexp cannot skip and steps all 4096
// prefix bytes of all 2987 files. Every match must contain "code generated"
// under case folding, so the fold search below is an exact gate. It is exact
// specifically because that literal is ASCII and contains no rune whose Unicode
// fold reaches outside ASCII (no k, no s, no i).
func isGeneratedHeader(prefix []byte) bool {
	if !containsFoldASCII(prefix, "code generated") {
		return false
	}
	return generatedHeader.Match(prefix)
}

func containsFoldASCII(data []byte, lowerNeedle string) bool {
	limit := len(data) - len(lowerNeedle)
	for start := 0; start <= limit; start++ {
		if matchesFoldASCIIAt(data[start:], lowerNeedle) {
			return true
		}
	}
	return false
}

func matchesFoldASCIIAt(data []byte, lowerNeedle string) bool {
	for offset := 0; offset < len(lowerNeedle); offset++ {
		if lowerASCII(data[offset]) != lowerNeedle[offset] {
			return false
		}
	}
	return true
}

func lowerASCII(value byte) byte {
	if value >= 'A' && value <= 'Z' {
		return value + ('a' - 'A')
	}
	return value
}

func buildQueryAttempt(ctx context.Context, root string, identity repositoryIdentity, status []string, statusSHA256, queryText string, queryTerms map[string]struct{}) (*Index, error) {
	entries, err := readTreeEntries(ctx, root, identity)
	if err != nil {
		return nil, err
	}
	exclusions, candidates, unsupported := admittedEntries(entries)
	if err := validateQueryBlobAdmission(candidates); err != nil {
		return nil, err
	}
	byPath := make(map[string]treeEntry, len(candidates))
	dirty := make(map[string]struct{}, len(status))
	for _, item := range status {
		dirty[item] = struct{}{}
	}
	sources, exclusions, loadedPaths, err := querySourceInventory(ctx, root, identity, candidates, exclusions, dirty)
	if err != nil {
		return nil, err
	}
	for _, entry := range candidates {
		byPath[entry.path] = entry
	}
	authorities := queryAuthorityEntries(candidates, sources)
	authorityPaths, err := loadQueryPaths(ctx, root, authorities, loadedPaths)
	if err != nil {
		return nil, err
	}
	for position, pinned := range pinSources(root, identity, authorities, func(entry treeEntry) []byte { return authorityPaths[entry.path] }, dirty) {
		entry := authorities[position]
		if reason := pinned.exclusionReason(); reason != "" {
			delete(sources, entry.path)
			exclusions = append(exclusions, Exclusion{entry.path, reason})
			continue
		}
		sources[entry.path] = pinned.source
	}
	gateEntries := queryGateEntries(candidates, sources)
	gateContent, err := loadQueryPaths(ctx, root, gateEntries, loadedPaths)
	if err != nil {
		return nil, err
	}
	for position, pinned := range pinSources(root, identity, gateEntries, func(entry treeEntry) []byte { return gateContent[entry.path] }, dirty) {
		entry := gateEntries[position]
		if reason := pinned.exclusionReason(); reason != "" {
			delete(sources, entry.path)
			exclusions = append(exclusions, Exclusion{entry.path, reason})
			continue
		}
		sources[entry.path] = pinned.source
	}
	index := queryIndex(root, identity, statusSHA256, sources, exclusions, dirty)
	index.UnsupportedSuffixCount = unsupported
	for _, entry := range authorities {
		source, ok := index.Sources[entry.path]
		if !ok {
			continue
		}
		text, valid, loaded := source.Text()
		if !loaded || !valid {
			continue
		}
		if document, ok := documentRecord(source, text, index.Sources); ok {
			index.Documents[document.ID] = document
		}
	}
	selected, err := selectQueryAuthority(index, queryText, queryTerms)
	if err != nil {
		return finishQueryIndex(index, dirty), nil
	}
	references := make([]string, 0, 8)
	seenReferences := make(map[string]struct{})
	for _, candidate := range selected[:min(2, len(selected))] {
		for _, reference := range stringsField(candidate.result["references"]) {
			if _, duplicate := seenReferences[reference]; duplicate {
				continue
			}
			seenReferences[reference] = struct{}{}
			references = append(references, reference)
		}
	}
	referenceEntries := make([]treeEntry, 0, len(references))
	for _, reference := range references {
		entry, ok := byPath[reference]
		if !ok {
			return nil, &Error{Code: "unsupported-query-authority", Message: "native Go authority-start query found an unavailable instruction reference"}
		}
		referenceEntries = append(referenceEntries, entry)
	}
	referencePaths, err := loadQueryPaths(ctx, root, referenceEntries, loadedPaths)
	if err != nil {
		return nil, err
	}
	for position, pinned := range pinSources(root, identity, referenceEntries, func(entry treeEntry) []byte { return referencePaths[entry.path] }, dirty) {
		entry := referenceEntries[position]
		if reason := pinned.exclusionReason(); reason != "" {
			delete(index.Sources, entry.path)
			index.Exclusions = append(index.Exclusions, Exclusion{entry.path, reason})
			return finishQueryIndex(index, dirty), nil
		}
		index.Sources[entry.path] = pinned.source
	}
	return finishQueryIndex(index, dirty), nil
}

func finishQueryIndex(index *Index, dirty map[string]struct{}) *Index {
	sort.Slice(index.Exclusions, func(left, right int) bool { return index.Exclusions[left].Path < index.Exclusions[right].Path })
	index.DirtyPaths = keys(dirty)
	return index
}

// cleanVerification is one entry's clean-file verdict. The verdicts are computed
// concurrently but folded in tree order, so the exclusion list, the dirty set,
// and the immutable list come out exactly as a sequential pass would leave them.
type cleanVerification struct {
	attempted bool
	ok        bool
	generated bool
}

// verifyCleanEntries hashes every clean regular file to prove it still matches
// the tree, which is the dominant cost of a query build: on a Beamfall-sized
// repository it reads tens of megabytes. The files are independent, so the work
// is split across the CPUs. Each worker keeps its own scratch buffer because
// the buffer and its generated-header prefix are reused per file.
func verifyCleanEntries(root string, identity repositoryIdentity, entries []treeEntry, dirty map[string]struct{}) []cleanVerification {
	verified := make([]cleanVerification, len(entries))
	for index, entry := range entries {
		regular := entry.mode == "100644" || entry.mode == "100755"
		_, changed := dirty[entry.path]
		verified[index].attempted = regular && !changed
	}
	// Scale the fan-out to the work, never to the machine: each worker keeps a
	// 36KiB scratch buffer, so spawning one per CPU over a small tree costs more
	// in allocation than the concurrency saves. Below entriesPerWorker the
	// verification stays single-buffered, exactly as the sequential pass was.
	attempted := 0
	for index := range verified {
		if verified[index].attempted {
			attempted++
		}
	}
	if attempted == 0 {
		return verified
	}
	workers := min(runtime.NumCPU(), max(attempted/entriesPerWorkerOverride, 1))
	if workers == 1 {
		opener, opened := newCleanFileOpener(root)
		if !opened {
			return verified
		}
		defer opener.close()
		var scratch cleanFileVerificationScratch
		for index, entry := range entries {
			if !verified[index].attempted {
				continue
			}
			generated, ok := verifyCleanFileUsing(opener, entry.path, entry.oid, identity.objectFormat, &scratch)
			verified[index].generated, verified[index].ok = generated, ok
		}
		return verified
	}
	var pending sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		pending.Add(1)
		// Contiguous chunks, never a stride: entries arrive in tree order, so a
		// worker that owns a run of them keeps hitting the same cached parent
		// directory. Striding would hand each directory to a different worker
		// and rebuild the chain for every file.
		low := worker * len(entries) / workers
		high := (worker + 1) * len(entries) / workers
		go func(low, high int) {
			defer pending.Done()
			opener, opened := newCleanFileOpener(root)
			if !opened {
				return
			}
			defer opener.close()
			var scratch cleanFileVerificationScratch
			for index := low; index < high; index++ {
				if !verified[index].attempted {
					continue
				}
				entry := entries[index]
				generated, ok := verifyCleanFileUsing(opener, entry.path, entry.oid, identity.objectFormat, &scratch)
				verified[index].generated, verified[index].ok = generated, ok
			}
		}(low, high)
	}
	pending.Wait()
	return verified
}

func querySourceInventory(ctx context.Context, root string, identity repositoryIdentity, entries []treeEntry, exclusions []Exclusion, dirty map[string]struct{}) (map[string]Source, []Exclusion, map[string][]byte, error) {
	sources := make(map[string]Source, len(entries))
	immutable := make([]treeEntry, 0)
	verified := verifyCleanEntries(root, identity, entries, dirty)
	for index, entry := range entries {
		if result := verified[index]; result.attempted {
			if result.ok {
				if result.generated {
					exclusions = append(exclusions, Exclusion{entry.path, "generated-file header excluded"})
					continue
				}
				sources[entry.path] = Source{Path: entry.path, BlobHash: entry.oid, Mode: entry.mode}
				continue
			}
		}
		immutable = append(immutable, entry)
	}
	blobs, err := readQueryBlobs(ctx, root, immutable)
	if err != nil {
		return nil, nil, nil, err
	}
	loadedPaths := make(map[string][]byte, len(immutable))
	for _, entry := range immutable {
		data := blobs[entry.oid]
		loadedPaths[entry.path] = data
		if generatedHeader.Match(data[:min(len(data), 4096)]) {
			exclusions = append(exclusions, Exclusion{entry.path, "generated-file header excluded"})
			continue
		}
		sources[entry.path] = Source{Path: entry.path, BlobHash: entry.oid, Mode: entry.mode}
	}
	return sources, exclusions, loadedPaths, nil
}

func loadQueryPaths(ctx context.Context, root string, entries []treeEntry, loadedPaths map[string][]byte) (map[string][]byte, error) {
	missing := make([]treeEntry, 0, len(entries))
	for _, entry := range entries {
		if _, loaded := loadedPaths[entry.path]; !loaded {
			missing = append(missing, entry)
		}
	}
	blobs, err := readQueryBlobs(ctx, root, missing)
	if err != nil {
		return nil, err
	}
	for _, entry := range missing {
		loadedPaths[entry.path] = blobs[entry.oid]
	}
	result := make(map[string][]byte, len(entries))
	for _, entry := range entries {
		result[entry.path] = loadedPaths[entry.path]
	}
	return result, nil
}

func queryAuthorityEntries(entries []treeEntry, sources map[string]Source) []treeEntry {
	result := make([]treeEntry, 0)
	for _, entry := range entries {
		if _, present := sources[entry.path]; present && documentKind(entry.path) == "instructions" && projectOperationPath(entry.path) {
			result = append(result, entry)
		}
	}
	return result
}

// queryGateEntries names the files the closing verification command is derived
// from. The standalone query inventory records every admitted path but loads
// content only for what it ranks. Before Text exposed whether bytes were loaded,
// the Makefile scan behind gateCommand mistook missing bytes for an empty file
// and selected the signal-free fallback. The gate file is loaded through the
// same pinning the authorities use, so divergence and generated-header handling
// are identical.
func queryGateEntries(entries []treeEntry, sources map[string]Source) []treeEntry {
	result := make([]treeEntry, 0, 1)
	for _, entry := range entries {
		if entry.path != "Makefile" && entry.path != "makefile" {
			continue
		}
		if _, present := sources[entry.path]; present {
			result = append(result, entry)
		}
	}
	return result
}

func sourcePresence(sources map[string]Source) func(string) bool {
	return func(path string) bool {
		_, present := sources[path]
		return present
	}
}

func queryIndex(root string, identity repositoryIdentity, statusSHA256 string, sources map[string]Source, exclusions []Exclusion, dirty map[string]struct{}) *Index {
	profileID := projectprofile.Detect(sourcePresence(sources)).ID
	return &Index{
		Root: root, ObjectFormat: identity.objectFormat, CommitRevision: identity.commitRevision,
		Revision: identity.treeRevision, StatusSHA256: statusSHA256, Sources: sources,
		Exclusions: exclusions, DirtyPaths: keys(dirty), Features: map[string]Record{},
		Scenarios: map[string]Record{}, Documents: map[string]Record{}, Markers: map[string][]Marker{},
		Imports: map[string]map[string]struct{}{}, ProfileID: profileID,
	}
}

// compile builds every index table. compileEval builds the strict subset the
// query profile ranks out of: it is the same walk with the whole-repository
// import extraction removed, replaced afterwards by the bounded marker-test
// extraction featureImplementationCandidates is the only consumer of.
func (index *Index) compile() {
	index.compileTables(nil)
	index.sortUnparsed()
	index.Vocabulary = index.buildVocabulary()
	index.Vocabulary.SymbolWindows = index.buildSymbolWindows()
}

func (index *Index) compileEval() {
	index.compileTables(noImports)
	index.compileMarkerTestImports()
	index.sortUnparsed()
}

// sortUnparsed puts the refusal record in a total order that does not depend on
// which compile path produced it: the chunked walk emits in path order and
// compileMarkerTestImports appends after it in map-iteration order.
func (index *Index) sortUnparsed() {
	sort.Slice(index.Unparsed, func(left, right int) bool {
		a, b := index.Unparsed[left], index.Unparsed[right]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.Facts < b.Facts
	})
}

// compileMarkerTestImports extracts imports for the Go marker test paths, which
// is exactly the Imports key set featureImplementationCandidates dereferences:
// it consults index.Imports[markerPath] only for a marker whose path isTestPath,
// and discards every imported value unless the marker path ends in ".go".
//
// The same test path can carry more than one marker (a file tagged with both a
// feature and a scenario ID lands in two index.Markers buckets), so this tracks
// paths already visited independently of extraction succeeding: index.Imports
// only records a path that yielded imports, and gating on that alone re-ran a
// refused extraction for every marker sharing the path, filing one Unparsed row
// per repeat instead of one per source and inflating unparsed.count past what
// compile's single per-source pass reports.
func (index *Index) compileMarkerTestImports() {
	visited := map[string]struct{}{}
	for _, markers := range index.Markers {
		for _, marker := range markers {
			if !strings.HasSuffix(marker.Path, ".go") || !isTestPath(marker.Path) {
				continue
			}
			if _, seen := visited[marker.Path]; seen {
				continue
			}
			visited[marker.Path] = struct{}{}
			text, valid, loaded := index.Sources[marker.Path].Text()
			if !loaded || !valid {
				continue
			}
			extraction := sourceImports(marker.Path, text)
			if len(extraction.imports) != 0 {
				index.Imports[marker.Path] = extraction.imports
			}
			if extraction.refusal != "" {
				index.Unparsed = append(index.Unparsed, Unparsed{Path: marker.Path, Facts: "imports", Reason: extraction.reason, Detail: extraction.refusal})
			}
		}
	}
}

func (index *Index) compileTables(imports importPolicy) {
	profile := projectprofile.Detect(sourcePresence(index.Sources))
	index.ProfileID = profile.ID
	if profile.FeaturesPath != "" {
		index.Features = parseRecords(index.Sources[profile.FeaturesPath], "features", "feature")
	}
	if profile.ScenariosPath != "" {
		index.Scenarios = parseRecords(index.Sources[profile.ScenariosPath], "scenarios", "scenario")
	}
	if source, ok := index.Sources["go.mod"]; ok {
		if text, valid, loaded := source.Text(); loaded && valid {
			if match := goModulePattern.FindStringSubmatch(text); match != nil {
				index.Module = match[1]
			}
		}
	}
	index.compileSources(imports)
	sort.Slice(index.Symbols, func(left, right int) bool {
		a, b := index.Symbols[left], index.Symbols[right]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Name < b.Name
	})
	for key := range index.Markers {
		sort.Slice(index.Markers[key], func(left, right int) bool {
			a, b := index.Markers[key][left], index.Markers[key][right]
			return a.Path < b.Path || (a.Path == b.Path && (a.Line < b.Line || (a.Line == b.Line && a.Column < b.Column)))
		})
	}
}

// sourcesPerWorker is how many sources have to be waiting before a second
// worker, and its own marker and import tables, earns its allocation.
const sourcesPerWorker = 128

type compiledSources struct {
	documents          []Record
	symbols            []Symbol
	notes              []ExtractionNote
	markers            map[string][]Marker
	imports            map[string]map[string]struct{}
	unparsed           []Unparsed
	approximateImports int
}

// compileSources walks each source once for the document record, markers,
// symbols and imports it carries. Sources are independent and the walk reads
// only index.Sources, so it is chunked across the CPUs over a sorted path order
// and merged back in that order. The symbol and marker tables are sorted after
// the merge exactly as before; the document table now resolves an id collision
// by path order, where the map-iteration walk resolved it arbitrarily.
func (index *Index) compileSources(imports importPolicy) {
	paths := make([]string, 0, len(index.Sources))
	for sourcePath := range index.Sources {
		paths = append(paths, sourcePath)
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return
	}
	workers := min(runtime.NumCPU(), max(len(paths)/sourcesPerWorker, 1))
	chunks := make([]compiledSources, workers)
	var pending sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		pending.Add(1)
		low := worker * len(paths) / workers
		high := (worker + 1) * len(paths) / workers
		go func(chunk *compiledSources, window []string) {
			defer pending.Done()
			chunk.collect(index, window, imports)
		}(&chunks[worker], paths[low:high])
	}
	pending.Wait()
	for _, chunk := range chunks {
		for _, document := range chunk.documents {
			index.Documents[document.ID] = document
		}
		index.Symbols = append(index.Symbols, chunk.symbols...)
		index.ExtractionNotes = append(index.ExtractionNotes, chunk.notes...)
		for key, markers := range chunk.markers {
			index.Markers[key] = append(index.Markers[key], markers...)
		}
		for sourcePath, imported := range chunk.imports {
			index.Imports[sourcePath] = imported
		}
		// Chunks are consecutive windows of the sorted path order and are
		// merged in that order, so the concatenation is already path-sorted.
		index.Unparsed = append(index.Unparsed, chunk.unparsed...)
		index.ApproximateImports += chunk.approximateImports
	}
	// Chunks merge in worker order over sorted path windows, so notes arrive
	// path-ordered already; sorting makes that a guarantee of this function
	// rather than a property of how the windows happen to be cut.
	sortNotes(index.ExtractionNotes)
}

func (chunk *compiledSources) collect(index *Index, paths []string, imports importPolicy) {
	chunk.markers = make(map[string][]Marker)
	chunk.imports = make(map[string]map[string]struct{})
	for _, sourcePath := range paths {
		source := index.Sources[sourcePath]
		// Text is taken once and handed on: documentRecord ignores the validity
		// bit exactly as it did when it read the body itself, so an unreadable
		// source still reaches it as the empty string.
		text, valid, loaded := source.Text()
		if !loaded {
			continue
		}
		if document, ok := documentRecord(source, text, index.Sources); ok {
			chunk.documents = append(chunk.documents, document)
		}
		if fact := index.blobFacts[sourcePath]; fact != nil {
			chunk.collectBlobFact(fact, imports, text)
			continue
		}
		if !valid {
			// Undecodable bytes drop markers, symbols and imports alike. The
			// source stays counted, so the loss is recorded rather than implied.
			chunk.unparsed = append(chunk.unparsed, Unparsed{Path: source.Path, BlobHash: source.BlobHash, Facts: "all", Reason: UnparsedNotText, Detail: "source bytes are not valid UTF-8 text"})
			continue
		}
		// markerPattern is case-sensitive and cannot match without one of these
		// two literals, so a file lacking both yields no markers on any line.
		// Gating here also skips the split: on the Beamfall corpus 560 of
		// 1,383,094 lines carry a marker, in 132 of 2987 files.
		if strings.Contains(text, "feature:") || strings.Contains(text, "scenario:") {
			for lineNumber, line := range strings.Split(text, "\n") {
				for _, match := range markerMatches(line) {
					key := match.kind + ":" + match.id
					chunk.markers[key] = append(chunk.markers[key], Marker{source.Path, source.BlobHash, lineNumber + 1, match.column})
				}
			}
		}
		if strings.HasSuffix(source.Path, ".py") {
			// Both facts come from one walk; see pythonSourceFacts.
			symbols, imported, refusal := pythonSourceFacts(source, text)
			if refusal != "" {
				// The grammar rejected the file, so EVERY definition and import
				// in it is missing -- not merely the construct that failed.
				chunk.unparsed = append(chunk.unparsed, Unparsed{Path: source.Path, BlobHash: source.BlobHash, Facts: "symbols", Reason: UnparsedPythonGrammar, Detail: refusal})
			}
			chunk.symbols = append(chunk.symbols, symbols...)
			if len(imported) != 0 && (imports == nil || imports(text)) {
				chunk.imports[source.Path] = imported
			}
			continue
		}
		if strings.HasSuffix(source.Path, ".go") {
			symbols, refusal := goSymbols(source, text)
			chunk.symbols = append(chunk.symbols, symbols...)
			if refusal != "" {
				chunk.unparsed = append(chunk.unparsed, Unparsed{Path: source.Path, BlobHash: source.BlobHash, Facts: "symbols", Reason: UnparsedGoGrammar, Detail: refusal})
			}
		}
		// The lexically-extracted languages report what they could not walk
		// alongside what they found, so a capped or unterminated file leaves a
		// trace instead of being counted as indexed with nothing to declare.
		if symbols, notes, handled := languageSymbols(source, text); handled {
			chunk.symbols = append(chunk.symbols, symbols...)
			chunk.notes = append(chunk.notes, notes...)
		}
		if imports != nil && !imports(text) {
			continue
		}
		chunk.recordImports(source.Path, text)
	}
}

// recordImports files one source's import edges together with the evidence of
// how they were derived, so neither a grammar refusal nor a scanner
// approximation reaches Index.Imports unannotated.
func (chunk *compiledSources) recordImports(sourcePath, text string) {
	extraction := sourceImports(sourcePath, text)
	if len(extraction.imports) != 0 {
		chunk.imports[sourcePath] = extraction.imports
	}
	if extraction.refusal != "" {
		chunk.unparsed = append(chunk.unparsed, Unparsed{Path: sourcePath, Facts: "imports", Reason: extraction.reason, Detail: extraction.refusal})
	}
	if extraction.approximate {
		chunk.approximateImports++
	}
}

func gitBlobHash(data []byte, objectFormat string) string {
	var header [32]byte
	prefix := append(header[:0], "blob "...)
	prefix = strconv.AppendInt(prefix, int64(len(data)), 10)
	prefix = append(prefix, 0)
	if objectFormat == "sha256" {
		hash := sha256.New()
		_, _ = hash.Write(prefix)
		_, _ = hash.Write(data)
		return hex.EncodeToString(hash.Sum(nil))
	}
	hash := sha1.New()
	_, _ = hash.Write(prefix)
	_, _ = hash.Write(data)
	return hex.EncodeToString(hash.Sum(nil))
}

func forbiddenPath(value string) string {
	for _, part := range strings.Split(value, "/") {
		if forbiddenParts[part] {
			return "vendor/build excluded"
		}
	}
	if strings.HasPrefix(value, "internal/store/migrate/") || strings.HasPrefix(value, "internal/conformance/testdata/") {
		return "protected path"
	}
	if generatedPath.MatchString(value) {
		return "generated path"
	}
	return ""
}

// ForbiddenPathReason exposes the IDX-SNAP-V0-018 path screen: the exclusion
// reason for value, or "" when the screen admits it. Learned-trace path
// admission consumes it so the two can never hold different sets (decision 0102).
func ForbiddenPathReason(value string) string { return forbiddenPath(value) }

func contains(values map[string]struct{}, key string) bool { _, ok := values[key]; return ok }

func keys(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func slicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func normalizedReference(documentPath, candidate string, sources map[string]Source) string {
	candidate = strings.TrimRight(candidate, ".,:;)")
	if _, ok := sources[candidate]; ok {
		return candidate
	}
	relative := path.Clean(path.Join(path.Dir(documentPath), candidate))
	if _, ok := sources[relative]; ok {
		return relative
	}
	return ""
}

func repositoryRelative(value string) bool {
	return value != "" && !filepath.IsAbs(value) && value != "."
}

// Observation is the header-level repository state a caller needs to re-prove
// that the repository did not move, without compiling an index.
type Observation struct {
	ObjectFormat, CommitRevision, Revision, StatusSHA256 string
	DirtyPaths                                           []string
}

// Observe reads exactly the header fields Build derives from Git, opening no
// source and parsing nothing. ProfileID is deliberately absent: it is
// projectprofile.Detect over the source set, which is a pure function of the
// tree, so an equal Revision already implies an equal ProfileID.
func Observe(ctx context.Context, root string) (Observation, error) {
	ctx, cancel := context.WithTimeout(ctx, gitDeadline)
	defer cancel()
	observed := observeRepository(ctx, root)
	if observed.identityErr != nil {
		return Observation{}, observed.identityErr
	}
	if observed.statusErr != nil {
		return Observation{}, observed.statusErr
	}
	return Observation{
		ObjectFormat:   observed.identity.objectFormat,
		CommitRevision: observed.identity.commitRevision,
		Revision:       observed.identity.treeRevision,
		StatusSHA256:   observed.statusSHA256,
		DirtyPaths:     observed.dirty,
	}, nil
}

// buildStable opens its stability window on a fresh observation.
func buildStable(ctx context.Context, root string, compile func(*Index)) (*Index, error) {
	return buildStableFrom(ctx, root, nil, compile)
}

// openingObservation is the loader's observation for the first attempt when
// a caller carried one in, and a fresh paired read otherwise. A retry after
// HEAD moved must observe again: the carried pair described the old HEAD.
func openingObservation(ctx context.Context, root string, attempt int, opening *repositoryObservation) repositoryObservation {
	if attempt == 0 && opening != nil {
		return *opening
	}
	return observeRepository(ctx, root)
}

// residualBlobs is one committed-blob fetch's result, carried across the
// goroutine boundary that lets the fetch overlap the worktree pin pass.
type residualBlobs struct {
	blobs map[string][]byte
	err   error
}

func fetchResidualBlobs(ctx context.Context, root string, all, needed []treeEntry) residualBlobs {
	blobs, err := readResidualBlobs(ctx, root, all, needed)
	return residualBlobs{blobs: blobs, err: err}
}

// foreseenResidualEntries are the entries the worktree pin pass will refuse
// before it reads a byte: dirty paths and non-regular modes
// (eligibleForWorktreePin). Their committed blobs are known to be needed at
// the start of the pass, so the one cat-file batch that fetches them runs
// beside the pass instead of after it. On a dirty checkout that is the whole
// residual set, and the spawn disappears from the critical path.
func foreseenResidualEntries(entries []treeEntry, dirty map[string]struct{}) []treeEntry {
	foreseen := make([]treeEntry, 0)
	for _, entry := range entries {
		if !eligibleForWorktreePin(entry, dirty) {
			foreseen = append(foreseen, entry)
		}
	}
	return foreseen
}

// driftedResidualEntries are the eligible entries whose worktree copy did not
// hash to the recorded oid -- clean by status, different by bytes. They are
// known only after the pass, so they cost a second batch when there are any;
// a clean tree with no filter has none and spawns nothing here.
func driftedResidualEntries(entries []treeEntry, dirty map[string]struct{}, fromWorktree []bool) []treeEntry {
	drifted := make([]treeEntry, 0)
	for position, entry := range entries {
		if fromWorktree[position] {
			continue
		}
		if eligibleForWorktreePin(entry, dirty) {
			drifted = append(drifted, entry)
		}
	}
	return drifted
}

func residualBlob(first, second map[string][]byte, oid string) []byte {
	if blob, ok := first[oid]; ok {
		return blob
	}
	return second[oid]
}
