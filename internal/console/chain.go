package console

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// The chain pane reads three artifacts the CLI already produces and adds
// none of its own (LAC-V0-033, decision 0362): the sealed change map committed
// by script/dogfood-seal.sh, the untracked OCM maps `make dogfood-change`
// leaves under .corvint/, and the local trace `corvint dogfood-record` appends.
const (
	changesDir     = ".corvint/changes"
	boundMapPath   = ".corvint/change.cem.json"
	ocmDir         = ".corvint"
	ocmProfile     = "ocm/0.1-experimental"
	traceDir       = ".context-corvint/traces"
	maxOCMBytes    = 8 << 20
	maxTraceBytes  = 16 << 20
	maxChainOCMs   = 64
	maxDetailLines = 400
)

// The gap classes of LAC-V0-035. A gap is an edge the artifacts do not
// establish; it is rendered as itself, never replaced by an inferred link.
const (
	GapMissing     = "missing"
	GapStale       = "stale"
	GapAmbiguous   = "ambiguous"
	GapUnverified  = "unverified"
	GapUnsupported = "unsupported"
)

var (
	objectIDPattern = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
	ocmFileName     = regexp.MustCompile(`^change\.ocm(?:\.[0-9]{3})?\.json$`)
	// requirementLine is the frozen OCM requirement-line syntax
	// (docs/specs/ocm-v0-dogfood.md, Wire profile). It is used only to show
	// the clause an obligation already names, never to find a requirement.
	requirementLine = regexp.MustCompile("^- (?:`([A-Z][A-Z0-9-]{2,31}-[0-9]{3})`:[ ]|\\*\\*([A-Z][A-Z0-9-]{2,31}-[0-9]{3})[.:]\\*\\*[ ])")
	cemProfiles     = map[string]bool{"cem/0.1": true, "cem/0.2": true, "cem/0.3": true}
)

type wireSpan struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type wireRange struct {
	Start int `json:"start"`
	Count int `json:"count"`
}

type wireSpanRef struct {
	ID         string   `json:"id"`
	Path       string   `json:"path"`
	BlobOid    string   `json:"blobOid"`
	Selector   string   `json:"selector"`
	Span       wireSpan `json:"span"`
	SpanSha256 string   `json:"spanSha256"`
}

type wireBasis struct {
	EvidenceID string `json:"evidenceId"`
	Relation   string `json:"relation"`
}

type wireHunk struct {
	ID          string      `json:"id"`
	Path        string      `json:"path"`
	Disposition string      `json:"disposition"`
	Reason      string      `json:"reason"`
	OldRange    wireRange   `json:"oldRange"`
	NewRange    wireRange   `json:"newRange"`
	Basis       []wireBasis `json:"basis"`
}

type wireCEM struct {
	Spec         string        `json:"spec"`
	BaseRevision string        `json:"baseRevision"`
	Evidence     []wireSpanRef `json:"evidence"`
	Hunks        []wireHunk    `json:"hunks"`
}

type wireObligation struct {
	ID          string   `json:"id"`
	Disposition string   `json:"disposition"`
	Reason      string   `json:"reason"`
	HunkIDs     []string `json:"hunkIds"`
	ClaimIDs    []string `json:"claimIds"`
}

type wireOCM struct {
	Spec           string      `json:"spec"`
	TargetRevision string      `json:"targetRevision"`
	IntentScope    wireSpanRef `json:"intentScope"`
	CEM            struct {
		MapSha256 string `json:"mapSha256"`
	} `json:"cem"`
	Claims      []wireSpanRef    `json:"claims"`
	Obligations []wireObligation `json:"obligations"`
}

type wireTrace struct {
	SchemaVersion int      `json:"schema_version"`
	Revision      string   `json:"revision"`
	TraceID       string   `json:"trace_id"`
	Task          string   `json:"task"`
	Outcome       string   `json:"outcome"`
	Verification  []string `json:"verification"`
}

