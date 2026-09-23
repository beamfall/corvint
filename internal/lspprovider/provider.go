// Package lspprovider is Corvint's optional local language-server evidence
// provider (external-evidence-provider-v0 EEP-V0-023..026, task-context-packet-v0
// TCP-V0-043..046, decision 0371). It starts gopls once per invocation in an
// owned process group, asks it for bounded one- and two-hop definition and
// reference expansion from a few seed files, and returns an
// external-evidence-provider/2 record of path-to-path relations. The record
// is external evidence: Core decodes, verifies and labels it exactly as any
// provider record, and it never becomes project authority.
package lspprovider

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/procgroup"
)

// Provider identity, schema and bounds (EEP-V0-024, EEP-V0-025).
const (
	ProviderID       = "gopls"
	Schema           = "external-evidence-provider/2"
	RepositoryID     = "root"
	MaxSeeds         = 3
	MaxHops          = 2
	MaxRows          = 32
	MaxRecordBytes   = 64 << 10
	MaxQueries       = 64
	perKind          = 8
	maxHopTwoOrigins = 3
	Timeout          = 20 * time.Second
	softDeadline     = 15 * time.Second
	maxSessionBytes  = 64 << 20
	maxStderrBytes   = 64 << 10
	maxText          = 512
)

// Request names the repository, the seed paths in priority order, the
// committed text and blob of a path, and the gopls executable.
type Request struct {
	Root, Revision string
	Seeds          []string
	Committed      func(path string) (text, blob string, ok bool)
	Executable     string
}

// Result is the record bytes, or a failure reason when there is no record,
// the paths the record's relations are anchored on, and the query summary
// that carries the query digest (EEP-V0-024).
type Result struct {
	Record  []byte
	Failure string
	Origins []string
	Query   map[string]any
}

type link struct {
	from, to, method, symbol string
	hop, line, character     int
	seed, via                string
}

type expansion struct {
	version  string
	links    []link
	queries  int
	stopped  string
	outside  int
	unpinned int
	failed   int
	first    string
	foreign  string
}

// Expand runs one bounded expansion. It never fails: an unusable seed set,
// a missing executable, or a failed or timed-out session is a Result with a
// Failure and no record (EEP-V0-026).
func Expand(ctx context.Context, request Request) Result {
	seeds := usableSeeds(request)
	result := Result{Query: querySummary(request, seeds)}
	if len(seeds) == 0 {
		result.Failure = "not applicable: no committed, unmodified Go file among the seeds"
		return result
	}
	if request.Executable == "" {
		result.Failure = "gopls executable not found"
		return result
	}
	root, err := filepath.EvalSymlinks(request.Root)
	if err != nil {
		result.Failure = "repository root unavailable"
		return result
	}
	origin, err := rootCommit(ctx, root, request.Revision)
	if err != nil {
		result.Failure = "root commit unavailable"
		return result
	}
	found, failure := run(ctx, root, request, seeds)
	if failure != "" {
		result.Failure = failure
		return result
	}
	result.Query["queries_issued"] = found.queries
	result.Query["failed_queries"] = found.failed
	if found.queries > 0 && found.failed == found.queries {
		result.Failure = fmt.Sprintf("gopls answered all %d queries with an error; first: %s", found.queries, clip(printable(found.first)))
		return result
	}
	result.Record, result.Origins = record(request, origin, found, result.Query)
	return result
}

// usableSeeds keeps at most MaxSeeds distinct Go seeds whose committed text
// equals the working tree, since gopls reads the working tree (EEP-V0-025).
func usableSeeds(request Request) []string {
	var seeds []string
	for _, seed := range request.Seeds {
		if len(seeds) == MaxSeeds {
			break
		}
		if strings.HasSuffix(seed, ".go") && !slices.Contains(seeds, seed) && pinned(request, seed) {
			seeds = append(seeds, seed)
		}
	}
	return seeds
}

func pinned(request Request, path string) bool {
	text, _, ok := request.Committed(path)
	if !ok {
		return false
	}
	disk, err := os.ReadFile(filepath.Join(request.Root, filepath.FromSlash(path)))
	return err == nil && string(disk) == text
}

// querySummary is the provider-independent query and its digest: the same
// revision, seeds and bounds give the same digest on every host.
func querySummary(request Request, seeds []string) map[string]any {
	bounds := map[string]any{
		"seeds": MaxSeeds, "hops": MaxHops, "rows": MaxRows, "record_bytes": MaxRecordBytes,
		"queries": MaxQueries, "queries_per_kind": perKind, "hop_two_origins": maxHopTwoOrigins,
		"wall_ms": Timeout.Milliseconds(),
	}
	canonical, _ := json.Marshal(map[string]any{"provider": ProviderID, "revision": request.Revision, "seeds": seeds, "bounds": bounds})
	digest := sha256.Sum256(canonical)
	if seeds == nil {
		seeds = []string{}
	}
	return map[string]any{"provider": ProviderID, "sha256": hex.EncodeToString(digest[:]), "seeds": seeds, "bounds": bounds}
}

