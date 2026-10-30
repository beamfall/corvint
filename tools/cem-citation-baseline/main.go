// Command cem-citation-baseline reproduces the per-map citation baseline
// (https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-12-cem-citation-baseline-593-maps.md) over a
// directory of committed change maps and splits it by citation class
// (docs/specs/daily-change-evidence-workflow-v0.md, DCW-V0-049).
//
// For every MAPS/<bind>.cem.json, in name order, it runs the in-process
// equivalent of
//
//	corvint cem status --map MAPS/<bind>.cem.json --expected-base <baseRevision> --target <bind> --citation
//
// and writes one JSON row per map to stdout, in the baseline row shape plus a
// "class" member. A cem/1.0 map takes its class from the status envelope's
// citationClass: change, chore, or record for a map whose hunks change only
// evidence records (CEM-V1-106, CEM-V1-170, DCW-V0-059). A retired cem/0.x map is never read: its row is
// the stable reader's unsupported-spec refusal with its fix line and no class,
// so the summary counts it as unclassified (CEM-V1-210). A map whose status
// carries no citation block is an error row and an ERR line on stderr.
// --summary writes the aggregate for all maps and for each class, record
// separately from change.
//
// An accepted map row also carries three arms scored by the verifier's own
// tier rules (DCW-V0-051..056): authored (the map's bases), grepbot (bases a
// bot derives from the diff alone) and preimage (each hunk's own old range),
// each with a seeded derangement null, plus the self-citation members.
//
// --targets names a file of "<bind> <target>" lines. A listed map is verified,
// and its arms derived, at that target instead of its bind commit: the
// CEM-V1-104 T' commit whose tree is the bind tree with only
// .corvint/change.cem.json replaced by the map bytes, for a map migrated after
// it was bound.
//
// Usage:
//
//	cem-citation-baseline [--root DIR] [--maps DIR] [--summary FILE] [--targets FILE]
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/cli"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// Citation classes, as `cem status --citation` names them for cem/1.0 maps.
const (
	classChange = "change"
	classChore  = "chore"
	classRecord = "record"
)

// tiers are the citation tiers, in the baseline's key order.
var tiers = []string{"anchored", "cited", "held", "mechanical-proved", "unknown"}

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("cem-citation-baseline", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "repository root")
	maps := flags.String("maps", ".corvint/changes", "map directory, relative to --root")
	summaryPath := flags.String("summary", "", "write the aggregate summary JSON to this file")
	targetsPath := flags.String("targets", "", "file of \"<bind> <target>\" verification targets")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: cem-citation-baseline [--root DIR] [--maps DIR] [--summary FILE] [--targets FILE]")
		return 2
	}
	targets, err := readTargets(*targetsPath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	absRoot, err := filepath.Abs(*root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	names, err := filepath.Glob(filepath.Join(absRoot, *maps, "*.cem.json"))
	if err != nil || len(names) == 0 {
		fmt.Fprintf(stderr, "no *.cem.json maps under %s\n", filepath.Join(absRoot, *maps))
		return 2
	}
	sort.Strings(names)
	var rows []row
	for _, name := range names {
		r := measure(ctx, absRoot, name, targets)
		rows = append(rows, r)
		fmt.Fprintln(stdout, pyJSON(r.fields(), -1, 0))
		if r.citation == nil {
			fmt.Fprintf(stderr, "ERR %s %s\n", r.stem, pyJSON(r.issues, -1, 0))
		}
	}
	if *summaryPath != "" {
		data := pyJSON(summarize(rows), 1, 0) + "\n"
		if err := os.WriteFile(*summaryPath, []byte(data), 0o644); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	}
	return 0
}

// row is one map's measurement.
type row struct {
	stem     string
	spec     string
	class    string
	valid    bool
	issues   []any
	citation map[string]any
	arms     *armsRow // nil without a citation block
}

// readMap reads one regular map file of at most the native map bound, the
// bound the status verifier applies, without following a symbolic link.
func readMap(name string) ([]byte, error) {
	info, err := os.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("map is not a regular file")
	}
	if info.Size() > wire.MaxMapBytes {
		return nil, errors.New("map exceeds the " + strconv.Itoa(wire.MaxMapBytes) + " byte bound")
	}
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, wire.MaxMapBytes+1))
	if err == nil && len(raw) > wire.MaxMapBytes {
		err = errors.New("map exceeds the " + strconv.Itoa(wire.MaxMapBytes) + " byte bound")
	}
	return raw, err
}

// readTargets reads the optional "<bind> <target>" file (DCW-V0-056): each named map is
// verified, and its arms derived, at its target instead of its bind commit.
func readTargets(path string) (map[string]string, error) {
	targets := map[string]string{}
	if path == "" {
		return targets, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	for i, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if len(fields) != 2 || !wire.IsGitOid(fields[0]) || !wire.IsGitOid(fields[1]) {
			return nil, fmt.Errorf("%s:%d: want \"<bind> <target>\" full lowercase OIDs", path, i+1)
		}
		targets[fields[0]] = fields[1]
	}
	return targets, nil
}