// ChainEdge is one edge of the chain. Artifact and Field name what justifies
// it (LAC-V0-034); a non-empty Gap means the artifacts do not establish it and
// Reason says why (LAC-V0-035).
type ChainEdge struct {
	Gap      string
	Reason   string
	Target   string
	Artifact string
	Field    string
	Pin      string
	Detail   string
	Anchor   string
	Axes     Axes
}

// ChainHunk is one hunk of the sealed map and its outgoing edges.
type ChainHunk struct {
	Field        string
	ID           string
	Path         string
	Disposition  string
	Reason       string
	Old          wireRange
	New          wireRange
	Evidence     []ChainEdge
	Requirements []ChainEdge
}

// ChainRequirement is one obligation of a bound OCM map: the requirement at
// its pinned intent scope, the hunks and test claims the map lists for it,
// and the recorded verification result of the change revision.
type ChainRequirement struct {
	ID           string
	Anchor       string
	Disposition  string
	Reason       string
	Clause       []ChainEdge
	Hunks        []ChainEdge
	Claims       []ChainEdge
	Verification []ChainEdge
}

// ChainArtifact is one untracked local artifact the pane read, and whether it
// is bound to this change.
type ChainArtifact struct {
	Path   string
	State  string
	Reason string
	Source Source
}

// ChainSpan is one cited span's bytes, read at the object id it pins.
type ChainSpan struct {
	Edge ChainEdge
	Text string
}

// ChainDetail is one hunk's lines at the revision its map pins, and the bytes
// of every span it cites.
type ChainDetail struct {
	Hunk  ChainHunk
	Lines *Blob
	Range string
	Text  string
	Spans []ChainSpan
	Err   string
}

// Chain is the rendered chain for one sealed change.
type Chain struct {
	Change       string
	Commit       string
	Base         string
	Profile      string
	SealedPath   string
	Sealed       *Blob
	Binding      []ChainEdge
	Artifacts    []ChainArtifact
	Verification []ChainEdge
	Requirements []ChainRequirement
	Hunks        []ChainHunk
	Detail       *ChainDetail
	Err          string
}

// boundOCM is one OCM map bound to the sealed map by digest and revision.
type boundOCM struct {
	path string
	doc  wireOCM
	axes Axes
}

// objects reads Git blobs by object id once per request. It is not a cache
// across requests: it dies with the request (LAC-V0-004).
type objects struct {
	worktree Worktree
	ctx      context.Context
	read     map[string]objectRead
}

type objectRead struct {
	bytes string
	err   error
}

func (o *objects) blob(id string) (string, error) {
	if !objectIDPattern.MatchString(id) {
		return "", errors.New("the object id " + strconv.Quote(id) + " is not a full lowercase hex object id")
	}
	if cached, ok := o.read[id]; ok {
		return cached.bytes, cached.err
	}
	text, err := o.worktree.git(o.ctx, "cat-file", "blob", id)
	o.read[id] = objectRead{text, err}
	return text, err
}

// spanOf checks a pinned span against the bytes at its object id. It returns
// the span's bytes, or the gap class and reason when the pin does not hold.
func (o *objects) spanOf(ref wireSpanRef) (string, string, string) {
	text, err := o.blob(ref.BlobOid)
	if err != nil {
		return "", GapMissing, "object " + ref.BlobOid + " is not readable: " + err.Error()
	}
	if ref.Span.Start < 0 || ref.Span.End < ref.Span.Start || ref.Span.End > len(text) {
		return "", GapStale, "span " + spanText(ref.Span) + " lies outside the " + strconv.Itoa(len(text)) + " bytes of object " + ref.BlobOid
	}
	span := text[ref.Span.Start:ref.Span.End]
	if digest := sha256Hex(span); digest != ref.SpanSha256 {
		return "", GapStale, "spanSha256 " + ref.SpanSha256 + " does not match the bytes at object " + ref.BlobOid + " (" + digest + ")"
	}
	return span, "", ""
}

