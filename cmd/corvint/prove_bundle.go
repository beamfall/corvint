package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"os/exec"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/secretscreen"
)

// Failure-reproduction bundles (FPK-V0-041..049, decision 0361). `prove
// --export-bundle` freezes one query or checkpoint result with the exact
// request, the Git identities and the engine that produced it; `prove
// --replay-bundle FILE` reruns the frozen request on another checkout and
// compares. Both are read commands: the bundle goes to stdout, nothing is
// written, and every document carries `historical: true`.
const (
	bundleVersion         = "corvint-failure-bundle/0"
	bundleReplayProfile   = "corvint-failure-replay/0"
	bundleDifferenceBound = 64
)

// bundleExcluded is the declared set of members a replay never compares
// (FPK-V0-046): the local ledger is never bundled, and a refusal message may
// carry checkout-local paths.
var bundleExcluded = []string{"error.message", "proof.ledger"}

// bundleModes are the only wrapped modes a bundle freezes (FPK-V0-041).
var bundleModes = map[string]bool{"query": true, "checkpoint": true}

// bundleCheckpointKey carries frozen checkpoint bytes to compileCheckpointProof
// so neither export nor replay rereads a caller path.
type bundleCheckpointKey struct{}

type proveBundle struct {
	Digest     string           `json:"bundle_sha256"`
	Engine     bundleEngine     `json:"engine"`
	Historical bool             `json:"historical"`
	Inputs     bundleInputs     `json:"inputs"`
	Original   bundleResult     `json:"original"`
	Repository bundleRepository `json:"repository"`
	Request    bundleRequest    `json:"request"`
	Tool       string           `json:"tool"`
	Version    string           `json:"version"`
}

type bundleEngine struct {
	Build          string `json:"build"`
	CorvintVersion string `json:"corvint_version"`
	ProveProfile   string `json:"prove_profile"`
}

type bundleInputs struct {
	CheckpointDocument *string `json:"checkpoint_document,omitempty"`
}

type bundleRepository struct {
	Blobs        []bundleBlob `json:"blobs"`
	Commit       string       `json:"commit"`
	ObjectFormat string       `json:"object_format"`
	Tree         string       `json:"tree"`
}

type bundleBlob struct {
	OID  string `json:"oid"`
	Path string `json:"path"`
}

type bundleRequest struct {
	Arguments []string `json:"arguments"`
	Mode      string   `json:"mode"`
}

type bundleResult struct {
	Error         *bundleError   `json:"error,omitempty"`
	Exit          int            `json:"exit"`
	Receipt       map[string]any `json:"receipt,omitempty"`
	ReceiptSHA256 string         `json:"receipt_sha256,omitempty"`
}

type bundleError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// proveBundleRequest splits `[--root P] prove REST` and names the bundle flag
// REST carries before `--`: "export", "replay", or "" for ordinary prove.
func proveBundleRequest(arguments []string) (roots, rest []string, kind string) {
	position := commandPositionAfterRoots(arguments)
	if position < 0 || arguments[position] != "prove" {
		return nil, nil, ""
	}
	roots, rest = arguments[:position], arguments[position+1:]
	for _, argument := range rest {
		if argument == "--" {
			break
		}
		if argument == "--replay-bundle" || strings.HasPrefix(argument, "--replay-bundle=") {
			return roots, rest, "replay"
		}
		if argument == "--export-bundle" {
			kind = "export"
		}
	}
	return roots, rest, kind
}

