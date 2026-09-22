// Copyright 2026 Russell Lewis
// Licensed under the Apache License, Version 2.0.

package main

// The fixture runner. It materializes a case's declared universe as a REAL
// temporary Git repository — real commits, a real `cem/0.2` sidecar produced by
// the real CEM two-phase workflow, a real OCM artifact bound to that map and
// patch, a real intent scope read out of a committed Markdown blob — and then
// invokes `frontier.Compute` over it through `internal/frontierrepo`.
//
// Nothing is stubbed, and deliberately so: a fixture that passed against a
// double would prove only that the double agreed with the fixture. The
// consequence is that some declared universes cannot be built at all, which is
// what the capability mechanism in capabilities.go is for.
//
// The fixture's identifiers are SYMBOLS. A suite authored before the producer
// existed cannot name a content-addressed hunk, evidence, or claim ID, so it
// names `hunk:sha256:0101…` and this runner reports, in Outcome.Symbols, what
// it bound that symbol to. Obligation IDs are not symbolic: they are
// caller-authored, they satisfy the OCM requirement grammar as written, and
// they appear verbatim in the committed intent scope.

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
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/workflow"
	"github.com/Beamfall/corvint/internal/frontier"
	"github.com/Beamfall/corvint/internal/frontierrepo"
)

func init() { RegisterRunner(repoRunner{}) }

type repoRunner struct{}

// Supports declares what this runner can build. Everything else is a real
// limit of a real-repository harness, named by CapabilityReason rather than
// hidden behind a blanket skip.
func (repoRunner) Supports(capability Capability) bool {
	return capability == CapCandidateDocument || capability == CapDynamicTestTuple ||
		capability == CapDeclaredItemCounts
}

func (repoRunner) Run(f Fixture, c Case) (Outcome, error) {
	if f.ID == "noncanonical-frontier-refusals" {
		return runCandidateDocument(c)
	}
	return runUniverse(c)
}

// Vocabulary for the generated repository. Each hunk and each evidence file
// gets a distinct basename stem and a distinct body marker, because LRF's
// subject terms are the union of body terms and the basename stem: two files
// that shared a stem would silently intersect and turn a declared rejection
// into a qualifying basis.
var (
	hunkWords     = []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel"}
	evidenceWords = []string{"kilo", "lima", "mike", "november", "oscar", "papa", "quebec", "romeo"}
)

// unrelatedBody is evidence text sharing no term with any generated hunk. It is
// what makes `insufficient-lexical-support` a real rejection rather than an
// accident of wording.
const unrelatedBody = "zulu victor whiskey xraytango yankeeuniform\n"

type builtUniverse struct {
	root    string
	git     gitRunner
	base    string
	target  string
	cemRaw  []byte
	ocmRaw  []byte
	symbols map[string]string
	// testRows are the execution identities of this universe's selected claim
	// edges, in claim order. A complete dynamic tuple's report has to name the
	// identity TCQ derives from the committed claims blob, and only the runner
	// that generated that blob knows what it is.
	testRows []testRow
	// mismatchedTarget resolves in this repository but is not the revision the
	// OCM artifact binds. It is empty unless the case declares that state.
	mismatchedTarget string
}

func runUniverse(c Case) (Outcome, error) {
	built, cleanup, err := buildUniverse(c)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return Outcome{}, err
	}
	outcome, err := built.compute(c)
	if err != nil {
		return Outcome{}, err
	}
	if c.Declared.Repeat == "" {
		return outcome, nil
	}
	// CF-V0-026: the same declared universe must produce byte-identical output.
	// A second computation over the same repository is the check the fixture
	// asks for; a second repository would have different Git object identities
	// and could not be the same universe.
	again, err := built.compute(c)
	if err != nil {
		return Outcome{}, err
	}
	if !bytes.Equal(outcome.Stdout, again.Stdout) || !bytes.Equal(outcome.Stderr, again.Stderr) ||
		outcome.ExitCode != again.ExitCode {
		return Outcome{}, fmt.Errorf("repeated invocation of the same universe produced different output")
	}
	return outcome, nil
}

func (u *builtUniverse) compute(c Case) (Outcome, error) {
	ctx := context.Background()
	adapter := frontierrepo.New(ctx, u.root)
	command, observation, report, err := u.tupleFor(c)
	if err != nil {
		return Outcome{}, err
	}
	document, encoded, err := frontier.Compute(ctx, frontier.Request{
		CEMBytes:     u.cemRaw,
		OCMBytes:     u.ocmRaw,
		ExpectedBase: revisionArgument(c.Declared.ExpectedBase, u.base, u.target),
		Target:       revisionArgument(c.Declared.Target, u.target, u.mismatchedTarget),
		Command:      command,
		Observation:  observation,
		JUnitReport:  report,
		Verifier:     adapter,
		TCQ:          adapter,
	})
	if err != nil {
		return Outcome{
			Stderr:   frontier.RenderError(err),
			ExitCode: frontier.ErrorExitCode,
			Symbols:  u.symbols,
		}, nil
	}
	return Outcome{Stdout: encoded, ExitCode: document.ExitCode(), Symbols: u.symbols}, nil
}

// revisionArgument applies the declared revision state. `mismatched` deliberately
// hands over a revision that resolves but is not the one the artifacts bind.
func revisionArgument(state, matching, mismatched string) string {
	switch state {
	case "missing":
		return ""
	case "mismatched":
		return mismatched
	}
	return matching
}

// canaryFor returns the declared canary for one body, so a CF-V0-024 fixture's
// planted string really is in the body it names.
func canaryFor(declared Declared, where string) string {
	for _, canary := range declared.Canaries {
		if canary.Where == where {
			return canary.Value
		}
	}
	return "none"
}

// plannedHunk is one declared hunk bound to a generated file. Several hunks may
// share a path, in which case each is a separate REGION of that file.
type plannedHunk struct {
	declared DeclaredHunk
	index    int
	path     string
	marker   string
}

