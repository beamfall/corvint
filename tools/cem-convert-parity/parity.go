package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/convert"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// Profile names the closed row and summary shapes.
const Profile = "cem-convert-parity/1"

// Classes are the CEM-V1-104 explanation classes plus NOT_OBSERVED and different.
var Classes = []string{"identical", "empty-member-explained", "unavailable-objects", "pre-existing-refusal", "native-stable", "NOT_OBSERVED", "different"}

// Options.Migrated selects the V1-1277 migration check: every committed map
// must already be cem/1.0, a converted map must equal the converter's output
// over its provenance-named source blob byte for byte, and the parity run then
// compares that committed map against its source.
type Options struct {
	Repository, Maps, Portable, ReachableRevs, Out string
	Jobs, Every                                    int
	Migrated                                       bool
}

// Verdict is the compared projection of one consumer result.
type Verdict struct {
	Outcome            string   `json:"outcome"`
	Stage              string   `json:"stage"`
	Code               string   `json:"code"`
	ChangeIntegrity    string   `json:"changeIntegrity"`
	ReferenceIntegrity string   `json:"referenceIntegrity"`
	Drift              []string `json:"drift"`
	HunksEqual         bool     `json:"hunksEqual"`
	VerdictSha256      string   `json:"verdictSha256"`
}

type Row struct {
	Profile              string  `json:"profile"`
	Map                  string  `json:"map"`
	Committed            string  `json:"committed,omitempty"`
	CommittedEqual       bool    `json:"committedEqualsConverted,omitempty"`
	SourceSpec           string  `json:"sourceSpec"`
	SourceBlobOid        string  `json:"sourceBlobOid"`
	BindPresent          bool    `json:"bindPresent"`
	BindReachable        bool    `json:"bindReachable"`
	SourceBinding        string  `json:"sourceBinding"`
	ConvertCode          string  `json:"convertCode"`
	ConvertedSha256      string  `json:"convertedSha256"`
	Idempotent           bool    `json:"idempotent"`
	ReconvertCode        string  `json:"reconvertCode"`
	SubstitutedTarget    string  `json:"substitutedTarget"`
	Source               Verdict `json:"source"`
	Native               Verdict `json:"native"`
	Portable             Verdict `json:"portable"`
	NativeEqualsPortable bool    `json:"nativeEqualsPortable"`
	DirectNative         Verdict `json:"directNative"`
	DirectPortable       Verdict `json:"directPortable"`
	DirectEqual          bool    `json:"directNativeEqualsPortable"`
	Class                string  `json:"class"`
	Reason               string  `json:"reason"`
}

