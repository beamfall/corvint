package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// The ci mode is the portable CI verifier (CEM-PILOT-020..023). It derives the exact base-to-head
// patch itself, reads the map from the head tree as data, delegates every structural and drift
// check to verify, and writes one fixed-schema report that carries no source or diff content.
// The verify mode and its adapter ABI (CEM-GO-002) are unchanged.

const (
	ciSchema         = "cem-ci-report/0"
	ciDefaultMapPath = ".corvint/change.cem.json"
	// maxCIReportBytes bounds one report line. The largest report holds maxEvidence drift items,
	// each at most about 1.3 KiB: a 512-byte path that JSON escaping can at most double, fixed-size
	// identifiers, digests and spans. 4096 items stay below 5.5 MiB (CEM-PILOT-021).
	maxCIReportBytes = 6 << 20
)

// ciExit is the exit status of each verdict (CEM-PILOT-022).
var ciExit = map[string]int{
	"accepted":            0,
	"rejected":            1,
	"operational":         2,
	"missing-evidence":    3,
	"unsupported-profile": 4,
	"repository-mismatch": 5,
}

// ciLimits is the fixed security-limitation statement every report carries.
var ciLimits = []string{"structural-integrity-only", "semantic-support-not-proven", "paths-and-digests-are-sensitive"}

type ciHunks struct {
	Supported  int `json:"supported"`
	Mechanical int `json:"mechanical"`
	Unknown    int `json:"unknown"`
}

type ciReport struct {
	Schema      string      `json:"schema"`
	Verdict     string      `json:"verdict"`
	Exit        int         `json:"exit"`
	Code        string      `json:"code"`
	Profile     string      `json:"profile"`
	Base        string      `json:"base"`
	Head        string      `json:"head"`
	MapPath     string      `json:"mapPath"`
	MapSHA256   string      `json:"mapSha256"`
	PatchSHA256 string      `json:"patchSha256"`
	Hunks       ciHunks     `json:"hunks"`
	Evidence    int         `json:"evidence"`
	Drift       []driftItem `json:"drift"`
	Limits      []string    `json:"limits"`
}

type ciArgs struct {
	repository string
	base       string
	head       string
	mapPath    string
}

// ciFailure is a verdict other than accepted together with its bounded reason code.
type ciFailure struct {
	verdict string
	code    string
}

func (r ciReport) finish(verdict, code string) ciReport {
	r.Verdict, r.Code, r.Exit = verdict, code, ciExit[verdict]
	return r
}

func fromCEMError(err *cemError) *ciFailure {
	if err.operational {
		return &ciFailure{verdict: "operational", code: err.code}
	}
	return &ciFailure{verdict: "rejected", code: err.code}
}

func runCI(argv []string) ciReport {
	r := ciReport{Schema: ciSchema, Profile: specVersion, Drift: []driftItem{}, Limits: ciLimits}
	a, ok := parseCIArgs(argv)
	if !ok {
		return r.finish("operational", "invocation")
	}
	r.Base, r.Head, r.MapPath = a.base, a.head, a.mapPath
	ctx, cancel := context.WithTimeout(context.Background(), verificationBudget)
	defer cancel()
	v := &verifier{ctx: ctx, repo: a.repository, blobs: map[string][]byte{}, trees: map[string]*treeEntry{}}
	mapBytes, failure := v.ciMap(a)
	if failure != nil {
		return r.finish(failure.verdict, failure.code)
	}
	r.MapSHA256 = shaHex(mapBytes)
	patch, failure := v.ciPatch(a)
	if failure != nil {
		return r.finish(failure.verdict, failure.code)
	}
	r.PatchSHA256 = shaHex(patch)
	drift, err := verifyCIInputs(a, mapBytes, patch)
	if err != nil {
		failure = fromCEMError(err)
		return r.finish(failure.verdict, failure.code)
	}
	var m cemMap
	_ = strictDecode(mapBytes, &m) // verify already accepted these exact bytes
	r.Hunks, r.Evidence, r.Drift = countHunks(m.Hunks), len(m.Evidence), drift
	return r.finish(ciConclusion(r))
}

func parseCIArgs(argv []string) (ciArgs, bool) {
	a := ciArgs{mapPath: ciDefaultMapPath}
	fields := map[string]*string{"--repository": &a.repository, "--base": &a.base, "--head": &a.head, "--map": &a.mapPath}
	seen := map[string]bool{}
	for i := 0; i < len(argv); i += 2 {
		field := fields[argv[i]]
		if field == nil || i+1 >= len(argv) || seen[argv[i]] {
			return ciArgs{}, false
		}
		seen[argv[i]], *field = true, argv[i+1]
	}
	valid := a.repository != "" && validOID(a.base) && validOID(a.head) && validCIMapPath(a.mapPath)
	return a, valid
}

// validCIMapPath admits the normalized map paths the documented patch profile can exclude with
// its plain pathspec, the same set examples/cem/verify-pr.sh admits.
func validCIMapPath(p string) bool {
	for _, c := range []byte(p) {
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == '/' || c == '-') {
			return false
		}
	}
	return validPath(p)
}

// ciMap checks the repository holds both declared commits, then reads the map from the head
// tree and classifies an absent map, an unsupported profile, and a base mismatch before verify.
func (v *verifier) ciMap(a ciArgs) ([]byte, *ciFailure) {
	if err := v.rejectAlternates(); err != nil {
		return nil, fromCEMError(err)
	}
	if failure := v.ciCommit(a.base, "base-unavailable"); failure != nil {
		return nil, failure
	}
	if failure := v.ciCommit(a.head, "head-unavailable"); failure != nil {
		return nil, failure
	}
	mapBytes, failure := v.ciMapBlob(a.head, a.mapPath)
	if failure != nil {
		return nil, failure
	}
	return mapBytes, ciMapHeader(mapBytes, a.base)
}