// bulkPath co-locates the filler hunks of a large counts universe. It sorts
// after every `src/h…` path, so the canonical patch — which orders hunks by
// path and then by position — still lists them in declared order.
//
// One file per hunk is the readable shape and the right one at fixture scale.
// It is the wrong one at 2,048: the shared verifier's Git budget is a frozen
// 1,024 operations, and a universe with one file per hunk exhausts it and fails
// `git-budget-exceeded` before Frontier ever projects an item. Regions of one
// blob derive the same 2,048 canonical hunks against a handful of operations.
const bulkPath = "src/zbulk.txt"

// hunkRegionSeparator is the unchanged text between two regions of one file. It
// is longer than twice Git's three-line diff context, so two changed regions
// never coalesce into one hunk.
const hunkRegionSeparator = "-\n-\n-\n-\n-\n-\n-\n-\n"

func buildUniverse(c Case) (*builtUniverse, func(), error) {
	declared := c.Declared
	declared.Obligations = planObligations(c)
	// bulk is how many of the declared hunks share one file. Only a counts
	// universe needs it: an enumerated fixture gets one readable file per hunk.
	bulk := 0
	if len(declared.Counts) > 0 {
		plan, unreachable := PlanCounts(c)
		if unreachable != "" {
			return nil, nil, fmt.Errorf("no universe realizes this declared count: %s", unreachable)
		}
		declared.Hunks, declared.Obligations = plan.declare()
		bulk = plan.unknownHunks
	}
	if len(declared.Hunks) > wire.MaxHunks {
		return nil, nil, fmt.Errorf("the universe needs %d hunks; a canonical CEM map holds at most %d",
			len(declared.Hunks), wire.MaxHunks)
	}
	workspace, err := os.MkdirTemp("", "frontier-v0-fixture-")
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() { os.RemoveAll(workspace) }
	resolved, err := filepath.EvalSymlinks(workspace)
	if err != nil {
		return nil, cleanup, err
	}
	root := filepath.Join(resolved, "repo")
	home := filepath.Join(resolved, "home")
	for _, dir := range []string{root, home} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, cleanup, err
		}
	}
	git := gitRunner{root: root, home: home}
	initArgs := []string{"init", "-q", "-b", "main"}
	if declared.ObjectFormat == "sha256" {
		initArgs = append(initArgs, "--object-format=sha256")
	}
	if _, err := git.run(initArgs...); err != nil {
		return nil, cleanup, err
	}

	hunks := planHunks(declared, bulk)
	if err := writeBaseTree(root, declared, hunks); err != nil {
		return nil, cleanup, err
	}
	if _, err := git.run("add", "."); err != nil {
		return nil, cleanup, err
	}
	if _, err := git.run("commit", "-qm", "base"); err != nil {
		return nil, cleanup, err
	}
	base, err := git.run("rev-parse", "HEAD")
	if err != nil {
		return nil, cleanup, err
	}
	if err := writeTargetTree(root, declared, hunks); err != nil {
		return nil, cleanup, err
	}
	if _, err := git.run("add", "-A"); err != nil {
		return nil, cleanup, err
	}
	if _, err := git.run("commit", "-qm", "target"); err != nil {
		return nil, cleanup, err
	}
	target, err := git.run("rev-parse", "HEAD")
	if err != nil {
		return nil, cleanup, err
	}

	cemRaw, symbols, err := buildCEM(root, base, target, hunks)
	if err != nil {
		return nil, cleanup, err
	}
	// A forged sidecar is a target-side artifact whose bytes differ from the
	// verified CEM input. The excluded path is outside the canonical patch, so
	// committing it changes the target commit without changing the patch.
	if declared.Sidecar == "forged" {
		forged := bytes.Replace(cemRaw, []byte(`"spec"`), []byte(`"Spec"`), 1)
		if bytes.Equal(forged, cemRaw) {
			return nil, cleanup, fmt.Errorf("could not forge the sidecar: no spec member")
		}
		if err := writeFile(root, wire.ExcludedCEMPath, string(forged)); err != nil {
			return nil, cleanup, err
		}
		if _, err := git.run("add", "-f", wire.ExcludedCEMPath); err != nil {
			return nil, cleanup, err
		}
		if _, err := git.run("commit", "-q", "--amend", "--no-edit"); err != nil {
			return nil, cleanup, err
		}
		if target, err = git.run("rev-parse", "HEAD"); err != nil {
			return nil, cleanup, err
		}
	}

	ocmRaw, testRows, err := buildOCM(git, root, target, cemRaw, declared, hunks, symbols)
	if err != nil {
		return nil, cleanup, err
	}
	cemRaw, ocmRaw = applyProfiles(declared, cemRaw, ocmRaw)

	built := &builtUniverse{root: root, git: git, base: base, target: target,
		cemRaw: cemRaw, ocmRaw: ocmRaw, symbols: symbols, testRows: testRows}
	// A mismatched caller target must still resolve, and the canonical patch
	// must still derive: an empty commit on top of the target has the identical
	// tree, so CEM precedence has nothing to refuse and the OCM's own target
	// binding is what fails.
	if declared.Target == "mismatched" {
		if _, err := git.run("commit", "-q", "--allow-empty", "-m", "sibling"); err != nil {
			return nil, cleanup, err
		}
		successor, err := git.run("rev-parse", "HEAD")
		if err != nil {
			return nil, cleanup, err
		}
		built.target = target
		built.mismatchedTarget = successor
	}
	return built, cleanup, nil
}

// fillerHunk is the smallest real hunk a universe can contain. A canonical
// patch is never empty, so a case that declares no hunk still needs one; it is
// a mechanically reverified whitespace-only change, which CF-V0-008 closes and
// which therefore adds nothing to the frontier the case is about.
var fillerHunk = DeclaredHunk{ID: "filler", Disposition: "whitespace-only"}

