package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/attest"
	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/workflow"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/mutate"
	"github.com/Beamfall/corvint/internal/liveverify/pyresolve"
	"github.com/Beamfall/corvint/internal/observations"
)

// proveProfile names the falsifiable-packet wire.
// It is a separate opt-in surface because the `query` packet is compared
// byte-for-byte against the Python oracle by cli-parity-v0 and cannot gain a
// member. The packet is embedded unchanged; every verdict lives beside it in
// `proof`.
const proveProfile = "falsifiable-packet/0"

const (
	falsifierHistory   = "history-consistent"
	falsifierReference = "reference-resolves"
	falsifierVerifier  = "verifier-accepts"
	falsifierMutant    = "test-kills-mutant"
	falsifierNone      = "none"

	falsifiedPass   = "PASS"
	falsifiedFail   = "FAIL"
	falsifiedNotRun = "NOT_RUN"

	// Change mode (FPK-V0-017): the affected-plan selector's tests become
	// rows of this kind, one per selected test, claiming the test covers the
	// changed path that reached its unit.
	kindAffectedTest    = "affected-test"
	authorityAffected   = "affected-selection"
	affectedClaimPrefix = "affected test for "
	samePackageClaim    = "same-package test for "
	// budgetExhaustedDetail is the verdict line of a mutation row reached
	// after the invocation's mutation budget was spent.
	budgetExhaustedDetail = "invocation mutation budget exhausted before this row"
	maxRangeChangedPaths  = 100
)

// proveBounds are the byte bounds on the Git streams prove reads and the
// invocation's mutation budget (FPK-V0-014). Every invocation uses the
// defaults proveBoundsFrom returns; nothing in the command overrides them. A
// test lowers one for a single invocation by carrying replacement bounds on
// the context under proveBoundsKey, so tests that do so share no package
// variable and can run in parallel.
type proveBounds struct {
	rangeChangeBytes    int
	rangeDiffBytes      int
	blobBytes           int
	checkpointTreeBytes int
	// mutationBudget bounds every mutation run of one invocation together:
	// rows are judged in document order, and a row reached after the budget
	// is spent is NOT_RUN rather than run late.
	mutationBudget time.Duration
}

type proveBoundsKey struct{}

func proveBoundsFrom(ctx context.Context) proveBounds {
	if bounds, ok := ctx.Value(proveBoundsKey{}).(proveBounds); ok {
		return bounds
	}
	return proveBounds{
		rangeChangeBytes:    1 << 20,
		rangeDiffBytes:      16 << 20,
		blobBytes:           64 << 20,
		checkpointTreeBytes: 64 << 20,
		mutationBudget:      30 * time.Minute,
	}
}

// falsifierByKindAndAuthority is consulted first, keyed on the result kind
// and the row authority. It is where a `syntax` row with a citing site (an
// importer, a same-package reference) parts from a `syntax` row that only
// matched vocabulary: the former can be falsified by parsing both sides, the
// latter by checking its citation. `reference-resolves` is judged for the
// languages in referenceLanguages; any other language falls through to the
// authority table.
var falsifierByKindAndAuthority = map[string]string{
	"test|test-convention":             falsifierMutant,
	"affected-test|affected-selection": falsifierMutant,
	"reverse-import|syntax":            falsifierReference,
	"reference|syntax":                 falsifierReference,
	"cem-hunk|cem-supported":           falsifierVerifier,
	"cem-hunk|cem-mechanical":          falsifierVerifier,
	"cem-hunk|cem-unknown":             falsifierNone,
}

// referenceLanguages lists the path suffixes `reference-resolves` can judge:
// Go through go/parser, Python through the pyresolve import grammar, and the
// web set through the jsresolve lexer (FPK-V0-018).
var referenceLanguages = map[string]bool{".go": true, ".py": true, ".js": true, ".mjs": true, ".cjs": true, ".jsx": true, ".ts": true, ".tsx": true}

// proveOptions carries the checkpoint file and CEM arguments: the map to judge, the
// independent inputs the verifier demands, and the attestation request.
// `attest` wraps the document as an in-toto Statement; `attestKey` names a
// PKCS#8 Ed25519 PEM whose signature seals it in a DSSE envelope; `attestCEM`
// adds the FPK-V0-031 CEM statement, `cem/v1` with `attestCEMV1` (FPK-V0-050).
// `envelopePath` and `publicKeyPath` are the verify-cem mode inputs.
type proveOptions struct {
	mapPath, expectedBase, target, patch, attestKey string
	checkpointPath, envelopePath, publicKeyPath     string
	patchGiven, attest, attestCEM, attestCEMV1      bool
}

// falsifierByAuthority is the closed assignment table. Authorities that cite
// content at a blob and line get `history-consistent`; authorities that carry
// only advisory history get `none` and can never count as proven. Any
// authority absent from the table also gets `none`.
var falsifierByAuthority = map[string]string{
	"project-instructions":          falsifierHistory,
	"instruction-reference":         falsifierHistory,
	"repository-spec":               falsifierHistory,
	"accepted-spec":                 falsifierHistory,
	"accepted-decision":             falsifierHistory,
	"non-binding-decision":          falsifierHistory,
	"accepted-contract":             falsifierHistory,
	"partially-superseded-contract": falsifierHistory,
	"document-reference":            falsifierHistory,
	"canonical-ledger":              falsifierHistory,
	"source-marker":                 falsifierHistory,
	"test-marker":                   falsifierHistory,
	"git-tree":                      falsifierHistory,
	"syntax":                        falsifierHistory,
	"git-history":                   falsifierNone,
	"local-task-trace":              falsifierNone,
	"unverified-contract":           falsifierNone,
	"unverified-ledger":             falsifierNone,
}

type proveReceipt struct {
	Mutates  bool           `json:"mutates"`
	OK       bool           `json:"ok"`
	Packet   map[string]any `json:"packet"`
	Profile  string         `json:"profile"`
	Proof    proveSummary   `json:"proof"`
	Revision string         `json:"revision"`
	State    string         `json:"state"`
	Tool     string         `json:"tool"`

	// subjects and mapBytes are off the wire: they feed attestProof only.
	subjects []attest.Subject
	mapBytes []byte
}

type proveSummary struct {
	// Affected describes the affected-plan selection change mode drew its
	// affected-test rows from; absent outside change mode.
	Affected      *proveAffected            `json:"affected,omitempty"`
	Claims        map[string]string         `json:"claims"`
	Counts        map[string]map[string]int `json:"counts"`
	FailedResults int                       `json:"failed_results"`
	// Ledger is the repository's falsification rate over the proofs the
	// local ledger retains (see prove-observe). It is advisory, read-only,
	// and absent until a proof has been recorded.
	Ledger          *observations.Falsification `json:"ledger,omitempty"`
	ProvenResults   int                         `json:"proven_results"`
	Rows            []proveRow                  `json:"rows"`
	UnprovenResults int                         `json:"unproven_results"`
}

// proveAffected is the part of the affected plan a reader needs to weigh the
// affected-test rows: which graph, whether the selection was complete, and how
// many tests and unknowns it named.
type proveAffected struct {
	GraphDigest string `json:"graph_digest"`
	// Paths is the coverage summary per changed path (decision 0018): one
	// entry, sorted by path, for every changed path an affected-test row
	// claims.
	Paths    []proveAffectedPath `json:"paths"`
	Scope    string              `json:"scope"`
	Selected int                 `json:"selected"`
	Unknown  int                 `json:"unknown"`
}

// proveAffectedPath folds the affected-test rows claiming one changed path:
// the tests whose row passed cover it; a FAIL row reached it without pinning
// it; a NOT_RUN row said nothing.
type proveAffectedPath struct {
	CoveredBy          []string `json:"covered_by"`
	NotRun             int      `json:"not_run"`
	Path               string   `json:"path"`
	ReachedNotCovering int      `json:"reached_not_covering"`
}

// verdict is the path's result verdict: covered is proven, reached only by
// tests that let the mutants survive is failed, anything else unproven.
func (path proveAffectedPath) verdict() string {
	if len(path.CoveredBy) > 0 {
		return falsifiedPass
	}
	if path.ReachedNotCovering > 0 {
		return falsifiedFail
	}
	return falsifiedNotRun
}

// proveRow is one verdict, keyed by the row it judges: the result id and the row's own citation,
// in packet order. kind and reason are the claim's shape and text; they steer the falsifier and stay off the wire.
type proveRow struct {
	Authority string `json:"authority"`
	BlobHash  string `json:"blob_hash"`
	Falsified string `json:"falsified"`
	Falsifier string `json:"falsifier"`
	// Kind is the result's kind. Two results of one packet can share an id
	// (an external-package test file is both a `test` and a `reverse-import`
	// result), so a row is bound to its result by kind and id together.
	Kind    string `json:"kind"`
	Line    int    `json:"line"`
	Path    string `json:"path"`
	Refusal string `json:"refusal,omitempty"`
	Result  string `json:"result"`
	Trust   string `json:"trust"`
	// Detail is the mutation runner's one-line report, present only on a `test-kills-mutant` row that ran.
	Detail  string                `json:"detail,omitempty"`
	Witness *proveMutationWitness `json:"witness,omitempty"`
	reason  string
	basis   []string
}