// parseProveBundleInvocation admits the two bundle forms and otherwise
// delegates to parseProveInvocation unchanged.
func parseProveBundleInvocation(arguments []string) (options, bool, error) {
	if _, requested, _ := parseHelpInvocation(arguments); requested {
		return options{}, false, nil
	}
	roots, rest, kind := proveBundleRequest(arguments)
	if kind == "replay" {
		if _, err := replayBundleFile(rest); err != nil {
			return options{}, true, err
		}
		parsed, err := parseProveCheckpointArguments(roots, nil)
		parsed.proveMode = "replay-bundle"
		return parsed, true, err
	}
	if kind != "export" {
		return parseProveInvocation(arguments)
	}
	kept, count := withoutExportFlag(rest)
	if count > 1 {
		return options{}, true, argumentError("argument --export-bundle: may not be repeated")
	}
	parsed, _, err := parseProveInvocation(append(append(append([]string{}, roots...), "prove"), kept...))
	if err != nil {
		return parsed, true, err
	}
	if !bundleModes[parsed.proveMode] {
		return parsed, true, argumentError("argument --export-bundle: only --task and --checkpoint results are exported")
	}
	return parsed, true, nil
}

// replayBundleFile admits exactly `--replay-bundle FILE` or `--replay-bundle=FILE`.
func replayBundleFile(rest []string) (string, error) {
	if len(rest) == 1 && strings.HasPrefix(rest[0], "--replay-bundle=") && rest[0] != "--replay-bundle=" {
		return strings.TrimPrefix(rest[0], "--replay-bundle="), nil
	}
	if len(rest) == 2 && rest[0] == "--replay-bundle" && !argparseOptionLike(rest[1]) && rest[1] != "" {
		return rest[1], nil
	}
	return "", argumentError("argument --replay-bundle: expected exactly one FILE and no other arguments")
}

// withoutExportFlag lifts `--export-bundle` before `--` and counts it.
func withoutExportFlag(arguments []string) ([]string, int) {
	kept := make([]string, 0, len(arguments))
	count := 0
	positional := false
	for _, argument := range arguments {
		if argument == "--" {
			positional = true
		}
		if argument == "--export-bundle" && !positional {
			count++
			continue
		}
		kept = append(kept, argument)
	}
	return kept, count
}

func runProveBundle(ctx context.Context, arguments []string, parsed options, stdout, stderr io.Writer) int {
	_, rest, kind := proveBundleRequest(arguments)
	if kind == "" {
		return runProve(ctx, parsed, stdout, stderr)
	}
	var encoded []byte
	var err error
	exit := 0
	if kind == "export" {
		kept, _ := withoutExportFlag(rest)
		encoded, err = exportProveBundle(ctx, parsed, kept)
	} else {
		filename, _ := replayBundleFile(rest)
		encoded, exit, err = replayProveBundle(ctx, parsed.root, filename)
	}
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	if _, err := stdout.Write(append(encoded, '\n')); err != nil {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot write failure bundle"})
		return 2
	}
	return exit
}

// exportProveBundle freezes one result on a clean checkout (FPK-V0-041..043).
func exportProveBundle(ctx context.Context, parsed options, recorded []string) ([]byte, error) {
	gitExecutable, err := exec.LookPath("git")
	if err != nil {
		return nil, proveGitExecutableRefusal()
	}
	commit, tree, err := checkpointRevision(ctx, gitExecutable, parsed.root)
	if err != nil {
		return nil, err
	}
	if err := bundleCleanWorktree(ctx, gitExecutable, parsed.root); err != nil {
		return nil, err
	}
	inputs, err := exportBundleInputs(parsed)
	if err != nil {
		return nil, err
	}
	original, err := bundleRun(bundleContext(ctx, inputs), parsed)
	if err != nil {
		return nil, err
	}
	head, after, err := checkpointRevision(ctx, gitExecutable, parsed.root)
	if err != nil || head != commit || after != tree {
		return nil, &gokernel.Error{Code: "unsupported-prove-drift", Message: "the worktree or HEAD changed while the bundle was being exported"}
	}
	bundle := proveBundle{
		Engine: bundleEngine{Build: build, CorvintVersion: version, ProveProfile: proveProfile}, Historical: true,
		Inputs: inputs, Original: original, Request: bundleRequest{Arguments: recorded, Mode: parsed.proveMode},
		Repository: bundleRepository{Blobs: bundleBlobs(original.Receipt), Commit: commit, ObjectFormat: bundleObjectFormat(commit), Tree: tree},
		Tool:       "prove", Version: bundleVersion,
	}
	return sealProveBundle(bundle)
}