func sha256Hex(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func spanText(span wireSpan) string {
	return strconv.Itoa(span.Start) + ".." + strconv.Itoa(span.End)
}

// ReadChain compiles the chain for one sealed change, read at commit. change
// must already be a sealed map the listing at commit named.
func (w Worktree) ReadChain(ctx context.Context, commit, change string) *Chain {
	chain := &Chain{Change: change, Commit: commit, SealedPath: changesDir + "/" + change + ".cem.json"}
	chain.Sealed = w.Read(ctx, commit, chain.SealedPath)
	document, err := decodeSealed(chain.Sealed)
	if err != nil {
		chain.Err = err.Error()
		return chain
	}
	chain.Base, chain.Profile = document.BaseRevision, document.Spec
	reader := &objects{worktree: w, ctx: ctx, read: map[string]objectRead{}}
	chain.Binding = w.changeBinding(ctx, chain)
	chain.Hunks = hunkEdges(chain, document, reader)
	bound, artifacts := readOCMs(w.Root, change, sha256Hex(chain.Sealed.Text))
	chain.Artifacts = artifacts
	trace, verification := readTrace(w.Root, change)
	chain.Artifacts = append(chain.Artifacts, trace)
	chain.Verification = verification
	chain.Requirements = requirementRows(chain, bound, document, reader)
	linkHunksToRequirements(chain, bound)
	return chain
}

// decodeSealed admits a sealed map the console can read. It is not the CEM
// verifier: `corvint cem verify` owns validity; this only refuses what the
// pane cannot render honestly.
func decodeSealed(sealed *Blob) (wireCEM, error) {
	var document wireCEM
	if sealed.Err != "" {
		return document, errors.New("the sealed map is not readable: " + sealed.Err)
	}
	if sealed.Truncated {
		return document, errors.New("the sealed map is larger than the console reads, so it is PARTIAL and no chain is drawn from it")
	}
	if err := json.Unmarshal([]byte(sealed.Text), &document); err != nil {
		return document, errors.New("the sealed map is not decodable JSON: " + err.Error())
	}
	if !cemProfiles[document.Spec] {
		return document, errors.New(GapUnsupported + ": the sealed map states profile " + strconv.Quote(document.Spec) + ", which this pane does not read")
	}
	return document, nil
}

// changeBinding confirms the sealed file's name: the commit it names must hold
// the same map object at the bound path the seal moved it from.
func (w Worktree) changeBinding(ctx context.Context, chain *Chain) []ChainEdge {
	edge := ChainEdge{Target: chain.Change, Artifact: chain.SealedPath,
		Field: "file name (script/dogfood-seal.sh names the sealed map by its bind commit)", Axes: chain.Sealed.Source.Axes}
	id, err := w.git(ctx, "rev-parse", "--verify", "--quiet", chain.Change+":"+boundMapPath)
	id = strings.TrimSpace(id)
	switch {
	case err != nil || id == "":
		edge.Gap, edge.Reason = GapMissing, "commit "+chain.Change+" holds no "+boundMapPath+", so the sealed file's name is not confirmed as its bind commit"
	case id != chain.Sealed.ObjectID:
		edge.Gap, edge.Reason = GapStale, chain.Change+":"+boundMapPath+" is object "+id+", not the sealed object "+chain.Sealed.ObjectID
	default:
		edge.Pin, edge.Detail = id, chain.Change+":"+boundMapPath+" and "+chain.SealedPath+" name the same object"
	}
	return []ChainEdge{edge}
}

// hunkEdges draws each hunk's evidence edges from its own basis fields.
func hunkEdges(chain *Chain, document wireCEM, reader *objects) []ChainHunk {
	byID := map[string][]int{}
	for index, evidence := range document.Evidence {
		byID[evidence.ID] = append(byID[evidence.ID], index)
	}
	hunks := make([]ChainHunk, 0, len(document.Hunks))
	for index, hunk := range document.Hunks {
		row := ChainHunk{Field: "hunks[" + strconv.Itoa(index) + "]", ID: hunk.ID, Path: hunk.Path,
			Disposition: hunk.Disposition, Reason: hunk.Reason, Old: hunk.OldRange, New: hunk.NewRange}
		row.Evidence = basisEdges(chain, row.Field, hunk, document.Evidence, byID, reader)
		hunks = append(hunks, row)
	}
	return hunks
}

func basisEdges(chain *Chain, field string, hunk wireHunk, evidence []wireSpanRef, byID map[string][]int, reader *objects) []ChainEdge {
	axes := chain.Sealed.Source.Axes
	if hunk.Disposition != "supported" || len(hunk.Basis) == 0 {
		return []ChainEdge{{Gap: GapUnsupported, Artifact: chain.SealedPath, Field: field + ".disposition", Axes: axes,
			Reason: "the map states disposition " + strconv.Quote(hunk.Disposition) + ", reason " + strconv.Quote(hunk.Reason) + ", and cites no evidence for this hunk"}}
	}
	edges := make([]ChainEdge, 0, len(hunk.Basis))
	for index, basis := range hunk.Basis {
		edge := ChainEdge{Target: basis.EvidenceID, Artifact: chain.SealedPath, Axes: axes,
			Field: field + ".basis[" + strconv.Itoa(index) + "].evidenceId"}
		edges = append(edges, evidenceEdge(edge, basis, evidence, byID[basis.EvidenceID], reader))
	}
	return edges
}

func evidenceEdge(edge ChainEdge, basis wireBasis, evidence []wireSpanRef, matches []int, reader *objects) ChainEdge {
	switch len(matches) {
	case 0:
		edge.Gap, edge.Reason = GapMissing, "no evidence[] entry has this id"
		return edge
	case 1:
	default:
		edge.Gap, edge.Reason = GapAmbiguous, strconv.Itoa(len(matches))+" evidence[] entries share this id; none is chosen"
		return edge
	}
	cited := evidence[matches[0]]
	edge.Field += " = evidence[" + strconv.Itoa(matches[0]) + "].id"
	edge.Pin = cited.Path + " @ object " + cited.BlobOid + " bytes " + spanText(cited.Span)
	edge.Detail = "relation " + basis.Relation
	if _, gap, reason := reader.spanOf(cited); gap != "" {
		edge.Gap, edge.Reason = gap, reason
	}
	return edge
}

// readOCMs reads every OCM map under .corvint/ and keeps those bound to the
// sealed map by both its digest and the change revision. A map naming the
// change with another digest is stale; a map naming another change is not
// this change's and is listed as unrelated.
func readOCMs(root, change, mapSha256 string) ([]boundOCM, []ChainArtifact) {
	names, err := ocmNames(root)
	if err != nil {
		return nil, []ChainArtifact{{Path: ocmDir, State: GapMissing, Reason: err.Error(), Source: localSource(root, ocmDir)}}
	}
	var bound []boundOCM
	var artifacts []ChainArtifact
	for _, name := range names {
		path := ocmDir + "/" + name
		artifact := ChainArtifact{Path: path, Source: localSource(root, path)}
		document, err := readOCM(root, path)
		artifact.State, artifact.Reason = ocmBinding(document, err, change, mapSha256)
		if artifact.State == "bound" {
			bound = append(bound, boundOCM{path: path, doc: document, axes: artifact.Source.Axes})
		}
		artifacts = append(artifacts, artifact)
	}
	if len(names) == 0 {
		artifacts = append(artifacts, ChainArtifact{Path: ocmDir + "/change.ocm*.json", State: GapMissing,
			Reason: "no OCM map exists in this worktree", Source: localSource(root, ocmDir)})
	}
	return bound, artifacts
}

func ocmNames(root string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(ocmDir)))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if ocmFileName.MatchString(entry.Name()) && len(names) < maxChainOCMs {
			names = append(names, entry.Name())
		}
	}
	return names, nil
}