type proveMutationWitness struct {
	Status           string              `json:"status"`
	CheckoutRevision string              `json:"checkout_revision,omitempty"`
	Mutant           *proveWitnessMutant `json:"mutant,omitempty"`
	KillingTest      string              `json:"killing_test,omitempty"`
	ObservedKill     string              `json:"observed_kill,omitempty"`
	Reason           string              `json:"reason,omitempty"`
}

type proveWitnessMutant struct {
	Path             string           `json:"path"`
	Blob             string           `json:"blob"`
	ByteSpan         proveWitnessSpan `json:"byte_span"`
	MutationOperator string           `json:"mutation_operator"`
}

type proveWitnessSpan struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// citedBlob is what Git says lives at `<revision>:<path>`: the object id, its
// type, its content, and how many lines it holds. It is read from Git
// directly, never from the index that produced the row, so the falsifier does
// not trust the claimant.
type citedBlob struct {
	oid, objectType string
	content         []byte
	lines           int
}

// parseProveInvocation recognises `[--root PATH] prove --task TEXT ...` and
// `[--root PATH] prove [--limit N] PATH...` and reuses the query or impact
// parser, so flags, limits, and root rules are identical to the wrapped
// command. Help forms are left to the ordinary help path.
func parseProveInvocation(arguments []string) (options, bool, error) {
	if _, requested, _ := parseHelpInvocation(arguments); requested {
		return options{}, false, nil
	}
	position := commandPositionAfterRoots(arguments)
	if position < 0 || arguments[position] != "prove" {
		return options{}, false, nil
	}
	if proveWrappedCommand(arguments[position+1:]) == "verify-cem" {
		parsed, err := parseProveVerifyCEMArguments(arguments[:position], arguments[position+1:])
		return parsed, true, err
	}
	if proveWrappedCommand(arguments[position+1:]) == "checkpoint" {
		parsed, err := parseProveCheckpointArguments(arguments[:position], arguments[position+1:])
		return parsed, true, err
	}
	if proveWrappedCommand(arguments[position+1:]) == "cem" {
		parsed, err := parseProveCEMArguments(arguments[:position], arguments[position+1:])
		return parsed, true, err
	}
	rest, mutateCount := withoutMutateFlag(arguments[position+1:])
	rewritten := append(append([]string{}, arguments[:position]...), proveWrappedCommand(rest))
	rewritten = append(rewritten, rest...)
	parsed, err := parse(rewritten)
	parsed.proveMode, parsed.command, parsed.proveMutate = rewritten[position], "prove", mutateCount > 0
	if err != nil {
		return parsed, true, err
	}
	if parsed.impactProfile != "" {
		return parsed, true, argumentError("prove does not support --range-profile")
	}
	if parsed.impactWorktree {
		return parsed, true, argumentError("prove judges tracked paths only: --working-tree-untracked is not supported")
	}
	if parsed.impactBaseSet {
		parsed.proveMode = "change"
	}
	if mutateCount > 1 {
		return parsed, true, argumentError("argument --mutate: may not be repeated")
	}
	if mutateCount > 0 && parsed.proveMode != "impact" && parsed.proveMode != "change" {
		return parsed, true, argumentError("argument --mutate: only impact and change modes run the mutation falsifier")
	}
	return parsed, true, nil
}

// withoutMutateFlag lifts `--mutate` out of the arguments after the command
// word, which the wrapped parser would otherwise refuse, and counts it. A
// token after `--` is positional and is never lifted.
func withoutMutateFlag(arguments []string) ([]string, int) {
	kept := make([]string, 0, len(arguments))
	count := 0
	positional := false
	for _, argument := range arguments {
		if argument == "--" {
			positional = true
		}
		if argument == "--mutate" && !positional {
			count++
			continue
		}
		kept = append(kept, argument)
	}
	return kept, count
}

// proveWrappedCommand selects the first wrapper flag before --; without one,
// positional paths select impact. Each selected parser owns its value tokens.
func proveWrappedCommand(rest []string) string {
	if proveVerifiesCEMAttestation(rest) {
		return "verify-cem"
	}
	for _, argument := range rest {
		if argument == "--" {
			break
		}
		if argument == "--checkpoint" || strings.HasPrefix(argument, "--checkpoint=") {
			return "checkpoint"
		}
		if argument == "--task" || strings.HasPrefix(argument, "--task=") {
			return "query"
		}
		if argument == "--cem" || strings.HasPrefix(argument, "--cem=") {
			return "cem"
		}
	}
	return "impact"
}

// parseProveCEMArguments reads `--cem MAP [--expected-base REV] [--target REV]
// [--patch FILE]`; every flag takes one value and none repeats.
func parseProveCEMArguments(rootArguments, rest []string) (options, error) {
	result := options{command: "prove", proveMode: "cem"}
	root := "."
	for index := 0; index < len(rootArguments); index++ {
		if rootArguments[index] == "--root" {
			root, index = rootArguments[index+1], index+1
		} else {
			root = strings.TrimPrefix(rootArguments[index], "--root=")
		}
	}
	seen := map[string]bool{}
	for index := 0; index < len(rest); {
		name, value, inline := strings.Cut(rest[index], "=")
		if name == "--attest" || name == "--attest-cem" || name == "--attest-cem-v1" {
			if inline {
				return result, argumentError("argument " + name + ": ignored explicit argument " + value)
			}
			if refusal := attestFlagRefusal(seen, name); refusal != nil {
				return result, refusal
			}
			seen[name] = true
			index++
			continue
		}
		field := proveCEMFlag(&result.prove, name)
		if field == nil {
			return result, argumentError("unrecognized arguments: " + rest[index])
		}
		if seen[name] {
			return result, argumentError("argument " + name + ": may not be repeated")
		}
		seen[name] = true
		if !inline {
			if index+1 >= len(rest) || argparseOptionLike(rest[index+1]) {
				return result, argumentError("argument " + name + ": expected one argument")
			}
			value, index = rest[index+1], index+2
		} else {
			index++
		}
		if value == "" {
			return result, argumentError("argument " + name + ": expected one argument")
		}
		*field = value
	}
	result.prove.patchGiven = seen["--patch"]
	result.prove.attestCEM, result.prove.attestCEMV1 = seen["--attest-cem"] || seen["--attest-cem-v1"], seen["--attest-cem-v1"]
	result.prove.attest = seen["--attest"] || seen["--attest-key"] || result.prove.attestCEM
	if result.prove.mapPath == "" {
		return result, argumentError("argument --cem: expected one argument")
	}
	resolved, err := resolveExplicitRoot(root)
	if err != nil {
		return result, err
	}
	result.root = resolved
	return result, nil
}

func proveCEMFlag(prove *proveOptions, name string) *string {
	switch name {
	case "--cem":
		return &prove.mapPath
	case "--expected-base":
		return &prove.expectedBase
	case "--target":
		return &prove.target
	case "--patch":
		return &prove.patch
	case "--attest-key":
		return &prove.attestKey
	}
	return nil
}

// commandPositionAfterRoots returns the index of the first argument after any
// leading --root pairs, or -1 when only roots were given.
func commandPositionAfterRoots(arguments []string) int {
	index := 0
	for index < len(arguments) && (arguments[index] == "--root" && rootPreambleValue(arguments, index+1) || strings.HasPrefix(arguments[index], "--root=")) {
		if arguments[index] == "--root" {
			index += 2
		} else {
			index++
		}
	}
	if index >= len(arguments) {
		return -1
	}
	return index
}

func runProve(ctx context.Context, options options, stdout, stderr io.Writer) int {
	if options.proveMode == "verify-cem" {
		return runVerifyCEMAttestation(options, stdout, stderr)
	}
	receipt, err := compileProof(ctx, options)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	var subjects []attest.Subject
	var mapBytes []byte
	if proof, ok := receipt.(proveReceipt); ok {
		proof.Proof.Ledger = proveLedger(options.root)
		subjects, mapBytes = proof.subjects, proof.mapBytes
		receipt = proof
	}
	encoded, err := gokernel.CanonicalJSON(receipt)
	if err != nil {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot encode falsifiable packet"})
		return 2
	}
	if options.prove.attest {
		encoded, err = attestProof(encoded, subjects, cemAttestationInput(options.prove, mapBytes), options.prove.attestKey)
	}
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	if _, err := stdout.Write(append(encoded, '\n')); err != nil {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot write falsifiable packet"})
		return 2
	}
	return 0
}

