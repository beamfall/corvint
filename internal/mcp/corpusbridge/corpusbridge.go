// Package corpusbridge exposes only capabilities declared by a revalidated
// documentation corpus. It never invokes a provider, test or shell command.
package corpusbridge

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/mcp/bridge"
	"github.com/Beamfall/corvint/internal/mcp/protocol"
	"github.com/Beamfall/corvint/internal/repoenvelope"
)

type Error struct{ Code string }

func (e *Error) Error() string { return e.Code }

type ToolFailure struct{ Code, Message string }

// Registry answers corpus tools. Startup reads only the artifact header; the
// first call rederives the artifact and later calls reuse it while the digest,
// root identity and every pinned revision still verify (DCP-V1-143).
type Registry struct {
	root, path, digest string
	identity           os.FileInfo
	header             header
	load               chan struct{}
	artifact           *doccorpus.Artifact
	pins               []string
	// pinStamp is the pin reuse key (pinReuseKey) captured before the last
	// successful verification of pins; while the key is unchanged the
	// verification is not repeated (DCP-V1-316). It is guarded by load.
	pinStamp string
	live     struct {
		sync.Mutex
		key      string
		artifact *doccorpus.Artifact
		overlay  doccorpus.FreshnessOverlay
		// stamp is the checkout stamp captured before the observation that
		// matched key; an equal later stamp reuses overlay without observing
		// (DCP-V1-315).
		stamp string
	}
	// artifactFile is the stat stamp of the configured artifact file captured
	// before the last read whose digest matched; while the file keeps that
	// stamp a call neither reads nor hashes it again (DCR-V1-417).
	artifactFile struct {
		sync.Mutex
		stamp string
		// flight is the artifact read in progress; concurrent calls wait for
		// it and share its bytes or error instead of each reading and holding
		// a copy (V1-3028).
		flight *artifactFlight
	}
	// index memoizes the query index of the memoized artifact, or its
	// deterministic refusal, so neither is recomputed per call or per batch
	// path (DCP-V1-195). It is guarded by load.
	index struct {
		artifact *doccorpus.Artifact
		value    *doccorpus.QueryIndex
		err      error
	}
}

// buildIndex derives the query index; tests replace it to count derivations.
var buildIndex = doccorpus.BuildQueryIndex

// observeCorpus and verifyPinned are the Git-backed reuse-key observation
// and pin verification; tests replace them to count runs (DCP-V1-315).
var (
	observeCorpus = doccorpus.CorpusObservation
	verifyPinned  = verifyPins
)

type header struct {
	Schema       string                 `json:"schema"`
	Capabilities []doccorpus.Capability `json:"capabilities"`
}

func (h header) has(op string) bool {
	if op == "inventory" && h.Schema != doccorpus.SchemaV2 {
		return false
	}
	for _, c := range h.Capabilities {
		if c.Name == doccorpus.CapabilityFor(op) {
			return c.State == "present"
		}
	}
	return false
}

// Timing reports where one call spent its time (DCP-V1-145). Load is
// "first-call" when this call rederived the artifact and "reused" otherwise;
// Freshness is "computed" or "reused" for the live freshness overlay.
type Timing struct {
	Load        string `json:"load"`
	Freshness   string `json:"freshness"`
	LoadMS      int64  `json:"load_ms"`
	VerifyMS    int64  `json:"verify_ms"`
	FreshnessMS int64  `json:"freshness_ms"`
	QueryMS     int64  `json:"query_ms"`
	TotalMS     int64  `json:"total_ms"`
	CPUMS       int64  `json:"cpu_ms"`
}

const (
	// MaxBatchPaths bounds one paths batch; MaxBatchBytes bounds what its
	// receipts cost in the one MCP result frame, which carries the batch twice:
	// as structured content and as escaped, enveloped text. The remainder of the
	// frame bound covers the envelope, timing and a reserved unresolved row per
	// path (batchRowReserve).
	MaxBatchPaths   = 100
	MaxBatchBytes   = protocol.MaxMessageBytes - 64<<10
	batchRowReserve = 160
	batchSchema     = "corvint-corpus-batch/1"
)

// SearchReceiptDescription states the docs_search receipt views (DCP-V1-653, DCP-V1-654).
const SearchReceiptDescription = `The receipt is compact by default: each anchor once in anchors, named elsewhere by id, each limitation once in limitations, named elsewhere by index, and no capabilities unless include names "capabilities"; receipt "full" returns the full receipt.`

// SearchIncludes are the docs_search include values (DCP-V1-328, DCP-V1-654).
var SearchIncludes = []string{"claims", "capabilities"}

// FullView asks docs_search for the full receipt (DCP-V1-653).
const FullView = "full"

// SearchIncludeDescription states what include ["claims"] does on docs_search (DCP-V1-579): it
// adds claim hits to the subject hits (DCP-V1-328) rather than restricting the result to claims.
const SearchIncludeDescription = `include ["claims"] adds claim hits to the subject hits, ranked together and labelled in result_kinds; for claims only, use the claims operation (corvint.docs_claims).`