// fillerObligation is the smallest real obligation a universe can contain. An
// OCM intent scope that enumerates no requirement is refused with
// `missing-requirements`, so a case that declares no obligation still needs
// one; it is an `unknown`/`unassessed` obligation, which CF-V0-011 gives
// exactly one INTENT_CHANGE item.
//
// It is supplied ONLY where the expectation is an operational failure and no
// document — and therefore no item projection — is compared. A case that
// expects a RESULT and declares no obligation is asserting a universe whose
// items are exactly its hunk items; supplying an obligation there would change
// the projection the case is about. That case is the recorded obligation-free
// spec gap (see IntentItemsPerObligation), not a missing filler, and
// TestEmptyStateIsUnreachableThroughTheEntryPoint drives the refusal for real.
var fillerObligation = DeclaredOblig{ID: "CF-V0-001", Disposition: "unknown", Reason: "unassessed"}

func planObligations(c Case) []DeclaredOblig {
	if len(c.Declared.Obligations) > 0 || c.Expect.Kind == "result" {
		return c.Declared.Obligations
	}
	return []DeclaredOblig{fillerObligation}
}

// planHunks binds each declared hunk to a generated file. The last `bulk` of
// them share one file as separate regions; every other one gets a file of its
// own.
func planHunks(declared Declared, bulk int) []plannedHunk {
	rows := declared.Hunks
	if len(rows) == 0 {
		rows = []DeclaredHunk{fillerHunk}
	}
	ownFile := len(rows) - bulk
	// The canonical patch orders hunks by path, and bindSymbols binds the Nth
	// declared hunk to the Nth patch hunk, so the generated paths must sort in
	// declared order. One digit does that for a handful of hunks and silently
	// stops doing it at ten, which is why the ordinal is padded to the width the
	// universe actually needs.
	width := len(fmt.Sprint(len(rows) - 1))
	planned := make([]plannedHunk, 0, len(rows))
	for index, hunk := range rows {
		word := hunkWord(index)
		path := bulkPath
		if index < ownFile {
			path = fmt.Sprintf("src/h%0*d%s.txt", width, index, word)
		}
		planned = append(planned, plannedHunk{
			declared: hunk, index: index, path: path, marker: word + "marker",
		})
	}
	return planned
}

// hunkWord gives every hunk a distinct basename stem and body marker. LRF's
// subject terms are the union of body terms and the basename stem, so two hunks
// sharing a stem would silently intersect; past the eight readable words the
// stem carries its own ordinal.
func hunkWord(index int) string {
	if index < len(hunkWords) {
		return hunkWords[index]
	}
	return fmt.Sprintf("%s%d", hunkWords[index%len(hunkWords)], index/len(hunkWords))
}

// writeBaseTree writes every file the universe needs at the base commit: the
// intent scope, the test claims, the hunk files, and one evidence file per basis
// that needs its own body.
func writeBaseTree(root string, declared Declared, hunks []plannedHunk) error {
	if err := writeFile(root, "docs/intent.md", intentDocument(declared, hunks)); err != nil {
		return err
	}
	for path, body := range claimsDocuments(declared) {
		if err := writeFile(root, path, body); err != nil {
			return err
		}
	}
	// CF-V0-024 names a sibling path and an exception text as bodies of their
	// own; a case that plants canaries there gets a real file carrying them.
	if sibling := canaryFor(declared, "sibling-path"); sibling != "none" {
		if err := writeFile(root, "docs/"+sibling+".txt",
			canaryLine(declared, "exception-text")); err != nil {
			return err
		}
	}
	for path, content := range hunkTrees(hunks, declared, false) {
		if err := writeFile(root, path, content); err != nil {
			return err
		}
	}
	for index, body := range evidenceBodies(hunks) {
		if err := writeFile(root, evidencePath(index), body); err != nil {
			return err
		}
	}
	return nil
}

func writeTargetTree(root string, declared Declared, hunks []plannedHunk) error {
	deleted := map[string]bool{}
	for _, hunk := range hunks {
		if hunk.declared.Disposition == "deletion" {
			deleted[hunk.path] = true
			if err := os.Remove(filepath.Join(root, filepath.FromSlash(hunk.path))); err != nil {
				return err
			}
		}
	}
	for path, content := range hunkTrees(hunks, declared, true) {
		if deleted[path] {
			continue
		}
		if err := writeFile(root, path, content); err != nil {
			return err
		}
	}
	return nil
}

// hunkTrees returns the content of every generated hunk file. Hunks that share
// a path become separate regions of it, joined by unchanged text wider than
// Git's diff context so the canonical patch still derives one hunk per region.
func hunkTrees(hunks []plannedHunk, declared Declared, atTarget bool) map[string]string {
	regions := map[string][]string{}
	for _, hunk := range hunks {
		body := baseBody(hunk, declared)
		if atTarget {
			body = targetBody(hunk, declared)
		}
		regions[hunk.path] = append(regions[hunk.path], body)
	}
	files := make(map[string]string, len(regions))
	for path, bodies := range regions {
		files[path] = strings.Join(bodies, hunkRegionSeparator)
	}
	return files
}

// baseBody and targetBody give each disposition a change the real verifier can
// reverify. A mechanical claim in particular is REVERIFIED by the CEM verifier
// (`unproven-mechanical` is its refusal), so the declared whitespace-only and
// line-ending-only hunks have to really be that.
func baseBody(hunk plannedHunk, declared Declared) string {
	switch hunk.declared.Disposition {
	case "whitespace-only":
		return "    " + hunk.marker + " indented body\n"
	case "line-ending-only":
		return hunk.marker + " terminated body\n"
	}
	return "baseline body for " + hunk.marker + "\n" + canaryLine(declared, "source-body")
}

// targetBody appends to the base body rather than rewriting it, so the derived
// hunk is exactly the declared change and nothing else.
func targetBody(hunk plannedHunk, declared Declared) string {
	switch hunk.declared.Disposition {
	case "whitespace-only":
		return "\t" + hunk.marker + " indented body\n"
	case "line-ending-only":
		return hunk.marker + " terminated body\r\n"
	}
	added := hunk.marker + " renderer\n"
	if hasBasis(hunk, "local-term-bound") {
		added = overflowingTerms() + "\n"
	}
	return baseBody(hunk, declared) + added + canaryLine(declared, "diff-body")
}