// compileProof brackets the whole operation: the dirty set is read before the
// packet is compiled and again after the cited blobs are read, and the HEAD
// tree must still be the packet's revision at the end. Any drift fails closed,
// so a PASS can only describe content the agent will read.
func compileProof(ctx context.Context, options options) (any, error) {
	gitExecutable, err := exec.LookPath("git")
	if err != nil {
		return proveReceipt{}, proveGitExecutableRefusal()
	}
	dirtyBefore, err := affected.DirtyPaths(ctx, gitExecutable, options.root)
	if err != nil {
		return proveReceipt{}, &gokernel.Error{Code: "unsupported-prove-history", Message: "cannot read the worktree status"}
	}
	if options.proveMode == "checkpoint" {
		return compileCheckpointProof(ctx, gitExecutable, options, dirtyBefore)
	}
	if options.proveMode == "cem" {
		return compileCEMProof(ctx, gitExecutable, options, dirtyBefore)
	}
	packet, err := provePacket(ctx, options)
	if err != nil {
		return proveReceipt{}, err
	}
	revision := stringAt(packet, "revision")
	if !validGitObjectID(revision) {
		return proveReceipt{}, &gokernel.Error{Code: "unsupported-prove-revision", Message: "query packet carries no resolvable revision"}
	}
	rows := assignFalsifiers(packet)
	var affectedSummary *proveAffected
	var spans map[string][]mutate.LineSpan
	if options.proveMode == "change" {
		extra, summary, err := affectedTestRows(ctx, gitExecutable, options)
		if err != nil {
			return proveReceipt{}, err
		}
		rows, affectedSummary = append(rows, extra...), summary
		if spans, err = rangeHunkSpans(ctx, gitExecutable, options.root, options.impactBase); err != nil {
			return proveReceipt{}, err
		}
	}
	checkoutRevision := ""
	if options.proveMutate {
		commit, tree, revisionErr := checkpointRevision(ctx, gitExecutable, options.root)
		if revisionErr != nil || tree != revision {
			return proveReceipt{}, &gokernel.Error{Code: "unsupported-prove-drift", Message: "the worktree or HEAD changed while the packet was being proved"}
		}
		checkoutRevision = commit
	}
	cited, err := readCitedBlobs(ctx, gitExecutable, options.root, revision, citedPaths(rows, options.proveMutate))
	if err != nil {
		return proveReceipt{}, err
	}
	citeAffectedBlobs(rows, cited)
	mutations, err := judgeMutations(ctx, gitExecutable, options, revision, checkoutRevision, rows, cited, stringSet(dirtyBefore), spans)
	if err != nil {
		return proveReceipt{}, err
	}
	dirtyAfter, err := affected.DirtyPaths(ctx, gitExecutable, options.root)
	if err != nil {
		return proveReceipt{}, &gokernel.Error{Code: "unsupported-prove-history", Message: "cannot read the worktree status"}
	}
	tree, err := proveTreeRevision(ctx, gitExecutable, options.root)
	if err != nil {
		return proveReceipt{}, err
	}
	if tree != revision || !equalStringSlices(dirtyBefore, dirtyAfter) {
		return proveReceipt{}, &gokernel.Error{Code: "unsupported-prove-drift", Message: "the worktree or HEAD changed while the packet was being proved"}
	}
	if options.proveMutate {
		commitAfter, _, revisionErr := checkpointRevision(ctx, gitExecutable, options.root)
		if revisionErr != nil || commitAfter != checkoutRevision {
			return proveReceipt{}, &gokernel.Error{Code: "unsupported-prove-drift", Message: "the worktree or HEAD changed while the packet was being proved"}
		}
	}
	dirty := stringSet(dirtyAfter)
	for index := range rows {
		rows[index].Falsified = falsifyRow(rows[index], cited, dirty)
	}
	for index, judged := range mutations {
		rows[index].Falsified, rows[index].Detail, rows[index].Witness = judged.falsified, judged.detail, judged.witness
	}
	summary := summarizeProof(packet, rows)
	if affectedSummary != nil {
		affectedSummary.Paths = affectedPaths(rows)
		summary.Affected = affectedSummary
	}
	return proveReceipt{
		Mutates: false, OK: true, Packet: packet, Profile: proveProfile, Proof: summary,
		Revision: revision, State: proveState(stringAt(packet, "state"), summary), Tool: "prove",
	}, nil
}

// compileCheckpointProof owns the complete closing bracket because compileProof
// dispatches here before its ordinary packet checks.
func compileCheckpointProof(ctx context.Context, gitExecutable string, options options, dirtyBefore []string) (map[string]any, error) {
	commit, revision, err := checkpointRevision(ctx, gitExecutable, options.root)
	if err != nil {
		return nil, err
	}
	tree, err := readCheckpointTree(ctx, gitExecutable, options.root, revision)
	if err != nil {
		return nil, err
	}
	document, err := checkpointDocumentFor(ctx, options.prove.checkpointPath)
	if err != nil {
		return nil, err
	}
	index, buildErr := contextindex.Build(ctx, options.root)
	if buildErr != nil {
		var contextError *contextindex.Error
		var kernelError *gokernel.Error
		if errors.As(buildErr, &contextError) && contextError.Code != "" {
			return nil, buildErr
		}
		if errors.As(buildErr, &kernelError) && kernelError.Code != "" {
			return nil, buildErr
		}
		return nil, &gokernel.Error{Code: "unsupported-prove-index", Message: buildErr.Error()}
	}
	if index.ObjectFormat != document.Repository.ObjectFormat {
		return nil, checkpointObjectFormatRefusal(options.prove.checkpointPath, index.ObjectFormat, document.Repository.ObjectFormat)
	}
	paths := map[string]struct{}{}
	for _, handle := range document.Handles {
		if checkpointPathFramable(handle.Path) {
			paths[handle.Path] = struct{}{}
		}
	}
	cited, err := readCitedBlobs(ctx, gitExecutable, options.root, revision, sortedKeys(paths))
	if err != nil {
		return nil, err
	}
	receipt := judgeCheckpoint(document, index, revision, tree, cited, dirtyBefore)
	if err := checkpointClosingRead(ctx, gitExecutable, options.root, commit, revision, dirtyBefore); err != nil {
		return nil, err
	}
	// Build may have observed a different stable revision in the middle of the
	// bracket. Its rows and history flags must share the opening commit and tree.
	if index.Revision != revision || index.CommitRevision != commit {
		return nil, checkpointError("unsupported-prove-drift", "the index revision changed while the checkpoint was being proved")
	}
	return receipt, nil
}

// affectedTestRows is change mode's second claim source (FPK-V0-017): the
// affected-plan selector over the range's changed paths names the tests the
// change reaches, and each becomes one affected-test row claiming that test
// covers the changed path that reached its unit. Rows follow plan order,
// deduplicated by test path. The blob each row cites is filled in once Git
// has answered for it (citeAffectedBlobs), so the claim is bound to the
// packet revision exactly as a packet row is.
func affectedTestRows(ctx context.Context, gitExecutable string, options options) ([]proveRow, *proveAffected, error) {
	changed, err := rangeChangedPaths(ctx, gitExecutable, options.root, options.impactBase)
	if err != nil {
		return nil, nil, err
	}
	graph, err := affected.Build(options.root, affectedLanguages()...)
	if err != nil {
		return nil, nil, &gokernel.Error{Code: "unsupported-prove-affected", Message: "the affected-plan graph could not be built: " + err.Error()}
	}
	plan := affected.Select(graph, changed)
	rows := make([]proveRow, 0, 8)
	seen := map[string]struct{}{}
	for _, selection := range plan.Selected {
		for _, test := range selection.Tests {
			if _, duplicate := seen[test]; duplicate {
				continue
			}
			seen[test] = struct{}{}
			rows = append(rows, affectedRow(test, selection.Witness.DirtyPath))
		}
	}
	summary := &proveAffected{GraphDigest: plan.GraphDigest, Scope: plan.Scope, Selected: len(rows), Unknown: len(plan.Unknown)}
	return rows, summary, nil
}

// affectedRow claims that test covers changed. A test reached only through a
// changed test file makes no mutation claim: mutating a test proves nothing
// about the code, so such a row keeps falsifier none.
func affectedRow(test, changed string) proveRow {
	row := proveRow{
		Authority: authorityAffected, Kind: kindAffectedTest, Line: 1, Path: test, Result: test,
		reason: affectedClaimPrefix + changed,
	}
	classifyTrust(&row)
	if !mutationClaim(test, changed) {
		row.Falsifier = falsifierNone
	}
	return row
}

// citeAffectedBlobs binds each affected-test row to the blob Git holds at the
// packet revision for its test path; a test that is not a blob there stays
// unbound and its mutation claim fails before anything runs.
func citeAffectedBlobs(rows []proveRow, cited map[string]citedBlob) {
	for index := range rows {
		if rows[index].Kind != kindAffectedTest {
			continue
		}
		if blob, present := cited[rows[index].Path]; present && blob.objectType == "blob" {
			rows[index].BlobHash = blob.oid
		}
	}
}

// rangeHunkSpans reads the new-side line ranges the range base..HEAD changed
// in every Go path, from `git diff -U0`, so change mode's mutants land inside
// the change (FPK-V0-017). A pure deletion names the line before it, the
// nearest line a mutant can still touch.
func rangeHunkSpans(ctx context.Context, gitExecutable, root, base string) (map[string][]mutate.LineSpan, error) {
	deadline, cancel := context.WithTimeout(ctx, proveGitDeadline)
	defer cancel()
	command := hermeticGitCommand(deadline, gitExecutable, root, "diff", "-U0", "--no-renames", "--no-ext-diff", "--no-textconv", "--no-color", "--src-prefix=a/", "--dst-prefix=b/", base, "HEAD", "--", "*.go", "*.py")
	output, err := boundedOutput(command, proveBoundsFrom(ctx).rangeDiffBytes)
	if errors.Is(err, errOutputBound) {
		return nil, rangeDiffBoundRefusal(base)
	}
	if err != nil {
		return nil, &gokernel.Error{Code: "unsupported-prove-history", Message: "cannot read the range's hunks"}
	}
	return parseHunkSpans(string(output)), nil
}