func measure(ctx context.Context, root, name string, targets map[string]string) row {
	stem := strings.TrimSuffix(filepath.Base(name), ".cem.json")
	target := stem
	if t, ok := targets[stem]; ok {
		target = t
	}
	r := row{stem: stem, issues: []any{}}
	raw, err := readMap(name)
	var head struct {
		Spec         string `json:"spec"`
		BaseRevision string `json:"baseRevision"`
	}
	if err == nil {
		err = json.Unmarshal(raw, &head)
	}
	if err != nil {
		r.issues = []any{issue("map-unreadable", err.Error())}
		return r
	}
	r.spec = head.Spec
	if wire.RetiredSpec(head.Spec) {
		_, err := wire.ParseStable(raw)
		r.issues = []any{issue(cemcode.CodeOf(err), cemcode.MessageOf(err))}
		return r
	}
	rel, _ := filepath.Rel(root, name)
	var out, errb bytes.Buffer
	// A cem/1.0 status requires an explicit absolute artifact root, which a
	// producer map without artifacts never opens (CEM-V1-146, CEM-V1-147);
	// root is already absolute.
	args := []string{"status", "--map", filepath.ToSlash(rel), "--expected-base", head.BaseRevision, "--target", target, "--citation", "--artifacts", root}
	cli.Run(ctx, root, args, &out, &errb)
	var envelope map[string]any
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		if err := json.Unmarshal(errb.Bytes(), &envelope); err != nil {
			r.issues = []any{issue("status-unreadable", strings.TrimSpace(errb.String()))}
			return r
		}
	}
	if stable, ok := envelope["stableVerification"].(map[string]any); ok {
		r.valid = stable["outcome"] == "ACCEPT"
		r.citation, _ = stable["citation"].(map[string]any)
		if class, ok := envelope["citationClass"].(map[string]any); ok {
			r.class, _ = class["class"].(string)
		}
		if codes, ok := stable["issueCodes"].([]any); ok {
			for _, code := range codes {
				r.issues = append(r.issues, issue(fmt.Sprint(code), ""))
			}
		}
		if code, ok := stable["code"].(string); ok && len(r.issues) == 0 {
			r.issues = []any{issue(code, "")}
		}
		if r.citation != nil {
			if r.valid {
				classes, _ := r.citation["hunksByClass"].(map[string]any)
				r.arms = measureArms(ctx, root, name, target, number(classes["anchored"]))
			} else {
				r.arms = &armsRow{failures: []any{armsIssue("arms-map-not-accepted", "")}}
			}
		}
	} else {
		code, _ := envelope["code"].(string)
		message, _ := envelope["error"].(string)
		r.issues = []any{issue(code, message)}
	}
	return r
}

func issue(code, message string) map[string]any {
	return map[string]any{"code": code, "message": message}
}

// fields is the baseline row: the citation counts for a map with a citation
// block, otherwise the error shape, each with the map's class.
func (r row) fields() map[string]any {
	if r.citation == nil {
		return map[string]any{"class": r.class, "hunksTotal": 0, "issues": r.issues, "map": r.stem, "spec": r.spec, "valid": false}
	}
	hunks, _ := r.citation["hunks"].([]any)
	fields := map[string]any{
		"annotationHunks":   r.citation["annotationHunks"],
		"class":             r.class,
		"concentratedHunks": r.citation["concentratedHunks"],
		"distinctEvidence":  r.citation["distinctEvidence"],
		"hunksByClass":      r.citation["hunksByClass"],
		"hunksTotal":        len(hunks),
		"invariant":         r.citation["invariant"],
		"map":               r.stem,
		"maxFanOut":         r.citation["maxFanOut"],
		"spec":              r.spec,
		"specificity":       r.citation["specificity"],
		"valid":             r.valid,
	}
	fields["samePathCitations"] = r.citation["samePathCitations"]
	for key, value := range r.arms.members() {
		fields[key] = value
	}
	return fields
}

// members are the row's self-citation members (DCW-V0-051) and arms
// (DCW-V0-052..DCW-V0-054); a map without arms carries null members and its
// armsIssues.
func (a *armsRow) members() map[string]any {
	out := map[string]any{"arms": nil, "armsIssues": []any{}, "diffAdjacentBases": nil, "diffAdjacentHunks": nil, "samePathBases": nil, "samePathHunks": nil, "supportedHunks": nil}
	if a == nil {
		return out
	}
	out["armsIssues"] = a.failures
	if a.arms == nil {
		return out
	}
	authored := a.arms[armAuthored]
	out["supportedHunks"] = a.hunks
	out["samePathBases"] = authored.samePath
	out["diffAdjacentBases"] = authored.adjacent
	out["samePathHunks"] = authored.samePathHunks
	out["diffAdjacentHunks"] = authored.adjacentHunks
	arms := map[string]any{}
	for _, name := range armNames {
		arm := a.arms[name]
		member := map[string]any{
			"anchored": arm.anchored, "anchoredExclAdjacent": arm.anchoredExcl, "bases": arm.bases, "diffAdjacentBases": arm.adjacent,
			"hunksWithBases": arm.hunksWithBases, "samePathBases": arm.samePath,
			"anchoredExcess": nil, "anchoredExcessExclAdjacent": nil, "nullMean": nil, "nullMeanExclAdjacent": nil, "nullSd": nil, "nullSdExclAdjacent": nil,
		}
		if arm.null != nil {
			member["nullMean"], member["nullSd"] = round(arm.null.mean, 4), round(arm.null.sd, 4)
			member["anchoredExcess"] = round(float64(arm.anchored)-arm.null.mean, 4)
			member["nullMeanExclAdjacent"], member["nullSdExclAdjacent"] = round(arm.nullExcl.mean, 4), round(arm.nullExcl.sd, 4)
			member["anchoredExcessExclAdjacent"] = round(float64(arm.anchoredExcl)-arm.nullExcl.mean, 4)
		}
		arms[name] = member
	}
	out["arms"] = arms
	return out
}