// canaryLine plants one declared CF-V0-024 canary in the body the fixture names
// it for, and nothing when the case declares none.
func canaryLine(declared Declared, where string) string {
	for _, canary := range declared.Canaries {
		if canary.Where == where {
			return canary.Value + "\n"
		}
	}
	return ""
}

func hasBasis(hunk plannedHunk, kind string) bool {
	for _, basis := range hunk.declared.Bases {
		if basis == kind {
			return true
		}
	}
	return false
}

// overflowingTerms produces more distinct qualifying subject terms than LRF's
// 256-term bound, which is the only way a local term-bound abstention arises
// from a real body.
func overflowingTerms() string {
	terms := make([]string, 0, 300)
	for index := 0; index < 300; index++ {
		terms = append(terms, fmt.Sprintf("q%c%c%c",
			'a'+byte(index/676%26), 'a'+byte(index/26%26), 'a'+byte(index%26)))
	}
	return strings.Join(terms, " ")
}

// evidenceBodies returns one body per declared basis, in hunk order then basis
// order. That ordering IS the symbol assignment: the fixture's Nth evidence
// placeholder is the Nth declared basis.
func evidenceBodies(hunks []plannedHunk) []string {
	bodies := []string{}
	for _, hunk := range hunks {
		for _, basis := range basesOf(hunk) {
			switch basis {
			case "qualifying", "ocm-lexical-candidate":
				bodies = append(bodies, hunk.marker+" renderer authority\n")
			case "rejected-span-too-broad":
				bodies = append(bodies, strings.Repeat(unrelatedBody, 25))
			case "rejected-self-referential":
				// Cited against the hunk's own path, so this file is unused.
				bodies = append(bodies, unrelatedBody)
			default:
				bodies = append(bodies, unrelatedBody)
			}
		}
	}
	return bodies
}

// basesOf returns the bases to materialize for one hunk. A CEM `supported`
// hunk always carries at least one basis — the workflow sets that disposition
// only by citing evidence — so a fixture that declares `supported` with no
// basis still gets one, and its own expectation decides whether that is right.
func basesOf(hunk plannedHunk) []string {
	switch hunk.declared.Disposition {
	case "unknown", "whitespace-only", "line-ending-only":
		return nil
	}
	if len(hunk.declared.Bases) == 0 {
		return []string{"rejected-insufficient-lexical-support"}
	}
	return hunk.declared.Bases
}

func evidencePath(index int) string {
	return fmt.Sprintf("docs/e%d%s.txt", index, evidenceWords[index%len(evidenceWords)])
}

// intentDocument writes the one `## Requirements` section the OCM verifier
// re-derives, one bullet per declared obligation, in declared order. Statement
// wording is load-bearing: LRF's obligation edge is a term intersection, so a
// statement that names a hunk's marker makes that hunk a lexical candidate and
// one that does not leaves the obligation without a material witness.
func intentDocument(declared Declared, hunks []plannedHunk) string {
	var out strings.Builder
	out.WriteString("# Intent\n\n## Requirements\n\n")
	for _, obligation := range declared.Obligations {
		out.WriteString("- `" + obligation.ID + "`: " + obligationStatement(declared, obligation, hunks) + "\n")
	}
	out.WriteString("\n## Next\n")
	return out.String()
}

func obligationStatement(declared Declared, obligation DeclaredOblig, hunks []plannedHunk) string {
	// A declared local term-bound abstention on the OBLIGATION edge can only
	// come from an overflowing statement: the referenced hunks are witnessed,
	// so their own terms do not overflow.
	if obligation.Reason == "subject-term-bound-exceeded" {
		return "Preserve " + overflowingTerms()
	}
	if marker := candidateMarker(declared, obligation, hunks); marker != "" {
		return "Preserve the " + marker + " behaviour under review."
	}
	return "Preserve the recorded invariant under review."
}

