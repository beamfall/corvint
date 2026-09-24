package contextindex

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/Beamfall/corvint/internal/projectprofile"

	"github.com/Beamfall/corvint/internal/runtimeenv"
)

// blobShardVersion is deliberately separate from the tree encoding. Full path
// context is conservative: parser diagnostics and language rules can depend on
// the filename. This prototype does not claim reuse across renames.
const blobShardVersion = "corvint-blob-facts/2"
const maxBlobShardBytes = 32 * maxSourceBytes
const maxBlobShardReadBytes = 512 * 1024 * 1024
const maxBlobShardTokens = 250_000
const maxBlobShardReadTokens = 8_000_000

type blobReadBudget struct{ bytes, tokens atomic.Int64 }

type blobFacts struct {
	Version, Engine, ObjectFormat string
	Source                        Source
	Terms                         map[string]uint32
	Words                         map[string]struct{}
	Symbols                       []Symbol
	Markers                       map[string][]Marker
	Imports                       map[string]struct{}
	Unparsed                      []Unparsed
	Notes                         []ExtractionNote
	ApproximateImports            int
}

func blobShardsEnabled() bool { return runtimeenv.Value("INDEX_SHARDS") == "1" }

func blobShardPath(directory, format, engineID, oid, path string) string {
	return filepath.Join(directory, "blobs", engineID, fmt.Sprintf("%s-%s-%x.afs", format, oid, sha256.Sum256([]byte(path))))
}

// BuildForSnapshot is the writer's full-table build. Other Build consumers do
// not opt into shard reads, preserving the oracle and harness build paths.
func BuildForSnapshot(ctx context.Context, root string) (*Index, error) {
	if blobShardsEnabled() {
		if index, err := buildWithBlobShards(ctx, root, nil, (*Index).compile); err == nil {
			return index, nil
		}
	}
	return Build(ctx, root)
}

func buildWithBlobShards(ctx context.Context, root string, opening *LoaderObservation, compile func(*Index)) (*Index, error) {
	ctx, cancel := context.WithTimeout(ctx, gitDeadline)
	defer cancel()
	engineID := analyzerEngine()
	if engineID == "" {
		return nil, errNoEngine
	}
	var observed *repositoryObservation
	if opening != nil {
		observed = &opening.observed
	}
	for attempt := 0; attempt < 3; attempt++ {
		before := openingObservation(ctx, root, attempt, observed)
		if before.identityErr != nil {
			return nil, before.identityErr
		}
		if before.statusErr != nil {
			return nil, before.statusErr
		}
		index, err := shardEvidence(ctx, root, before, engineID)
		if err != nil {
			return nil, err
		}
		closing := make(chan repositoryObservation, 1)
		go func() { closing <- observeRepository(ctx, root) }()
		compile(index)
		after := <-closing
		if after.identityErr != nil {
			return nil, after.identityErr
		}
		if after.statusErr != nil {
			return nil, after.statusErr
		}
		if before.identity != after.identity {
			continue
		}
		adoptStatus(index, after)
		return index, nil
	}
	return nil, &Error{Message: "repository HEAD changed while building context index"}
}

func shardEvidence(ctx context.Context, root string, observation repositoryObservation, engineID string) (*Index, error) {
	identity := observation.identity
	skipped := make(map[string]struct{})
	entries, err := readTreeEntries(ctx, root, identity, skipped)
	if err != nil {
		return nil, err
	}
	exclusions, candidates, unsupported := admittedEntries(entries)
	if err := validateBlobAdmission(candidates); err != nil {
		return nil, err
	}
	if _, err := uniqueBlobEntries(candidates); err != nil {
		return nil, err
	}
	facts, err := readBlobFacts(root, identity.objectFormat, engineID, candidates)
	if err != nil {
		return nil, err
	}
	missing := make([]treeEntry, 0)
	for _, entry := range candidates {
		if facts[entry.path] == nil {
			missing = append(missing, entry)
		}
	}
	blobs, err := readResidualBlobs(ctx, root, candidates, missing)
	if err != nil {
		return nil, err
	}
	sources := make(map[string]Source, len(candidates))
	for _, entry := range candidates {
		if fact := facts[entry.path]; fact != nil {
			source := fact.Source
			source.Mode = entry.mode
			sources[entry.path] = source
			continue
		}
		pinned := pinnedFrom(entry, blobs[entry.oid])
		if reason := pinned.exclusionReason(); reason != "" {
			exclusions = append(exclusions, Exclusion{entry.path, reason})
			continue
		}
		sources[entry.path] = pinned.source
	}
	sort.Slice(exclusions, func(i, j int) bool { return exclusions[i].Path < exclusions[j].Path })
	tracked := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		tracked[entry.path] = struct{}{}
	}
	index := &Index{Root: root, ObjectFormat: identity.objectFormat, CommitRevision: identity.commitRevision,
		Revision: identity.treeRevision, Sources: sources, Tracked: tracked, Skipped: skipped, Exclusions: exclusions, UnsupportedSuffixCount: unsupported,
		Features: map[string]Record{}, Scenarios: map[string]Record{}, Documents: map[string]Record{},
		Markers: map[string][]Marker{}, Imports: map[string]map[string]struct{}{}, ProfileID: projectprofile.Fallback().ID,
		blobFacts: facts}
	adoptStatus(index, observation)
	return index, nil
}