func round(value float64, places int) float64 {
	scale := math.Pow(10, float64(places))
	return math.Round(value*scale) / scale
}

// summarize aggregates every map, then each class, in the baseline summary
// shape.
func summarize(rows []row) ordered {
	byClass := map[string][]row{}
	for _, r := range rows {
		byClass[r.class] = append(byClass[r.class], r)
	}
	out := ordered{{"all", aggregate(rows)}, {classChange, aggregate(byClass[classChange])}, {classChore, aggregate(byClass[classChore])},
		{classRecord, aggregate(byClass[classRecord])}}
	if unresolved := byClass[""]; len(unresolved) > 0 {
		out = append(out, member{"unclassified", aggregate(unresolved)})
	}
	return out
}

func aggregate(rows []row) ordered {
	tierCount := map[string]int{}
	var withCitation, invalid, hunks, annotations, invariant, positive, zero, negative int
	specificity, fanOut := map[int]int{}, map[int]int{}
	for _, r := range rows {
		if !r.valid || r.citation == nil {
			invalid++
		}
		if r.citation == nil {
			continue
		}
		withCitation++
		list, _ := r.citation["hunks"].([]any)
		hunks += len(list)
		classes, _ := r.citation["hunksByClass"].(map[string]any)
		for _, tier := range tiers {
			tierCount[tier] += number(classes[tier])
		}
		annotations += number(r.citation["annotationHunks"])
		if r.citation["invariant"] == true {
			invariant++
		}
		value := number(r.citation["specificity"])
		switch {
		case value > 0:
			positive++
		case value == 0:
			zero++
		default:
			negative++
		}
		specificity[value]++
		fanOut[number(r.citation["maxFanOut"])]++
	}
	tierMembers := ordered{}
	for _, tier := range tiers {
		tierMembers = append(tierMembers, member{tier, tierCount[tier]})
	}
	return ordered{
		{"maps", len(rows)},
		{"withCitation", withCitation},
		{"invalidOrError", invalid},
		{"hunks", hunks},
		{"tiers", tierMembers},
		{"annotationHunks", annotations},
		{"invariantMaps", invariant},
		{"specificity", ordered{{"positive", positive}, {"zero", zero}, {"negative", negative}}},
		{"specificityHistogram", histogram(specificity)},
		{"maxFanOutHistogram", histogram(fanOut)},
		{"selfCitation", selfCitation(rows)},
		{"arms", armsSummary(rows)},
		{"killStatistic", killStatistic(rows)},
	}
}

func histogram(counts map[int]int) ordered {
	keys := make([]int, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	out := ordered{}
	for _, key := range keys {
		out = append(out, member{strconv.Itoa(key), counts[key]})
	}
	return out
}

func number(value any) int {
	if f, ok := value.(float64); ok {
		return int(f)
	}
	return 0
}

// member and ordered keep the summary's member order.
type member struct {
	key   string
	value any
}

type ordered []member

// pyJSON encodes like the baseline's Python json.dumps: ", " and ": "
// separators on one line (indent < 0, map keys sorted), or one member per
// line at the given indent; ordered keeps its own order.
func pyJSON(value any, indent, depth int) string {
	var members []member
	switch v := value.(type) {
	case ordered:
		members = v
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			members = append(members, member{key, v[key]})
		}
	case []any:
		parts := make([]string, len(v))
		for i, item := range v {
			parts[i] = pyJSON(item, indent, depth+1)
		}
		return join("[", "]", parts, indent, depth)
	default:
		data, _ := json.Marshal(v)
		return string(data)
	}
	parts := make([]string, len(members))
	for i, m := range members {
		key, _ := json.Marshal(m.key)
		parts[i] = string(key) + ": " + pyJSON(m.value, indent, depth+1)
	}
	return join("{", "}", parts, indent, depth)
}

func join(open, close string, parts []string, indent, depth int) string {
	if len(parts) == 0 {
		return open + close
	}
	if indent < 0 {
		return open + strings.Join(parts, ", ") + close
	}
	inner := "\n" + strings.Repeat(" ", indent*(depth+1))
	return open + inner + strings.Join(parts, ","+inner) + "\n" + strings.Repeat(" ", indent*depth) + close
}

// Arms (DCW-V0-052..DCW-V0-054). Every arm keeps the authored map's
// supported hunks H (non-mechanical hunks with a non-empty basis, in hunk ID
// order) and swaps only their bases, so the arms compare paired over the same
// hunks; the tier of each hunk comes from verify.ComputeCitation, the stable
// verifier's own citation computation, run in process on a synthetic map.
const (
	armAuthored = "authored"
	armGrepbot  = "grepbot"
	armPreimage = "preimage"

	// nullDraws is the number of derangements of each arm's basis sets.
	nullDraws = 200
	// adjacentLines widens a hunk's old range on both sides for diff adjacency.
	adjacentLines = 3
	// grepbotBases is the most bases the grep bot cites for one hunk.
	grepbotBases = 2
	// gitOutputLimit and gitTimeout bound each of the tool's own git grep and
	// cat-file reads; exceeding either is arms-git-failed for that map.
	gitOutputLimit = 256 << 20
	gitTimeout     = 5 * time.Minute
)

var armNames = []string{armAuthored, armGrepbot, armPreimage}

// grepbotID is the requirement-ID grammar the grep bot reads from the added
// bytes; a match must not touch an ID byte [A-Za-z0-9_-] on either side.
var grepbotID = regexp.MustCompile(`[A-Z][A-Z0-9]+-V[0-9]+-[0-9]{3}`)