// candidateMarker returns the hunk marker this obligation should name, if any.
// A hunk declared `ocm-lexical-candidate` is a candidate for the FIRST
// obligation that references it, which is what lets one hunk be a candidate for
// one obligation and merely referenced by another.
func candidateMarker(declared Declared, obligation DeclaredOblig, hunks []plannedHunk) string {
	for _, hunk := range hunks {
		if !hasBasis(hunk, "ocm-lexical-candidate") {
			continue
		}
		owner := ""
		for _, other := range declared.Obligations {
			if contains(other.HunkIDs, hunk.declared.ID) {
				owner = other.ID
				break
			}
		}
		if owner == obligation.ID {
			return hunk.marker
		}
	}
	return ""
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// claimsDocuments writes one Go test per declared claim, across one or more
// test blobs. The anchor is the obligation ID and nothing else: the OCM
// verifier requires the anchor to carry the exact obligation ID with no
// identifier character beside it, and the admitted Go table-case grammar leaves
// no room for anything longer.
//
// Chunk 0 always exists, even for a universe with no claim: the OCM artifact is
// built against a committed blob either way.
func claimsDocuments(declared Declared) map[string]string {
	goBody := claimsHeader()
	var pythonBody *strings.Builder
	index := 0
	for _, obligation := range declared.Obligations {
		for range obligation.ClaimIDs {
			if isPythonClaimSource(obligation.ClaimSource) {
				if pythonBody == nil {
					pythonBody = pythonClaimsHeader()
				}
				pythonBody.WriteString(fmt.Sprintf(
					"\n\ndef %s():\n    \"\"\"%s\"\"\"\n    assert True\n",
					pythonClaimName(index), obligation.ID))
			} else {
				goBody.WriteString(fmt.Sprintf(
					"\nfunc TestClaim%s(t *testing.T) {\n\t_ = []struct{ name string }{{name: %q}}\n}\n",
					claimSuffix(index), obligation.ID))
			}
			index++
		}
	}
	documents := map[string]string{goClaimsPath: goBody.String()}
	if pythonBody != nil {
		body := pythonBody.String()
		if hasClaimSource(declared, claimSourcePythonPost39) {
			body += post39GrammarTail
		}
		if hasClaimSource(declared, claimSourcePythonCRLF) {
			body = strings.ReplaceAll(body, "\n", "\r\n")
		}
		documents[pythonClaimsPath] = body
	}
	return documents
}

func hasClaimSource(declared Declared, source string) bool {
	for _, obligation := range declared.Obligations {
		if obligation.ClaimSource == source {
			return true
		}
	}
	return false
}

func claimsHeader() *strings.Builder {
	body := &strings.Builder{}
	body.WriteString("package tests\n\nimport \"testing\"\n")
	return body
}

func pythonClaimsHeader() *strings.Builder {
	body := &strings.Builder{}
	body.WriteString("\"\"\"Committed test claims.\"\"\"\n")
	return body
}

// post39GrammarTail is a structured-pattern statement: Python 3.10 grammar,
// which `python-ast/1` is frozen against (TCQ-V0-013) and which the Go host's
// own Python grammar (internal/pythonsyntax, reached from lrfrepo's
// `pythonSyntaxValid`) accepts. It is what makes a `python-post-3.9` claims
// blob a blob the shared verifier admits and the frozen grammar refuses.
const post39GrammarTail = "\n\nMODE = 1\nmatch MODE:\n    case 1:\n        SELECTED = True\n"

// claimsPathFor names the blob a claim of one declared source shares.
//
// This used to shard claims across blobs at sixteen apiece, because the OCM
// verifier re-derived each claim's selector by scanning the whole blob it sat
// in. That scan was CUBIC in claims per blob, not quadratic as first recorded:
// every claim re-derived the parent of every table case, and each derivation
// was itself a whole-blob scan, so the blob growing with the claim count added
// the third factor. The ceiling is maxClaims = 512 -- not the 256 first
// recorded, and nothing splits claims across blobs -- which cost hours against
// a Git budget frozen at 20 SECONDS. lrfrepo now derives those facts once per
// blob, so the whole ceiling fits one file in well under a second and the
// sharding has nothing left to buy.
func claimsPathFor(source string) string {
	if isPythonClaimSource(source) {
		return pythonClaimsPath
	}
	return goClaimsPath
}

func isPythonClaimSource(source string) bool {
	return source == claimSourcePythonPost39 || source == claimSourcePythonCRLF
}

const (
	// claimSourcePythonPost39 is the declared claim source whose document the
	// frozen `python-ast/1` grammar refuses.
	claimSourcePythonPost39 = "python-post-3.9"
	// claimSourcePythonCRLF is admitted by the frozen grammar and CPython but
	// contains CR bytes rejected by the approximate OCM grammar.
	claimSourcePythonCRLF = "python-crlf"
	goClaimsPath          = "tests/claims_test.go"
	pythonClaimsPath      = "tests/test_claims.py"
)

// pythonClaimName is the extractor's Python unit name for one claim: the same
// bijective suffix the Go document uses, lowercased into the `test_[a-z]+`
// shape TCQ-V0-013 admits.
func pythonClaimName(index int) string {
	return "test_claim_" + strings.ToLower(claimSuffix(index))
}

// claimSuffix names a test function with letters only, so the selector's parent
// is a single Go identifier the extractor re-derives unambiguously. It is
// bijective base-26, not `index%26`: a universe with more than 26 claims would
// otherwise define one function name twice, and TCQ-V0-021 reports a duplicated
// execution key as AMBIGUOUS before it consults any report row.
func claimSuffix(index int) string {
	letters := []rune{}
	for {
		letters = append([]rune{rune('A' + index%26)}, letters...)
		index = index/26 - 1
		if index < 0 {
			return string(letters)
		}
	}
}

type gitRunner struct{ root, home string }

func (g gitRunner) run(args ...string) (string, error) {
	command := exec.Command("git", args...)
	command.Dir = g.root
	command.Env = gitEnvironment(g.home)
	out, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %v: %w\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out)), nil
}

// gitEnvironment pins identity and dates so a generated repository's commit
// OIDs are a function of its content alone.
func gitEnvironment(home string) []string {
	return append(os.Environ(),
		"GIT_CONFIG_NOSYSTEM=1", "HOME="+home, "XDG_CONFIG_HOME="+home,
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid",
		"GIT_AUTHOR_DATE=2000-01-01T00:00:00+0000", "GIT_COMMITTER_DATE=2000-01-01T00:00:00+0000")
}

func writeFile(root, path, content string) error {
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, []byte(content), 0o644)
}

func digestHex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// buildCEM runs the real two-phase workflow and then applies each declared
// disposition through the real commands: `cite` for a basis, `mark` for an
// explicit unknown or mechanical claim.
func buildCEM(root, base, target string, hunks []plannedHunk) ([]byte, map[string]string, error) {
	session, err := workflow.Open(root)
	if err != nil {
		return nil, nil, err
	}
	if _, err := session.Prepare(context.Background(), workflow.PrepareOptions{Base: base, Target: target}); err != nil {
		return nil, nil, fmt.Errorf("cem prepare: %w", err)
	}
	evidenceIndex := 0
	for _, hunk := range hunks {
		selector := fmt.Sprint(hunk.index + 1)
		switch hunk.declared.Disposition {
		case "unknown":
			// `prepare` already leaves every hunk `unknown`/`no-evidence` with an
			// empty basis, so marking one of those rewrites the map to the bytes
			// it already holds. That is invisible for eight hunks and quadratic
			// for two thousand — each mark reparses and reserializes the whole
			// map — so the redundant call is skipped rather than the workflow
			// bypassed: every other reason still goes through the real command.
			if hunk.declared.Reason == "no-evidence" {
				continue
			}
			if err := markHunk(root, selector, "unknown", hunk.declared.Reason); err != nil {
				return nil, nil, err
			}
		case "whitespace-only", "line-ending-only":
			if err := markHunk(root, selector, "mechanical", hunk.declared.Disposition); err != nil {
				return nil, nil, err
			}
		default:
			for _, basis := range basesOf(hunk) {
				path, lines := evidenceSelection(hunk, basis, evidenceIndex)
				if err := citeHunk(root, selector, path, lines); err != nil {
					return nil, nil, err
				}
				evidenceIndex++
			}
		}
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(wire.ExcludedCEMPath)))
	if err != nil {
		return nil, nil, err
	}
	symbols, err := bindSymbols(raw, hunks)
	if err != nil {
		return nil, nil, err
	}
	return raw, symbols, nil
}