// exportBundleInputs freezes the caller-owned checkpoint bytes; a query needs none.
func exportBundleInputs(parsed options) (bundleInputs, error) {
	if parsed.proveMode != "checkpoint" {
		return bundleInputs{}, nil
	}
	raw, err := readBoundedFile(parsed.prove.checkpointPath, checkpointByteBound)
	if err != nil {
		return bundleInputs{}, checkpointError("unreadable-checkpoint-document", "cannot read checkpoint document within 256 KiB")
	}
	if !utf8.Valid(raw) {
		return bundleInputs{}, &gokernel.Error{Code: "unsupported-bundle-input", Message: "a checkpoint document that is not UTF-8 cannot be bundled"}
	}
	document := string(raw)
	return bundleInputs{CheckpointDocument: &document}, nil
}

func bundleContext(ctx context.Context, inputs bundleInputs) context.Context {
	if inputs.CheckpointDocument == nil {
		return ctx
	}
	return context.WithValue(ctx, bundleCheckpointKey{}, []byte(*inputs.CheckpointDocument))
}

// checkpointDocumentFor reads frozen bundle bytes when the context carries
// them, and the caller's file otherwise.
func checkpointDocumentFor(ctx context.Context, filename string) (checkpointDocument, error) {
	if raw, ok := ctx.Value(bundleCheckpointKey{}).([]byte); ok {
		return decodeCheckpointDocument(raw)
	}
	return readCheckpointDocument(filename)
}

// bundleRun compiles the wrapped result exactly as prove does, minus the
// local ledger, and records a refusal as its code and message.
func bundleRun(ctx context.Context, parsed options) (bundleResult, error) {
	receipt, err := compileProof(ctx, parsed)
	if err != nil {
		return bundleResult{Error: &bundleError{Code: bundleErrorCode(err), Message: err.Error()}, Exit: 2}, nil
	}
	encoded, object, err := normalizedJSON(receipt)
	if err != nil {
		return bundleResult{}, &gokernel.Error{Code: "output-failed", Message: "cannot encode falsifiable packet"}
	}
	digest := sha256.Sum256(encoded)
	return bundleResult{Exit: 0, Receipt: object, ReceiptSHA256: hex.EncodeToString(digest[:])}, nil
}

// bundleErrorCode is the code emitError would print for err.
func bundleErrorCode(err error) string {
	var contextError *contextindex.Error
	if errors.As(err, &contextError) && contextError.Code != "" {
		return contextError.Code
	}
	var kernelError *gokernel.Error
	if errors.As(err, &kernelError) {
		return kernelError.Code
	}
	return "internal-error"
}

func bundleCleanWorktree(ctx context.Context, gitExecutable, root string) error {
	dirty, err := affected.DirtyPaths(ctx, gitExecutable, root)
	if err != nil {
		return &gokernel.Error{Code: "unsupported-prove-history", Message: "cannot read the worktree status"}
	}
	if len(dirty) > 0 {
		return &gokernel.Error{Code: "mixed-worktree", Message: "failure bundles need a clean worktree: tracked or untracked changes are present"}
	}
	return nil
}

// bundleBlobs lists every distinct (path, blob_hash) pair the receipt cites.
func bundleBlobs(receipt map[string]any) []bundleBlob {
	found := map[bundleBlob]bool{}
	collectBundleBlobs(receipt, found)
	blobs := make([]bundleBlob, 0, len(found))
	for blob := range found {
		blobs = append(blobs, blob)
	}
	sort.Slice(blobs, func(i, j int) bool {
		return blobs[i].Path+"\x00"+blobs[i].OID < blobs[j].Path+"\x00"+blobs[j].OID
	})
	return blobs
}