func readBlobFacts(root, format, engineID string, entries []treeEntry) (map[string]*blobFacts, error) {
	store := locateSnapshotStore(root)
	results := make([]*blobFacts, len(entries))
	failures := make([]error, len(entries))
	workers := min(4, runtime.NumCPU(), max(1, len(entries)))
	var pending sync.WaitGroup
	var budget blobReadBudget
	for worker := 0; worker < workers; worker++ {
		pending.Add(1)
		go func(worker int) {
			defer pending.Done()
			for i := worker; i < len(entries); i += workers {
				results[i], failures[i] = readBlobFact(store.base, store.directory, format, engineID, entries[i], &budget)
			}
		}(worker)
	}
	pending.Wait()
	facts := make(map[string]*blobFacts, len(entries))
	for i, entry := range entries {
		if failures[i] != nil {
			return nil, failures[i]
		}
		if results[i] != nil {
			facts[entry.path] = results[i]
		}
	}
	return facts, nil
}

// readBlobFact reads one fact under the store directory; base anchors the
// no-follow walk (snapshotStore).
func readBlobFact(base, directory, format, engineID string, entry treeEntry, budget *blobReadBudget) (*blobFacts, error) {
	target := blobShardPath(directory, format, engineID, entry.oid, entry.path)
	if err := shardRegularPath(base, target); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	file, err := openBlobShard(base, target)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !stat.Mode().IsRegular() || stat.Size() < sha256.Size || stat.Size() > maxBlobShardBytes {
		return nil, errors.New("blob shard size or mode refused")
	}
	if budget != nil && budget.bytes.Add(stat.Size()) > maxBlobShardReadBytes {
		return nil, errors.New("blob shard aggregate read bound exceeded")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxBlobShardBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) != stat.Size() {
		return nil, errors.New("blob shard size changed")
	}
	payload := data[sha256.Size:]
	if sha256.Sum256(payload) != [sha256.Size]byte(data[:sha256.Size]) {
		return nil, errors.New("blob shard digest mismatch")
	}
	// Token/depth admission runs before typed decoding, so collection counts
	// cannot amplify a small forged record into an unbounded map or slice.
	count, err := countBlobFactTokens(payload)
	if err != nil {
		return nil, err
	}
	if budget != nil && budget.tokens.Add(count) > maxBlobShardReadTokens {
		return nil, errors.New("blob shard aggregate fact bound exceeded")
	}
	var fact blobFacts
	if err := json.Unmarshal(payload, &fact); err != nil {
		return nil, err
	}
	if fact.Version != blobShardVersion || fact.Engine != engineID || fact.ObjectFormat != format {
		return nil, errors.New("blob shard analyzer mismatch")
	}
	if fact.Source.Path != entry.path || fact.Source.BlobHash != entry.oid || len(fact.Source.Data) != entry.size {
		return nil, errors.New("blob shard identity mismatch")
	}
	if gitBlobHash(fact.Source.Data, format) != entry.oid {
		return nil, errors.New("blob shard body mismatch")
	}
	pinned := pinnedFrom(entry, fact.Source.Data)
	if pinned.exclusionReason() != "" {
		return nil, errors.New("blob shard contains excluded source")
	}
	fact.Source = pinned.source
	return &fact, nil
}

// Reject symlinks at every shard path component. The store is derived local
// state; symlinked stores are unsupported and reads fall back to Git.
func shardRegularPath(root, target string) error {
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return err
	}
	current := root
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("symlinked blob shard refused")
		}
	}
	return nil
}