// evidenceSelection returns the path and one-based inclusive line span to cite
// for one declared basis kind.
func evidenceSelection(hunk plannedHunk, basis string, index int) (string, string) {
	switch basis {
	case "rejected-self-referential":
		return hunk.path, "1:1"
	case "rejected-span-too-broad":
		// LRF rejects an evidence span wider than 20 logical lines.
		return evidencePath(index), "1:25"
	}
	return evidencePath(index), "1:1"
}

func citeHunk(root, selector, evidencePath, lines string) error {
	session, err := workflow.Open(root)
	if err != nil {
		return err
	}
	if _, err := session.Cite(context.Background(), workflow.CiteOptions{
		MapPath: wire.ExcludedCEMPath, Hunk: selector,
		EvidencePath: evidencePath, Lines: lines, Relation: "specification",
	}); err != nil {
		return fmt.Errorf("cem cite %s %s: %w", selector, evidencePath, err)
	}
	return nil
}

func markHunk(root, selector, disposition, reason string) error {
	session, err := workflow.Open(root)
	if err != nil {
		return err
	}
	if _, err := session.Mark(context.Background(), workflow.MarkOptions{
		MapPath: wire.ExcludedCEMPath, Hunk: selector,
		Disposition: disposition, Reason: reason,
	}); err != nil {
		return fmt.Errorf("cem mark %s %s/%s: %w", selector, disposition, reason, err)
	}
	return nil
}

// cemDocument is the narrow view of the produced map this runner reads back:
// the real hunk and evidence identities it must bind the fixture's symbols to.
type cemDocument struct {
	Hunks []struct {
		ID    string `json:"id"`
		Basis []struct {
			EvidenceID string `json:"evidenceId"`
		} `json:"basis"`
	} `json:"hunks"`
	Evidence []struct {
		ID string `json:"id"`
	} `json:"evidence"`
}

// bindSymbols maps the fixture's placeholder identifiers onto the real ones.
// Hunks bind by canonical patch order, which is the order the fixture declares
// them in; evidence binds by citation order, which is hunk order then basis
// order.
func bindSymbols(raw []byte, hunks []plannedHunk) (map[string]string, error) {
	var document cemDocument
	if err := json.Unmarshal(raw, &document); err != nil {
		return nil, err
	}
	if len(document.Hunks) != len(hunks) {
		return nil, fmt.Errorf("generated %d hunks for %d declared hunks", len(document.Hunks), len(hunks))
	}
	symbols := map[string]string{}
	for index, hunk := range hunks {
		symbols[hunk.declared.ID] = document.Hunks[index].ID
	}
	ordered := []string{}
	for index := range document.Hunks {
		for _, basis := range document.Hunks[index].Basis {
			ordered = append(ordered, basis.EvidenceID)
		}
	}
	for position, evidenceID := range ordered {
		symbols[placeholder("evidence", position+1)] = evidenceID
	}
	return symbols, nil
}

// placeholder rebuilds the fixture's own symbolic form: a prefix plus a 64-hex
// body of one repeated byte, `01` for the first, `02` for the second.
func placeholder(prefix string, ordinal int) string {
	return prefix + ":sha256:" + strings.Repeat(fmt.Sprintf("%02x", ordinal), 32)
}

// buildOCM writes a real OCM artifact bound to the exact CEM map, canonical
// patch, intent scope and claims of this universe. It returns that artifact and,
// alongside it, the execution identity of every selected claim edge in claim
// order — what a dynamic report has to name to match.
func buildOCM(git gitRunner, root, target string, cemRaw []byte, declared Declared,
	hunks []plannedHunk, symbols map[string]string) ([]byte, []testRow, error) {
	patchSHA256, err := patchDigest(cemRaw)
	if err != nil {
		return nil, nil, err
	}
	intentBody := intentDocument(declared, hunks)
	intentBlobOID, err := git.run("rev-parse", target+":docs/intent.md")
	if err != nil {
		return nil, nil, err
	}
	start := strings.Index(intentBody, "## Requirements")
	end := start + strings.Index(intentBody[start:], "## Next")
	if start < 0 || end < start {
		return nil, nil, fmt.Errorf("generated intent scope has no bounded Requirements section")
	}
	claimBlobs, err := readClaimBlobs(git, target, claimsDocuments(declared))
	if err != nil {
		return nil, nil, err
	}

	claims := []any{}
	obligations := []any{}
	testRows := []testRow{}
	occurrence := map[string]int{}
	claimIndex := 0
	for _, obligation := range declared.Obligations {
		hunkIDs := []string{}
		for _, symbolic := range obligation.HunkIDs {
			hunkIDs = append(hunkIDs, symbols[symbolic])
		}
		sort.Strings(hunkIDs)
		claimIDs := []string{}
		for _, symbolic := range obligation.ClaimIDs {
			path := claimsPathFor(obligation.ClaimSource)
			claim, err := buildClaim(claimBlobs[path], obligation.ID, obligation.ClaimSource, claimIndex, occurrence)
			if err != nil {
				return nil, nil, err
			}
			claims = append(claims, claim)
			identity := claim["id"].(string)
			symbols[symbolic] = identity
			claimIDs = append(claimIDs, identity)
			testRows = append(testRows, testRow{Path: path, Name: testRowName(obligation, claimIndex)})
			claimIndex++
		}
		sort.Strings(claimIDs)
		disposition, reason := "unknown", obligation.Reason
		if obligation.Disposition == "linked" {
			disposition, reason = "linked", "change-and-test-linked"
		}
		obligations = append(obligations, map[string]any{
			"id": obligation.ID, "disposition": disposition, "reason": reason,
			"hunkIds": hunkIDs, "claimIds": claimIDs,
		})
	}
	// OCM-V0-004: claims and every reference array sort by ID and are unique.
	sort.Slice(claims, func(left, right int) bool {
		return claims[left].(map[string]any)["id"].(string) < claims[right].(map[string]any)["id"].(string)
	})
	document := map[string]any{
		"spec": "ocm/0.1-experimental", "targetRevision": target,
		"intentScope": map[string]any{
			"path": "docs/intent.md", "blobOid": intentBlobOID,
			"span":       map[string]any{"start": start, "end": end},
			"spanSha256": digestHex([]byte(intentBody[start:end])),
		},
		"cem":         map[string]any{"mapSha256": digestHex(cemRaw), "patchSha256": patchSHA256},
		"claims":      claims,
		"obligations": obligations,
	}
	raw, err := json.Marshal(document)
	if err != nil {
		return nil, nil, err
	}
	return append(raw, '\n'), testRows, nil
}