func collectBundleBlobs(value any, found map[bundleBlob]bool) {
	if items, ok := value.([]any); ok {
		for _, item := range items {
			collectBundleBlobs(item, found)
		}
		return
	}
	object, ok := value.(map[string]any)
	if !ok {
		return
	}
	if oid, path := stringAt(object, "blob_hash"), stringAt(object, "path"); validGitObjectID(oid) && path != "" {
		found[bundleBlob{OID: oid, Path: path}] = true
	}
	for _, member := range object {
		collectBundleBlobs(member, found)
	}
}

func bundleObjectFormat(commit string) string {
	if len(commit) == 64 {
		return "sha256"
	}
	return "sha1"
}

// sealProveBundle digests the canonical bundle without its digest member,
// then applies the size bound and the secret screen to every string value
// (FPK-V0-043).
func sealProveBundle(bundle proveBundle) ([]byte, error) {
	_, object, err := normalizedJSON(bundle)
	if err != nil {
		return nil, &gokernel.Error{Code: "output-failed", Message: "cannot encode failure bundle"}
	}
	object["bundle_sha256"] = bundleDigest(object)
	encoded, err := gokernel.CanonicalJSON(object)
	if err != nil {
		return nil, &gokernel.Error{Code: "output-failed", Message: "cannot encode failure bundle"}
	}
	if len(encoded) > maxProofDocumentBytes {
		return nil, &gokernel.Error{Code: "bundle-bound-exceeded", Message: "failure bundle exceeds 8 MiB"}
	}
	if bundleHasSecret(object) {
		return nil, &gokernel.Error{Code: "bundle-secret-detected", Message: "failure bundle contains secret-shaped text and was not written"}
	}
	return encoded, nil
}

// bundleHasSecret screens every string value; member names and numbers are
// the wire's own vocabulary, so a verdict count never reads as a credential.
func bundleHasSecret(value any) bool {
	switch typed := value.(type) {
	case string:
		return secretscreen.MatchString(typed)
	case []any:
		return slices.ContainsFunc(typed, bundleHasSecret)
	case map[string]any:
		return slices.ContainsFunc(slices.Collect(maps.Values(typed)), bundleHasSecret)
	}
	return false
}

// bundleDigest is the SHA-256 of the canonical object without bundle_sha256.
func bundleDigest(object map[string]any) string {
	unsealed := make(map[string]any, len(object))
	for key, value := range object {
		unsealed[key] = value
	}
	delete(unsealed, "bundle_sha256")
	encoded, _ := gokernel.CanonicalJSON(unsealed)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

// normalizedJSON encodes value, decodes it as a plain object with exact
// numbers, and re-encodes it, so struct field order never reaches a digest.
func normalizedJSON(value any) ([]byte, map[string]any, error) {
	encoded, err := gokernel.CanonicalJSON(value)
	if err != nil {
		return nil, nil, err
	}
	object, err := decodeJSONObject(encoded)
	if err != nil {
		return nil, nil, err
	}
	canonical, err := gokernel.CanonicalJSON(object)
	return canonical, object, err
}

func decodeJSONObject(raw []byte) (map[string]any, error) {
	var object map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&object); err != nil {
		return nil, err
	}
	if object == nil || decoder.More() {
		return nil, errors.New("not one JSON object")
	}
	return object, nil
}

func bundleRefusal(code, message string) error {
	return &gokernel.Error{Code: code, Message: message}
}