var tools = map[string]string{"corvint.docs_inventory": "inventory", "corvint.docs_info": "info", "corvint.docs_validate": "validate", "corvint.docs_concept": "concept", "corvint.docs_claims": "claims", "corvint.docs_flow": "flow", "corvint.docs_dependencies": "dependencies", "corvint.docs_recommend_tests": "recommend-tests", "corvint.docs_navigation": "navigation", "corvint.docs_vocabulary": "vocabulary", "corvint.docs_intent": "intent", "corvint.docs_search": "search", "corvint.docs_get": "get", "corvint.docs_locate": "locate", "corvint.docs_find_related": "related", "corvint.docs_coverage": "coverage", "corvint.docs_gaps": "gaps", "corvint.docs_get_journey": "journey", "corvint.docs_get_stability": "stability", "corvint.docs_trace": "trace"}

func New(root, path string) (*Registry, *Error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, &Error{"invalid-root"}
	}
	identity, err := os.Lstat(absolute)
	if err != nil || !identity.IsDir() || identity.Mode()&os.ModeSymlink != 0 {
		return nil, &Error{"invalid-root"}
	}
	raw, err := doccorpus.ReadCorpusFile(absolute, path)
	if err != nil {
		return nil, &Error{"corpus-unavailable"}
	}
	h, err := readHeader(raw)
	if err != nil || h.Schema == "" {
		return nil, &Error{"corpus-unavailable"}
	}
	r := &Registry{root: absolute, path: path, digest: doccorpus.Digest(raw), identity: identity, header: h, load: make(chan struct{}, 1)}
	// DCP-V1-143: startup keeps only the header and digest; return the
	// artifact bytes to the OS instead of holding them until the first call.
	raw = nil
	debug.FreeOSMemory()
	return r, nil
}

// readHeader decodes the top-level schema and capabilities and skips every
// other member without materializing it.
func readHeader(raw []byte) (header, error) {
	var h header
	dec := jsontext.NewDecoder(bytes.NewReader(raw))
	if tok, err := dec.ReadToken(); err != nil || tok.Kind() != '{' {
		return h, errors.New("artifact is not an object")
	}
	for dec.PeekKind() != '}' {
		name, err := dec.ReadToken()
		if err != nil {
			return h, err
		}
		switch name.String() {
		case "schema":
			err = json.UnmarshalDecode(dec, &h.Schema)
		case "capabilities":
			err = json.UnmarshalDecode(dec, &h.Capabilities)
		default:
			err = dec.SkipValue()
		}
		if err != nil {
			return h, err
		}
	}
	if _, err := dec.ReadToken(); err != nil {
		return h, err
	}
	if _, err := dec.ReadToken(); err != io.EOF {
		return h, errors.New("trailing data after artifact")
	}
	return h, nil
}
func (r *Registry) Tools() []bridge.ToolDescriptor { return descriptors(r.header) }

// AllTools lists every corpus tool as a SchemaV2 artifact declaring every
// capability would. The unified corvint-mcp server lists it while its configured
// artifact cannot be opened, and answers each of them corpus-refused (MMR-V0-014).
func AllTools() []bridge.ToolDescriptor {
	h := header{Schema: doccorpus.SchemaV2}
	for _, op := range tools {
		h.Capabilities = append(h.Capabilities, doccorpus.Capability{Name: doccorpus.CapabilityFor(op), State: "present"})
	}
	return descriptors(h)
}

