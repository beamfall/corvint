// Package corpusbridge exposes only capabilities declared by a revalidated
// documentation corpus. It never invokes a provider, test or shell command.
package corpusbridge

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/contextindex"
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
	live               struct {
		sync.Mutex
		key      string
		artifact *doccorpus.Artifact
		overlay  doccorpus.FreshnessOverlay
	}
}

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
func (r *Registry) Tools() []bridge.ToolDescriptor {
	result := []bridge.ToolDescriptor{}
	for name, op := range tools {
		if !r.header.has(op) {
			continue
		}
		properties := map[string]any{"limit": map[string]any{"type": "integer", "minimum": 1, "maximum": doccorpus.MaxResults}}
		if r.header.Schema == doccorpus.SchemaV2 {
			properties["offset"] = map[string]any{"type": "integer", "minimum": 0, "maximum": doccorpus.MaxCorpusRecords * 4}
		}
		properties["retirement"] = doccorpus.RetirementInputSchema()
		required := []any{}
		if doccorpus.OperationInput(op) == "query" {
			properties["query"] = map[string]any{"type": "string", "minLength": 1, "maxLength": 1024}
			required = append(required, "query")
		}
		if doccorpus.OperationInput(op) == "id" {
			properties["id"] = map[string]any{"type": "string", "minLength": 1, "maxLength": 1024}
			required = append(required, "id")
		}
		if doccorpus.OperationInput(op) == "path" {
			// DCP-V1-145: path or a paths batch, exactly one.
			properties["path"] = map[string]any{"type": "string", "minLength": 1, "maxLength": 1024}
			properties["paths"] = map[string]any{"type": "array", "minItems": 1, "maxItems": MaxBatchPaths, "uniqueItems": true, "items": map[string]any{"type": "string", "minLength": 1, "maxLength": 1024}}
		}
		if op == "gaps" {
			properties["id"] = map[string]any{"type": "string", "maxLength": 1024}
		}
		if op == "flow" {
			// DCP-V1-170, DCP-V1-171: optional section and record-kind filters.
			name := map[string]any{"type": "string", "minLength": 1, "maxLength": 1024}
			properties["section"] = map[string]any{"oneOf": []any{name, map[string]any{"type": "array", "minItems": 1, "maxItems": doccorpus.MaxFlowSections, "uniqueItems": true, "items": name}}}
			properties["include"] = map[string]any{"type": "array", "minItems": 1, "maxItems": len(doccorpus.FlowRecordKinds), "uniqueItems": true, "items": map[string]any{"type": "string", "enum": doccorpus.FlowRecordKinds}}
		}
		if op == "trace" {
			// DCP-V1-098: downstream flow-relation depth for a flow subject.
			properties["depth"] = map[string]any{"type": "integer", "minimum": 1, "maximum": doccorpus.MaxFlowDepth}
		}
		result = append(result, bridge.ToolDescriptor{Name: name, Description: "Read source-revalidated experimental documentation corpus evidence; generated authority and explicit limitations remain.", Annotations: bridge.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: false, DestructiveHint: false}, InputSchema: map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}
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
		allowed := key == "retirement" || key == "offset" && r.header.Schema == doccorpus.SchemaV2 || key == "limit" || key == "query" && kind == "query" || (key == "path" || key == "paths") && kind == "path" || key == "id" && (kind == "id" || op == "gaps") || key == "depth" && op == "trace" || (key == "section" || key == "include") && op == "flow"
		if !allowed || value == nil {
			return nil, "", nil, &Error{"invalid-arguments"}, finish()
		}
	}
	if input.Offset < 0 || input.Offset > doccorpus.MaxCorpusRecords*4 || input.Depth < 0 || input.Depth > doccorpus.MaxFlowDepth || members["depth"] != nil && input.Depth == 0 {
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
	if kind == "query" && input.Query == "" || kind == "path" && (input.Path == "") == !batch || kind == "id" && input.ID == "" {
		return nil, "", nil, &Error{"invalid-arguments"}, finish()
	}
	if kind != "query" && input.Query != "" || kind != "path" && input.Path != "" || kind != "id" && op != "gaps" && input.ID != "" {
		return nil, "", nil, &Error{"invalid-arguments"}, finish()
	}
	if batch && !validBatch(input.Paths) {
		return nil, "", nil, &Error{"invalid-arguments"}, finish()
	}
	identity, err := os.Lstat(r.root)
	if err != nil || !os.SameFile(identity, r.identity) {
		return nil, "", &ToolFailure{"corpus-input-unavailable", "repository root changed"}, nil, finish()
	}
	raw, err := doccorpus.ReadCorpusFile(r.root, r.path)
	if err != nil {
		return failure(err, finish())
	}
	if doccorpus.Digest(raw) != r.digest {
		return nil, "", &ToolFailure{"corpus-refused", "configured artifact changed; restart with the new artifact"}, nil, finish()
	}
	sections, ok := flowSections(input.Section)
	if !ok || !doccorpus.ValidFlowFilter(sections, input.Include) {
		return nil, "", nil, &Error{"invalid-arguments"}, finish()
	}
	request := doccorpus.Request{Operation: op, Retirement: input.Retirement, Query: input.Query, ID: input.ID, Path: input.Path, Limit: limit, Offset: input.Offset, Depth: input.Depth, Sections: sections, Include: input.Include}
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
	a, err := r.artifactFor(ctx, raw, &timing)
	if err != nil {
		return prepared(err)
	}
	step := time.Now()
	overlay, err := r.freshness(ctx, a, &timing)
	timing.FreshnessMS = time.Since(step).Milliseconds()
	if err != nil {
		return prepared(err)
	}
	step = time.Now()
	var data []byte
	if batch {
		data, err = queryBatch(ctx, a, request, overlay, input.Paths)
	} else {
		data, err = query(ctx, a, request, overlay)
	}
	var result map[string]any
	if err == nil {
		err = json.Unmarshal(data, &result)
	}
	timing.QueryMS = time.Since(step).Milliseconds()
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
		budget -= 2 * (len(path) + batchRowReserve)
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
// envelope-escaped text.
func frameCost(path string, receipt []byte) int {
	row := len(path) + len(receipt) + 32
	text, _ := json.Marshal(repoenvelope.EscapeHidden(`{"path":` + strconv.Quote(path) + `,"receipt":` + string(receipt) + `}`))
	return row + len(text)
}

func writeJSON(out *bytes.Buffer, value any) {
	data, _ := json.Marshal(value, json.Deterministic(true))
	out.Write(data)
}

// artifactFor returns the rederived artifact. The first call runs the full
// rederivation; later calls reuse it while every pinned revision still
// resolves to a commit and tree. A failed check drops the memo and rederives,
// so a missing input reports the native typed failure (DCP-V1-011, DCP-V1-143).
func (r *Registry) artifactFor(ctx context.Context, raw []byte, timing *Timing) (*doccorpus.Artifact, error) {
	select {
	case r.load <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-r.load }()
	if r.artifact != nil {
		step := time.Now()
		err := verifyPins(ctx, r.root, r.pins)
		timing.VerifyMS = time.Since(step).Milliseconds()
		if err == nil {
			return r.artifact, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		r.artifact, r.pins = nil, nil
	}
	step := time.Now()
	timing.Load = "first-call"
	a, err := doccorpus.Open(ctx, r.root, raw)
	timing.LoadMS = time.Since(step).Milliseconds()
	if err != nil {
		return nil, err
	}
	r.artifact, r.pins = a, pinnedRevisions(a.Manifest)
	return a, nil
}

// freshness reuses the live overlay, and the symbol checks it memoizes, while
// the repository observation (commit, tree and worktree status) is unchanged:
// the same key the context snapshot store reuses an index under. The key is
// read before the build, so a change during the build only forces a rebuild.
func (r *Registry) freshness(ctx context.Context, a *doccorpus.Artifact, timing *Timing) (doccorpus.FreshnessOverlay, error) {
	observed, err := contextindex.Observe(ctx, r.root)
	if err != nil {
		return doccorpus.FreshnessOverlay{}, err
	}
	// The memo is keyed by the state the overlay was built from, never by an
	// earlier observation, so a change during the build cannot be reused.
	key := doccorpus.ObservationKey(observed)
	r.live.Lock()
	if r.live.artifact == a && r.live.key == key {
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
	r.live.key, r.live.artifact, r.live.overlay = overlay.Observation(), a, overlay
	r.live.Unlock()
	return overlay, nil
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

func verifyPins(ctx context.Context, root string, revisions []string) error {
	auth, err := gitauth.Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		return err
	}
	for _, revision := range revisions {
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
	var e *doccorpus.Error
	if errors.As(err, &e) {
		return e.Code, e.Message
	}
	return "corpus-refused", "native corpus validation failed"
}