// replayProveBundle checks the bundle and this checkout in the FPK-V0-045
// order, reruns the frozen request, and reports reproduced (exit 0) or
// diverged (exit 1) with the differing member paths.
func replayProveBundle(ctx context.Context, root, filename string) ([]byte, int, error) {
	bundle, err := readProveBundle(filename)
	if err != nil {
		return nil, 0, err
	}
	gitExecutable, err := exec.LookPath("git")
	if err != nil {
		return nil, 0, proveGitExecutableRefusal()
	}
	if err := bundleCheckout(ctx, gitExecutable, root, bundle); err != nil {
		return nil, 0, err
	}
	parsed, err := bundleReplayOptions(root, bundle.Request)
	if err != nil {
		return nil, 0, err
	}
	replayed, err := bundleRun(bundleContext(ctx, bundle.Inputs), parsed)
	if err != nil {
		return nil, 0, err
	}
	if replayed.Error != nil && replayed.Error.Code == "unsupported-prove-drift" && !sameErrorCode(bundle.Original, "unsupported-prove-drift") {
		return nil, 0, bundleRefusal("drift", "the checkout moved while the bundle was being replayed")
	}
	differences := bundleDifferences(bundle.Original, replayed)
	outcome, exit := "reproduced", 0
	if len(differences) > 0 {
		outcome, exit = "diverged", 1
	}
	report := map[string]any{
		"bundle_sha256": bundle.Digest, "differences": differences, "excluded": bundleExcluded,
		"historical": true, "mode": bundle.Request.Mode, "mutates": false, "ok": true, "outcome": outcome,
		"profile": bundleReplayProfile, "repository": map[string]any{"commit": bundle.Repository.Commit, "tree": bundle.Repository.Tree},
		"tool": "prove",
	}
	encoded, err := gokernel.CanonicalJSON(report)
	if err != nil {
		return nil, 0, &gokernel.Error{Code: "output-failed", Message: "cannot encode replay report"}
	}
	return encoded, exit, nil
}

func sameErrorCode(result bundleResult, code string) bool {
	return result.Error != nil && result.Error.Code == code
}

// readProveBundle admits a bundle in the FPK-V0-045 order: invalid-bundle,
// unsupported-version, tampered, missing-input, incompatible-engine.
func readProveBundle(filename string) (proveBundle, error) {
	var bundle proveBundle
	raw, err := readBoundedFile(filename, maxProofDocumentBytes+1)
	if err != nil {
		return bundle, bundleRefusal("invalid-bundle", "cannot read a failure bundle within 8 MiB")
	}
	raw = bytes.TrimSuffix(raw, []byte{'\n'})
	object, err := decodeJSONObject(raw)
	if err != nil || !utf8.Valid(raw) || len(raw) > maxProofDocumentBytes {
		return bundle, bundleRefusal("invalid-bundle", "failure bundle must be one canonical JSON object within 8 MiB")
	}
	if canonical, err := gokernel.CanonicalJSON(object); err != nil || !bytes.Equal(canonical, raw) {
		return bundle, bundleRefusal("invalid-bundle", "failure bundle must be one canonical JSON object within 8 MiB")
	}
	if stringAt(object, "version") != bundleVersion {
		return bundle, bundleRefusal("unsupported-version", "failure bundle version is not "+bundleVersion)
	}
	if stringAt(object, "bundle_sha256") != bundleDigest(object) {
		return bundle, bundleRefusal("tampered", "bundle_sha256 does not match the bundle content")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&bundle); err != nil {
		return bundle, bundleRefusal("invalid-bundle", "failure bundle members do not match "+bundleVersion)
	}
	if err := bundleComplete(bundle); err != nil {
		return bundle, err
	}
	if err := bundleReceiptIntact(bundle.Original); err != nil {
		return bundle, err
	}
	if bundle.Engine.CorvintVersion != version || bundle.Engine.ProveProfile != proveProfile {
		return bundle, bundleRefusal("incompatible-engine", "bundle was produced by Corvint "+bundle.Engine.CorvintVersion+" "+bundle.Engine.ProveProfile+"; this is Corvint "+version+" "+proveProfile)
	}
	return bundle, nil
}