func rootCommit(ctx context.Context, root, revision string) (string, error) {
	command := exec.CommandContext(ctx, "git", "-C", root, "rev-list", "--max-parents=0", revision)
	output, err := command.Output()
	if err != nil {
		return "", err
	}
	roots := strings.Fields(string(output))
	if len(roots) == 0 {
		return "", errors.New("no root commit")
	}
	sort.Strings(roots)
	return roots[0], nil
}

// run owns one gopls process for the whole session: started here, bounded
// by Timeout, and its process group proven cleaned up before a record is
// trusted. gopls's file cache lives in a private temporary directory that is
// removed afterwards; module downloads and toolchain switches are disabled.
func run(ctx context.Context, root string, request Request, seeds []string) (expansion, string) {
	cache, err := os.MkdirTemp("", "corvint-gopls-")
	if err != nil {
		return expansion{}, "gopls cache directory unavailable"
	}
	defer os.RemoveAll(cache)
	var found expansion
	observation := procgroup.Run(ctx, procgroup.Spec{
		Argv: []string{request.Executable, "serve"}, Dir: root, Env: environment(cache),
		Timeout: Timeout, OutputLimit: 1 << 20, StderrLimit: maxStderrBytes, OverflowPolicy: procgroup.OverflowTruncate,
		Dialogue: func(reader io.Reader, writer io.WriteCloser) error {
			var err error
			found, err = dialogue(root, request, seeds, reader, writer)
			return err
		},
	})
	switch {
	case !observation.Started:
		return found, "gopls did not start"
	case observation.TimedOut:
		return found, fmt.Sprintf("gopls exceeded %s wall time; process group killed", Timeout)
	case observation.Cancelled:
		return found, "gopls cancelled"
	case found.foreign != "":
		return found, fmt.Sprintf("language server identified as %s, not gopls; refused", found.foreign)
	case found.version == "":
		return found, "gopls session failed"
	case !observation.ExitObserved || observation.ExitStatus != 0:
		return found, "gopls did not exit cleanly"
	case !observation.OwnedProcessGroupCleanup:
		return found, "gopls process group not proven cleaned up"
	}
	return found, ""
}