func readOCM(root, path string) (wireOCM, error) {
	var document wireOCM
	raw, err := readBounded(root, filepath.FromSlash(path), maxOCMBytes)
	if err != nil {
		return document, err
	}
	if err := json.Unmarshal(raw, &document); err != nil {
		return document, errors.New("not decodable JSON: " + err.Error())
	}
	return document, nil
}

func ocmBinding(document wireOCM, err error, change, mapSha256 string) (string, string) {
	switch {
	case err != nil:
		return GapMissing, "not readable: " + err.Error()
	case document.Spec != ocmProfile:
		return GapUnsupported, "states profile " + strconv.Quote(document.Spec) + ", which this pane does not read"
	case document.TargetRevision == change && document.CEM.MapSha256 == mapSha256:
		return "bound", "targetRevision names this change and cem.mapSha256 is the SHA-256 of the sealed map"
	case document.TargetRevision == change:
		return GapStale, "targetRevision names this change but cem.mapSha256 " + document.CEM.MapSha256 + " is not the SHA-256 of the sealed map (" + mapSha256 + ")"
	case document.CEM.MapSha256 == mapSha256:
		return GapStale, "cem.mapSha256 names the sealed map but targetRevision names " + document.TargetRevision
	}
	return "unrelated", "targetRevision " + document.TargetRevision + " and cem.mapSha256 name another change"
}

