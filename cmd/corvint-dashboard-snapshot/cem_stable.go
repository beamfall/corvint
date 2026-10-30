package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/lrfrepo"
)

// The `cem-stable` subcommand is the dashboard's cem/1.0 read path
// (CEM-V1-290..294). It is a separate document beside the frozen
// corvint-dashboard-snapshot/0 schema and its hash-pinned adapter registry,
// which stay unchanged: the snapshot command keeps refusing every bundle flag
// (CEM-V1-168/217).
const (
	cemStableProfile        = "corvint-dashboard-cem-stable/0"
	cemStableRefusalProfile = "corvint-dashboard-cem-stable-refusal/0"
	notObserved             = "NOT_OBSERVED"
)

type cemStableOptions struct {
	Root, CEMPath, OCMPath, ExpectedBase, Target, ArtifactRoot string
}

// cemStableInput binds the exact bytes the command read.
type cemStableInput struct {
	CEMPath   string `json:"cemPath"`
	CEMSha256 string `json:"cemSha256"`
	OCMPath   string `json:"ocmPath"`
	OCMSha256 string `json:"ocmSha256"`
}

type cemStableObligation struct {
	ID          string   `json:"id"`
	Disposition string   `json:"disposition"`
	Reason      string   `json:"reason"`
	HunkIDs     []string `json:"hunkIds"`
	ClaimIDs    []string `json:"claimIds"`
}

type cemStableIntent struct {
	Path       string `json:"path"`
	BlobOID    string `json:"blobOid"`
	Start      int64  `json:"start"`
	End        int64  `json:"end"`
	SpanSHA256 string `json:"spanSha256"`
}

// cemStableView is one accepted stable universe. MapBase64 is the exact
// authenticated map bytes, standard base64, whose SHA-256 is MapSha256 (a
// json.RawMessage would be compacted on encode); Verification is the
// verifier's whole result. Neither is re-shaped. Task completion and criterion satisfaction
// are never derived here, so they stay NOT_OBSERVED.
type cemStableView struct {
	Profile               string                `json:"profile"`
	Input                 cemStableInput        `json:"input"`
	Spec                  string                `json:"spec"`
	MapSha256             string                `json:"mapSha256"`
	BaseRevision          string                `json:"baseRevision"`
	TargetRevision        string                `json:"targetRevision"`
	ObjectFormat          string                `json:"objectFormat"`
	PatchSha256           string                `json:"patchSha256"`
	OCMSha256             string                `json:"ocmSha256"`
	Intent                cemStableIntent       `json:"intent"`
	Obligations           []cemStableObligation `json:"obligations"`
	Verification          verify.StableResult   `json:"verification"`
	MapBase64             []byte                `json:"mapBase64"`
	TaskCompletion        string                `json:"taskCompletion"`
	CriterionSatisfaction string                `json:"criterionSatisfaction"`
}

// cemStableRefusal is the stderr document for a refused binding. It keeps
// the input digests, the verifier step reached and, when the verifier ran,
// its whole result; when it did not run, verificationReason says why
// (CEM-V1-349, V1-2034).
type cemStableRefusal struct {
	Profile            string               `json:"profile"`
	Code               string               `json:"code"`
	Input              cemStableInput       `json:"input"`
	VerifierStage      string               `json:"verifierStage"`
	VerificationReason string               `json:"verificationReason,omitempty"`
	Verification       *verify.StableResult `json:"verification"`
}

// Verifier steps a cem-stable refusal reports (CEM-V1-349).
const (
	verifierStageNotRun   = "NOT_RUN"
	verifierStageRefused  = "refused"
	verifierStageAccepted = "accepted"
	// verifierNotRunReason is the NOT_RUN reason: the map was refused before
	// the stable verifier ran (a retired cem/0.x map or an unreadable
	// stable document); the refusal code names which.
	verifierNotRunReason = "refused-before-verification"
)

func isCEMStableCommand(arguments []string) bool {
	return len(arguments) > 0 && arguments[0] == "cem-stable"
}

func parseCEMStableArguments(arguments []string) (cemStableOptions, bool) {
	var parsed cemStableOptions
	targets := map[string]*string{
		"--root": &parsed.Root, "--cem": &parsed.CEMPath, "--ocm": &parsed.OCMPath,
		"--expected-base": &parsed.ExpectedBase, "--target": &parsed.Target, "--artifact-root": &parsed.ArtifactRoot,
	}
	seen := make(map[string]bool)
	for index := 1; index < len(arguments); index += 2 {
		flag := arguments[index]
		slot, known := targets[flag]
		if !known || seen[flag] || index+1 >= len(arguments) {
			return cemStableOptions{}, false
		}
		seen[flag] = true
		*slot = arguments[index+1]
	}
	if len(seen) != len(targets) || !validRoot(parsed.Root) || !validRoot(parsed.ArtifactRoot) ||
		!validRelativePath(parsed.CEMPath) || !validRelativePath(parsed.OCMPath) ||
		!validArgumentString(parsed.ExpectedBase) || !validArgumentString(parsed.Target) {
		return cemStableOptions{}, false
	}
	return parsed, true
}