// parseHunkSpans folds a unified diff into new-side spans per path.
func parseHunkSpans(diff string) map[string][]mutate.LineSpan {
	spans := map[string][]mutate.LineSpan{}
	current := ""
	for _, line := range strings.Split(diff, "\n") {
		if header, ok := strings.CutPrefix(line, "+++ "); ok {
			current = newSidePath(header)
			continue
		}
		if !strings.HasPrefix(line, "@@ ") || current == "" {
			continue
		}
		if span, ok := hunkSpan(line); ok {
			spans[current] = append(spans[current], span)
		}
	}
	return spans
}

// newSidePath reads the path a `+++` header names: `b/PATH`, or Git's
// C-quoted `"b/PATH"` for a path with unusual bytes. A deleted file's
// `/dev/null` and anything unrecognised name nothing, so the hunks that follow
// attach to no path and the row for that path admits no mutant.
func newSidePath(header string) string {
	if strings.HasPrefix(header, "\"") {
		unquoted, err := strconv.Unquote(header)
		if err != nil {
			return ""
		}
		header = unquoted
	}
	path, ok := strings.CutPrefix(header, "b/")
	if !ok {
		return ""
	}
	return path
}

// hunkSpan reads the `+start[,count]` half of a hunk header.
func hunkSpan(header string) (mutate.LineSpan, bool) {
	fields := strings.Fields(header)
	if len(fields) < 3 || !strings.HasPrefix(fields[2], "+") {
		return mutate.LineSpan{}, false
	}
	start, count := 0, 1
	if _, err := fmt.Sscanf(fields[2], "+%d,%d", &start, &count); err != nil {
		count = 1
		if _, err := fmt.Sscanf(fields[2], "+%d", &start); err != nil {
			return mutate.LineSpan{}, false
		}
	}
	if count == 0 {
		return mutate.LineSpan{Start: max(1, start), End: max(1, start)}, true
	}
	return mutate.LineSpan{Start: start, End: start + count - 1}, true
}

// rangeChangedPaths lists the paths the range base..HEAD changed, as Git
// names them, bounded in bytes and count; the same change set RangeImpact
// admitted, read once more from Git rather than from the packet.
func rangeChangedPaths(ctx context.Context, gitExecutable, root, base string) ([]string, error) {
	deadline, cancel := context.WithTimeout(ctx, proveGitDeadline)
	defer cancel()
	command := hermeticGitCommand(deadline, gitExecutable, root, "diff-tree", "-r", "-z", "--name-only", "--no-renames", "--diff-filter=ACMT", base, "HEAD")
	output, err := boundedOutput(command, proveBoundsFrom(ctx).rangeChangeBytes)
	if errors.Is(err, errOutputBound) {
		return nil, rangeChangedPathsBoundRefusal(base)
	}
	if err != nil {
		return nil, &gokernel.Error{Code: "unsupported-prove-history", Message: "cannot list the changed paths of the range"}
	}
	paths := make([]string, 0, 8)
	for _, entry := range bytes.Split(output, []byte{0}) {
		if len(entry) != 0 {
			paths = append(paths, string(entry))
		}
	}
	if len(paths) > maxRangeChangedPaths {
		return nil, &gokernel.Error{Code: "unsupported-prove-history", Message: fmt.Sprintf("the range changes more than %d paths", maxRangeChangedPaths)}
	}
	return paths, nil
}

// compileCEMProof judges a CEM map: the embedded document is what `cem verify`
// would emit for the same inputs, and every hunk becomes one row whose
// verdict is the verifier's. The verifier reads committed objects only, so the
// bracket checks the dirty set, the HEAD tree, and that the map bytes the rows
// and the attestation subject describe are the bytes the verifier read; a
// worktree edit to the map itself is otherwise caught by the verifier's own
// target-side sidecar comparison.
func compileCEMProof(ctx context.Context, gitExecutable string, options options, dirtyBefore []string) (proveReceipt, error) {
	document, mapBytes, err := readProveMap(options.root, options.prove.mapPath)
	if err != nil {
		return proveReceipt{}, err
	}
	treeBefore, err := proveTreeRevision(ctx, gitExecutable, options.root)
	if err != nil {
		return proveReceipt{}, err
	}
	session, err := workflow.Open(options.root)
	if err != nil {
		return proveReceipt{}, cemProveError(err)
	}
	packet, err := session.Read(ctx, "verify", workflow.ReadOptions{
		MapPath: options.prove.mapPath, PatchPath: options.prove.patch, PatchGiven: options.prove.patchGiven,
		ExpectedBase: options.prove.expectedBase, Target: options.prove.target,
	})
	if err != nil {
		return proveReceipt{}, cemProveError(err)
	}
	dirtyAfter, err := affected.DirtyPaths(ctx, gitExecutable, options.root)
	if err != nil {
		return proveReceipt{}, &gokernel.Error{Code: "unsupported-prove-history", Message: "cannot read the worktree status"}
	}
	_, mapAfter, err := readProveMap(options.root, options.prove.mapPath)
	if err != nil {
		return proveReceipt{}, err
	}
	treeAfter, err := proveTreeRevision(ctx, gitExecutable, options.root)
	if err != nil {
		return proveReceipt{}, err
	}
	if treeBefore != treeAfter || !equalStringSlices(dirtyBefore, dirtyAfter) || !bytes.Equal(mapBytes, mapAfter) {
		return proveReceipt{}, &gokernel.Error{Code: "unsupported-prove-drift", Message: "the worktree, HEAD tree, or map changed while the map was being proved"}
	}
	verification, _ := packet["verification"].(map[string]any)
	rows := cemRows(document)
	for index := range rows {
		rows[index].Falsified = judgeVerifier(rows[index], verification)
	}
	summary := summarizeCEMProof(rows)
	revision := cemRevision(document, verification)
	return proveReceipt{
		Mutates: false, OK: true, Packet: packet, Profile: proveProfile, Proof: summary,
		Revision: revision, State: cemState(verification, summary), Tool: "prove",
		subjects: cemSubjects(options.prove.mapPath, mapBytes, revision), mapBytes: mapBytes,
	}, nil
}

// cemSubjects names the artifacts an attestation is about: the map bytes the
// verifier read and the target commit it judged. The prove document itself is
// the predicate, so its fields are not restated as subjects.
func cemSubjects(mapPath string, mapBytes []byte, revision string) []attest.Subject {
	mapDigest := sha256.Sum256(mapBytes)
	return []attest.Subject{
		{Name: filepath.ToSlash(mapPath), Digest: map[string]string{"sha256": hex.EncodeToString(mapDigest[:])}},
		{Name: "git:" + revision, Digest: map[string]string{"gitCommit": revision}},
	}
}

// attestProof wraps the encoded prove document as an in-toto Statement and,
// when a key path is given, signs it into a DSSE envelope. A non-nil cem adds
// the FPK-V0-031 CEM statement as a second line, signed by the same key read
// once. The key is only ever read; a missing or malformed key is an error and
// never a generated one.
func attestProof(document []byte, subjects []attest.Subject, cem *cemAttestation, keyPath string) ([]byte, error) {
	statement, err := attest.Statement(document, subjects)
	if err != nil {
		return nil, &gokernel.Error{Code: "attest-failed", Message: "cannot build the in-toto statement"}
	}
	statements := [][]byte{statement}
	if cem != nil {
		cemStatement, err := cem.statement()
		if err != nil {
			return nil, &gokernel.Error{Code: "attest-failed", Message: "cannot build the CEM statement"}
		}
		statements = append(statements, cemStatement)
	}
	if keyPath == "" {
		return bytes.Join(statements, []byte{'\n'}), nil
	}
	key, err := attest.ReadPrivateKey(keyPath)
	if err != nil {
		return nil, attestKeyRefusal(keyPath)
	}
	for index := range statements {
		if statements[index], err = attest.Envelope(statements[index], key); err != nil {
			return nil, &gokernel.Error{Code: "attest-failed", Message: "cannot sign the DSSE envelope"}
		}
	}
	return bytes.Join(statements, []byte{'\n'}), nil
}

// readProveMap reads a repository-relative map the way the verifier will: an
// absolute path or one that climbs out of the root is refused before any read.
func readProveMap(root, mapPath string) (*wire.Map, []byte, error) {
	data, err := readCEMMapBytes(root, mapPath)
	if err != nil {
		return nil, nil, err
	}
	document, err := wire.ParseMap(data)
	if err != nil {
		return nil, nil, cemProveError(err)
	}
	return document, data, nil
}

// containedRelativePath reports whether value is safe to resolve under a
// repository root: relative, non-climbing, and free of any segment that
// case-folds equal to `.git` (a case-insensitive filesystem makes `.GIT` the
// same directory as `.git`, mirroring internal/companionrelease.refuseUnsafePath).
// It is a lexical check only; readBoundedFileUnderRoot separately refuses a
// symlinked path component so a symlinked parent cannot walk the read
// outside root either.
func containedRelativePath(value string) bool {
	if value == "" || filepath.IsAbs(value) || strings.HasPrefix(value, "/") {
		return false
	}
	for _, segment := range strings.Split(filepath.ToSlash(value), "/") {
		if segment == ".." {
			return false
		}
		if strings.EqualFold(segment, ".git") {
			return false
		}
	}
	return true
}