func (index *Index) buildVocabulary() *TermTable {
	if index.blobFacts == nil {
		return buildTermTable(index.Sources)
	}
	paths := blobSourcePaths(index.Sources)
	perSource := make([]sourceTerms, len(paths))
	tokeniser := newTokeniser()
	for i, path := range paths {
		if fact := index.blobFacts[path]; fact != nil {
			perSource[i] = sourceTerms{terms: fact.Terms, words: fact.Words, pathTerms: lexicalTerms(path)}
			continue
		}
		perSource[i] = tokeniseSource(path, index.Sources[path], tokeniser)
	}
	return &TermTable{Paths: paths, Terms: invertCounts(perSource),
		Words:     invertPresence(perSource, func(s sourceTerms) map[string]struct{} { return s.words }),
		PathTerms: invertPresence(perSource, func(s sourceTerms) map[string]struct{} { return stringSetOf(s.pathTerms) })}
}

func (chunk *compiledSources) collectBlobFact(fact *blobFacts, imports importPolicy, text string) {
	chunk.symbols = append(chunk.symbols, fact.Symbols...)
	chunk.notes = append(chunk.notes, fact.Notes...)
	for key, markers := range fact.Markers {
		chunk.markers[key] = append(chunk.markers[key], markers...)
	}
	includeImports := imports == nil || imports(text)
	for _, refusal := range fact.Unparsed {
		if refusal.Facts != "imports" || includeImports {
			chunk.unparsed = append(chunk.unparsed, refusal)
		}
	}
	if !includeImports {
		return
	}
	if len(fact.Imports) != 0 {
		chunk.imports[fact.Source.Path] = fact.Imports
	}
	chunk.approximateImports += fact.ApproximateImports
}

// writeBlobShards is called only by WriteSnapshot, after store ignores exist.
// Publication is a synced temporary followed by rename. Existing valid facts
// are immutable; corrupt facts are replaceable only by this explicit writer.
func writeBlobShards(index *Index, store snapshotStore, engineID string) error {
	base, directory := store.base, store.directory
	paths := blobSourcePaths(index.Sources)
	tokeniser := newTokeniser()
	for _, path := range paths {
		source := index.Sources[path]
		entry := treeEntry{path, source.BlobHash, source.Mode, len(source.Data)}
		if fact, err := readBlobFact(base, directory, index.ObjectFormat, engineID, entry, nil); err == nil && fact != nil {
			continue
		}
		local := &Index{Sources: map[string]Source{path: source}}
		var chunk compiledSources
		chunk.collect(local, []string{path}, nil)
		terms := tokeniseSource(path, source, tokeniser)
		fact := blobFacts{Version: blobShardVersion, Engine: engineID, ObjectFormat: index.ObjectFormat, Source: source,
			Terms: terms.terms, Words: terms.words, Symbols: chunk.symbols, Markers: chunk.markers,
			Imports: chunk.imports[path], Unparsed: chunk.unparsed, Notes: chunk.notes, ApproximateImports: chunk.approximateImports}
		payload, err := json.Marshal(fact)
		if err != nil {
			return err
		}
		if len(payload)+sha256.Size > maxBlobShardBytes {
			continue
		}
		if _, err := countBlobFactTokens(payload); err != nil {
			continue
		}
		digest := sha256.Sum256(payload)
		target := blobShardPath(directory, index.ObjectFormat, engineID, source.BlobHash, path)
		if err := publishBlobFact(base, target, append(digest[:], payload...)); err != nil {
			return err
		}
	}
	return nil
}

func blobSourcePaths(sources map[string]Source) []string {
	paths := make([]string, 0, len(sources))
	for path := range sources {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

// JSON has no caller-controlled allocation hints. The token and depth limits
// also bound decoded collection expansion; scalar bytes already have the
// encoded record/aggregate budget. No facts are retained during this pass.
func countBlobFactTokens(data []byte) (int64, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	var count int64
	depth := 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return count, nil
		}
		if err != nil {
			return 0, err
		}
		count++
		if count > maxBlobShardTokens {
			return 0, errors.New("blob shard fact count exceeded")
		}
		delimiter, ok := token.(json.Delim)
		if !ok {
			continue
		}
		switch delimiter {
		case '{', '[':
			depth++
		case '}', ']':
			depth--
		}
		if depth > 32 {
			return 0, errors.New("blob shard nesting bound exceeded")
		}
	}
}