func runCEMStableContext(ctx context.Context, arguments []string, stdout, stderr io.Writer) (exit int) {
	defer func() {
		if recover() != nil {
			emitError(stderr, errorInternal)
			exit = 2
		}
	}()
	parsed, ok := parseCEMStableArguments(arguments)
	if !ok {
		emitError(stderr, errorInvalidArgument)
		return 2
	}
	cemRaw, cemErr := readCEMStableInput(parsed.Root, parsed.CEMPath)
	ocmRaw, ocmErr := readCEMStableInput(parsed.Root, parsed.OCMPath)
	if cemErr != nil || ocmErr != nil {
		emitError(stderr, errorRepository)
		return 2
	}
	input := cemStableInput{
		CEMPath: parsed.CEMPath, CEMSha256: sha256Hex(cemRaw), OCMPath: parsed.OCMPath, OCMSha256: sha256Hex(ocmRaw),
	}
	universe, err := lrfrepo.VerifyStableUniverse(ctx, cemRaw, ocmRaw, lrfrepo.StableUniverseOptions{
		Root: parsed.Root, ExpectedBase: parsed.ExpectedBase, Target: parsed.Target, ArtifactRoot: parsed.ArtifactRoot,
	})
	if err != nil {
		if ctx.Err() != nil {
			emitError(stderr, errorInterrupted)
			return 2
		}
		refusal := cemStableRefusal{Profile: cemStableRefusalProfile, Code: lrfrepo.CodeOf(err), Input: input}
		if refusal.Code == "" {
			refusal.Code = errorInternal
		}
		var stable *lrfrepo.StableRefusal
		var admission *lrfrepo.StableAdmissionRefusal
		switch {
		case errors.As(err, &admission):
			// The verifier accepted; the sidecar or the OCM/LRF admission
			// refused after it.
			result := admission.Result
			refusal.VerifierStage, refusal.Verification = verifierStageAccepted, &result
		case errors.As(err, &stable):
			result := stable.Result
			refusal.VerifierStage, refusal.Verification = verifierStageRefused, &result
		default:
			refusal.VerifierStage, refusal.VerificationReason = verifierStageNotRun, verifierNotRunReason
		}
		return writeCEMStableDocument(stderr, stderr, refusal, 2)
	}
	return writeCEMStableDocument(stdout, stderr, cemStableProjection(universe, input), 0)
}

func cemStableProjection(universe *lrfrepo.StableUniverse, input cemStableInput) cemStableView {
	obligations := make([]cemStableObligation, 0, len(universe.Obligations))
	for _, row := range universe.Obligations {
		obligations = append(obligations, cemStableObligation{
			ID: row.ID, Disposition: row.Disposition, Reason: row.Reason,
			HunkIDs: nonNil(row.HunkIDs), ClaimIDs: nonNil(row.ClaimIDs),
		})
	}
	return cemStableView{
		Profile: cemStableProfile, Input: input, Spec: universe.Spec, MapSha256: universe.MapSHA256,
		BaseRevision: universe.BaseRevision, TargetRevision: universe.TargetRevision, ObjectFormat: universe.ObjectFormat,
		PatchSha256: universe.PatchSHA256, OCMSha256: universe.OCMSHA256,
		Intent: cemStableIntent{
			Path: universe.Intent.Path, BlobOID: universe.Intent.BlobOID, Start: universe.Intent.Start,
			End: universe.Intent.End, SpanSHA256: universe.Intent.SpanSHA256,
		},
		Obligations: obligations, Verification: universe.Verification, MapBase64: universe.OriginalBytes(),
		TaskCompletion: notObserved, CriterionSatisfaction: notObserved,
	}
}

// writeCEMStableDocument writes one JSON document plus LF; a failed
// encoding or write is OUTPUT_WRITE_FAILED on stderr.
func writeCEMStableDocument(output, stderr io.Writer, document any, exit int) int {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(document); err != nil || !writeSnapshot(output, buffer.Bytes()) {
		emitError(stderr, errorOutputWrite)
		return 2
	}
	return exit
}

// readCEMStableInput reads one regular, bounded, repository-relative file
// under root without following any symlink (CEM-V1-292). Each directory
// component is opened through its parent's descriptor (os.Root) and must be
// the same directory its parent's Lstat saw, so a symlinked or swapped
// component refuses instead of leaving root. The final file opens
// non-blocking and its descriptor must be the regular file Lstat saw, so a
// swapped-in FIFO, device or symlink refuses without blocking.
func readCEMStableInput(root, relative string) ([]byte, error) {
	current, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	segments := strings.Split(relative, "/")
	for _, segment := range segments[:len(segments)-1] {
		next, err := openCEMStableDirectory(current, segment)
		current.Close()
		if err != nil {
			return nil, err
		}
		current = next
	}
	defer current.Close()
	name := segments[len(segments)-1]
	before, err := current.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() || before.Size() > wire.MaxMapBytes {
		return nil, os.ErrInvalid
	}
	return readCEMStableRegular(current, name, before)
}

// openCEMStableDirectory opens name under parent only when it is a real
// directory, not a symlink, and the opened descriptor is that directory.
func openCEMStableDirectory(parent *os.Root, name string) (*os.Root, error) {
	before, err := parent.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !before.IsDir() {
		return nil, os.ErrInvalid
	}
	child, err := parent.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	opened, err := child.Stat(".")
	if err != nil || !opened.IsDir() || !os.SameFile(before, opened) {
		child.Close()
		return nil, os.ErrInvalid
	}
	return child, nil
}

// readCEMStableRegular opens name non-blocking, requires the descriptor to
// be the regular file before describes, and reads at most the map bound.
func readCEMStableRegular(directory *os.Root, name string, before os.FileInfo) ([]byte, error) {
	file, err := directory.OpenFile(name, os.O_RDONLY|cemStableOpenFlags, 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		return nil, os.ErrInvalid
	}
	data, err := io.ReadAll(io.LimitReader(file, wire.MaxMapBytes+1))
	if err != nil || len(data) > wire.MaxMapBytes {
		return nil, os.ErrInvalid
	}
	return data, nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