func readBoundedFile(path string, bound int) ([]byte, error) {
	file, err := openBoundedRegularFile(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, int64(bound)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > bound {
		return nil, errors.New("map exceeds bound")
	}
	return data, nil
}

func cemProveError(err error) error {
	code := cemcode.CodeOf(err)
	if code == "" {
		code = "unsupported-prove-cem"
	}
	return &gokernel.Error{Code: code, Message: cemcode.MessageOf(err)}
}

// cemRows lists one row per hunk: the hunk id is the result, the hunk's path
// and new-range start are the citation, and the first basis evidence's blob is
// the blob hash. The disposition becomes the authority, so the table can tell
// a supported hunk from an explicit unknown.
func cemRows(document *wire.Map) []proveRow {
	blobs := map[string]string{}
	for _, evidence := range document.Evidence {
		blobs[evidence.ID] = evidence.BlobOid
	}
	rows := make([]proveRow, 0, len(document.Hunks))
	for _, hunk := range document.Hunks {
		row := proveRow{
			Authority: "cem-" + hunk.Disposition, Line: int(max(hunk.NewRange.Start, 1)), Path: hunk.Path,
			Result: hunk.ID, Kind: "cem-hunk", reason: hunk.Reason,
		}
		for _, basis := range hunk.Basis {
			row.basis = append(row.basis, basis.EvidenceID)
		}
		if len(row.basis) != 0 {
			row.BlobHash = blobs[row.basis[0]]
		}
		classifyTrust(&row)
		rows = append(rows, row)
	}
	return rows
}

// judgeVerifier is the `verifier-accepts` verdict: the map must verify, and
// every basis evidence of the hunk must be stable or relocated at the target
// when a drift check ran. The verifier's contract is whole-map acceptance, so
// a rejected map fails every claim row.
func judgeVerifier(row proveRow, verification map[string]any) string {
	if row.Falsifier != falsifierVerifier {
		return falsifiedNotRun
	}
	if verification["valid"] != true {
		return falsifiedFail
	}
	drift := map[string]string{}
	for _, item := range mapsFromAny(verification["drift"]) {
		drift[stringAt(item, "evidenceId")] = stringAt(item, "status")
	}
	for _, evidenceID := range row.basis {
		if status, checked := drift[evidenceID]; checked && status != "stable" && status != "relocated" {
			return falsifiedFail
		}
	}
	return falsifiedPass
}

func summarizeCEMProof(rows []proveRow) proveSummary {
	summary := proveSummary{Claims: proofClaims(), Counts: map[string]map[string]int{}, Rows: rows}
	for _, row := range rows {
		if summary.Counts[row.Falsifier] == nil {
			summary.Counts[row.Falsifier] = map[string]int{}
		}
		summary.Counts[row.Falsifier][row.Falsified]++
		switch {
		case row.Falsifier == falsifierNone:
			summary.UnprovenResults++
		case row.Falsified == falsifiedPass:
			summary.ProvenResults++
		case row.Falsified == falsifiedFail:
			summary.FailedResults++
		default:
			summary.UnprovenResults++
		}
	}
	return summary
}

func cemRevision(document *wire.Map, verification map[string]any) string {
	if target := stringAt(verification, "targetRevision"); validGitObjectID(target) {
		return target
	}
	return document.BaseRevision
}

// cemState: REJECTED when the verifier refused the map, UNPROVEN when it
// accepted a map with no supported hunk, READY otherwise.
func cemState(verification map[string]any, summary proveSummary) string {
	if verification["valid"] != true {
		return "REJECTED"
	}
	if summary.ProvenResults == 0 {
		return "UNPROVEN"
	}
	return "READY"
}

// provePacket compiles the wrapped command's packet on its ordinary read path,
// without the ledger observation the plain commands make on failure.
func provePacket(ctx context.Context, options options) (map[string]any, error) {
	if options.proveMode == "query" {
		return standaloneQueryContext(ctx, options)
	}
	if options.proveMode == "change" {
		index, err := snapshotOrBuild(ctx, options.root)
		if err != nil {
			return nil, err
		}
		return contextindex.RangeImpact(ctx, index, options.impactBase, options.impactLimit)
	}
	// Impact mode reads the snapshot as path impact does (IDX-SNAP-V0-020).
	return overSnapshot(
		func() *contextindex.Index { return deferredSnapshotIndex(ctx, options.root) },
		func() *contextindex.Index { return snapshotIndex(ctx, options.root) },
		func() (*contextindex.Index, error) { return contextindex.Build(ctx, options.root) },
		func(index *contextindex.Index) (map[string]any, error) {
			return contextindex.Impact(index, options.impactPaths, options.impactLimit)
		},
	)
}

// assignFalsifiers lists every evidence row under results[] with its
// falsifier, in packet order. The packet itself is never modified.
func assignFalsifiers(packet map[string]any) []proveRow {
	rows := []proveRow{}
	for _, result := range mapsFromAny(packet["results"]) {
		for _, item := range mapsFromAny(result["evidence"]) {
			row := proveRow{
				Authority: stringAt(item, "authority"), BlobHash: stringAt(item, "blob_hash"),
				Line: integerAt(item, "line"), Path: stringAt(item, "path"), Result: stringAt(result, "id"),
				Kind: stringAt(result, "kind"), reason: stringAt(item, "reason"),
			}
			classifyTrust(&row)
			rows = append(rows, row)
		}
	}
	return rows
}

func falsifierFor(row proveRow) string {
	if falsifier, ok := falsifierByKindAndAuthority[row.Kind+"|"+row.Authority]; ok {
		if falsifier == falsifierReference && referenceLanguages[filepath.Ext(row.Path)] {
			return falsifier
		}
		if falsifier == falsifierMutant && mutationTestPath(row.Path) {
			return falsifier
		}
		if falsifier != falsifierReference && falsifier != falsifierMutant {
			return falsifier
		}
	}
	if falsifier, ok := falsifierByAuthority[row.Authority]; ok {
		return falsifier
	}
	return falsifierNone
}

// falsifyRow is the `history-consistent` verdict for one row: PASS when Git
// holds a blob with the cited id at the cited path in the packet's tree and
// the cited line exists; FAIL when the object is missing, is not a blob, has
// another id, or has no such line; NOT_RUN when the path is dirty in the
// worktree, because the committed blob is not what the agent will read on
// disk, or when the path cannot be framed for Git. Rows with no falsifier are
// always NOT_RUN.
func falsifyRow(row proveRow, cited map[string]citedBlob, dirty map[string]struct{}) string {
	switch row.Falsifier {
	case falsifierHistory:
		return judgeHistory(row, cited, dirty)
	case falsifierReference:
		return judgeReference(row, cited, dirty)
	}
	return falsifiedNotRun
}

func judgeHistory(row proveRow, cited map[string]citedBlob, dirty map[string]struct{}) string {
	if !verifiablePath(row.Path, dirty) {
		return falsifiedNotRun
	}
	blob, present := cited[row.Path]
	if !present || blob.objectType != "blob" || blob.oid != row.BlobHash {
		return falsifiedFail
	}
	if row.Line < 1 || row.Line > blob.lines {
		return falsifiedFail
	}
	return falsifiedPass
}

// judgeReference is the `reference-resolves` verdict for Go. The citing blob
// must hold the cited blob id and must parse; an import row passes when an
// import spec with the claimed path sits on the cited line, and a reference
// row passes when an identifier with the claimed name sits on the cited line
// and the declaring blob declares that name at top level. It judges
// identifier occurrence, not binding: a same-named identifier bound elsewhere
// still passes.
func judgeReference(row proveRow, cited map[string]citedBlob, dirty map[string]struct{}) string {
	if !verifiablePath(row.Path, dirty) {
		return falsifiedNotRun
	}
	citing, present := cited[row.Path]
	if !present || citing.objectType != "blob" || citing.oid != row.BlobHash {
		return falsifiedFail
	}
	if imported, ok := strings.CutPrefix(row.reason, "imports "); ok {
		return verdict(importsAtLine(row.Path, citing.content, imported, row.Line))
	}
	name, declaredBy, ok := parseReferenceClaim(row.reason)
	if !ok {
		return falsifiedFail
	}
	if filepath.Ext(row.Path) != ".go" && !jsPath(row.Path) {
		return falsifiedNotRun
	}
	if !verifiablePath(declaredBy, dirty) {
		return falsifiedNotRun
	}
	declaring, present := cited[declaredBy]
	if !present || declaring.objectType != "blob" {
		return falsifiedFail
	}
	return verdict(referenceResolves(row.Path, citing.content, declaring.content, name, row.Line))
}

// mutationVerdict is one `test-kills-mutant` judgment: the row verdict and
// the runner's deterministic detail line.
type mutationVerdict struct {
	falsified, detail string
	witness           *proveMutationWitness
}

// judgeMutations runs the mutation falsifier for every `test-kills-mutant`
// row when --mutate was given. Each row claims that the cited test covers the
// changed file, so the runner exports the packet revision, mutates the
// changed file, and asks whether that test notices: KILLED passes, SURVIVED
// fails, and a run that could not reach a judgment stays NOT_RUN. The cited
// test blob must sit at the cited path in the packet revision, else the claim
// is FAIL before anything runs. The packet revision is exported once and
// every row is judged on that copy, sandboxed, sharing its build cache; the
// copy is removed afterwards. Runs happen inside the drift bracket, and a
// runner failure is an error rather than a verdict. Without --mutate every
// such row stays NOT_RUN. Go rows that claim the same changed path and reach
// the runner are judged together (`mutate.JudgeGroup`: one run per package
// per mutant) when the first of them is reached in document order, so the
// invocation budget is checked once for the group; every other row goes
// through judgeMutation alone.
func judgeMutations(ctx context.Context, gitExecutable string, options options, revision, checkoutRevision string, rows []proveRow, cited map[string]citedBlob, dirty map[string]struct{}, spans map[string][]mutate.LineSpan) (map[int]mutationVerdict, error) {
	verdicts := map[int]mutationVerdict{}
	if !options.proveMutate {
		return verdicts, nil
	}
	if !anyMutantRow(rows) {
		return verdicts, nil
	}
	budgeted, cancel := context.WithTimeout(ctx, proveBoundsFrom(ctx).mutationBudget)
	defer cancel()
	exported, err := mutate.Open(budgeted, mutate.Request{Root: options.root, Git: gitExecutable, Revision: revision})
	if err != nil && budgeted.Err() != nil && ctx.Err() == nil {
		return exhaustedVerdicts(rows), nil
	}
	if err != nil {
		return nil, &gokernel.Error{Code: "unsupported-prove-mutation", Message: "the mutation runner could not export the packet revision"}
	}
	defer exported.Close()
	groups := mutationGroups(rows, cited, dirty, spans)
	for index, row := range rows {
		if row.Falsifier != falsifierMutant {
			continue
		}
		if _, judged := verdicts[index]; judged {
			continue
		}
		if budgeted.Err() != nil && ctx.Err() == nil {
			verdicts[index] = mutationVerdict{
				falsified: falsifiedNotRun, detail: budgetExhaustedDetail,
				witness: mutationWitnessNotProduced("mutation-budget-exhausted"),
			}
			continue
		}
		if group, grouped := groups[index]; grouped {
			if err := judgeMutationGroup(budgeted, exported, group, rows, checkoutRevision, cited, verdicts); err != nil {
				return nil, &gokernel.Error{Code: "unsupported-prove-mutation", Message: "the mutation runner failed for " + group.changed}
			}
			continue
		}
		judged, err := judgeMutation(budgeted, exported, row, checkoutRevision, cited, dirty, spans)
		if err != nil {
			return nil, &gokernel.Error{Code: "unsupported-prove-mutation", Message: "the mutation runner failed for " + row.Path}
		}
		verdicts[index] = judged
	}
	return verdicts, nil
}

// mutationGroup is every Go mutation row of one changed path that reaches the
// runner, in document order, with the lines its mutants are confined to.
type mutationGroup struct {
	changed string
	lines   []mutate.LineSpan
	rows    []int
}

// mutationGroups keys each runnable Go row by the changed path it claims;
// every row of one path shares one group, found under the index of its first
// row. A row judgeMutation decides before any run (an unrecognized claim, a
// dirty path, a misplaced blob, a path the range gave no hunk) is left out
// and keeps that path.
func mutationGroups(rows []proveRow, cited map[string]citedBlob, dirty map[string]struct{}, spans map[string][]mutate.LineSpan) map[int]*mutationGroup {
	byPath := map[string]*mutationGroup{}
	groups := map[int]*mutationGroup{}
	for index, row := range rows {
		if row.Falsifier != falsifierMutant || !strings.HasSuffix(row.Path, "_test.go") {
			continue
		}
		changed, lines, runnable := mutationGroupKey(row, cited, dirty, spans)
		if !runnable {
			continue
		}
		group, known := byPath[changed]
		if !known {
			group = &mutationGroup{changed: changed, lines: lines}
			byPath[changed] = group
		}
		group.rows = append(group.rows, index)
		groups[index] = group
	}
	return groups
}

// mutationGroupKey repeats judgeMutation's pre-run checks without deciding
// the row: it names the changed path and confined lines a row would be run
// against, or reports that judgeMutation decides it before any run.
func mutationGroupKey(row proveRow, cited map[string]citedBlob, dirty map[string]struct{}, spans map[string][]mutate.LineSpan) (string, []mutate.LineSpan, bool) {
	changed, ok := testClaimTarget(row.reason)
	if !ok {
		return "", nil, false
	}
	if !verifiablePath(row.Path, dirty) || !verifiablePath(changed, dirty) {
		return "", nil, false
	}
	citing, present := cited[row.Path]
	if !present || citing.objectType != "blob" || citing.oid != row.BlobHash {
		return "", nil, false
	}
	lines, ok := confinedSpans(spans, changed)
	if !ok {
		return "", nil, false
	}
	return changed, lines, true
}

// judgeMutationGroup judges one group's rows together and records each row's
// verdict from the report of its own test path.
func judgeMutationGroup(ctx context.Context, exported *mutate.Export, group *mutationGroup, rows []proveRow, checkoutRevision string, cited map[string]citedBlob, verdicts map[int]mutationVerdict) error {
	tests := make([]string, 0, len(group.rows))
	for _, index := range group.rows {
		tests = append(tests, rows[index].Path)
	}
	reports, err := exported.JudgeGroup(ctx, group.changed, group.lines, tests)
	if err != nil {
		return err
	}
	for _, index := range group.rows {
		report := reports[rows[index].Path]
		verdicts[index] = mutationVerdict{
			falsified: mutationFalsified(report.Verdict), detail: report.Detail,
			witness: mutationWitnessFromReport(report, group.changed, checkoutRevision, cited),
		}
	}
	return nil
}

// exhaustedVerdicts leaves every mutation row NOT_RUN: the invocation budget
// was spent before the packet revision was even exported.
func exhaustedVerdicts(rows []proveRow) map[int]mutationVerdict {
	verdicts := map[int]mutationVerdict{}
	for index, row := range rows {
		if row.Falsifier == falsifierMutant {
			verdicts[index] = mutationVerdict{
				falsified: falsifiedNotRun, detail: budgetExhaustedDetail,
				witness: mutationWitnessNotProduced("mutation-budget-exhausted"),
			}
		}
	}
	return verdicts
}

func anyMutantRow(rows []proveRow) bool {
	for _, row := range rows {
		if row.Falsifier == falsifierMutant {
			return true
		}
	}
	return false
}

func judgeMutation(ctx context.Context, exported *mutate.Export, row proveRow, checkoutRevision string, cited map[string]citedBlob, dirty map[string]struct{}, spans map[string][]mutate.LineSpan) (mutationVerdict, error) {
	changed, ok := testClaimTarget(row.reason)
	if !ok {
		return mutationVerdict{falsified: falsifiedFail, detail: "unrecognized test claim", witness: mutationWitnessNotProduced("invalid-mutation-claim")}, nil
	}
	if !verifiablePath(row.Path, dirty) || !verifiablePath(changed, dirty) {
		return mutationVerdict{falsified: falsifiedNotRun, detail: "a cited path is dirty in the worktree", witness: mutationWitnessNotProduced("mutation-input-unavailable")}, nil
	}
	citing, present := cited[row.Path]
	if !present || citing.objectType != "blob" || citing.oid != row.BlobHash {
		return mutationVerdict{falsified: falsifiedFail, detail: "the cited test blob is not at the cited path in the packet revision", witness: mutationWitnessNotProduced("mutation-input-unavailable")}, nil
	}
	lines, ok := confinedSpans(spans, changed)
	if !ok {
		return mutationVerdict{falsified: falsifiedNotRun, detail: "the range changed no lines of " + changed, witness: mutationWitnessNotProduced("no-mutant-in-range")}, nil
	}
	if strings.HasSuffix(changed, ".py") {
		judged, err := judgePythonMutation(ctx, exported, changed, row.Path, lines)
		judged.witness = mutationWitnessNotProduced("python-mutation-witness-not-produced")
		return judged, err
	}
	report, err := exported.Judge(ctx, mutate.Request{ChangedPath: changed, TestPath: row.Path, Lines: lines})
	if err != nil {
		return mutationVerdict{}, err
	}
	return mutationVerdict{
		falsified: mutationFalsified(report.Verdict), detail: report.Detail,
		witness: mutationWitnessFromReport(report, changed, checkoutRevision, cited),
	}, nil
}

func mutationWitnessFromReport(report mutate.Report, changed, checkoutRevision string, cited map[string]citedBlob) *proveMutationWitness {
	if report.Verdict == mutate.Unsupported && strings.HasPrefix(report.Detail, "cited tests cannot be sandboxed:") {
		return mutationWitnessNotProduced("mutation-runner-unavailable")
	}
	if report.Verdict != mutate.Killed || report.Witness == nil {
		return mutationWitnessNotProduced("no-replayable-kill-observed")
	}
	blob, present := cited[changed]
	if !present || blob.objectType != "blob" || checkoutRevision == "" {
		return mutationWitnessNotProduced("mutation-input-unavailable")
	}
	witness := report.Witness
	if witness.Operator == "" || witness.KillingTest == "" || witness.End <= witness.Start {
		return mutationWitnessNotProduced("mutation-input-unavailable")
	}
	return &proveMutationWitness{
		Status: "PRODUCED", CheckoutRevision: checkoutRevision,
		Mutant: &proveWitnessMutant{
			Path: changed, Blob: blob.oid,
			ByteSpan:         proveWitnessSpan{Start: witness.Start, End: witness.End},
			MutationOperator: witness.Operator,
		},
		KillingTest: witness.KillingTest, ObservedKill: "KILLED",
	}
}

func mutationWitnessNotProduced(reason string) *proveMutationWitness {
	return &proveMutationWitness{Status: "NOT_PRODUCED", Reason: reason}
}

// confinedSpans returns the lines a mutant of changed may touch: every line in
// impact mode, which has no span map, else the range's hunks in changed. A
// change-mode path with no hunk (a mode-only change, or a header the diff
// parser did not recognise) admits no mutant at all, so a row can never pass on
// a mutant outside the change.
func confinedSpans(spans map[string][]mutate.LineSpan, changed string) ([]mutate.LineSpan, bool) {
	if spans == nil {
		return nil, true
	}
	lines, present := spans[changed]
	return lines, present
}

// testClaimTarget reads the changed path a test row claims to cover, from
// either claim form: impact's same-package row or change mode's affected row.
func testClaimTarget(reason string) (string, bool) {
	if changed, ok := strings.CutPrefix(reason, samePackageClaim); ok {
		return changed, true
	}
	return strings.CutPrefix(reason, affectedClaimPrefix)
}

// mutationFalsified maps the runner's verdict onto the row vocabulary: a
// kill passes, a survivor fails, and NO_MUTANTS, BUDGET_EXCEEDED and
// UNSUPPORTED reached no judgment.
func mutationFalsified(verdict mutate.Verdict) string {
	byVerdict := map[mutate.Verdict]string{mutate.Killed: falsifiedPass, mutate.Survived: falsifiedFail}
	if falsified, ok := byVerdict[verdict]; ok {
		return falsified
	}
	return falsifiedNotRun
}

// parseReferenceClaim reads `references NAME declared by PATH`, the reason
// text an impact reference row carries.
func parseReferenceClaim(reason string) (name, declaredBy string, ok bool) {
	rest, ok := strings.CutPrefix(reason, "references ")
	if !ok {
		return "", "", false
	}
	name, declaredBy, ok = strings.Cut(rest, " declared by ")
	if !ok || name == "" || declaredBy == "" {
		return "", "", false
	}
	return name, declaredBy, true
}

// importsAtLine dispatches the import claim on the citing file's language.
func importsAtLine(path string, content []byte, imported string, line int) bool {
	if filepath.Ext(path) == ".py" {
		return pythonImportsAtLine(path, content, imported, line)
	}
	if jsPath(path) {
		return jsImportsAtLine(content, imported, line)
	}
	return goImportsAtLine(content, imported, line)
}

// pythonImportsAtLine accepts an absolute spelling the statement binds, or a
// relative import that resolves to the claimed module from the citing file's
// package. The package is the file's directory with a leading `src`/`lib`
// dropped, the same rule the index used to resolve the claim.
func pythonImportsAtLine(path string, content []byte, imported string, line int) bool {
	if pyresolve.ImportsAtLine(content, imported, line) {
		return true
	}
	imports, err := pyresolve.Imports(content)
	if err != nil {
		return false
	}
	importingPackage := pythonImportingPackage(path)
	for _, statement := range imports {
		if statement.Line != line {
			continue
		}
		for _, name := range statement.Names {
			if resolvePythonRelative(importingPackage, name) == imported {
				return true
			}
		}
	}
	return false
}

func resolvePythonRelative(importingPackage, name string) string {
	dots := len(name) - len(strings.TrimLeft(name, "."))
	if dots == 0 {
		return ""
	}
	return pyresolve.ResolveRelative(importingPackage, dots, name[dots:])
}

func pythonImportingPackage(path string) string {
	parts := strings.Split(strings.TrimSuffix(path, ".py"), "/")
	if parts[0] == "src" || parts[0] == "lib" {
		parts = parts[1:]
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts[:len(parts)-1], ".")
}

func goImportsAtLine(content []byte, imported string, line int) bool {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "", content, parser.ImportsOnly)
	if err != nil {
		return false
	}
	for _, spec := range file.Imports {
		path, unquoteErr := strconv.Unquote(spec.Path.Value)
		if unquoteErr == nil && path == imported && fileSet.Position(spec.Path.Pos()).Line == line {
			return true
		}
	}
	return false
}