// localSource attributes an untracked local artifact. It carries no content
// digest and no owning verifier, so it states none of the six axes
// (LAC-V0-007), exactly like the dogfood report.
func localSource(root, path string) Source {
	return Source{Argv: []string{"read", filepath.Join(root, filepath.FromSlash(path))},
		Worktree: root, ObservedAt: time.Now().UTC(), Axes: UnstatedAxes()}
}

// readTrace reads the recorded verification results for the change revision.
// Only a row whose revision field names the change is an edge; the result is
// the outcome recorded for the whole revision, not for one requirement.
func readTrace(root, change string) (ChainArtifact, []ChainEdge) {
	path := traceDir + "/" + change + ".jsonl"
	artifact := ChainArtifact{Path: path, Source: localSource(root, path)}
	raw, err := readBounded(root, filepath.FromSlash(path), maxTraceBytes)
	if errors.Is(err, fs.ErrNotExist) {
		artifact.State, artifact.Reason = GapMissing, "no trace file names this revision"
		return artifact, []ChainEdge{{Gap: GapUnverified, Artifact: path, Field: "file", Axes: artifact.Source.Axes,
			Reason: "no recorded verification result names revision " + change}}
	}
	if err != nil {
		artifact.State, artifact.Reason = GapMissing, "not readable: "+err.Error()
		return artifact, []ChainEdge{{Gap: GapUnverified, Artifact: path, Field: "file", Axes: artifact.Source.Axes,
			Reason: "the trace file is not readable, so no recorded verification result is shown: " + err.Error()}}
	}
	artifact.State, artifact.Reason = "read", "rows are matched by their revision field only"
	return artifact, traceEdges(path, change, string(raw), artifact.Source.Axes)
}

func traceEdges(path, change, raw string, axes Axes) []ChainEdge {
	var edges []ChainEdge
	recorded := 0
	for index, line := range strings.Split(strings.TrimRight(raw, "\n"), "\n") {
		edge := traceEdge(path, change, index+1, line, axes)
		if edge.Gap == "" {
			recorded++
		}
		edges = append(edges, edge)
	}
	if recorded == 0 {
		edges = append(edges, ChainEdge{Gap: GapUnverified, Artifact: path, Field: "revision", Axes: axes,
			Reason: "no row of this file records a verification result for revision " + change})
	}
	if recorded > 1 {
		edges = append(edges, ChainEdge{Gap: GapAmbiguous, Artifact: path, Field: "revision", Axes: axes,
			Reason: strconv.Itoa(recorded) + " rows record a result for this revision; every one is shown and none is chosen"})
	}
	return edges
}

func traceEdge(path, change string, number int, line string, axes Axes) ChainEdge {
	edge := ChainEdge{Artifact: path, Field: "line " + strconv.Itoa(number) + " revision", Axes: axes}
	var row wireTrace
	if err := json.Unmarshal([]byte(line), &row); err != nil || row.SchemaVersion != 1 {
		edge.Gap, edge.Reason = GapUnsupported, "the row is not a schema_version 1 trace record this pane reads"
		return edge
	}
	if row.Revision != change {
		edge.Gap, edge.Reason = GapStale, "the row names revision "+row.Revision+", not "+change
		return edge
	}
	edge.Target, edge.Pin = row.TraceID, row.Revision
	edge.Detail = "recorded outcome " + strconv.Quote(row.Outcome) + " for task " + strconv.Quote(row.Task) +
		"; recorded commands: " + strings.Join(row.Verification, " · ")
	return edge
}