// Run writes parity.jsonl and summary.json under o.Out and returns the summary.
func Run(o Options) ([]byte, error) {
	if o.Jobs < 1 {
		o.Jobs = 1
	}
	if o.Every < 1 {
		o.Every = 1
	}
	names, err := mapNames(o.Maps)
	if err != nil {
		return nil, err
	}
	var sample []string
	for i, n := range names {
		if i%o.Every == 0 {
			sample = append(sample, n)
		}
	}
	reachable := map[string]bool{}
	if o.ReachableRevs != "" {
		raw, err := os.ReadFile(o.ReachableRevs)
		if err != nil {
			return nil, err
		}
		for _, line := range strings.Fields(string(raw)) {
			reachable[line] = true
		}
	}
	work, err := os.MkdirTemp("", "cem-convert-parity-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(work)
	artifacts := filepath.Join(work, "artifacts")
	if err := os.Mkdir(artifacts, 0o755); err != nil {
		return nil, err
	}
	rows := make([]Row, len(sample))
	var wg sync.WaitGroup
	next := make(chan int)
	for j := 0; j < o.Jobs; j++ {
		wg.Add(1)
		go func(job int) {
			defer wg.Done()
			for i := range next {
				rows[i] = runOne(o, sample[i], reachable, filepath.Join(work, fmt.Sprintf("job-%d", job)), artifacts)
				fmt.Fprintf(os.Stderr, "%s %s\n", rows[i].Map[:12], rows[i].Class)
			}
		}(j)
	}
	for i := range sample {
		next <- i
	}
	close(next)
	wg.Wait()
	if err := os.MkdirAll(o.Out, 0o755); err != nil {
		return nil, err
	}
	var jsonl bytes.Buffer
	for _, r := range rows {
		b, err := json.Marshal(r)
		if err != nil {
			return nil, err
		}
		jsonl.Write(b)
		jsonl.WriteByte('\n')
	}
	if err := os.WriteFile(filepath.Join(o.Out, "parity.jsonl"), jsonl.Bytes(), 0o644); err != nil {
		return nil, err
	}
	if o.Migrated {
		if err := os.WriteFile(filepath.Join(o.Out, "targets.txt"), targets(rows), 0o644); err != nil {
			return nil, err
		}
	}
	summary, err := json.MarshalIndent(summarize(rows, len(names), o.Every, sha256Hex(jsonl.Bytes())), "", "  ")
	if err != nil {
		return nil, err
	}
	summary = append(summary, '\n')
	return summary, os.WriteFile(filepath.Join(o.Out, "summary.json"), summary, 0o644)
}

func mapNames(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.Type().IsRegular() && convert.BindCommitFromName(e.Name()) != "" {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

func runOne(o Options, name string, reachable map[string]bool, work, artifacts string) Row {
	ctx := context.Background()
	bind := convert.BindCommitFromName(name)
	row := Row{Profile: Profile, Map: bind, BindReachable: reachable[bind]}
	src, err := os.ReadFile(filepath.Join(o.Maps, name))
	if err != nil {
		row.Class, row.Reason = "different", "source-unreadable"
		return row
	}
	var committed []byte
	if o.Migrated {
		committed = src
		var reason string
		if src, reason = migratedSource(ctx, o.Repository, bind, committed, &row); reason != "" {
			row.Class, row.Reason = "different", reason
			return row
		}
		if src == nil {
			return nativeStableRow(ctx, o, row, committed, bind, work, artifacts)
		}
	}
	row.SourceBlobOid = convert.BlobOid(src)
	row.BindPresent = git(ctx, o.Repository, nil, nil, "cat-file", "-e", bind+"^{commit}") == nil
	converted, report, err := convert.Convert(src, convert.Inputs{BindCommit: bind, Git: convert.NewGitReader(ctx, o.Repository)})
	row.SourceBinding = report.SourceBinding
	if report.SourceSpec != nil {
		row.SourceSpec = *report.SourceSpec
	}
	if err != nil {
		row.ConvertCode = cemcode.CodeOf(err)
		row.Class, row.Reason = "different", "converter-refused"
		return row
	}
	row.ConvertedSha256 = sha256Hex(converted)
	if o.Migrated {
		if row.CommittedEqual = bytes.Equal(committed, converted); !row.CommittedEqual {
			row.Class, row.Reason = "different", "committed-differs-from-converter"
			return row
		}
	}
	again, _, err := convert.Convert(src, convert.Inputs{BindCommit: bind})
	row.Idempotent = err == nil && bytes.Equal(again, converted)
	if _, _, err := convert.Convert(converted, convert.Inputs{BindCommit: bind}); err != nil {
		row.ReconvertCode = cemcode.CodeOf(err)
	}
	if err := os.MkdirAll(work, 0o755); err != nil {
		row.Class, row.Reason = "different", "work-directory"
		return row
	}
	mapPath := filepath.Join(work, "converted.cem.json")
	if err := os.WriteFile(mapPath, converted, 0o644); err != nil {
		row.Class, row.Reason = "different", "work-directory"
		return row
	}
	expected := expectedHunks(src)
	row.Source = sourceVerdict(ctx, o.Repository, src, bind)
	run := func(target string) (Verdict, Verdict, bool) {
		nativeRaw := nativeStable(ctx, o.Repository, converted, row.ConvertedBase(src), target, artifacts)
		portableRaw := portableStable(ctx, o.Portable, o.Repository, mapPath, row.ConvertedBase(src), target, artifacts)
		return stableVerdict(nativeRaw, expected), stableVerdict(portableRaw, expected), jsonEqual(nativeRaw, portableRaw)
	}
	row.DirectNative, row.DirectPortable, row.DirectEqual = run(bind)
	if !row.BindPresent {
		row.Class, row.Reason = "NOT_OBSERVED", "bind-commit-absent"
		return row
	}
	target, err := substitute(ctx, o.Repository, filepath.Join(work, "index"), bind, converted)
	if err != nil {
		row.Class, row.Reason = "different", "substitution-failed"
		return row
	}
	row.SubstitutedTarget = target
	row.Native, row.Portable, row.NativeEqualsPortable = run(target)
	row.Class, row.Reason = classify(row)
	return row
}

// migratedSource returns the provenance-named source blob of a committed
// cem/1.0 map, or nil with no reason for a natively produced stable map that
// carries no provenance. A nonempty reason fails the row.
func migratedSource(ctx context.Context, repository, bind string, committed []byte, row *Row) ([]byte, string) {
	if !wire.DeclaresStable(committed) {
		return nil, "not-migrated"
	}
	doc, err := wire.ParseStable(committed)
	if doc == nil {
		row.ConvertCode = cemcode.CodeOf(err)
		return nil, "committed-unparseable"
	}
	p := doc.Provenance
	if p == nil {
		row.Committed = "stable-native"
		return nil, ""
	}
	row.Committed = "migrated"
	if p.BindCommit != bind || p.Converter != convert.Converter {
		return nil, "provenance-mismatch"
	}
	var blob bytes.Buffer
	if err := gitOut(ctx, repository, nil, nil, &blob, "cat-file", "blob", p.SourceBlobOid); err != nil {
		return nil, "source-blob-unavailable"
	}
	src := blob.Bytes()
	if convert.BlobOid(src) != p.SourceBlobOid || sha256Hex(src) != p.SourceSha256 {
		return nil, "source-digest-mismatch"
	}
	return src, ""
}

// nativeStableRow verifies a natively produced stable map where it was bound:
// both stable consumers at the bind commit, whose sidecar holds these bytes.
func nativeStableRow(ctx context.Context, o Options, row Row, committed []byte, bind, work, artifacts string) Row {
	row.SourceBlobOid, row.ConvertedSha256 = convert.BlobOid(committed), sha256Hex(committed)
	if _, _, err := convert.Convert(committed, convert.Inputs{BindCommit: bind}); err != nil {
		row.ReconvertCode = cemcode.CodeOf(err)
	}
	if err := os.MkdirAll(work, 0o755); err != nil {
		row.Class, row.Reason = "different", "work-directory"
		return row
	}
	mapPath := filepath.Join(work, "committed.cem.json")
	if err := os.WriteFile(mapPath, committed, 0o644); err != nil {
		row.Class, row.Reason = "different", "work-directory"
		return row
	}
	doc, _ := wire.ParseStable(committed)
	base, expected := doc.Change.BaseRevision, expectedHunks(committed)
	nativeRaw := nativeStable(ctx, o.Repository, committed, base, bind, artifacts)
	portableRaw := portableStable(ctx, o.Portable, o.Repository, mapPath, base, bind, artifacts)
	row.DirectNative, row.DirectPortable, row.DirectEqual = stableVerdict(nativeRaw, expected), stableVerdict(portableRaw, expected), jsonEqual(nativeRaw, portableRaw)
	row.Native, row.Portable, row.NativeEqualsPortable = row.DirectNative, row.DirectPortable, row.DirectEqual
	row.SubstitutedTarget = bind
	switch {
	case row.ReconvertCode != convert.CodeSourceIsStable:
		row.Class, row.Reason = "different", "reconvert-not-refused"
	case !row.DirectEqual:
		row.Class, row.Reason = "different", "native-portable-differ"
	default:
		row.Class, row.Reason = "native-stable", row.DirectNative.Outcome
	}
	return row
}

// targets lists "<bind> <target>" for every row with a verification target:
// the CEM-V1-104 substituted target for a converted map, the bind commit for a
// natively produced stable map. cem-citation-baseline --targets reads it.
func targets(rows []Row) []byte {
	var b bytes.Buffer
	for _, r := range rows {
		if r.SubstitutedTarget != "" {
			fmt.Fprintf(&b, "%s %s\n", r.Map, r.SubstitutedTarget)
		}
	}
	return b.Bytes()
}

// ConvertedBase is the base revision the source map pins; the stable consumers
// take it as the independent expected base, as the 0.x verifier does.
func (r Row) ConvertedBase(src []byte) string {
	m, err := wire.ParseMap(src)
	if err != nil {
		return ""
	}
	return m.BaseRevision
}

func classify(r Row) (string, string) {
	n, p, s := r.Native, r.Portable, r.Source
	switch {
	case !r.Idempotent:
		return "different", "not-idempotent"
	case r.ReconvertCode != convert.CodeSourceIsStable:
		return "different", "reconvert-not-refused"
	case !r.NativeEqualsPortable:
		// Both consumers may agree with the source refusal and with each other
		// on outcome, stage, code and hunks while their full results differ:
		// the observed case is the partial drift list a resource-limit abort
		// carries. The row keeps nativeEqualsPortable=false and the summary
		// lists it under nativePortableDiffer.
		if s.Outcome == "UNSUPPORTED" && n.Outcome == s.Outcome && p.Outcome == s.Outcome && n.Code == s.Code && p.Code == s.Code &&
			n.Stage == s.Stage && p.Stage == s.Stage && n.ChangeIntegrity == p.ChangeIntegrity && n.HunksEqual && p.HunksEqual {
			if reflect.DeepEqual(n.Drift, p.Drift) {
				return "pre-existing-refusal", s.Code + " consumer-result-differs"
			}
			return "pre-existing-refusal", s.Code + " consumer-partial-drift-differs"
		}
		return "different", "native-portable-differ"
	}
	if s.Outcome == "UNSUPPORTED" {
		// CEM-V1-104: a pre-existing refusal repeats the source's code, stage
		// and hunks on both consumers (equal to each other here).
		if n.Outcome == "UNSUPPORTED" && n.Code == s.Code {
			if s.Code == cemcode.RepositoryObjectUnavailable {
				return "unavailable-objects", s.Code
			}
			if n.Stage == s.Stage && n.HunksEqual {
				return "pre-existing-refusal", s.Code
			}
		}
		return "different", "refusal-differs"
	}
	if n.Outcome == s.Outcome && n.Code == s.Code && n.ChangeIntegrity == s.ChangeIntegrity && reflect.DeepEqual(n.Drift, s.Drift) && n.HunksEqual {
		return "identical", ""
	}
	if s.Outcome == "ACCEPT" && (n.Stage == "references" || n.Stage == "artifacts") && n.ChangeIntegrity == s.ChangeIntegrity {
		return "empty-member-explained", n.Code
	}
	return "different", "verdict-differs"
}

// refusalCode maps a 0.x verifier code to the stable consumers' refusal code,
// or "" when the code is a verification failure (REJECT).
func refusalCode(code string) string {
	switch code {
	case "git-output-exceeded", "git-budget-exceeded":
		return "unsupported-resource-limit"
	case "git-start-failed", "git-exit-failure", "git-read-failed":
		return "git-read-failed"
	case "git-diff-timeout", "git-timeout":
		return "git-timeout"
	case "unsupported-repository-attributes":
		return "unsupported-repository-envelope"
	case "repository-object-unavailable", "unsupported-object-alternates", "git-diff-failed", "git-cancelled":
		return code
	}
	return ""
}

func sourceVerdict(ctx context.Context, repository string, src []byte, bind string) Verdict {
	v := Verdict{Outcome: "UNSUPPORTED", Stage: "repository", ChangeIntegrity: "NOT_OBSERVED", ReferenceIntegrity: "NOT_APPLICABLE", Drift: []string{}, HunksEqual: true}
	repo, err := gitauth.Open(repository, gitrun.NewDefaultBudget())
	if err != nil {
		v.Code = refusalOr(cemcode.CodeOf(err))
		return v
	}
	doc, err := wire.ParseMap(src)
	if err != nil {
		v.Outcome, v.Stage, v.Code = "REJECT", "wire", cemcode.CodeOf(err)
		return v
	}
	outcome, _, err := verify.Canonical(ctx, repo, doc, verify.CanonicalOptions{ExpectedBase: doc.BaseRevision, Target: bind, RawMapBytes: src})
	if outcome != nil {
		for _, d := range outcome.Drift {
			v.Drift = append(v.Drift, d.EvidenceID+" "+string(d.Status))
		}
		sort.Strings(v.Drift)
	}
	if err == nil {
		v.Outcome, v.Stage, v.ChangeIntegrity = "ACCEPT", "complete", "VERIFIED"
		return v
	}
	code := cemcode.CodeOf(err)
	if r := refusalCode(code); r != "" {
		v.Code = r
		return v
	}
	v.Outcome, v.Stage, v.Code, v.ChangeIntegrity = "REJECT", "verification", code, "FAILED"
	return v
}

func refusalOr(code string) string {
	if r := refusalCode(code); r != "" {
		return r
	}
	return code
}

func nativeStable(ctx context.Context, repository string, converted []byte, base, target, artifacts string) []byte {
	r, _ := verify.Stable(ctx, converted, verify.StableOptions{Repository: repository, ExpectedBase: base, Target: target, ArtifactRoot: artifacts})
	b, err := json.Marshal(r)
	if err != nil {
		return nil
	}
	return b
}

func portableStable(ctx context.Context, exe, repository, mapPath, base, target, artifacts string) []byte {
	cmd := exec.CommandContext(ctx, exe, "verify-stable-canonical", "--repository", repository, "--map", mapPath, "--expected-base", base, "--target", target, "--artifacts", artifacts)
	out, _ := cmd.Output()
	return out
}

func stableVerdict(raw []byte, expected any) Verdict {
	v := Verdict{Outcome: "NO_RESULT", Drift: []string{}}
	var r struct {
		Outcome    string            `json:"outcome"`
		Stage      string            `json:"stage"`
		Code       *string           `json:"code"`
		IssueCodes []string          `json:"issueCodes"`
		Axes       map[string]string `json:"axes"`
		Drift      []map[string]any  `json:"drift"`
		Hunks      []any             `json:"hunks"`
		Verdict    string            `json:"verdictSha256"`
		References map[string][]any  `json:"references"`
	}
	if json.Unmarshal(raw, &r) != nil {
		return v
	}
	v.Outcome, v.Stage, v.VerdictSha256 = r.Outcome, r.Stage, r.Verdict
	if r.Code != nil {
		v.Code = *r.Code
	} else if len(r.IssueCodes) > 0 {
		v.Code = strings.Join(r.IssueCodes, ",")
	}
	v.ChangeIntegrity, v.ReferenceIntegrity = r.Axes["changeIntegrity"], r.Axes["referenceIntegrity"]
	for _, d := range r.Drift {
		v.Drift = append(v.Drift, fmt.Sprint(d["evidenceId"])+" "+fmt.Sprint(d["status"]))
	}
	sort.Strings(v.Drift)
	v.HunksEqual = len(r.Hunks) == 0 && v.Outcome == "UNSUPPORTED" || reflect.DeepEqual(any(r.Hunks), expected)
	return v
}

// expectedHunks is the source map's hunks array as generic JSON: disposition,
// reason, basis, ranges, ids and optional witnesses compared together.
func expectedHunks(src []byte) any {
	var m struct {
		Hunks []any `json:"hunks"`
	}
	if json.Unmarshal(src, &m) != nil {
		return nil
	}
	return any(m.Hunks)
}

func jsonEqual(a, b []byte) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	return reflect.DeepEqual(x, y)
}

// substitute writes, into the scratch clone only, the commit whose tree is the
// bind commit's tree with only the excluded sidecar replaced by the converted
// bytes. Fixed identity and dates make it a function of its inputs.
func substitute(ctx context.Context, repository, index, bind string, converted []byte) (string, error) {
	os.Remove(index)
	env := []string{"GIT_INDEX_FILE=" + index}
	if err := git(ctx, repository, env, nil, "read-tree", bind+"^{tree}"); err != nil {
		return "", err
	}
	var blob bytes.Buffer
	if err := gitOut(ctx, repository, env, converted, &blob, "hash-object", "-w", "--stdin"); err != nil {
		return "", err
	}
	if err := git(ctx, repository, env, nil, "update-index", "--cacheinfo", "100644,"+strings.TrimSpace(blob.String())+","+wire.ExcludedCEMPath); err != nil {
		return "", err
	}
	var tree, commit bytes.Buffer
	if err := gitOut(ctx, repository, env, nil, &tree, "write-tree"); err != nil {
		return "", err
	}
	identity := []string{"GIT_AUTHOR_NAME=cem-convert-parity", "GIT_AUTHOR_EMAIL=parity@example.invalid", "GIT_COMMITTER_NAME=cem-convert-parity", "GIT_COMMITTER_EMAIL=parity@example.invalid", "GIT_AUTHOR_DATE=2026-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z"}
	if err := gitOut(ctx, repository, append(env, identity...), nil, &commit, "commit-tree", strings.TrimSpace(tree.String()), "-p", bind, "-m", "cem-convert-parity substituted sidecar "+bind); err != nil {
		return "", err
	}
	return strings.TrimSpace(commit.String()), nil
}

func git(ctx context.Context, repository string, env []string, stdin []byte, args ...string) error {
	return gitOut(ctx, repository, env, stdin, nil, args...)
}

func gitOut(ctx context.Context, repository string, env []string, stdin []byte, stdout *bytes.Buffer, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", repository}, args...)...)
	cmd.Env = append(os.Environ(), env...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	if stdout != nil {
		cmd.Stdout = stdout
	}
	return cmd.Run()
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

type Summary struct {
	Profile             string         `json:"profile"`
	Converter           string         `json:"converter"`
	GoVersion           string         `json:"goVersion"`
	CorpusMaps          int            `json:"corpusMaps"`
	SampleEvery         int            `json:"sampleEvery"`
	Rows                int            `json:"rows"`
	ParitySha256        string         `json:"paritySha256"`
	Classes             map[string]int `json:"classes"`
	Idempotent          int            `json:"idempotent"`
	ReconvertCodes      map[string]int `json:"reconvertCodes"`
	SourceBindings      map[string]int `json:"sourceBindings"`
	BindReachable       int            `json:"bindReachable"`
	SourceOutcomes      map[string]int `json:"sourceOutcomes"`
	NativeOutcomes      map[string]int `json:"substitutedNativeOutcomes"`
	NativeEqualPortable int            `json:"substitutedNativeEqualsPortable"`
	DirectNative        map[string]int `json:"directNativeOutcomes"`
	DirectPortable      map[string]int `json:"directPortableOutcomes"`
	DirectEqual         int            `json:"directNativeEqualsPortable"`
	Committed           map[string]int `json:"committed,omitempty"`
	CommittedEqual      int            `json:"committedEqualsConverted,omitempty"`
	Different           []string       `json:"different"`
	NativePortable      []string       `json:"nativePortableDiffer"`
}

func summarize(rows []Row, corpus, every int, digest string) Summary {
	s := Summary{Profile: Profile + "-summary", Converter: convert.Converter, GoVersion: runtime.Version(), CorpusMaps: corpus, SampleEvery: every, Rows: len(rows), ParitySha256: digest,
		Classes: map[string]int{}, ReconvertCodes: map[string]int{}, SourceBindings: map[string]int{}, SourceOutcomes: map[string]int{}, NativeOutcomes: map[string]int{}, DirectNative: map[string]int{}, DirectPortable: map[string]int{}, Different: []string{}, NativePortable: []string{}}
	for _, c := range Classes {
		s.Classes[c] = 0
	}
	key := func(v Verdict) string { return strings.TrimSuffix(v.Outcome+" "+v.Code, " ") }
	for _, r := range rows {
		s.Classes[r.Class]++
		if r.Committed != "" {
			if s.Committed == nil {
				s.Committed = map[string]int{}
			}
			s.Committed[r.Committed]++
		}
		if r.CommittedEqual {
			s.CommittedEqual++
		}
		if r.Idempotent {
			s.Idempotent++
		}
		if r.BindReachable {
			s.BindReachable++
		}
		if r.NativeEqualsPortable {
			s.NativeEqualPortable++
		} else {
			s.NativePortable = append(s.NativePortable, r.Map+" "+r.Reason)
		}
		if r.DirectEqual {
			s.DirectEqual++
		}
		s.ReconvertCodes[r.ReconvertCode]++
		s.SourceBindings[r.SourceBinding]++
		s.SourceOutcomes[key(r.Source)]++
		s.NativeOutcomes[key(r.Native)]++
		s.DirectNative[key(r.DirectNative)]++
		s.DirectPortable[key(r.DirectPortable)]++
		if r.Class == "different" {
			s.Different = append(s.Different, r.Map+" "+r.Reason)
		}
	}
	return s
}