func goIdentifierAtLine(content []byte, name string, line int) bool {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "", content, parser.SkipObjectResolution)
	if err != nil {
		return false
	}
	found := false
	ast.Inspect(file, func(node ast.Node) bool {
		identifier, ok := node.(*ast.Ident)
		if ok && identifier.Name == name && fileSet.Position(identifier.Pos()).Line == line {
			found = true
		}
		return !found
	})
	return found
}

// goDeclaresAtTopLevel mirrors the index's own notion of a Go symbol: a
// function or method name, a type name, or a var/const name, at top level.
func goDeclaresAtTopLevel(content []byte, name string) bool {
	file, err := parser.ParseFile(token.NewFileSet(), "", content, parser.SkipObjectResolution)
	if err != nil {
		return false
	}
	for _, declaration := range file.Decls {
		if stringIn(topLevelNames(declaration), name) {
			return true
		}
	}
	return false
}

func topLevelNames(declaration ast.Decl) []string {
	switch typed := declaration.(type) {
	case *ast.FuncDecl:
		return []string{typed.Name.Name}
	case *ast.GenDecl:
		names := []string{}
		for _, spec := range typed.Specs {
			switch typedSpec := spec.(type) {
			case *ast.TypeSpec:
				names = append(names, typedSpec.Name.Name)
			case *ast.ValueSpec:
				for _, identifier := range typedSpec.Names {
					names = append(names, identifier.Name)
				}
			}
		}
		return names
	}
	return nil
}