// requirementRows lists every obligation of every bound map with its pinned
// clause, its hunk and claim references, and the change's verification edges.
func requirementRows(chain *Chain, bound []boundOCM, document wireCEM, reader *objects) []ChainRequirement {
	counts := obligationCounts(bound)
	hunkIDs := map[string]bool{}
	for _, hunk := range document.Hunks {
		hunkIDs[hunk.ID] = true
	}
	var rows []ChainRequirement
	for _, ocm := range bound {
		for index, obligation := range ocm.doc.Obligations {
			field := "obligations[" + strconv.Itoa(index) + "]"
			row := ChainRequirement{ID: obligation.ID, Anchor: "req-" + obligation.ID,
				Disposition: obligation.Disposition, Reason: obligation.Reason}
			row.Clause = []ChainEdge{clauseEdge(ocm, field, obligation, counts[obligation.ID], reader)}
			if counts[obligation.ID] > 1 {
				row.Anchor = ""
			}
			row.Hunks = obligationHunks(ocm, field, obligation, counts[obligation.ID], hunkIDs, chain)
			row.Claims = claimEdges(ocm, field, obligation)
			row.Verification = chain.Verification
			rows = append(rows, row)
		}
	}
	return rows
}

// obligationCounts counts the bound maps' obligations under each id.
func obligationCounts(bound []boundOCM) map[string]int {
	counts := map[string]int{}
	for _, ocm := range bound {
		for _, obligation := range ocm.doc.Obligations {
			counts[obligation.ID]++
		}
	}
	return counts
}

// obligationHunkGap is the state the requirement and hunk panels both give an
// obligation's edge to a hunk the sealed map lists (V1-0152): a shared id is
// ambiguous, and a disposition other than linked is unsupported.
func obligationHunkGap(obligation wireObligation, count int) (string, string) {
	if count > 1 {
		return GapAmbiguous, strconv.Itoa(count) + " obligations of the bound maps share this id; none is chosen"
	}
	if obligation.Disposition != "linked" {
		return GapUnsupported, "the obligation lists this hunk but states disposition " + strconv.Quote(obligation.Disposition)
	}
	return "", ""
}

func clauseEdge(ocm boundOCM, field string, obligation wireObligation, count int, reader *objects) ChainEdge {
	scope := ocm.doc.IntentScope
	edge := ChainEdge{Target: obligation.ID, Artifact: ocm.path, Field: field + ".id in intentScope", Axes: ocm.axes,
		Pin: scope.Path + " @ object " + scope.BlobOid + " bytes " + spanText(scope.Span)}
	if count > 1 {
		edge.Gap, edge.Reason = GapAmbiguous, strconv.Itoa(count)+" obligations of the bound maps share this id; none is chosen"
		return edge
	}
	span, gap, reason := reader.spanOf(scope)
	if gap != "" {
		edge.Gap, edge.Reason = gap, "the pinned intent scope does not hold: "+reason
		return edge
	}
	clause := clauseLine(span, obligation.ID)
	if clause == "" {
		edge.Gap, edge.Reason = GapStale, "the pinned intent scope defines no requirement line for "+obligation.ID
		return edge
	}
	edge.Detail = clause
	return edge
}

// clauseLine returns the requirement line of id inside a verified intent span.
func clauseLine(span, id string) string {
	for _, line := range strings.Split(span, "\n") {
		match := requirementLine.FindStringSubmatch(line)
		if match != nil && (match[1] == id || match[2] == id) {
			return line
		}
	}
	return ""
}