// source is one evidence record an arm cites: its path, its one-based line
// range in that path, and its span bytes.
type source struct {
	id          string
	path        string
	first, last int64
	span        []byte
}

// armResult is one arm's counts over H, in the full column and in the column
// without diff-adjacent bases.
type armResult struct {
	bases, samePath, adjacent, hunksWithBases int
	samePathHunks, adjacentHunks              int
	anchored, anchoredExcl                    int
	null, nullExcl                            *nullStat
}

type nullStat struct{ mean, sd float64 }

// armsRow is one map's members and arms.
type armsRow struct {
	hunks    int // |H|
	arms     map[string]*armResult
	failures []any
}

// armsIssue is a closed arms failure code with its detail.
func armsIssue(code, message string) map[string]any { return issue(code, message) }

// measureArms derives the canonical patch of base..target, checks it against
// the map's patchSha256, builds the three arms over H and scores each, with
// its derangement null, by the verifier's citation tiers.
func measureArms(ctx context.Context, root, name, target string, statusAnchored int) *armsRow {
	out := &armsRow{arms: map[string]*armResult{}, failures: []any{}}
	fail := func(code, message string) *armsRow {
		out.failures = append(out.failures, armsIssue(code, message))
		out.arms = nil
		return out
	}
	raw, err := readMap(name)
	if err != nil {
		return fail("arms-map-unreadable", err.Error())
	}
	document, err := wire.ParseStable(raw)
	if err != nil {
		return fail("arms-map-unreadable", cemcode.MessageOf(err))
	}
	change := wire.Map(document.Change)
	repository, err := gitauth.Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		return fail("arms-patch-unavailable", cemcode.CodeOf(err))
	}
	data, err := repository.CanonicalDiff(ctx, change.BaseRevision, target)
	if err != nil {
		return fail("arms-patch-unavailable", cemcode.CodeOf(err))
	}
	digest := sha256.Sum256(data)
	if hex.EncodeToString(digest[:]) != change.PatchSha256 {
		return fail("arms-patch-digest-mismatch", "canonical patch does not match patchSha256")
	}
	parsed, err := patch.Parse(data)
	if err != nil {
		return fail("arms-patch-unavailable", cemcode.CodeOf(err))
	}
	derived := map[string]*patch.Hunk{}
	for _, hunk := range parsed.Hunks {
		derived[hunk.ID] = hunk
	}
	hunks := append([]wire.Hunk(nil), change.Hunks...)
	sort.Slice(hunks, func(i, j int) bool { return hunks[i].ID < hunks[j].ID })
	var supported []wire.Hunk
	var subset patch.Patch
	for _, hunk := range hunks {
		if hunk.Disposition == "mechanical" || len(hunk.Basis) == 0 {
			continue
		}
		if derived[hunk.ID] == nil {
			return fail("arms-hunk-not-in-patch", hunk.ID)
		}
		supported = append(supported, hunk)
		subset.Hunks = append(subset.Hunks, derived[hunk.ID])
	}
	out.hunks = len(supported)

	// Object reads: the authored evidence blobs, then each H hunk's old path
	// at the base revision, in one cat-file batch.
	var objects []string
	for _, record := range change.Evidence {
		objects = append(objects, record.BlobOid)
	}
	for _, hunk := range subset.Hunks {
		if hunk.OldPath != nil {
			objects = append(objects, change.BaseRevision+":"+*hunk.OldPath)
		}
	}
	blobs, err := catFile(ctx, root, objects)
	if err != nil {
		return fail("arms-git-failed", err.Error())
	}
	authoredSources := map[string]source{}
	for _, record := range change.Evidence {
		blob, ok := blobs[record.BlobOid]
		if !ok || record.Span.Start < 0 || record.Span.End > int64(len(blob)) || record.Span.Start > record.Span.End {
			return fail("arms-evidence-unreadable", record.ID)
		}
		first, last := spanLines(blob, record.Span.Start, record.Span.End)
		authoredSources[record.ID] = source{id: record.ID, path: record.Path, first: first, last: last, span: blob[record.Span.Start:record.Span.End]}
	}
	sets := map[string][][]source{armAuthored: make([][]source, len(supported))}
	for i, hunk := range supported {
		seen := map[string]bool{}
		for _, basis := range hunk.Basis {
			if !seen[basis.EvidenceID] {
				seen[basis.EvidenceID] = true
				sets[armAuthored][i] = append(sets[armAuthored][i], authoredSources[basis.EvidenceID])
			}
		}
		sort.Slice(sets[armAuthored][i], func(a, b int) bool { return sets[armAuthored][i][a].id < sets[armAuthored][i][b].id })
	}
	if sets[armGrepbot], err = grepbotSources(ctx, root, change.BaseRevision, supported, subset.Hunks); err != nil {
		return fail("arms-git-failed", err.Error())
	}
	if sets[armPreimage], err = preimageSources(change.BaseRevision, subset.Hunks, blobs); err != nil {
		return fail("arms-git-failed", err.Error())
	}

	draws := derangements(change.PatchSha256, len(supported))
	for _, arm := range armNames {
		result, mismatch := scoreArm(supported, &subset, sets[arm], draws)
		if mismatch != "" {
			return fail("arms-pair-mismatch", arm+" "+mismatch)
		}
		out.arms[arm] = result
	}
	if out.arms[armAuthored].anchored != statusAnchored {
		return fail("arms-authored-mismatch", fmt.Sprintf("authored arm anchors %d hunks; status anchors %d", out.arms[armAuthored].anchored, statusAnchored))
	}
	return out
}

