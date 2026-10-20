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
	"path/filepath"

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

// cemStableView is one accepted stable universe. Map is the exact
// authenticated raw stable graph and Verification is the verifier's whole
// result; neither is re-shaped. Task completion and criterion satisfaction
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
	Map                   json.RawMessage       `json:"map"`
	TaskCompletion        string                `json:"taskCompletion"`
	CriterionSatisfaction string                `json:"criterionSatisfaction"`
}

// cemStableRefusal is the stderr document for a refused binding. It keeps
// the input digests and, when the verifier ran, its whole result.
type cemStableRefusal struct {
	Profile      string               `json:"profile"`
	Code         string               `json:"code"`
	Input        cemStableInput       `json:"input"`
	Verification *verify.StableResult `json:"verification"`
}

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
		if errors.As(err, &stable) {
			result := stable.Result
			refusal.Verification = &result
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
		Obligations: obligations, Verification: universe.Verification, Map: universe.OriginalBytes(),
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
// under root without following a final symlink.
func readCEMStableInput(root, relative string) ([]byte, error) {
	name := filepath.Join(root, filepath.FromSlash(relative))
	info, err := os.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > wire.MaxMapBytes {
		return nil, os.ErrInvalid
	}
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
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