func (v *verifier) ciCommit(oid, missingCode string) *ciFailure {
	_, err := v.resolveCommit(oid)
	if err != nil && err.code == "repository-io" {
		return &ciFailure{verdict: "repository-mismatch", code: missingCode}
	}
	if err != nil {
		return fromCEMError(err)
	}
	return nil
}

func (v *verifier) ciMapBlob(head, mapPath string) ([]byte, *ciFailure) {
	entry, err := v.treeEntry(head, mapPath)
	if err != nil {
		return nil, fromCEMError(err)
	}
	if entry == nil {
		return nil, &ciFailure{verdict: "missing-evidence", code: "map-absent"}
	}
	if !regularFile(entry) {
		return nil, &ciFailure{verdict: "rejected", code: "map-entry"}
	}
	b, err := v.blob(entry.oid)
	if err != nil {
		return nil, fromCEMError(err)
	}
	if len(b) > maxJSONBytes {
		return nil, &ciFailure{verdict: "rejected", code: "resource"}
	}
	return b, nil
}

// ciMapHeader classifies an unsupported profile, then applies verify's structural checks, and only
// then compares the map base with the declared base, so a malformed map is rejected rather than
// reported as a repository mismatch.
func ciMapHeader(mapBytes []byte, base string) *ciFailure {
	if unsupportedSpec(mapBytes) {
		return &ciFailure{verdict: "unsupported-profile", code: "unsupported-profile"}
	}
	m, err := decodeMap(mapBytes)
	if err != nil {
		return fromCEMError(err)
	}
	if m.BaseRevision != base {
		return &ciFailure{verdict: "repository-mismatch", code: "base-revision-mismatch"}
	}
	return nil
}

// unsupportedSpec reports a strictly parsed JSON object whose one string spec member names a profile
// other than cem/0.1. Anything else is left to the structural checks, which reject it.
func unsupportedSpec(mapBytes []byte) bool {
	var header struct {
		Spec *string `json:"spec"`
	}
	if strictJSON(mapBytes) != nil || json.Unmarshal(mapBytes, &header) != nil {
		return false
	}
	return header.Spec != nil && *header.Spec != specVersion
}

// ciPatch derives the patch with the documented CI profile (docs/CEM-CI.md), excluding only the
// map path, and bounds it to the verifier's patch limit.
func (v *verifier) ciPatch(a ciArgs) ([]byte, *ciFailure) {
	out := &cappedBuffer{limit: maxPatchBytes}
	err := v.gitTo(out, nil, "-c", "core.quotePath=false", "-c", "diff.algorithm=myers", "-c", "diff.context=3",
		"diff", "--binary", "--full-index", "--no-color", "--no-ext-diff", "--no-textconv", "--no-renames",
		"--no-indent-heuristic", "--diff-algorithm=myers", "--unified=3",
		"--src-prefix=a/", "--dst-prefix=b/", "--ignore-submodules=none",
		a.base, a.head, "--", ".", ":(exclude)"+a.mapPath)
	if out.over {
		return nil, &ciFailure{verdict: "rejected", code: "resource"}
	}
	if err != nil {
		return nil, fromCEMError(err)
	}
	return out.Bytes(), nil
}

// verifyCIInputs hands the map and patch bytes to verify through private temporary files.
func verifyCIInputs(a ciArgs, mapBytes, patch []byte) ([]driftItem, *cemError) {
	dir, err := os.MkdirTemp("", "cem-ci-")
	if err != nil {
		return nil, operational("temporary-io")
	}
	defer os.RemoveAll(dir)
	mapFile, patchFile := filepath.Join(dir, "change.cem.json"), filepath.Join(dir, "change.patch")
	if os.WriteFile(mapFile, mapBytes, 0o600) != nil || os.WriteFile(patchFile, patch, 0o600) != nil {
		return nil, operational("temporary-io")
	}
	return verify(cliArgs{repository: a.repository, mapFile: mapFile, patchFile: patchFile, target: a.head, targetSet: true})
}

func countHunks(hunks []mappedHunk) ciHunks {
	var c ciHunks
	for _, h := range hunks {
		c.Supported += boolCount(h.Disposition == "supported")
		c.Mechanical += boolCount(h.Disposition == "mechanical")
		c.Unknown += boolCount(h.Disposition == "unknown")
	}
	return c
}

func boolCount(b bool) int {
	if b {
		return 1
	}
	return 0
}

// ciConclusion ranks a structurally valid map: unsafe drift rejects, then any unknown hunk is
// missing evidence, and only then is the change accepted.
func ciConclusion(r ciReport) (string, string) {
	for _, item := range r.Drift {
		if item.Status != "stable" && item.Status != "relocated" {
			return "rejected", "unsafe-drift"
		}
	}
	if r.Hunks.Unknown > 0 {
		return "missing-evidence", "unknown-hunks"
	}
	return "accepted", "accepted"
}

// encodeCIReport renders one report line without HTML escaping, so the size bound holds.
func encodeCIReport(r ciReport) []byte {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(r) // every member is a string, integer, or slice of them
	return b.Bytes()
}

func writeCIReport(r ciReport) {
	_, _ = fmt.Fprint(os.Stdout, string(encodeCIReport(r)))
	os.Exit(r.Exit)
}

type cappedBuffer struct {
	bytes.Buffer
	limit int
	over  bool
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	if c.Len()+len(p) > c.limit {
		c.over = true
		return 0, errSizeLimit
	}
	return c.Buffer.Write(p)
}