// spanLines returns the one-based first and last line a byte span covers.
func spanLines(blob []byte, start, end int64) (int64, int64) {
	first := 1 + int64(bytes.Count(blob[:start], []byte{'\n'}))
	if end <= start {
		return first, first
	}
	return first, 1 + int64(bytes.Count(blob[:end-1], []byte{'\n'}))
}

// adjacent reports whether a source is diff-adjacent to a hunk: it lies in
// the hunk's own path and its lines intersect the hunk's old range widened by
// adjacentLines on each side.
func adjacent(hunk wire.Hunk, s source) bool {
	if s.path != hunk.Path {
		return false
	}
	count := hunk.OldRange.Count
	if count < 1 {
		count = 1
	}
	low, high := hunk.OldRange.Start-adjacentLines, hunk.OldRange.Start+count-1+adjacentLines
	return s.first <= high && s.last >= low
}

// scoreArm scores one arm's basis sets over H: the real anchored count from
// ComputeCitation on the whole synthetic map, the per-pair anchors from one
// ComputeCitation per distinct source (every H hunk citing that source
// alone), and the derangement null from the pair table. A hunk anchors a set
// iff it anchors some member, the verifier's OR over bases, so the pair table
// must reproduce the real count; mismatch names the column when it does not.
func scoreArm(supported []wire.Hunk, subset *patch.Patch, sets [][]source, draws [][]int) (*armResult, string) {
	result := &armResult{}
	excl := make([][]source, len(sets))
	for i, set := range sets {
		if len(set) > 0 {
			result.hunksWithBases++
		}
		samePath, near := false, false
		for _, s := range set {
			result.bases++
			if s.path == supported[i].Path {
				result.samePath++
				samePath = true
			}
			if adjacent(supported[i], s) {
				result.adjacent++
				near = true
				continue
			}
			excl[i] = append(excl[i], s)
		}
		if samePath {
			result.samePathHunks++
		}
		if near {
			result.adjacentHunks++
		}
	}
	pairs := pairTable(supported, subset, sets)
	result.anchored = anchoredCount(supported, subset, sets)
	result.anchoredExcl = anchoredCount(supported, subset, excl)
	if tableCount(pairs, sets, nil) != result.anchored {
		return nil, "full"
	}
	if tableCount(pairs, excl, nil) != result.anchoredExcl {
		return nil, "exclAdjacent"
	}
	if draws != nil {
		result.null = nullOf(pairs, sets, draws)
		result.nullExcl = nullOf(pairs, excl, draws)
	}
	return result, ""
}

// synthetic is the map ComputeCitation reads: the H hunks with the given
// basis sets and one evidence record per distinct source.
func synthetic(supported []wire.Hunk, sets [][]source) (*wire.Map, map[string][]byte) {
	document := &wire.Map{Spec: wire.StableSpec}
	spans := map[string][]byte{}
	for i, hunk := range supported {
		hunk.Basis = nil
		for _, s := range sets[i] {
			hunk.Basis = append(hunk.Basis, wire.Basis{EvidenceID: s.id, Relation: "specification"})
			if _, ok := spans[s.id]; !ok {
				spans[s.id] = s.span
				document.Evidence = append(document.Evidence, wire.Evidence{ID: s.id, Path: s.path})
			}
		}
		document.Hunks = append(document.Hunks, hunk)
	}
	return document, spans
}

func anchoredCount(supported []wire.Hunk, subset *patch.Patch, sets [][]source) int {
	document, spans := synthetic(supported, sets)
	return verify.ComputeCitation(document, subset, spans).HunksByClass.Anchored
}

// pairTable maps each distinct source ID to the H hunks it anchors alone.
func pairTable(supported []wire.Hunk, subset *patch.Patch, sets [][]source) map[string][]bool {
	distinct := map[string]source{}
	for _, set := range sets {
		for _, s := range set {
			distinct[s.id] = s
		}
	}
	index := map[string]int{}
	for i, hunk := range supported {
		index[hunk.ID] = i
	}
	table := map[string][]bool{}
	for id, s := range distinct {
		alone := make([][]source, len(supported))
		for i := range alone {
			alone[i] = []source{s}
		}
		document, spans := synthetic(supported, alone)
		row := make([]bool, len(supported))
		for _, hunk := range verify.ComputeCitation(document, subset, spans).Hunks {
			row[index[hunk.HunkID]] = hunk.Tier == verify.TierAnchored
		}
		table[id] = row
	}
	return table
}

// tableCount counts the hunks i that anchor the set at draw[i] (or their own
// set when draw is nil).
func tableCount(pairs map[string][]bool, sets [][]source, draw []int) int {
	count := 0
	for i := range sets {
		j := i
		if draw != nil {
			j = draw[i]
		}
		for _, s := range sets[j] {
			if pairs[s.id][i] {
				count++
				break
			}
		}
	}
	return count
}

func nullOf(pairs map[string][]bool, sets [][]source, draws [][]int) *nullStat {
	values := make([]float64, len(draws))
	var sum float64
	for k, draw := range draws {
		values[k] = float64(tableCount(pairs, sets, draw))
		sum += values[k]
	}
	mean := sum / float64(len(values))
	var squares float64
	for _, value := range values {
		squares += (value - mean) * (value - mean)
	}
	return &nullStat{mean: mean, sd: math.Sqrt(squares / float64(len(values)))}
}