// testRowName is the execution identity a dynamic report must name for one
// claim edge: the Go table case's own subtest path, or the Python unit name.
func testRowName(obligation DeclaredOblig, index int) string {
	if isPythonClaimSource(obligation.ClaimSource) {
		return pythonClaimName(index)
	}
	return "TestClaim" + claimSuffix(index) + "/" + obligation.ID
}

// patchDigest reads the canonical patch digest the CEM map already recorded,
// rather than rederiving the patch: the map is the authority on what was
// verified, and a second derivation could disagree with it silently.
func patchDigest(cemRaw []byte) (string, error) {
	var document struct {
		PatchSHA256 string `json:"patchSha256"`
	}
	if err := json.Unmarshal(cemRaw, &document); err != nil {
		return "", err
	}
	if document.PatchSHA256 == "" {
		return "", fmt.Errorf("cem map records no patch digest")
	}
	return document.PatchSHA256, nil
}

// claimBlob is one committed test document and its Git object identity.
type claimBlob struct {
	path string
	body string
	oid  string
}

// readClaimBlobs resolves every generated test document to its committed OID.
func readClaimBlobs(git gitRunner, target string, documents map[string]string) (map[string]claimBlob, error) {
	blobs := make(map[string]claimBlob, len(documents))
	for path, body := range documents {
		oid, err := git.run("rev-parse", target+":"+path)
		if err != nil {
			return nil, err
		}
		blobs[path] = claimBlob{path: path, body: body, oid: oid}
	}
	return blobs, nil
}

// buildClaim mirrors the OCM claim identity rule: the ID digests the canonical
// body without the ID, so a hand-written claim binds exactly its own fields.
// Offsets are relative to the claim's OWN blob, which is why the blob is passed
// rather than assumed.
func buildClaim(blob claimBlob, obligationID, source string, index int, occurrence map[string]int) (map[string]any, error) {
	// A Python claim is anchored on the leading docstring, which is the only
	// Python anchor profile that can carry the exact obligation ID a linked
	// obligation requires; a Go claim is anchored on the table case.
	anchor := obligationID
	selector := "test:TestClaim" + claimSuffix(index) + "/case:" + selectorSlug(obligationID)
	if isPythonClaimSource(source) {
		anchor = `"""` + obligationID + `"""`
		selector = "test:" + pythonClaimName(index) + "#doc"
	}
	seen := blob.path + "\x00" + anchor
	offset := 0
	for repeat := 0; repeat <= occurrence[seen]; repeat++ {
		found := strings.Index(blob.body[offset:], anchor)
		if found < 0 {
			return nil, fmt.Errorf("claim anchor %q is absent from %s", anchor, blob.path)
		}
		offset += found
		if repeat < occurrence[seen] {
			offset += len(anchor)
		}
	}
	occurrence[seen]++
	fields := map[string]any{
		"extractor": "corvint-test-claim/1", "path": blob.path,
		"blobOid":    blob.oid,
		"selector":   selector,
		"span":       map[string]any{"start": offset, "end": offset + len(anchor)},
		"spanSha256": digestHex([]byte(anchor)),
	}
	encoded, err := json.Marshal(fields)
	if err != nil {
		return nil, err
	}
	claim := make(map[string]any, len(fields)+1)
	for key, value := range fields {
		claim[key] = value
	}
	claim["id"] = "claim:sha256:" + digestHex(encoded)
	return claim, nil
}

// selectorSlug reproduces the extractor's own word slug for an anchor. A
// guessed selector is rejected as not re-extractable, so this is derived from
// the same rule: split on case and separator boundaries, lowercase, drop
// digit-only words, join with hyphens.
func selectorSlug(anchor string) string {
	words := []string{}
	current := []rune{}
	flush := func() {
		if len(current) == 0 {
			return
		}
		word := strings.ToLower(string(current))
		current = nil
		if word[0] >= 'a' && word[0] <= 'z' {
			words = append(words, word)
		}
	}
	runes := []rune(anchor)
	for index, character := range runes {
		if character == '_' || character == '-' {
			flush()
			continue
		}
		if index > 0 && isLowerOrDigit(runes[index-1]) && character >= 'A' && character <= 'Z' {
			flush()
		}
		current = append(current, character)
	}
	flush()
	return strings.Join(words, "-")
}

func isLowerOrDigit(character rune) bool {
	return character >= 'a' && character <= 'z' || character >= '0' && character <= '9'
}