// bundleComplete names the first required member a bundle lacks.
func bundleComplete(bundle proveBundle) error {
	missing := map[string]bool{
		"historical":                  !bundle.Historical,
		"tool":                        bundle.Tool != "prove",
		"engine":                      bundle.Engine.CorvintVersion == "" || bundle.Engine.ProveProfile == "",
		"repository.commit":           !validGitObjectID(bundle.Repository.Commit),
		"repository.tree":             !validGitObjectID(bundle.Repository.Tree),
		"request.mode":                !bundleModes[bundle.Request.Mode],
		"request.arguments":           len(bundle.Request.Arguments) == 0,
		"inputs.checkpoint_document":  bundle.Request.Mode == "checkpoint" && bundle.Inputs.CheckpointDocument == nil,
		"original.receipt":            bundle.Original.Exit == 0 && (bundle.Original.Receipt == nil || bundle.Original.ReceiptSHA256 == ""),
		"original.error":              bundle.Original.Exit == 2 && (bundle.Original.Error == nil || bundle.Original.Error.Code == ""),
		"original.exit":               bundle.Original.Exit != 0 && bundle.Original.Exit != 2,
		"repository.blobs[].oid/path": !bundleBlobsValid(bundle.Repository.Blobs),
	}
	names := make([]string, 0, len(missing))
	for name, absent := range missing {
		if absent {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return nil
	}
	sort.Strings(names)
	return bundleRefusal("missing-input", "failure bundle lacks required input "+names[0])
}

func bundleBlobsValid(blobs []bundleBlob) bool {
	for _, blob := range blobs {
		if !validGitObjectID(blob.OID) || blob.Path == "" {
			return false
		}
	}
	return true
}

func bundleReceiptIntact(original bundleResult) error {
	if original.Receipt == nil {
		return nil
	}
	encoded, err := gokernel.CanonicalJSON(original.Receipt)
	digest := sha256.Sum256(encoded)
	if err != nil || hex.EncodeToString(digest[:]) != original.ReceiptSHA256 {
		return bundleRefusal("tampered", "receipt_sha256 does not match the original receipt")
	}
	return nil
}

// bundleCheckout refuses a checkout that cannot reproduce the bundle:
// missing-git-object, then drift, then mixed-worktree.
func bundleCheckout(ctx context.Context, gitExecutable, root string, bundle proveBundle) error {
	if err := bundleObjectsPresent(ctx, gitExecutable, root, bundle.Repository); err != nil {
		return err
	}
	commit, tree, err := checkpointRevision(ctx, gitExecutable, root)
	if err != nil {
		return err
	}
	if commit != bundle.Repository.Commit || tree != bundle.Repository.Tree {
		return bundleRefusal("drift", "HEAD is "+commit+"; the bundle was exported at "+bundle.Repository.Commit)
	}
	return bundleCleanWorktree(ctx, gitExecutable, root)
}

// bundleObjectsPresent asks the local object store only: the scrubbed
// environment sets GIT_NO_LAZY_FETCH, so a partial clone never fetches.
func bundleObjectsPresent(ctx context.Context, gitExecutable, root string, repository bundleRepository) error {
	objects := [][2]string{{repository.Commit, "commit"}, {repository.Tree, "tree"}}
	for _, blob := range repository.Blobs {
		objects = append(objects, [2]string{blob.OID, "blob"})
	}
	request := make([]string, 0, len(objects))
	for _, object := range objects {
		request = append(request, object[0])
	}
	deadline, cancel := context.WithTimeout(ctx, proveGitDeadline)
	defer cancel()
	command := exec.CommandContext(deadline, gitExecutable, "--no-optional-locks", "-C", root, "cat-file", "--batch-check=%(objectname) %(objecttype)")
	command.Env = scrubbedGitEnvironment()
	command.Stdin = strings.NewReader(strings.Join(request, "\n") + "\n")
	output, err := boundedOutput(command, maxProofDocumentBytes)
	if err != nil {
		return bundleRefusal("missing-git-object", "cannot read the local object store")
	}
	answered := strings.Split(strings.TrimSuffix(string(output), "\n"), "\n")
	for index, object := range objects {
		if index >= len(answered) || answered[index] != object[0]+" "+object[1] {
			return bundleRefusal("missing-git-object", "the local object store lacks "+object[1]+" "+object[0])
		}
	}
	return nil
}

// bundleReplayOptions reparses the frozen arguments against this checkout's
// root; the wrapped parsers refuse a later --root, and a request that parses
// to another mode is not a bundle request.
func bundleReplayOptions(root string, request bundleRequest) (options, error) {
	arguments := append([]string{"--root", root, "prove"}, request.Arguments...)
	if _, _, kind := proveBundleRequest(arguments); kind != "" {
		return options{}, bundleRefusal("invalid-bundle", "bundle arguments may not name a bundle flag")
	}
	parsed, isProve, err := parseProveInvocation(arguments)
	if !isProve || err != nil || parsed.proveMode != request.Mode {
		return options{}, bundleRefusal("invalid-bundle", "bundle arguments do not parse as a "+request.Mode+" request")
	}
	return parsed, nil
}

// bundleDifferences compares exit, refusal code and receipt; the declared
// exclusions never enter the comparison (FPK-V0-046).
func bundleDifferences(original, replayed bundleResult) []string {
	_, left, _ := normalizedJSON(bundleComparable(original))
	_, right, _ := normalizedJSON(bundleComparable(replayed))
	return jsonDifferences("", left, right, []string{})
}

func bundleComparable(result bundleResult) map[string]any {
	view := map[string]any{"exit": result.Exit, "receipt": result.Receipt}
	if result.Error != nil {
		view["error"] = map[string]any{"code": result.Error.Code}
	}
	if proof, ok := result.Receipt["proof"].(map[string]any); ok {
		unledgered := make(map[string]any, len(proof))
		for key, value := range proof {
			unledgered[key] = value
		}
		delete(unledgered, "ledger")
		view["receipt"] = withMember(result.Receipt, "proof", unledgered)
	}
	return view
}

func withMember(object map[string]any, key string, value any) map[string]any {
	copied := make(map[string]any, len(object))
	for name, member := range object {
		copied[name] = member
	}
	copied[key] = value
	return copied
}

// jsonDifferences lists the member paths where left and right differ, in
// key order, capped at bundleDifferenceBound.
func jsonDifferences(path string, left, right any, found []string) []string {
	if len(found) >= bundleDifferenceBound {
		return found
	}
	leftObject, leftIsObject := left.(map[string]any)
	rightObject, rightIsObject := right.(map[string]any)
	if leftIsObject && rightIsObject {
		return objectDifferences(path, leftObject, rightObject, found)
	}
	leftList, leftIsList := left.([]any)
	rightList, rightIsList := right.([]any)
	if leftIsList && rightIsList && len(leftList) == len(rightList) {
		for index := range leftList {
			found = jsonDifferences(path+"["+strconv.Itoa(index)+"]", leftList[index], rightList[index], found)
		}
		return found
	}
	leftEncoded, _ := gokernel.CanonicalJSON(left)
	rightEncoded, _ := gokernel.CanonicalJSON(right)
	if !bytes.Equal(leftEncoded, rightEncoded) {
		found = append(found, path)
	}
	return found
}

func objectDifferences(path string, left, right map[string]any, found []string) []string {
	keys := map[string]bool{}
	for key := range left {
		keys[key] = true
	}
	for key := range right {
		keys[key] = true
	}
	names := make([]string, 0, len(keys))
	for key := range keys {
		names = append(names, key)
	}
	sort.Strings(names)
	prefix := path
	if prefix != "" {
		prefix += "."
	}
	for _, key := range names {
		found = jsonDifferences(prefix+key, left[key], right[key], found)
	}
	return found
}