// derangements returns nullDraws permutations of 0..n-1 without fixed
// points, or nil when n < 2. The PRNG is math/rand/v2 PCG seeded with the
// big-endian uint64s of bytes 0..7 and 8..15 of the hex-decoded patchSha256;
// each draw is a Fisher-Yates shuffle (i from n-1 down to 1, j = Uint64() mod
// (i+1)) of the identity, redrawn until it has no fixed point.
func derangements(patchSha256 string, n int) [][]int {
	if n < 2 {
		return nil
	}
	seed, err := hex.DecodeString(patchSha256)
	if err != nil || len(seed) < 16 {
		seed = make([]byte, 16)
	}
	generator := rand.New(rand.NewPCG(binary.BigEndian.Uint64(seed[0:8]), binary.BigEndian.Uint64(seed[8:16])))
	draws := make([][]int, 0, nullDraws)
	for len(draws) < nullDraws {
		draw := make([]int, n)
		for i := range draw {
			draw[i] = i
		}
		for i := n - 1; i > 0; i-- {
			j := int(generator.Uint64() % uint64(i+1))
			draw[i], draw[j] = draw[j], draw[i]
		}
		fixed := false
		for i, j := range draw {
			if i == j {
				fixed = true
				break
			}
		}
		if !fixed {
			draws = append(draws, draw)
		}
	}
	return draws
}