// applyProfiles downgrades the declared upstream profiles. CF-V0-001 rejects
// `cem/0.1` and an OCM bound to `cem/0.1` at cascade stage 3, before the shared
// verification call, so the artifacts only need to DECLARE the unadmitted
// profile.
func applyProfiles(declared Declared, cemRaw, ocmRaw []byte) ([]byte, []byte) {
	if declared.CEMProfile == "cem/0.1" {
		cemRaw = bytes.Replace(cemRaw, []byte(`"cem/0.2"`), []byte(`"cem/0.1"`), 1)
	}
	if declared.OCMProfile == "ocm/0.1-bound-to-cem-0.1" {
		ocmRaw = bytes.Replace(ocmRaw, []byte(`"cem":{`), []byte(`"cem":{"spec":"cem/0.1",`), 1)
	}
	return cemRaw, ocmRaw
}

// runCandidateDocument serves the CF-V0-004/CF-V0-018 refusal rows. Those cases
// are not universes to compute: their input is a candidate Frontier document
// that is almost right, and the required outcome is that an independent
// consumer refuses it rather than half-reading it (CF-V0-028). So they route
// through the document-verification seam instead of the producing one.
//
// The candidate is derived from a REAL emitted document, then perturbed in
// exactly the one way the case names. Hand-writing the candidate would only
// prove the verifier rejects hand-written JSON; perturbing a document the
// implementation itself just produced proves it rejects its own output the
// moment that output stops satisfying the clause.
func runCandidateDocument(c Case) (Outcome, error) {
	built, cleanup, err := buildUniverse(candidateSeedCase(c))
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return Outcome{}, err
	}
	seed, err := built.compute(candidateSeedCase(c))
	if err != nil {
		return Outcome{}, err
	}
	if len(seed.Stdout) == 0 {
		return Outcome{}, fmt.Errorf("the seed universe produced no document: %s", seed.Stderr)
	}
	candidate, err := perturbDocument(seed.Stdout, c.ID)
	if err != nil {
		return Outcome{}, err
	}
	verifyErr := frontier.VerifyDocument(candidate)
	if verifyErr == nil {
		return Outcome{Stdout: candidate, ExitCode: 1, Symbols: built.symbols}, nil
	}
	return Outcome{
		Stderr:   frontier.RenderError(verifyErr),
		ExitCode: frontier.ErrorExitCode,
		Symbols:  built.symbols,
	}, nil
}

// candidateSeedCase is the universe the candidate is derived from: one CEM
// unknown hunk, which yields an OPEN document with exactly one item. Both the
// "OPEN with no items" and the "EMPTY with items" perturbations need that.
func candidateSeedCase(c Case) Case {
	seed := c
	seed.Declared.Hunks = []DeclaredHunk{{
		ID: placeholder("hunk", 1), Disposition: "unknown", Reason: "no-evidence",
	}}
	// The universe still needs one obligation: an OCM intent scope enumerating
	// no requirement is refused before any document exists.
	seed.Declared.Obligations = []DeclaredOblig{{
		ID: "CF-V0-001", Disposition: "unknown", Reason: "unassessed",
	}}
	seed.Declared.DynamicTuple = "absent"
	return seed
}

// perturbDocument applies exactly the one violation the case names. Each
// perturbation is the smallest edit that breaks one clause, so the refusal
// cannot be attributed to anything else.
func perturbDocument(document []byte, caseID string) ([]byte, error) {
	text := string(document)
	switch caseID {
	case "open-with-empty-items":
		// CF-V0-004: OPEN requires at least one item.
		return replaceItems(text, "")
	case "empty-with-items":
		// CF-V0-004: EMPTY requires exactly `items: []`.
		return replaceState(text, "EMPTY")
	case "extra-stop-decision-field":
		// CF-V0-004/CF-V0-018: the document shape is closed, and V0 in
		// particular has no stop-decision field.
		return insertBefore(text, `"universeId":`, `"stopDecision":"STOP",`)
	case "retrieval-packet-term-in-wire":
		// CF-V0-005: the state vocabulary is closed; a retrieval-packet term is
		// not a Frontier state.
		return replaceState(text, "READY")
	case "decimal-offset-beyond-int64-range":
		// CF-V0-019: every offset is a decimal string that must fit a signed
		// 64-bit integer; a verifier rejects a longer digit run rather than
		// wrapping it onto a bound value.
		return replaceQuotedField(text, `"start":"`, "99999999999999999999")
	}
	return nil, fmt.Errorf("no perturbation is defined for candidate case %q", caseID)
}

// replaceQuotedField substitutes the value of the first `"key":"..."` field
// matched by marker (e.g. `"start":"`) with value, leaving everything else
// byte-identical.
func replaceQuotedField(text, marker, value string) ([]byte, error) {
	open := strings.Index(text, marker)
	if open < 0 {
		return nil, fmt.Errorf("emitted document has no field matching %q", marker)
	}
	rest := text[open+len(marker):]
	closing := strings.Index(rest, `"`)
	if closing < 0 {
		return nil, fmt.Errorf("emitted field matching %q is unterminated", marker)
	}
	return []byte(text[:open+len(marker)] + value + rest[closing:]), nil
}

func replaceItems(text, body string) ([]byte, error) {
	open := strings.Index(text, `"items":[`)
	closing := strings.Index(text, `],"profile":`)
	if open < 0 || closing < open {
		return nil, fmt.Errorf("emitted document has no items array to replace")
	}
	return []byte(text[:open+len(`"items":[`)] + body + text[closing:]), nil
}

func replaceState(text, state string) ([]byte, error) {
	open := strings.Index(text, `"frontierState":"`)
	if open < 0 {
		return nil, fmt.Errorf("emitted document has no frontierState")
	}
	rest := text[open+len(`"frontierState":"`):]
	closing := strings.Index(rest, `"`)
	if closing < 0 {
		return nil, fmt.Errorf("emitted frontierState is unterminated")
	}
	return []byte(text[:open+len(`"frontierState":"`)] + state + rest[closing:]), nil
}

func insertBefore(text, anchor, insertion string) ([]byte, error) {
	at := strings.Index(text, anchor)
	if at < 0 {
		return nil, fmt.Errorf("emitted document has no %s member", anchor)
	}
	return []byte(text[:at] + insertion + text[at:]), nil
}