func obligationHunks(ocm boundOCM, field string, obligation wireObligation, count int, hunkIDs map[string]bool, chain *Chain) []ChainEdge {
	var edges []ChainEdge
	for index, id := range obligation.HunkIDs {
		edge := ChainEdge{Target: id, Artifact: ocm.path, Field: field + ".hunkIds[" + strconv.Itoa(index) + "]",
			Axes: Weakest(ocm.axes, chain.Sealed.Source.Axes)}
		edge.Gap, edge.Reason = obligationHunkGap(obligation, count)
		if !hunkIDs[id] {
			edge.Gap, edge.Reason = GapMissing, "the sealed map lists no hunk with this id"
		}
		edges = append(edges, edge)
	}
	if len(edges) == 0 {
		edges = append(edges, ChainEdge{Gap: GapUnsupported, Artifact: ocm.path, Field: field + ".hunkIds", Axes: ocm.axes,
			Reason: "the obligation lists no hunk (disposition " + strconv.Quote(obligation.Disposition) + ", reason " + strconv.Quote(obligation.Reason) + ")"})
	}
	return edges
}

func claimEdges(ocm boundOCM, field string, obligation wireObligation) []ChainEdge {
	byID := map[string][]int{}
	for index, claim := range ocm.doc.Claims {
		byID[claim.ID] = append(byID[claim.ID], index)
	}
	var edges []ChainEdge
	for index, id := range obligation.ClaimIDs {
		edge := ChainEdge{Target: id, Artifact: ocm.path, Field: field + ".claimIds[" + strconv.Itoa(index) + "]", Axes: ocm.axes}
		edges = append(edges, claimEdge(edge, ocm.doc.Claims, byID[id]))
	}
	if len(edges) == 0 {
		edges = append(edges, ChainEdge{Gap: GapUnsupported, Artifact: ocm.path, Field: field + ".claimIds", Axes: ocm.axes,
			Reason: "the obligation lists no test claim"})
	}
	return edges
}

func claimEdge(edge ChainEdge, claims []wireSpanRef, matches []int) ChainEdge {
	switch len(matches) {
	case 0:
		edge.Gap, edge.Reason = GapMissing, "no claims[] entry has this id"
		return edge
	case 1:
	default:
		edge.Gap, edge.Reason = GapAmbiguous, strconv.Itoa(len(matches))+" claims[] entries share this id; none is chosen"
		return edge
	}
	claim := claims[matches[0]]
	edge.Field += " = claims[" + strconv.Itoa(matches[0]) + "].id"
	edge.Pin = claim.Path + " @ object " + claim.BlobOid + " bytes " + spanText(claim.Span)
	edge.Detail = "structural test claim " + strconv.Quote(claim.Selector) + " (a witness that names the requirement, not a result)"
	return edge
}

// linkHunksToRequirements draws each hunk's requirement edges from the bound
// maps' hunkIds fields alone.
func linkHunksToRequirements(chain *Chain, bound []boundOCM) {
	for index := range chain.Hunks {
		chain.Hunks[index].Requirements = hunkRequirementEdges(chain, bound, chain.Hunks[index].ID)
	}
}

func hunkRequirementEdges(chain *Chain, bound []boundOCM, hunkID string) []ChainEdge {
	if len(bound) == 0 {
		return []ChainEdge{{Gap: GapMissing, Artifact: ocmDir + "/change.ocm*.json", Field: "cem.mapSha256, targetRevision",
			Axes: UnstatedAxes(), Reason: "no OCM map bound to this sealed map by digest and revision was found, so no requirement is linked"}}
	}
	counts := obligationCounts(bound)
	var edges []ChainEdge
	for _, ocm := range bound {
		for index, obligation := range ocm.doc.Obligations {
			edges = append(edges, obligationEdge(chain, ocm, index, obligation, counts[obligation.ID], hunkID)...)
		}
	}
	if len(edges) == 0 {
		edges = append(edges, ChainEdge{Gap: GapMissing, Artifact: ocmDir + "/change.ocm*.json", Field: "obligations[].hunkIds",
			Axes: UnstatedAxes(), Reason: "no obligation of the bound OCM maps lists this hunk id"})
	}
	return edges
}