func descriptors(h header) []bridge.ToolDescriptor {
	result := []bridge.ToolDescriptor{}
	for name, op := range tools {
		if !h.has(op) {
			continue
		}
		properties := map[string]any{"limit": map[string]any{"type": "integer", "minimum": 1, "maximum": doccorpus.MaxResults}}
		if h.Schema == doccorpus.SchemaV2 {
			properties["offset"] = map[string]any{"type": "integer", "minimum": 0, "maximum": doccorpus.MaxCorpusRecords * 4}
		}
		properties["retirement"] = doccorpus.RetirementInputSchema()
		// DCP-V1-332: every tool takes an optional caller revision.
		properties["caller_revision"] = map[string]any{"type": "string", "pattern": "^[0-9a-f]{40}([0-9a-f]{24})?$"}
		required := []any{}
		if doccorpus.OperationInput(op) == "query" {
			properties["query"] = map[string]any{"type": "string", "minLength": 1, "maxLength": 1024}
			required = append(required, "query")
		}
		if doccorpus.OperationInput(op) == "id" {
			properties["id"] = map[string]any{"type": "string", "minLength": 1, "maxLength": 1024}
			// DCP-V1-319: docs_intent without an id lists every intent comparison.
			// DCS-V1-597: docs_navigation takes an id or a query.
			if op != "intent" && op != "navigation" {
				required = append(required, "id")
			}
		}
		if doccorpus.OperationInput(op) == "path" {
			// DCP-V1-145: path or a paths batch, exactly one.
			properties["path"] = map[string]any{"type": "string", "minLength": 1, "maxLength": 1024}
			properties["paths"] = map[string]any{"type": "array", "minItems": 1, "maxItems": MaxBatchPaths, "uniqueItems": true, "items": map[string]any{"type": "string", "minLength": 1, "maxLength": 1024}}
		}
		if op == "claims" {
			// DCP-V1-313: a ranked claim text query in place of path or paths.
			properties["query"] = map[string]any{"type": "string", "minLength": 1, "maxLength": 1024}
			// DCP-V1-318: or a supplied status filter.
			properties["state"] = map[string]any{"type": "string", "enum": doccorpus.SuppliedStatusStates}
			properties["qualifier"] = map[string]any{"type": "string", "enum": doccorpus.SuppliedStatusQualifiers}
			properties["run"] = map[string]any{"type": "string", "minLength": 1, "maxLength": 1024}
			// DCS-V1-592: optional claim kind filter and exclusion, one kind or a list.
			kind := map[string]any{"type": "string", "enum": doccorpus.ClaimKinds}
			properties["kind"] = map[string]any{"oneOf": []any{kind, map[string]any{"type": "array", "minItems": 1, "maxItems": len(doccorpus.ClaimKinds), "uniqueItems": true, "items": kind}}}
			properties["exclude_kind"] = properties["kind"]
		}
		if op == "navigation" {
			// DCS-V1-597: or a query over flow ids, names and linked test files and titles.
			properties["query"] = map[string]any{"type": "string", "minLength": 1, "maxLength": 1024}
		}
		if op == "locate" || op == "recommend-tests" {
			// DCP-V1-310: an optional declared repository's root-commit identity.
			properties["repository"] = map[string]any{"type": "string", "minLength": 40, "maxLength": 64}
		}
		if op == "locate" {
			// DCP-V1-650: or a ranked query over module and symbol subjects,
			// optionally narrowed to one of the two kinds.
			properties["query"] = map[string]any{"type": "string", "minLength": 1, "maxLength": 1024}
			kind := map[string]any{"type": "string", "enum": doccorpus.LocateQueryKinds}
			properties["kind"] = map[string]any{"oneOf": []any{kind, map[string]any{"type": "array", "minItems": 1, "maxItems": len(doccorpus.LocateQueryKinds), "uniqueItems": true, "items": kind}}}
		}
		if op == "gaps" {
			properties["id"] = map[string]any{"type": "string", "maxLength": 1024}
			// DCP-V1-284: optional gap kind filter, one kind or a list.
			kind := map[string]any{"type": "string", "minLength": 1, "maxLength": 128}
			properties["kind"] = map[string]any{"oneOf": []any{kind, map[string]any{"type": "array", "minItems": 1, "maxItems": doccorpus.MaxGapKinds, "uniqueItems": true, "items": kind}}}
		}
		if op == "flow" {
			// DCP-V1-170, DCP-V1-171: optional section and record-kind filters.
			name := map[string]any{"type": "string", "minLength": 1, "maxLength": 1024}
			properties["section"] = map[string]any{"oneOf": []any{name, map[string]any{"type": "array", "minItems": 1, "maxItems": doccorpus.MaxFlowSections, "uniqueItems": true, "items": name}}}
			properties["include"] = map[string]any{"type": "array", "minItems": 1, "maxItems": len(doccorpus.FlowRecordKinds), "uniqueItems": true, "items": map[string]any{"type": "string", "enum": doccorpus.FlowRecordKinds}}
		}
		if op == "search" {
			// DCP-V1-328: opt-in claim hits ranked with the subject hits.
			// DCP-V1-654: or the corpus capabilities, which a compact receipt omits.
			properties["include"] = map[string]any{"type": "array", "minItems": 1, "maxItems": len(SearchIncludes), "uniqueItems": true, "items": map[string]any{"type": "string", "enum": SearchIncludes}}
			// DCP-V1-653: the compact receipt by default, or the full one.
			properties["receipt"] = map[string]any{"type": "string", "enum": []string{CompactView, FullView}}
			// DCP-V1-331: optional subject kind filter, one kind or a list.
			kind := map[string]any{"type": "string", "minLength": 1, "maxLength": 128}
			properties["kind"] = map[string]any{"oneOf": []any{kind, map[string]any{"type": "array", "minItems": 1, "maxItems": doccorpus.MaxGapKinds, "uniqueItems": true, "items": kind}}}
		}
		if op == "trace" {
			// DCP-V1-098: downstream flow-relation depth for a flow subject.
			properties["depth"] = map[string]any{"type": "integer", "minimum": 1, "maximum": doccorpus.MaxFlowDepth}
		}
		description := "Read source-revalidated experimental documentation corpus evidence; generated authority and explicit limitations remain."
		if op == "search" {
			description += " " + SearchIncludeDescription + " " + SearchReceiptDescription
		}
		result = append(result, bridge.ToolDescriptor{Name: name, Description: description, Annotations: bridge.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: false, DestructiveHint: false}, InputSchema: map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

// ArtifactSHA256 is the configured artifact's digest, the corpus identity an
// outcome log is bound to (DCP-V1-285).
func (r *Registry) ArtifactSHA256() string { return r.digest }

func (r *Registry) Call(ctx context.Context, name string, arguments []byte) (map[string]any, string, *ToolFailure, *Error) {
	result, text, failure, protocol, _ := r.CallTimed(ctx, name, arguments)
	return result, text, failure, protocol
}

// CallTimed is Call plus its timing. A paths batch shares one verification and
// freshness overlay; each path's receipt is byte-identical to its single call.
func (r *Registry) CallTimed(ctx context.Context, name string, arguments []byte) (map[string]any, string, *ToolFailure, *Error, Timing) {
	started, cpu := time.Now(), cpuTime()
	timing := Timing{Load: "reused", Freshness: "reused"}
	finish := func() Timing {
		timing.TotalMS = time.Since(started).Milliseconds()
		timing.CPUMS = (cpuTime() - cpu).Milliseconds()
		return timing
	}
	op, ok := tools[name]
	if !ok || !r.header.has(op) {
		return nil, "", nil, &Error{"unsupported-tool"}, finish()
	}
	if ctx.Err() != nil {
		return nil, "", nil, &Error{"cancelled"}, finish()
	}
	var input struct {
		Retirement *doccorpus.RetirementPolicy `json:"retirement,omitempty"`
		Query      string                      `json:"query"`
		ID         string                      `json:"id"`
		Path       string                      `json:"path"`
		Paths      []string                    `json:"paths"`
		Limit      *int                        `json:"limit"`
		Offset     int                         `json:"offset"`
		Depth      int                         `json:"depth"`
		Section    jsontext.Value              `json:"section"`
		Include    []string                    `json:"include"`
		Receipt    string                      `json:"receipt"`
		Repository string                      `json:"repository"`
		Kind       jsontext.Value              `json:"kind"`
		Exclude    jsontext.Value              `json:"exclude_kind"`
		State      string                      `json:"state"`
		Qualifier  string                      `json:"qualifier"`
		Run        string                      `json:"run"`
		Caller     string                      `json:"caller_revision"`
	}
	if len(arguments) > 16<<10 || json.Unmarshal(arguments, &input, json.RejectUnknownMembers(true)) != nil {
		return nil, "", nil, &Error{"invalid-arguments"}, finish()
	}
	var members map[string]any
	if json.Unmarshal(arguments, &members) != nil || members == nil {
		return nil, "", nil, &Error{"invalid-arguments"}, finish()
	}
	kind := doccorpus.OperationInput(op)
	for key, value := range members {
		allowed := key == "retirement" || key == "caller_revision" || key == "offset" && r.header.Schema == doccorpus.SchemaV2 || key == "limit" || key == "query" && (kind == "query" || op == "claims" || op == "locate" || op == "navigation") || (key == "path" || key == "paths") && kind == "path" || key == "id" && (kind == "id" || op == "gaps") || key == "depth" && op == "trace" || (key == "section" || key == "include") && op == "flow" || (key == "include" || key == "receipt") && op == "search" || key == "repository" && (op == "locate" || op == "recommend-tests") || key == "kind" && (op == "gaps" || op == "search" || op == "locate" || op == "claims") || key == "exclude_kind" && op == "claims" || (key == "state" || key == "qualifier" || key == "run") && op == "claims"
		if !allowed || value == nil {
			return nil, "", nil, &Error{"invalid-arguments"}, finish()
		}
	}
	if input.Offset < 0 || input.Offset > doccorpus.MaxCorpusRecords*4 || input.Depth < 0 || input.Depth > doccorpus.MaxFlowDepth || members["depth"] != nil && input.Depth == 0 || members["repository"] != nil && input.Repository == "" {
		return nil, "", nil, &Error{"invalid-arguments"}, finish()
	}
	limit := 20
	if input.Limit != nil {
		limit = *input.Limit
		if limit < 1 || limit > doccorpus.MaxResults {
			return nil, "", nil, &Error{"invalid-arguments"}, finish()
		}
	}
	_, batch := members["paths"]
	// DCP-V1-313: docs_claims takes exactly one of path, paths or query;
	// DCP-V1-650: so does docs_locate, whose kind filter and repository
	// qualifier apply only to the query and the path forms respectively.
	// DCS-V1-597: docs_navigation takes exactly one of id or query.
	_, claimQuery := members["query"]
	claimQuery = claimQuery && (op == "claims" || op == "locate" || op == "navigation")
	_, hasPath := members["path"]
	if claimQuery && (input.Query == "" || hasPath || batch) {
		return nil, "", nil, &Error{"invalid-arguments"}, finish()
	}
	if _, hasKind := members["kind"]; op == "locate" && (hasKind && !claimQuery || claimQuery && members["repository"] != nil) {
		return nil, "", nil, &Error{"invalid-arguments"}, finish()
	}
	// DCP-V1-318: or a state, with an optional qualifier and run.
	_, claimStatus := members["state"]
	_, hasQualifier := members["qualifier"]
	_, hasRun := members["run"]
	if (hasQualifier || hasRun) && !claimStatus || claimStatus && (claimQuery || hasPath || batch || hasQualifier && input.Qualifier == "" || hasRun && input.Run == "" || !doccorpus.ValidStatusFilter(input.State, input.Qualifier, input.Run)) {
		return nil, "", nil, &Error{"invalid-arguments"}, finish()
	}
	// DCP-V1-319: docs_intent may omit its id, but a present id is non-empty.
	_, hasID := members["id"]
	if op == "navigation" && claimQuery && hasID || kind == "query" && input.Query == "" || kind == "path" && !claimQuery && !claimStatus && (input.Path == "") == !batch || kind == "id" && !claimQuery && input.ID == "" && (op != "intent" || hasID) {
		return nil, "", nil, &Error{"invalid-arguments"}, finish()
	}
	if kind != "query" && !claimQuery && input.Query != "" || kind != "path" && input.Path != "" || kind != "id" && op != "gaps" && input.ID != "" {
		return nil, "", nil, &Error{"invalid-arguments"}, finish()
	}
	if batch && !validBatch(input.Paths) {
		return nil, "", nil, &Error{"invalid-arguments"}, finish()
	}
	// DCP-V1-332: a present caller revision is a full lowercase Git object ID.
	if _, has := members["caller_revision"]; has && (input.Caller == "" || !doccorpus.ValidCallerRevision(input.Caller)) {
		return nil, "", nil, &Error{"invalid-arguments"}, finish()
	}
	identity, err := os.Lstat(r.root)
	if err != nil || !os.SameFile(identity, r.identity) {
		return nil, "", &ToolFailure{"corpus-input-unavailable", "repository root changed"}, nil, finish()
	}
	read := r.artifactReader()
	if !read.unchanged {
		if _, err := read.bytes(); err != nil {
			if errors.Is(err, errArtifactChanged) {
				return nil, "", &ToolFailure{"corpus-refused", "configured artifact changed; restart with the new artifact"}, nil, finish()
			}
			return failure(err, finish())
		}
	}
	// DCP-V1-653, DCP-V1-654: the receipt view and include ["capabilities"]
	// shape only the corvint-corpus-mcp wire receipt; the corpus query sees
	// include ["claims"] or no include.
	if op == "search" {
		if _, has := members["receipt"]; has && input.Receipt != CompactView && input.Receipt != FullView {
			return nil, "", nil, &Error{"invalid-arguments"}, finish()
		}
		if input.Include != nil {
			seen := map[string]bool{}
			for _, x := range input.Include {
				if seen[x] || !slices.Contains(SearchIncludes, x) {
					return nil, "", nil, &Error{"invalid-arguments"}, finish()
				}
				seen[x] = true
			}
			if len(input.Include) == 0 {
				return nil, "", nil, &Error{"invalid-arguments"}, finish()
			}
			input.Include = nil
			if seen["claims"] {
				input.Include = []string{"claims"}
			}
		}
	}
	sections, ok := flowSections(input.Section)
	if !ok || !doccorpus.ValidFlowFilter(sections, input.Include) {
		return nil, "", nil, &Error{"invalid-arguments"}, finish()
	}
	// DCP-V1-328: docs_search admits include only as exactly ["claims"].
	if op == "search" && input.Include != nil && !slices.Equal(input.Include, []string{"claims"}) {
		return nil, "", nil, &Error{"invalid-arguments"}, finish()
	}
	kinds, ok := flowSections(input.Kind)
	if !ok || !doccorpus.ValidGapKinds(kinds) || op == "locate" && slices.ContainsFunc(kinds, func(k string) bool { return !slices.Contains(doccorpus.LocateQueryKinds, k) }) {
		return nil, "", nil, &Error{"invalid-arguments"}, finish()
	}
	// DCS-V1-592: a claim kind filter and exclusion name distinct known kinds.
	excluded, ok := flowSections(input.Exclude)
	if !ok || op == "claims" && (!doccorpus.ValidClaimKinds(kinds) || !doccorpus.ValidClaimKinds(excluded) || slices.ContainsFunc(kinds, func(k string) bool { return slices.Contains(excluded, k) })) {
		return nil, "", nil, &Error{"invalid-arguments"}, finish()
	}
	request := doccorpus.Request{Operation: op, Retirement: input.Retirement, Query: input.Query, ID: input.ID, Path: input.Path, Limit: limit, Offset: input.Offset, Depth: input.Depth, Sections: sections, Include: input.Include, Repository: input.Repository, Kinds: kinds, ExcludeKinds: excluded, State: input.State, Qualifier: input.Qualifier, Run: input.Run, CallerRevision: input.Caller}
	// A batch whose deadline passes while the artifact or freshness is still
	// being prepared names every path unresolved (DCP-V1-145).
	prepared := func(err error) (map[string]any, string, *ToolFailure, *Error, Timing) {
		if batch && ctx.Err() != nil {
			data, _ := queryBatch(ctx, nil, request, doccorpus.FreshnessOverlay{}, input.Paths)
			var result map[string]any
			if json.Unmarshal(data, &result) == nil {
				return result, string(data), nil, nil, finish()
			}
		}
		return failure(err, finish())
	}
	a, stamp, err := r.artifactFor(ctx, read.bytes, &timing)
	if err != nil {
		return prepared(err)
	}
	// An indexed read answers from, or is refused by, the one index derived for
	// this artifact, before any freshness work (DCP-V1-195).
	queried := a
	// docs_validate rederives the artifact from its pinned inputs on every call,
	// as the CLI validate does; other tools answer from the verified read
	// (DCR-V1-413). The memo and its freshness stay keyed by the verified read.
	if op == "validate" {
		step := time.Now()
		raw, err := read.bytes()
		var rederived *doccorpus.Artifact
		if err == nil {
			rederived, err = doccorpus.OpenRead(ctx, r.root, raw, doccorpus.ReadOptions{Rederive: true})
		}
		timing.VerifyMS += time.Since(step).Milliseconds()
		if err != nil {
			return prepared(err)
		}
		queried = rederived
	}
	var indexMS int64
	if doccorpus.IndexedOperation(op) {
		step := time.Now()
		index, err := r.indexFor(ctx, a)
		indexMS = time.Since(step).Milliseconds()
		if err != nil {
			timing.QueryMS = indexMS
			return prepared(err)
		}
		indexed := *a
		indexed.RuntimeIndex = index
		queried = &indexed
	}
	step := time.Now()
	overlay, err := r.freshness(ctx, a, stamp, &timing)
	timing.FreshnessMS = time.Since(step).Milliseconds()
	if err != nil {
		return prepared(err)
	}
	step = time.Now()
	var data []byte
	if batch {
		data, err = queryBatch(ctx, queried, request, overlay, input.Paths)
	} else if rankProbe(ctx) {
		data, err = query(ctx, queried, request, overlay)
	} else {
		data, err = boundedQuery(ctx, queried, request, overlay, singleFrameBytes)
	}
	var result map[string]any
	if err == nil {
		err = json.Unmarshal(data, &result)
	}
	timing.QueryMS = indexMS + time.Since(step).Milliseconds()
	if err != nil {
		return failure(err, finish())
	}
	return result, string(data), nil, nil, finish()
}

// flowSections reads the flow section argument: absent, one name, or a list
// of names (DCP-V1-170).
func flowSections(raw jsontext.Value) ([]string, bool) {
	if len(raw) == 0 {
		return nil, true
	}
	var one string
	if json.Unmarshal(raw, &one) == nil {
		return []string{one}, true
	}
	var many []string
	if json.Unmarshal(raw, &many) == nil && many != nil {
		return many, true
	}
	return nil, false
}

func validBatch(paths []string) bool {
	if len(paths) == 0 || len(paths) > MaxBatchPaths {
		return false
	}
	seen := map[string]bool{}
	for _, path := range paths {
		if path == "" || len(path) > 1024 || seen[path] {
			return false
		}
		seen[path] = true
	}
	return true
}

func query(ctx context.Context, a *doccorpus.Artifact, request doccorpus.Request, overlay doccorpus.FreshnessOverlay) ([]byte, error) {
	receipt, err := doccorpus.QueryLive(ctx, a, request, overlay)
	if err != nil {
		return nil, err
	}
	return doccorpus.Encode(receipt)
}

// singleFrameBytes bounds what one single-call receipt costs in its MCP result
// frame; it shares the batch reserve for the envelope, timing and _meta.
var singleFrameBytes = MaxBatchBytes

// boundedQuery answers one single call within its MCP result frame
// (DCP-V1-200). A paged receipt over the bound is asked again with a smaller
// limit, so the caller gets the CLI's own receipt for that limit and continues
// at next_offset (DCP-V1-201); a receipt that cannot shrink is refused as
// output-bound-exceeded naming the bound (DCP-V1-202), never an internal error.
func boundedQuery(ctx context.Context, a *doccorpus.Artifact, request doccorpus.Request, overlay doccorpus.FreshnessOverlay, budget int) ([]byte, error) {
	for {
		receipt, err := doccorpus.QueryLive(ctx, a, request, overlay)
		if err != nil {
			return nil, err
		}
		data, err := doccorpus.Encode(receipt)
		if err != nil {
			return nil, err
		}
		cost := singleFrameCost(data)
		if cost <= budget {
			return data, nil
		}
		shown := len(receipt.Results)
		if a.Schema != doccorpus.SchemaV2 || shown <= 1 {
			return nil, &doccorpus.Error{Code: "output-bound-exceeded", Message: fmt.Sprintf("receipt of %d results needs %d of the %d-byte MCP result bound; ask a narrower query", shown, cost, budget)}
		}
		request.Limit = max(1, min(shown-1, shown*budget/cost))
	}
}

// singleFrameCost is what one receipt costs in its MCP result frame: its JSON
// bytes once as structured content, and once as the JSON string of its
// enveloped text.
func singleFrameCost(receipt []byte) int {
	text, _ := json.Marshal(repoenvelope.EscapeHidden(string(receipt)))
	return len(receipt) + len(text) + len(repoenvelope.Prefix) + len(repoenvelope.Suffix)
}

// queryBatch answers each path in order. A path whose answer is a typed
// refusal, or that is not reached before the deadline or output bound, is
// listed in unresolved with its reason instead of failing the whole batch.
func queryBatch(ctx context.Context, a *doccorpus.Artifact, request doccorpus.Request, overlay doccorpus.FreshnessOverlay, paths []string) ([]byte, error) {
	var out bytes.Buffer
	out.WriteString(`{"schema":"` + batchSchema + `","operation":`)
	writeJSON(&out, request.Operation)
	out.WriteString(`,"receipts":[`)
	unresolved := []map[string]string{}
	answered := 0
	budget := MaxBatchBytes
	for _, path := range paths {
		// An unresolved row carries the path JSON-escaped, and its text carries that
		// escaped form escaped again, so the reserve counts both encodings (V1-3198).
		escaped, _ := json.Marshal(path, json.Deterministic(true))
		text, _ := json.Marshal(repoenvelope.EscapeHidden(string(escaped)))
		budget -= len(escaped) + len(text) + 2*batchRowReserve
	}
	for i, path := range paths {
		if ctx.Err() != nil {
			for _, rest := range paths[i:] {
				unresolved = append(unresolved, map[string]string{"path": rest, "code": "deadline-exceeded", "message": "not answered before the call deadline"})
			}
			break
		}
		request.Path = path
		data, err := query(ctx, a, request, overlay)
		if err != nil {
			if ctx.Err() != nil {
				unresolved = append(unresolved, map[string]string{"path": path, "code": "deadline-exceeded", "message": "not answered before the call deadline"})
				continue
			}
			code, message := failureCode(err)
			unresolved = append(unresolved, map[string]string{"path": path, "code": code, "message": message})
			continue
		}
		if cost := frameCost(path, data); cost > budget {
			unresolved = append(unresolved, map[string]string{"path": path, "code": "output-bound-exceeded", "message": "batch output bound reached; ask for this path separately"})
			continue
		} else {
			budget -= cost
		}
		if answered > 0 {
			out.WriteByte(',')
		}
		answered++
		out.WriteString(`{"path":`)
		writeJSON(&out, path)
		out.WriteString(`,"receipt":`)
		out.Write(bytes.TrimSuffix(data, []byte("\n")))
		out.WriteByte('}')
	}
	out.WriteString(`],"unresolved":`)
	writeJSON(&out, unresolved)
	out.WriteString("}\n")
	return out.Bytes(), nil
}

// frameCost is what one receipt row adds to the MCP result frame: its JSON
// bytes once as structured content, and once as the JSON string of its
// envelope-escaped text. Both halves are measured on the row as queryBatch
// writes it, with the path JSON-escaped, not on len(path) (V1-3198). The ten
// bytes beyond the row cover its separating comma and keep the former slack.
func frameCost(path string, receipt []byte) int {
	var row bytes.Buffer
	row.WriteString(`{"path":`)
	writeJSON(&row, path)
	row.WriteString(`,"receipt":`)
	row.Write(receipt)
	row.WriteByte('}')
	text, _ := json.Marshal(repoenvelope.EscapeHidden(row.String()))
	return row.Len() + 10 + len(text)
}

func writeJSON(out *bytes.Buffer, value any) {
	data, _ := json.Marshal(value, json.Deterministic(true))
	out.Write(data)
}

// artifactFor returns the verified artifact. The first call runs the verified
// read (DCR-V1-410, DCR-V1-416); later calls reuse it while every pinned
// revision still resolves to a commit and tree. A failed check drops the memo
// and reads again, so a missing revision reports the native typed failure
// (DCP-V1-011, DCP-V1-143).
// A reused artifact also returns the checkout stamp captured before its
// checks, which freshness pairs with its observation; the first call returns
// none. The pins are not verified again while that stamp equals the one
// captured before their last successful verification (DCP-V1-316).
func (r *Registry) artifactFor(ctx context.Context, read func() ([]byte, error), timing *Timing) (*doccorpus.Artifact, string, error) {
	select {
	case r.load <- struct{}{}:
	case <-ctx.Done():
		return nil, "", ctx.Err()
	}
	defer func() { <-r.load }()
	// An evaluation suite answers every case from the artifact its session opened, and checks
	// the checkout once after the last case instead of before each (DCX-V1-474).
	if s := sessionOf(ctx); s != nil && r.artifact == s.artifact {
		return r.artifact, s.stamp, nil
	}
	if r.artifact != nil {
		step := time.Now()
		stamp := checkoutStamp(ctx, stampDirs(r.root, r.artifact.Manifest, r.pins))
		// A cancelled call stops here and releases the load slot (V1-3029).
		if ctx.Err() != nil {
			return nil, "", ctx.Err()
		}
		// The reuse key is the stamp and the exact pin set it verified: a pin
		// that resolves inside an already stamped checkout leaves the stamp
		// unchanged, so the key must name the pins too (DCP-V1-316, V1-2544).
		key := pinReuseKey(stamp, r.pins)
		var err error
		if key == "" || key != r.pinStamp {
			err = verifyPinned(ctx, r.root, r.artifact.Manifest, r.pins)
		}
		timing.VerifyMS = time.Since(step).Milliseconds()
		if err == nil {
			r.pinStamp = key
			return r.artifact, stamp, nil
		}
		if ctx.Err() != nil {
			return nil, "", ctx.Err()
		}
		r.artifact, r.pins, r.pinStamp = nil, nil, ""
		r.index.artifact, r.index.value, r.index.err = nil, nil, nil
	}
	step := time.Now()
	timing.Load = "first-call"
	raw, err := read()
	if err != nil {
		return nil, "", err
	}
	a, err := doccorpus.OpenRead(ctx, r.root, raw, doccorpus.ReadOptions{})
	timing.LoadMS = time.Since(step).Milliseconds()
	if err != nil {
		return nil, "", err
	}
	r.artifact, r.pins = a, pinnedRevisions(a.Manifest)
	return a, "", nil
}

// pinReuseKey is the pin verification reuse key: the checkout stamp and the
// ordered pin set it was captured for, or "" when the stamp is unavailable.
func pinReuseKey(stamp string, pins []string) string {
	if stamp == "" {
		return ""
	}
	return stamp + "\x00" + strings.Join(pins, "\x00")
}

// indexFor returns the query index of a, deriving it on the first indexed
// call for this artifact. The index, or a corpus refusal such as the capacity
// bound, is kept until the artifact memo is replaced; a cancellation is not.
func (r *Registry) indexFor(ctx context.Context, a *doccorpus.Artifact) (*doccorpus.QueryIndex, error) {
	select {
	case r.load <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-r.load }()
	if r.index.artifact == a {
		return r.index.value, r.index.err
	}
	index, err := buildIndex(ctx, a)
	var refused *doccorpus.Error
	if err == nil || errors.As(err, &refused) {
		r.index.artifact, r.index.value, r.index.err = a, index, err
	}
	return index, err
}

// freshness reuses the live overlay, and the symbol checks it memoizes, while
// the repository observation (commit, tree and worktree status) is unchanged:
// the same key the context snapshot store reuses an index under. The key is
// read before the build, so a change during the build only forces a rebuild.
// A stamp captured before the observation is kept with the memo: while a
// later call's stamp is equal, nothing the observation reads has changed, so
// the overlay is reused without running Git (DCP-V1-315). An unavailable
// stamp is empty and never matches.
func (r *Registry) freshness(ctx context.Context, a *doccorpus.Artifact, stamp string, timing *Timing) (doccorpus.FreshnessOverlay, error) {
	if s := sessionOf(ctx); s != nil && s.artifact == a {
		return s.overlay, nil
	}
	if stamp != "" {
		r.live.Lock()
		if r.live.artifact == a && r.live.stamp == stamp {
			overlay := r.live.overlay
			r.live.Unlock()
			return overlay, nil
		}
		r.live.Unlock()
	}
	// Every declared checkout is observed, so an additional repository's
	// change forces a rebuild and an unchanged set reuses (DCP-V1-280).
	key, err := observeCorpus(ctx, r.root, a)
	if err != nil {
		return doccorpus.FreshnessOverlay{}, err
	}
	// The memo is keyed by the state the overlay was built from, never by an
	// earlier observation, so a change during the build cannot be reused.
	r.live.Lock()
	if r.live.artifact == a && r.live.key == key {
		r.live.stamp = stamp
		overlay := r.live.overlay
		r.live.Unlock()
		return overlay, nil
	}
	r.live.Unlock()
	timing.Freshness = "computed"
	overlay, err := doccorpus.LiveFreshness(ctx, r.root, a)
	if err != nil {
		return overlay, err
	}
	r.live.Lock()
	r.live.key, r.live.artifact, r.live.overlay, r.live.stamp = overlay.Observation(), a, overlay, stamp
	r.live.Unlock()
	return overlay, nil
}

// readArtifactFile reads the configured artifact; tests replace it to count
// reads (DCR-V1-417).
var readArtifactFile = doccorpus.ReadCorpusFile

// errArtifactChanged is a configured artifact whose bytes no longer match the
// startup digest.
var errArtifactChanged = errors.New("configured artifact changed")

// artifactRead reads the configured artifact at most once per call, and not
// at all while its file keeps the stat stamp captured before the last read
// whose digest matched (DCR-V1-417).
type artifactRead struct {
	r         *Registry
	stamp     string
	unchanged bool
	raw       []byte
}

func (r *Registry) artifactReader() *artifactRead {
	stamp := artifactFileStamp(r.root, r.path)
	r.artifactFile.Lock()
	unchanged := stamp != "" && stamp == r.artifactFile.stamp
	r.artifactFile.Unlock()
	return &artifactRead{r: r, stamp: stamp, unchanged: unchanged}
}

// artifactFlight is one shared artifact read and digest check.
type artifactFlight struct {
	done chan struct{}
	raw  []byte
	err  error
}

// bytes reads the artifact and checks it against the startup digest. The
// stamp it records was captured before the read, so a change during the read
// leaves a stamp no later state matches. A call that finds a read already in
// flight waits for it and shares its result, so concurrent cold calls hold one
// artifact copy and the refusal of a changed artifact is unchanged (V1-3028).
func (a *artifactRead) bytes() ([]byte, error) {
	if a.raw != nil {
		return a.raw, nil
	}
	f := &a.r.artifactFile
	f.Lock()
	if flight := f.flight; flight != nil {
		f.Unlock()
		<-flight.done
		if flight.err != nil {
			return nil, flight.err
		}
		a.raw = flight.raw
		return a.raw, nil
	}
	flight := &artifactFlight{done: make(chan struct{})}
	f.flight = flight
	f.Unlock()
	flight.raw, flight.err = a.read()
	f.Lock()
	if flight.err == nil {
		f.stamp = a.stamp
	}
	f.flight = nil
	f.Unlock()
	close(flight.done)
	if flight.err != nil {
		return nil, flight.err
	}
	a.raw = flight.raw
	return a.raw, nil
}

func (a *artifactRead) read() ([]byte, error) {
	raw, err := readArtifactFile(a.r.root, a.r.path)
	if err != nil {
		return nil, err
	}
	if doccorpus.Digest(raw) != a.r.digest {
		return nil, errArtifactChanged
	}
	return raw, nil
}

func pinnedRevisions(m doccorpus.Manifest) []string {
	seen := map[string]bool{m.Repository.Revision: true}
	for _, s := range m.Scopes {
		seen[s.Revision] = true
	}
	for _, in := range m.Inputs {
		seen[in.Revision] = true
	}
	revisions := []string{}
	for revision := range seen {
		revisions = append(revisions, revision)
	}
	sort.Strings(revisions)
	return revisions
}

// verifyPins resolves each pinned revision in the checkout that owns it: the
// primary root or a declared additional repository (DCP-V1-178).
func verifyPins(ctx context.Context, root string, m doccorpus.Manifest, revisions []string) error {
	auths := map[string]*gitauth.Repository{}
	for _, revision := range revisions {
		dir := m.RevisionRoot(root, revision)
		auth, ok := auths[dir]
		if !ok {
			var err error
			if auth, err = gitauth.Open(dir, gitrun.NewDefaultBudget()); err != nil {
				return err
			}
			auths[dir] = auth
		}
		if _, err := auth.CommitTree(ctx, revision); err != nil {
			return err
		}
	}
	return nil
}

func failure(err error, timing Timing) (map[string]any, string, *ToolFailure, *Error, Timing) {
	code, message := failureCode(err)
	return nil, "", &ToolFailure{code, message}, nil, timing
}

func failureCode(err error) (string, string) {
	err = doccorpus.Typed(err) // the CLI types Git capacity failures the same way (DCP-V1-158)
	var e *doccorpus.Error
	if errors.As(err, &e) {
		return e.Code, e.Message
	}
	return "corpus-refused", "native corpus validation failed"
}