func verdict(passed bool) string {
	if passed {
		return falsifiedPass
	}
	return falsifiedFail
}

// verifiablePath: a dirty path is not what the agent will read on disk, and an
// unframable path can never be asked of Git; both are NOT_RUN, never FAIL.
func verifiablePath(path string, dirty map[string]struct{}) bool {
	if _, isDirty := dirty[path]; isDirty {
		return false
	}
	return framablePath(path)
}

// framablePath rejects paths that the LF-delimited cat-file batch protocol
// cannot carry. The index admits them (its tree listing is NUL-delimited), so
// they are refused here rather than mis-framed.
func framablePath(path string) bool {
	return path != "" && !strings.ContainsAny(path, "\n\r")
}

func summarizeProof(packet map[string]any, rows []proveRow) proveSummary {
	summary := proveSummary{Claims: proofClaims(), Counts: map[string]map[string]int{}, Rows: rows}
	for _, row := range rows {
		if summary.Counts[row.Falsifier] == nil {
			summary.Counts[row.Falsifier] = map[string]int{}
		}
		summary.Counts[row.Falsifier][row.Falsified]++
	}
	for _, result := range mapsFromAny(packet["results"]) {
		summary.count(resultVerdict(stringAt(result, "kind"), stringAt(result, "id"), rows))
	}
	for _, path := range affectedPaths(rows) {
		summary.count(path.verdict())
	}
	return summary
}

// count folds one result's verdict into the proven, failed, or unproven tally.
func (summary *proveSummary) count(verdict string) {
	switch verdict {
	case falsifiedPass:
		summary.ProvenResults++
	case falsifiedFail:
		summary.FailedResults++
	default:
		summary.UnprovenResults++
	}
}

// affectedPaths folds the affected-test rows per changed path (FPK-V0-017,
// decision 0018): each row keeps its own verdict, and the packet's question,
// whether the change is covered, is answered once per path, so a covered
// path reached by many tests that do not pin it is one proven result rather
// than one failure per reached test.
func affectedPaths(rows []proveRow) []proveAffectedPath {
	byPath := map[string]*proveAffectedPath{}
	for _, row := range rows {
		if row.Kind != kindAffectedTest {
			continue
		}
		changed, _ := testClaimTarget(row.reason)
		entry := byPath[changed]
		if entry == nil {
			entry = &proveAffectedPath{CoveredBy: []string{}, Path: changed}
			byPath[changed] = entry
		}
		entry.fold(row)
	}
	paths := make([]proveAffectedPath, 0, len(byPath))
	for _, entry := range byPath {
		sort.Strings(entry.CoveredBy)
		paths = append(paths, *entry)
	}
	sort.Slice(paths, func(i, j int) bool { return paths[i].Path < paths[j].Path })
	return paths
}

// fold adds one row's verdict to the path's tally.
func (path *proveAffectedPath) fold(row proveRow) {
	switch row.Falsified {
	case falsifiedPass:
		path.CoveredBy = append(path.CoveredBy, row.Path)
	case falsifiedFail:
		path.ReachedNotCovering++
	default:
		path.NotRun++
	}
}