// addedIDs returns the distinct grep-bot requirement IDs in a hunk's added
// bytes, in ascending order.
func addedIDs(hunk *patch.Hunk) []string {
	var added []byte
	for _, line := range hunk.Body {
		if line.Prefix == '+' {
			added = append(added, line.Payload...)
		}
	}
	seen := map[string]bool{}
	var ids []string
	for _, match := range grepbotID.FindAllIndex(added, -1) {
		if match[0] > 0 && idByte(added[match[0]-1]) || match[1] < len(added) && idByte(added[match[1]]) {
			continue
		}
		id := string(added[match[0]:match[1]])
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func idByte(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_' || c == '-'
}

// grepLine is one git grep match at the base revision.
type grepLine struct {
	path    string
	line    int64
	content []byte
}

func (g grepLine) source(arm string) source {
	return source{id: arm + ":" + g.path + ":" + strconv.FormatInt(g.line, 10), path: g.path, first: g.line, last: g.line, span: append(append([]byte(nil), g.content...), '\n')}
}

func earlier(a, b grepLine) bool { return a.path < b.path || a.path == b.path && a.line < b.line }

// grepbotSources is the grep bot, which reads only the diff: for each hunk,
// up to grepbotBases requirement IDs from its added bytes (ascending) whose
// defining line "- `ID`:" (after leading spaces and tabs) exists under
// docs/specs at the base revision, citing the first such line by (path,
// line); a hunk with none cites the first base line by (path, line), in a
// file under docs/ or directly in the hunk's own directory other than the
// hunk's own path, that contains the hunk path's basename.
func grepbotSources(ctx context.Context, root, base string, supported []wire.Hunk, hunks []*patch.Hunk) ([][]source, error) {
	ids := make([][]string, len(hunks))
	var patterns []string
	seen := map[string]bool{}
	for i, hunk := range hunks {
		ids[i] = addedIDs(hunk)
		for _, id := range ids[i] {
			if !seen[id] {
				seen[id] = true
				patterns = append(patterns, "`"+id+"`:")
			}
		}
	}
	definitions := map[string]grepLine{}
	matches, err := gitGrep(ctx, root, base, patterns, []string{"docs/specs"})
	if err != nil {
		return nil, err
	}
	for _, match := range matches {
		line := bytes.TrimLeft(match.content, " \t")
		if !bytes.HasPrefix(line, []byte("- `")) {
			continue
		}
		end := bytes.IndexByte(line[3:], '`')
		if end <= 0 || 3+end+1 >= len(line) || line[3+end+1] != ':' {
			continue
		}
		id := string(line[3 : 3+end])
		if current, ok := definitions[id]; seen[id] && (!ok || earlier(match, current)) {
			definitions[id] = match
		}
	}
	out := make([][]source, len(hunks))
	var fallback []int
	for i := range hunks {
		for _, id := range ids[i] {
			if definition, ok := definitions[id]; ok && len(out[i]) < grepbotBases {
				out[i] = append(out[i], definition.source(armGrepbot))
			}
		}
		if len(out[i]) == 0 {
			fallback = append(fallback, i)
		}
	}
	if len(fallback) == 0 {
		return out, nil
	}
	names := map[string]bool{}
	scopes := map[string]bool{"docs/": true}
	for _, i := range fallback {
		names[path.Base(supported[i].Path)] = true
		if dir := path.Dir(supported[i].Path); dir == "." {
			scopes[":(glob)*"] = true
		} else {
			scopes[":(glob)"+dir+"/*"] = true
		}
	}
	matches, err = gitGrep(ctx, root, base, sortedKeys(names), sortedKeys(scopes))
	if err != nil {
		return nil, err
	}
	for _, i := range fallback {
		own := supported[i].Path
		name, dir := []byte(path.Base(own)), path.Dir(own)
		var best *grepLine
		for k := range matches {
			match := matches[k]
			if match.path == own || !bytes.Contains(match.content, name) {
				continue
			}
			if !strings.HasPrefix(match.path, "docs/") && path.Dir(match.path) != dir {
				continue
			}
			if best == nil || earlier(match, *best) {
				best = &matches[k]
			}
		}
		if best != nil {
			out[i] = []source{best.source(armGrepbot)}
		}
	}
	return out, nil
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// preimageSources cites each H hunk's own old range in its own old path at
// the base revision (an insertion cites the line it follows); a created file
// has no preimage and no base. A hunk whose old path has no blob at the base
// revision is an error (arms-git-failed), never an empty basis.
func preimageSources(base string, hunks []*patch.Hunk, blobs map[string][]byte) ([][]source, error) {
	out := make([][]source, len(hunks))
	for i, hunk := range hunks {
		if hunk.OldPath == nil || hunk.OldRange.Start < 1 {
			continue
		}
		blob, ok := blobs[base+":"+*hunk.OldPath]
		if !ok {
			return nil, fmt.Errorf("no blob for %s at %s", *hunk.OldPath, base)
		}
		first, last := hunk.OldRange.Start, hunk.OldRange.Start+hunk.OldRange.Count-1
		if last < first {
			last = first
		}
		start, end := lineOffset(blob, first), lineOffset(blob, last+1)
		out[i] = []source{{id: armPreimage + ":" + *hunk.OldPath + ":" + strconv.FormatInt(first, 10) + ":" + strconv.FormatInt(last, 10), path: *hunk.OldPath, first: first, last: last, span: blob[start:end]}}
	}
	return out, nil
}

// lineOffset is the byte offset where one-based line n starts, or len(blob).
func lineOffset(blob []byte, n int64) int {
	offset := 0
	for line := int64(1); line < n; line++ {
		k := bytes.IndexByte(blob[offset:], '\n')
		if k < 0 {
			return len(blob)
		}
		offset += k + 1
	}
	return offset
}

// gitEnv is the scrubbed environment of the tool's own Git reads.
func gitEnv() []string {
	env := []string{}
	for _, key := range []string{"PATH", "TMPDIR", "SystemRoot"} {
		if value, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+value)
		}
	}
	return append(env, "LC_ALL=C", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "GIT_NO_REPLACE_OBJECTS=1")
}

// errGitOutputLimit is the refusal for a git read past gitOutputLimit.
var errGitOutputLimit = fmt.Errorf("output exceeds %d bytes", gitOutputLimit)

// cappedBuffer keeps at most gitOutputLimit bytes and fails the write past it.
type cappedBuffer struct{ bytes.Buffer }

func (c *cappedBuffer) Write(p []byte) (int, error) {
	if c.Len()+len(p) > gitOutputLimit {
		return 0, errGitOutputLimit
	}
	return c.Buffer.Write(p)
}

// gitOutput runs one of the tool's own git reads with the scrubbed
// environment, gitTimeout and gitOutputLimit on stdout and stderr.
func gitOutput(ctx context.Context, stdin io.Reader, args ...string) ([]byte, []byte, error) {
	ctx, cancel := context.WithTimeout(ctx, gitTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, "git", args...)
	command.Env = gitEnv()
	command.Stdin = stdin
	var stdout, stderr cappedBuffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	if err == nil && ctx.Err() != nil {
		err = ctx.Err()
	}
	return stdout.Bytes(), stderr.Bytes(), err
}

// gitGrep returns every fixed-string match of patterns in the base tree
// under the pathspecs; none is an empty list.
func gitGrep(ctx context.Context, root, base string, patterns, pathspecs []string) ([]grepLine, error) {
	if len(patterns) == 0 {
		return nil, nil
	}
	args := []string{"-C", root, "grep", "-I", "-n", "-z", "--no-color", "-F"}
	for _, pattern := range patterns {
		args = append(args, "-e", pattern)
	}
	args = append(append(args, base, "--"), pathspecs...)
	data, stderr, err := gitOutput(ctx, nil, args...)
	if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 1 && len(stderr) == 0 {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("git grep: %v: %s", err, strings.TrimSpace(string(stderr)))
	}
	var out []grepLine
	prefix := base + ":"
	for _, record := range bytes.Split(data, []byte{'\n'}) {
		parts := bytes.SplitN(record, []byte{0}, 3)
		if len(parts) != 3 {
			continue
		}
		line, err := strconv.ParseInt(string(parts[1]), 10, 64)
		if err != nil {
			continue
		}
		out = append(out, grepLine{path: strings.TrimPrefix(string(parts[0]), prefix), line: line, content: parts[2]})
	}
	return out, nil
}

// catFile reads every named object that exists through one git cat-file
// --batch; a missing object is absent from the result.
func catFile(ctx context.Context, root string, objects []string) (map[string][]byte, error) {
	out := map[string][]byte{}
	if len(objects) == 0 {
		return out, nil
	}
	data, stderr, err := gitOutput(ctx, strings.NewReader(strings.Join(objects, "\n")+"\n"), "-C", root, "cat-file", "--batch")
	if err != nil {
		return nil, fmt.Errorf("git cat-file: %v: %s", err, strings.TrimSpace(string(stderr)))
	}
	for _, object := range objects {
		k := bytes.IndexByte(data, '\n')
		if k < 0 {
			return nil, errors.New("git cat-file: truncated output")
		}
		line := string(data[:k])
		data = data[k+1:]
		if strings.HasSuffix(line, " missing") || strings.HasSuffix(line, " ambiguous") {
			continue
		}
		header := strings.Fields(line)
		if len(header) != 3 {
			return nil, errors.New("git cat-file: malformed header")
		}
		size, err := strconv.Atoi(header[2])
		if err != nil || size+1 > len(data) {
			return nil, errors.New("git cat-file: malformed output")
		}
		if header[1] == "blob" {
			out[object] = data[:size]
		}
		data = data[size+1:]
	}
	return out, nil
}

// withArms returns the rows whose arms were scored.
func withArms(rows []row) []*armsRow {
	var out []*armsRow
	for _, r := range rows {
		if r.arms != nil && r.arms.arms != nil {
			out = append(out, r.arms)
		}
	}
	return out
}

// share is part/whole rounded to six places, or null for an empty whole.
func share(part, whole int) any {
	if whole == 0 {
		return nil
	}
	return round(float64(part)/float64(whole), 6)
}

// selfCitation totals the authored self-citation members (DCW-V0-051) over
// the maps with arms; hunk shares are over supported hunks, base shares over
// supported hunks' distinct bases.
func selfCitation(rows []row) ordered {
	var maps, samePathCitations, hunks, bases, samePath, adjacent, samePathHunks, adjacentHunks int
	for _, r := range rows {
		if r.arms == nil || r.arms.arms == nil {
			continue
		}
		maps++
		samePathCitations += number(r.citation["samePathCitations"])
		authored := r.arms.arms[armAuthored]
		hunks += r.arms.hunks
		bases += authored.bases
		samePath += authored.samePath
		adjacent += authored.adjacent
		samePathHunks += authored.samePathHunks
		adjacentHunks += authored.adjacentHunks
	}
	return ordered{
		{"maps", maps},
		{"supportedHunks", hunks},
		{"bases", bases},
		{"samePathCitations", samePathCitations},
		{"samePathBases", samePath},
		{"diffAdjacentBases", adjacent},
		{"samePathHunks", samePathHunks},
		{"diffAdjacentHunks", adjacentHunks},
		{"samePathBaseShare", share(samePath, bases)},
		{"diffAdjacentBaseShare", share(adjacent, bases)},
		{"samePathHunkShare", share(samePathHunks, hunks)},
		{"diffAdjacentHunkShare", share(adjacentHunks, hunks)},
	}
}

// armsSummary pools each arm over the maps with arms (DCW-V0-055). The
// pooled excess is the ratio estimator R = sum(excess_m) / sum(n_m) over the
// maps with a null (n_m >= 2 supported hunks); its 95% interval is the normal
// approximation R +- 1.96 * sqrt(k/(k-1) * sum((excess_m - R*n_m)^2)) /
// sum(n_m) over those k maps, null when k < 2.
func armsSummary(rows []row) ordered {
	scored := withArms(rows)
	failed := 0
	for _, r := range rows {
		if r.arms != nil && r.arms.arms == nil {
			failed++
		}
	}
	out := ordered{{"maps", len(scored)}, {"armsIssueMaps", failed}}
	nullMaps := 0
	for _, a := range scored {
		if a.arms[armAuthored].null != nil {
			nullMaps++
		}
	}
	out = append(out, member{"nullMaps", nullMaps})
	for _, name := range armNames {
		var hunks, anchored, anchoredExcl, bases, withBases int
		var full, excl [][2]float64
		for _, a := range scored {
			arm := a.arms[name]
			hunks += a.hunks
			anchored += arm.anchored
			anchoredExcl += arm.anchoredExcl
			bases += arm.bases
			withBases += arm.hunksWithBases
			if arm.null != nil {
				full = append(full, [2]float64{float64(arm.anchored) - arm.null.mean, float64(a.hunks)})
				excl = append(excl, [2]float64{float64(arm.anchoredExcl) - arm.nullExcl.mean, float64(a.hunks)})
			}
		}
		out = append(out, member{name, ordered{
			{"hunks", hunks},
			{"hunksWithBases", withBases},
			{"bases", bases},
			{"anchored", anchored},
			{"anchoredShare", share(anchored, hunks)},
			{"anchoredExclAdjacent", anchoredExcl},
			{"anchoredExcess", pooled(full)},
			{"anchoredExcessExclAdjacent", pooled(excl)},
		}})
	}
	return out
}

// pooled is the ratio estimator and its interval over (excess, n) pairs.
func pooled(maps [][2]float64) ordered {
	var excess, hunks float64
	for _, m := range maps {
		excess += m[0]
		hunks += m[1]
	}
	out := ordered{{"maps", len(maps)}, {"hunks", int(hunks)}, {"excess", round(excess, 4)}}
	if hunks == 0 {
		return append(out, member{"pooled", nil}, member{"ci95", nil})
	}
	ratio := excess / hunks
	out = append(out, member{"pooled", round(ratio, 6)})
	if len(maps) < 2 {
		return append(out, member{"ci95", nil})
	}
	var squares float64
	for _, m := range maps {
		residual := m[0] - ratio*m[1]
		squares += residual * residual
	}
	k := float64(len(maps))
	half := 1.96 * math.Sqrt(k/(k-1)*squares) / hunks
	return append(out, member{"ci95", []any{round(ratio-half, 6), round(ratio+half, 6)}})
}

// killStatistic counts the maps with at least one supported hunk on which the
// grep bot anchors at least as many hunks as the authored map (DCW-V0-055).
func killStatistic(rows []row) ordered {
	compared, atLeast := 0, 0
	for _, a := range withArms(rows) {
		if a.hunks == 0 {
			continue
		}
		compared++
		if a.arms[armGrepbot].anchored >= a.arms[armAuthored].anchored {
			atLeast++
		}
	}
	return ordered{{"mapsCompared", compared}, {"grepbotAtLeastAuthored", atLeast}, {"share", share(atLeast, compared)}}
}