// environment passes what the Go toolchain needs to load the module and
// nothing that could fetch: no proxy, no checksum database, no toolchain
// download.
func environment(cache string) []string {
	var out []string
	for _, name := range []string{"PATH", "HOME", "TMPDIR", "GOPATH", "GOMODCACHE", "GOCACHE", "GOROOT", "GOFLAGS", "GOWORK"} {
		if value, ok := os.LookupEnv(name); ok && !strings.ContainsRune(value, 0) {
			out = append(out, name+"="+value)
		}
	}
	return append(out, "GOPLSCACHE="+cache, "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "LANG=C", "LC_ALL=C")
}

// dialogue is the whole LSP exchange: initialize, hop one from every seed,
// hop two from the strongest hop-one files, shutdown, exit, then drain until
// the server closes its output.
func dialogue(root string, request Request, seeds []string, reader io.Reader, writer io.WriteCloser) (expansion, error) {
	counted := &limitedReader{reader: reader, remaining: maxSessionBytes}
	s := &session{reader: bufio.NewReader(counted), writer: writer, root: root}
	initialized, err := s.call("initialize", map[string]any{
		"processId": nil, "rootUri": fileURI(root), "capabilities": map[string]any{},
		"workspaceFolders": []any{map[string]any{"uri": fileURI(root), "name": "root"}},
	})
	if err != nil {
		return expansion{}, err
	}
	if name := serverName(initialized); name != ProviderID {
		return expansion{foreign: name}, errForeign
	}
	if err := s.notify("initialized", map[string]any{}); err != nil {
		return expansion{}, err
	}
	walk := walker{session: s, root: root, request: request, started: time.Now(), known: map[string]bool{}}
	for _, seed := range seeds {
		walk.known[seed] = true
	}
	for _, seed := range seeds {
		if err := walk.expand(seed, 1, seed, ""); err != nil {
			return expansion{}, err
		}
	}
	for _, origin := range walk.hopTwoOrigins() {
		if err := walk.expand(origin.path, 2, origin.seed, origin.path); err != nil {
			return expansion{}, err
		}
	}
	walk.found.version = serverVersion(initialized)
	if _, err := s.call("shutdown", nil); err != nil {
		return expansion{}, err
	}
	if err := s.notify("exit", nil); err != nil {
		return expansion{}, err
	}
	_ = writer.Close()
	_, _ = io.Copy(io.Discard, counted)
	return walk.found, nil
}

type walker struct {
	session *session
	root    string
	request Request
	started time.Time
	known   map[string]bool
	found   expansion
}

// expand issues every query of one origin until a budget stops the walk and
// records one link per distinct in-repository file each answer names.
func (walk *walker) expand(origin string, hop int, seed, via string) error {
	text, _, _ := walk.request.Committed(origin)
	for _, query := range targets(text, perKind) {
		if walk.found.stopped != "" {
			return nil
		}
		if walk.found.queries == MaxQueries {
			walk.found.stopped = "query-budget"
			return nil
		}
		if time.Since(walk.started) > softDeadline {
			walk.found.stopped = "soft-deadline"
			return nil
		}
		walk.found.queries++
		paths, err := walk.ask(origin, query)
		if err != nil {
			return err
		}
		for _, path := range paths {
			if path == origin || (hop == 2 && walk.known[path]) {
				continue
			}
			walk.found.links = append(walk.found.links, link{from: origin, to: path, method: query.method, symbol: query.symbol, hop: hop, line: query.line, character: query.character, seed: seed, via: via})
		}
	}
	return nil
}

// ask sends one query and returns the sorted distinct repository-relative
// paths of its locations; locations outside the repository are counted.
func (walk *walker) ask(origin string, query target) ([]string, error) {
	params := map[string]any{
		"textDocument": map[string]any{"uri": fileURI(filepath.Join(walk.root, filepath.FromSlash(origin)))},
		"position":     map[string]any{"line": query.line, "character": query.character},
	}
	if query.method == queryReferences {
		params["context"] = map[string]any{"includeDeclaration": false}
	}
	raw, err := walk.session.call(query.method, params)
	if err != nil {
		if errors.Is(err, errSession) {
			return nil, err
		}
		walk.found.failed++
		if walk.found.first == "" {
			walk.found.first = err.Error()
		}
		return nil, nil
	}
	var locations []struct {
		URI string `json:"uri"`
	}
	if json.Unmarshal(raw, &locations) != nil {
		var single struct {
			URI string `json:"uri"`
		}
		_ = json.Unmarshal(raw, &single)
		locations = append(locations, single)
	}
	seen := map[string]bool{}
	for _, location := range locations {
		path, inside := relativePath(walk.root, location.URI)
		if location.URI != "" && !inside {
			walk.found.outside++
		}
		if inside {
			seen[path] = true
		}
	}
	paths := make([]string, 0, len(seen))
	for path := range seen {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths, nil
}

type hopOrigin struct{ path, seed string }

// hopTwoOrigins ranks hop-one files by how many hop-one links reach them,
// then by path, and keeps the first maxHopTwoOrigins as hop-two origins.
func (walk *walker) hopTwoOrigins() []hopOrigin {
	counts := map[string]int{}
	seeds := map[string]string{}
	for _, found := range walk.found.links {
		if walk.known[found.to] {
			continue
		}
		counts[found.to]++
		if _, ok := seeds[found.to]; !ok {
			seeds[found.to] = found.seed
		}
	}
	paths := make([]string, 0, len(counts))
	for path := range counts {
		paths = append(paths, path)
	}
	sort.Slice(paths, func(i, j int) bool {
		if counts[paths[i]] != counts[paths[j]] {
			return counts[paths[i]] > counts[paths[j]]
		}
		return paths[i] < paths[j]
	})
	var out []hopOrigin
	for _, path := range paths {
		walk.known[path] = true
	}
	for _, path := range paths {
		if len(out) < maxHopTwoOrigins && strings.HasSuffix(path, ".go") && pinned(walk.request, path) {
			out = append(out, hopOrigin{path: path, seed: seeds[path]})
		}
	}
	return out
}

// record builds the external-evidence-provider/2 record: one relation per
// distinct (origin, file, type) whose both sides are pinned to their
// committed blobs, in walk order, bounded by MaxRows and MaxRecordBytes.
func record(request Request, origin string, found expansion, query map[string]any) ([]byte, []string) {
	type key struct{ from, to, kind string }
	seen := map[key]bool{}
	relations := []any{}
	var origins []string
	omitted := 0
	for _, found := range found.links {
		kind := relationType(found.method)
		if seen[key{found.from, found.to, kind}] {
			continue
		}
		seen[key{found.from, found.to, kind}] = true
		fromBlob, toBlob, ok := blobs(request, found)
		if !ok || len(relations) == MaxRows {
			omitted++
			continue
		}
		relations = append(relations, relation(found, kind, fromBlob, toBlob, query["sha256"].(string)))
		if !slices.Contains(origins, found.from) {
			origins = append(origins, found.from)
		}
	}
	query["stopped"] = found.stopped
	query["outside_repository"] = found.outside
	document := map[string]any{
		"schema":       Schema,
		"provider":     map[string]any{"id": ProviderID, "revision": found.version},
		"repositories": []any{map[string]any{"id": RepositoryID, "origin": origin, "revision": request.Revision}},
		"entities":     []any{},
		"relations":    relations,
		"capabilities": map[string]any{"schemas": []any{Schema}, "evidence_kinds": []any{"inferred"}},
	}
	encoded, _ := json.Marshal(document)
	for len(encoded) > MaxRecordBytes && len(relations) > 0 {
		relations = relations[:len(relations)-1]
		document["relations"] = relations
		omitted++
		encoded, _ = json.Marshal(document)
	}
	query["omitted_rows"] = omitted
	return encoded, origins
}

func blobs(request Request, found link) (string, string, bool) {
	_, fromBlob, fromOK := request.Committed(found.from)
	_, toBlob, toOK := request.Committed(found.to)
	if !fromOK || !toOK || !pinned(request, found.to) {
		return "", "", false
	}
	return fromBlob, toBlob, true
}

func relationType(method string) string {
	if method == queryReferences {
		return typeReferencedBy
	}
	return typeUsesDefinition
}

// relation names the hop origin in every row: `from` is the file the query
// ran in, and the reference names the hop, the seed and, at hop two, the
// hop-one file it came through (EEP-V0-025).
func relation(found link, kind, fromBlob, toBlob, digest string) map[string]any {
	origin := fmt.Sprintf("hop %d from seed %s", found.hop, found.seed)
	if found.via != "" {
		origin += " via " + found.via
	}
	return map[string]any{
		"from":      map[string]any{"repository": RepositoryID, "path": found.from, "blob": fromBlob},
		"to":        map[string]any{"repository": RepositoryID, "path": found.to, "blob": toBlob},
		"type":      kind,
		"evidence":  "inferred",
		"rule":      clip(fmt.Sprintf("gopls %s at %s:%d:%d on %s", found.method, found.from, found.line+1, found.character+1, found.symbol)),
		"reference": clip(fmt.Sprintf("query sha256:%s; %s", digest, origin)),
	}
}

var versionUnsafe = regexp.MustCompile(`[^A-Za-z0-9._/-]`)

var errForeign = errors.New("language server is not gopls")

// serverName is the initialize response's serverInfo.name, reduced to the
// identifier grammar (`unnamed` when absent); only `gopls` is accepted
// (EEP-V0-024).
func serverName(initialized json.RawMessage) string {
	var response struct {
		ServerInfo struct {
			Name string `json:"name"`
		} `json:"serverInfo"`
	}
	_ = json.Unmarshal(initialized, &response)
	name := versionUnsafe.ReplaceAllString(response.ServerInfo.Name, "-")
	if name == "" {
		return "unnamed"
	}
	return name[:min(len(name), 64)]
}

// printable replaces control characters in server-supplied error text.
func printable(text string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, text)
}

// serverVersion reads gopls's version from the initialize response's
// serverInfo, whose version member is gopls's JSON build description.
func serverVersion(initialized json.RawMessage) string {
	var response struct {
		ServerInfo struct {
			Version string `json:"version"`
		} `json:"serverInfo"`
	}
	_ = json.Unmarshal(initialized, &response)
	value := response.ServerInfo.Version
	var build struct {
		Version string
		Main    struct{ Version string }
	}
	if json.Unmarshal([]byte(value), &build) == nil {
		value = build.Main.Version
		if value == "" {
			value = build.Version
		}
	}
	value = strings.TrimLeft(versionUnsafe.ReplaceAllString(value, "-"), "._/-")
	if value == "" {
		return "unknown"
	}
	return value[:min(len(value), 128)]
}

func fileURI(path string) string {
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
}

func relativePath(root, uri string) (string, bool) {
	parsed, err := url.Parse(uri)
	if err != nil || parsed.Scheme != "file" {
		return "", false
	}
	relative, err := filepath.Rel(root, filepath.FromSlash(parsed.Path))
	if err != nil || relative == "." || strings.HasPrefix(relative, "..") || filepath.IsAbs(relative) {
		return "", false
	}
	return filepath.ToSlash(relative), true
}

func clip(text string) string {
	if len(text) <= maxText {
		return text
	}
	text = text[:maxText]
	for !utf8.ValidString(text) {
		text = text[:len(text)-1]
	}
	return text
}

// limitedReader fails the session once the server has written more than
// its bound, so a runaway server cannot grow the client without limit.
type limitedReader struct {
	reader    io.Reader
	remaining int
}

func (limited *limitedReader) Read(buffer []byte) (int, error) {
	if limited.remaining <= 0 {
		return 0, errSession
	}
	buffer = buffer[:min(len(buffer), limited.remaining)]
	n, err := limited.reader.Read(buffer)
	limited.remaining -= n
	return n, err
}