// resultVerdict: a result is proven only when it has at least one
// falsifier-bearing row and every such row passed; it fails when any row
// failed; a NOT_RUN row, or no falsifier at all, leaves it unproven.
func resultVerdict(kind, resultID string, rows []proveRow) string {
	verdict := falsifiedNotRun
	for _, row := range rows {
		if row.Kind != kind || row.Result != resultID || row.Falsifier == falsifierNone {
			continue
		}
		switch row.Falsified {
		case falsifiedFail:
			return falsifiedFail
		case falsifiedNotRun:
			return falsifiedNotRun
		}
		verdict = falsifiedPass
	}
	return verdict
}

func proveState(packetState string, summary proveSummary) string {
	if packetState == "READY" && citedSyntaxOnly(summary.Rows) {
		return "CITED"
	}
	if packetState == "READY" && summary.ProvenResults == 0 {
		return "UNPROVEN"
	}
	return packetState
}

func citedSyntaxOnly(rows []proveRow) bool {
	found := false
	for _, row := range rows {
		if row.Falsifier == falsifierNone {
			continue
		}
		if row.Authority != "syntax" || row.Falsifier != falsifierHistory {
			return false
		}
		found = true
	}
	return found
}

func proofClaims() map[string]string {
	return map[string]string{
		falsifierHistory:   "the cited blob and line still exist at the packet revision; this is a check of the citation, not evidence that the row is relevant",
		falsifierReference: "the cited line holds the claimed import or identifier and the declaring blob declares it; this is a check of the reference, not evidence that the row is relevant",
		falsifierVerifier:  "the frozen CEM verifier accepted the map and this hunk's evidence is stable or relocated at the target",
		falsifierMutant:    "the cited test failed on a mutant of the changed lines and passed on the baseline",
		falsifierNone:      "nothing about this row was checked",
	}
}

// citedPaths names every path a falsifier will ask Git about: the cited path
// of each falsifier-bearing row and, for reference rows, the declaring path.
// An affected-test row is cited whatever its falsifier: FPK-V0-017 binds its
// `blob_hash` to the test's blob at the revision even when nothing is checked.
func citedPaths(rows []proveRow, includeMutationTargets bool) []string {
	paths := map[string]struct{}{}
	for _, row := range rows {
		if row.Kind == kindAffectedTest && framablePath(row.Path) {
			paths[row.Path] = struct{}{}
		}
		if row.Falsifier == falsifierNone {
			continue
		}
		if framablePath(row.Path) {
			paths[row.Path] = struct{}{}
		}
		if changed, ok := testClaimTarget(row.reason); includeMutationTargets && row.Falsifier == falsifierMutant && ok && framablePath(changed) {
			paths[changed] = struct{}{}
		}
		if _, declaredBy, ok := parseReferenceClaim(row.reason); ok && row.Falsifier == falsifierReference && framablePath(declaredBy) {
			paths[declaredBy] = struct{}{}
		}
	}
	return sortedKeys(paths)
}

const proveGitDeadline = 30 * time.Second

func proveTreeRevision(ctx context.Context, gitExecutable, root string) (string, error) {
	deadline, cancel := context.WithTimeout(ctx, proveGitDeadline)
	defer cancel()
	command := hermeticGitCommand(deadline, gitExecutable, root, "rev-parse", "--verify", "--quiet", "HEAD^{tree}")
	output, err := command.Output()
	tree := string(bytes.TrimSpace(output))
	if err != nil || !validGitObjectID(tree) {
		return "", &gokernel.Error{Code: "unsupported-prove-revision", Message: "repository has no resolvable HEAD tree"}
	}
	return tree, nil
}

// readCitedBlobs asks `git cat-file --batch` for `<revision>:<path>` of every
// cited path in one bounded, sanitized invocation and records the object id,
// type, and line count of each.
func readCitedBlobs(ctx context.Context, gitExecutable, root, revision string, paths []string) (map[string]citedBlob, error) {
	cited := map[string]citedBlob{}
	if len(paths) == 0 {
		return cited, nil
	}
	deadline, cancel := context.WithTimeout(ctx, proveGitDeadline)
	defer cancel()
	command := hermeticGitCommand(deadline, gitExecutable, root, "cat-file", "--batch")
	command.Stdin = strings.NewReader(revision + ":" + strings.Join(paths, "\n"+revision+":") + "\n")
	output, err := boundedOutput(command, proveBoundsFrom(ctx).blobBytes)
	if errors.Is(err, errOutputBound) {
		return nil, &gokernel.Error{Code: "unsupported-prove-history", Message: "cited blobs exceed the proof output bound"}
	}
	if err != nil {
		return nil, &gokernel.Error{Code: "unsupported-prove-history", Message: "cannot read cited blobs at the packet revision"}
	}
	return parseCatFileBatch(output, revision, paths)
}

// errOutputBound reports a Git stream that ran past its byte bound.
var errOutputBound = errors.New("output exceeds the byte bound")

// boundedOutput runs command and returns its stdout, reading at most one byte
// past bound: a stream that reaches that byte is errOutputBound and the
// process is killed, so memory is bounded before the bound rejects rather
// than after the whole stream is buffered.
func boundedOutput(command *exec.Cmd, bound int) ([]byte, error) {
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := command.Start(); err != nil {
		return nil, err
	}
	output, readErr := io.ReadAll(io.LimitReader(stdout, int64(bound)+1))
	if len(output) > bound {
		_ = command.Process.Kill()
		_ = command.Wait()
		return nil, errOutputBound
	}
	if readErr != nil {
		_ = command.Wait()
		return nil, readErr
	}
	if err := command.Wait(); err != nil {
		return nil, err
	}
	return output, nil
}

// parseCatFileBatch walks the --batch stream in request order: a header line
// `<oid> <type> <size>` followed by <size> bytes and a newline, or
// `<object> missing` for an object Git does not hold.
func parseCatFileBatch(output []byte, revision string, paths []string) (map[string]citedBlob, error) {
	cited := map[string]citedBlob{}
	rest := output
	for _, path := range paths {
		header, remainder, found := bytes.Cut(rest, []byte{'\n'})
		if !found {
			return nil, malformedCatFile()
		}
		if bytes.Equal(header, []byte(revision+":"+path+" missing")) {
			rest = remainder
			continue
		}
		fields := strings.Fields(string(header))
		if len(fields) != 3 {
			return nil, malformedCatFile()
		}
		size, err := strconv.Atoi(fields[2])
		if err != nil || size < 0 || size >= len(remainder) {
			return nil, malformedCatFile()
		}
		if remainder[size] != '\n' {
			return nil, malformedCatFile()
		}
		content := remainder[:size]
		cited[path] = citedBlob{oid: fields[0], objectType: fields[1], content: content, lines: countLines(content)}
		rest = remainder[size+1:]
	}
	if len(rest) != 0 {
		return nil, malformedCatFile()
	}
	return cited, nil
}

func malformedCatFile() error {
	return &gokernel.Error{Code: "unsupported-prove-history", Message: "git cat-file produced an unreadable batch stream"}
}

// countLines treats an empty blob as one line so a line-1 citation of an
// empty file, which names the file rather than a span, still resolves.
func countLines(content []byte) int {
	lines := bytes.Count(content, []byte{'\n'})
	if len(content) > 0 && content[len(content)-1] != '\n' {
		lines++
	}
	return max(lines, 1)
}

// hermeticGitCommand is the one Git read shape in cmd/corvint (FPK-V0-052):
// the -c set of internal/contextindex gitRaw plus core.hooksPath, so no
// repository-configured program runs; replace refs ignored, so cited bytes are
// the named objects. Discovery stops at root's parent only when root holds its own
// .git entry; a subdirectory root still reaches its enclosing worktree.
func hermeticGitCommand(ctx context.Context, gitExecutable, root string, arguments ...string) *exec.Cmd {
	prefix := []string{"--no-optional-locks", "-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false",
		"-c", "core.excludesFile=", "-c", "credential.helper=", "-c", "submodule.recurse=false",
		"-c", "core.hooksPath=/dev/null", "-C", root}
	command := exec.CommandContext(ctx, gitExecutable, append(prefix, arguments...)...)
	command.Env = []string{
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "GIT_NO_LAZY_FETCH=1",
		"GIT_NO_REPLACE_OBJECTS=1", "GIT_ALLOW_PROTOCOL=", "LANG=C", "LC_ALL=C",
	}
	if _, err := os.Lstat(filepath.Join(root, ".git")); err == nil {
		command.Env = append(command.Env, "GIT_CEILING_DIRECTORIES="+filepath.Dir(root))
	}
	return command
}

func mapsFromAny(value any) []map[string]any {
	items, _ := value.([]any)
	maps := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if entry, ok := item.(map[string]any); ok {
			maps = append(maps, entry)
		}
	}
	return maps
}

func stringAt(entry map[string]any, key string) string {
	value, _ := entry[key].(string)
	return value
}

func integerAt(entry map[string]any, key string) int {
	switch value := entry[key].(type) {
	case int:
		return value
	case float64:
		return int(value)
	}
	return 0
}

func stringIn(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func stringSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}

func sortedKeys(set map[string]struct{}) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