func obligationEdge(chain *Chain, ocm boundOCM, index int, obligation wireObligation, count int, hunkID string) []ChainEdge {
	position := indexOf(obligation.HunkIDs, hunkID)
	if position < 0 {
		return nil
	}
	edge := ChainEdge{Target: obligation.ID, Anchor: "req-" + obligation.ID, Artifact: ocm.path,
		Field: "obligations[" + strconv.Itoa(index) + "].hunkIds[" + strconv.Itoa(position) + "]",
		Axes:  Weakest(ocm.axes, chain.Sealed.Source.Axes)}
	edge.Gap, edge.Reason = obligationHunkGap(obligation, count)
	if count > 1 {
		edge.Anchor = ""
	}
	return []ChainEdge{edge}
}

func indexOf(values []string, want string) int {
	for index, value := range values {
		if value == want {
			return index
		}
	}
	return -1
}

// HunkDetail reads one hunk's lines at the revision its map pins and the bytes
// of every evidence span it cites, each at its own object id.
func (w Worktree) HunkDetail(ctx context.Context, chain *Chain, hunkID string) *ChainDetail {
	position := -1
	for index, hunk := range chain.Hunks {
		if hunk.ID == hunkID {
			position = index
		}
	}
	if position < 0 {
		return &ChainDetail{Err: "the sealed map lists no hunk with this id"}
	}
	detail := &ChainDetail{Hunk: chain.Hunks[position]}
	detail.Lines, detail.Range = w.hunkLines(ctx, chain, detail.Hunk)
	detail.Text = lineRange(detail.Lines, detail.Hunk)
	detail.Spans = w.citedSpans(ctx, chain, detail.Hunk)
	return detail
}

// hunkLines reads the post-image at the change commit, or the pre-image at the
// base for a hunk that adds no line.
func (w Worktree) hunkLines(ctx context.Context, chain *Chain, hunk ChainHunk) (*Blob, string) {
	if hunk.New.Count == 0 {
		if !objectIDPattern.MatchString(chain.Base) {
			return &Blob{Err: "the map's baseRevision is not a full object id"}, "old"
		}
		return w.Read(ctx, chain.Base, hunk.Path), "old"
	}
	return w.Read(ctx, chain.Change, hunk.Path), "new"
}

func lineRange(blob *Blob, hunk ChainHunk) string {
	if blob == nil || blob.Err != "" {
		return ""
	}
	span := hunk.New
	if hunk.New.Count == 0 {
		span = hunk.Old
	}
	lines := strings.Split(blob.Text, "\n")
	start, end := span.Start-1, span.Start-1+min(span.Count, maxDetailLines)
	if start < 0 || end > len(lines) || start > end {
		return ""
	}
	return strings.Join(lines[start:end], "\n")
}

func (w Worktree) citedSpans(ctx context.Context, chain *Chain, hunk ChainHunk) []ChainSpan {
	document, err := decodeSealed(chain.Sealed)
	if err != nil {
		return nil
	}
	byID := map[string]wireSpanRef{}
	for _, evidence := range document.Evidence {
		byID[evidence.ID] = evidence
	}
	reader := &objects{worktree: w, ctx: ctx, read: map[string]objectRead{}}
	var spans []ChainSpan
	for _, edge := range hunk.Evidence {
		span := ChainSpan{Edge: edge}
		if edge.Gap == "" {
			span.Text, _, _ = reader.spanOf(byID[edge.Target])
		}
		spans = append(spans, span)
	}
	return spans
}

// ChangeIDs names the sealed changes a listing of .corvint/changes holds,
// newest-named last as Git lists them.
func ChangeIDs(listing *Listing) []string {
	var ids []string
	for _, entry := range listing.Entries {
		id, found := strings.CutSuffix(entry.Name, ".cem.json")
		if found && objectIDPattern.MatchString(id) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

// GapCount counts the gap rows of a set of edges.
func GapCount(edges []ChainEdge) int {
	count := 0
	for _, edge := range edges {
		if edge.Gap != "" {
			count++
		}
	}
	return count
}
