package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/publish"
	"github.com/Beamfall/corvint/internal/cem/workflow"
	"github.com/Beamfall/corvint/internal/extevidence"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/lrfrepo"
)

// obligationsInputCeiling bounds each sidecar input read: the CEM raw ceiling
// is 4 MiB and an impact receipt never exceeds it (EFO-V0-001).
const obligationsInputCeiling = 4 << 20

// obligationsOptions is the whole argument surface of `corvint obligations`.
// The verb reads two files and no repository, so it takes no --root.
type obligationsOptions struct {
	cem, impact string
	limit       int
	// stable is the explicit cem/1.0 verification context; all four members
	// are given together or not at all (CEM-V1-135).
	stable extevidence.StableObligationsOptions
	// intentOverlay is the absolute, out-of-tree reviewer intent file read
	// for the advisory overlay (CEM-V1-642).
	intentOverlay string
}

// stableMode reports whether the invocation selected the stable sidecar.
func (o obligationsOptions) stableMode() bool {
	return o.stable.Repository != "" || o.stable.ExpectedBase != "" || o.stable.Target != "" || o.stable.ArtifactRoot != ""
}

// parseObligationsInvocation reports whether argv is an obligations run and
// its parsed options (EFO-V0-001).
func parseObligationsInvocation(arguments []string) (obligationsOptions, bool, error) {
	if len(arguments) == 0 || arguments[0] != "obligations" {
		return obligationsOptions{}, false, nil
	}
	options := obligationsOptions{limit: 64}
	values := map[string]*string{"--cem": &options.cem, "--impact": &options.impact}
	limit := ""
	values["--limit"] = &limit
	values["--repository"] = &options.stable.Repository
	values["--expected-base"] = &options.stable.ExpectedBase
	values["--target"] = &options.stable.Target
	values["--stable-artifact-root"] = &options.stable.ArtifactRoot
	values["--intent-overlay"] = &options.intentOverlay
	rest := arguments[1:]
	for len(rest) > 0 {
		// --allow-absent-sidecar is the one valueless flag (CEM-V1-639).
		if rest[0] == "--allow-absent-sidecar" {
			if options.stable.AllowAbsentSidecar {
				return options, true, argumentError("argument --allow-absent-sidecar: given twice")
			}
			options.stable.AllowAbsentSidecar, rest = true, rest[1:]
			continue
		}
		flag, inline, hasInline := strings.Cut(rest[0], "=")
		target, known := values[flag]
		if !known {
			return options, true, argumentError(fmt.Sprintf("unrecognized arguments: %s", rest[0]))
		}
		if *target != "" {
			return options, true, argumentError(fmt.Sprintf("argument %s: given twice", flag))
		}
		if !hasInline {
			if len(rest) < 2 || rest[1] == "" {
				return options, true, argumentError(fmt.Sprintf("argument %s: requires exactly one value", flag))
			}
			inline, rest = rest[1], rest[1:]
		}
		if inline == "" {
			return options, true, argumentError(fmt.Sprintf("argument %s: requires exactly one value", flag))
		}
		*target, rest = inline, rest[1:]
	}
	if options.cem == "" || options.impact == "" {
		return options, true, argumentError("obligations requires --cem FILE and --impact FILE")
	}
	stable := options.stable
	if options.stableMode() && (stable.Repository == "" || stable.ExpectedBase == "" || stable.Target == "" || stable.ArtifactRoot == "") {
		return options, true, argumentError("obligations stable mode requires --repository DIR, --expected-base REV, --target REV and --stable-artifact-root DIR together")
	}
	if options.stable.AllowAbsentSidecar && !options.stableMode() {
		return options, true, argumentError("argument --allow-absent-sidecar: requires the stable mode flags")
	}
	if options.intentOverlay != "" && !options.stableMode() {
		return options, true, argumentError("argument --intent-overlay: requires the stable mode flags")
	}
	if limit != "" {
		parsed, err := strconv.Atoi(limit)
		if err != nil || parsed < 1 {
			return options, true, argumentError("argument --limit: must be a positive integer")
		}
		options.limit = parsed
	}
	return options, true, nil
}

// runObligations composes and writes the sidecar. It reads exactly the two
// named files and touches nothing else (EFO-V0-001, EFO-V0-007).
func runObligations(options obligationsOptions, stdout, stderr io.Writer) int {
	cem, err := readObligationsInput("cem", options.cem)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	impact, err := readObligationsInput("impact", options.impact)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	var overlay map[string]any
	if options.intentOverlay != "" {
		if overlay, err = readIntentOverlay(options.stable.Repository, options.intentOverlay); err != nil {
			emitError(stderr, err)
			return 2
		}
	}
	// Without the stable context this refuses as the retired
	// external-frontier-obligations/0 route (CEM-V1-217).
	document, err := extevidence.StableObligations(context.Background(), cem, impact, options.limit, options.stable)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	// The overlay joins after the sidecar's id is computed and is never read
	// back, so the state, hunks, providers, criteria, id and exit status are
	// those of the same run without it (CEM-V1-642).
	if overlay != nil {
		document["reviewer_overlay"] = overlay
	}
	encoded, err := gokernel.CanonicalJSON(document)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	if _, err := stdout.Write(append(encoded, '\n')); err != nil {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot write obligations sidecar"})
		return 2
	}
	return 0
}

// readObligationsInput reads a --cem or --impact input through the bounded
// regular-file reader (V1-3017): a FIFO or other non-regular file is refused
// without being opened for a blocking read, and at most ceiling+1 bytes are
// read, so an oversized input is refused before it is held in memory. A
// symlink is resolved first, as os.ReadFile followed it before.
func readObligationsInput(name, path string) ([]byte, error) {
	cannot := &gokernel.Error{Code: "invalid-obligations-input", Message: fmt.Sprintf("cannot read %s input", name)}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, cannot
	}
	data, err := publish.ReadBoundedFile(resolved, obligationsInputCeiling, "invalid-obligations-input")
	if err == nil {
		return data, nil
	}
	if info, statErr := os.Stat(resolved); statErr == nil && info.Mode().IsRegular() && info.Size() > obligationsInputCeiling {
		return nil, &gokernel.Error{Code: "invalid-obligations-input", Message: fmt.Sprintf("%s input exceeds %d bytes", name, obligationsInputCeiling)}
	}
	return nil, cannot
}

// intentOverlayNote is the overlay's fixed statement of what it is not.
const intentOverlayNote = "Reviewer-supplied intent read from outside the target tree. It is not project authority, binds no blob of the target, and is never an input to this sidecar's state, hunks, providers, criteria, id or exit status; each obligation stays UNKNOWN until a committed intent and OCM link it."

// readIntentOverlay reads the reviewer's out-of-tree intent file under the
// CEM-V1-641 map-path rule and derives its advisory obligations
// (CEM-V1-642): the path is absolute and outside the repository and its Git
// directories, a symlink or a missing file is refused, and the bytes are
// bounded. Requirement IDs are enumerated by the OCM-V0-001 rule; every
// derived obligation is UNKNOWN, so the overlay can never upgrade one.
func readIntentOverlay(repository, path string) (map[string]any, error) {
	refuse := func(message string) error {
		return &gokernel.Error{Code: "invalid-obligations-input", Message: message}
	}
	if !filepath.IsAbs(path) {
		return nil, refuse("--intent-overlay must be an absolute path outside the repository and its Git directory")
	}
	session, err := workflow.Open(repository)
	if err != nil {
		return nil, refuse("--intent-overlay needs a readable --repository")
	}
	data, err := session.ReadExternalMap(path)
	if cemcode.CodeOf(err) == cemcode.InvalidArguments {
		return nil, refuse("--intent-overlay must lie outside the repository and its Git directory")
	}
	if err != nil {
		return nil, refuse("--intent-overlay is not a readable regular file within the size bound (a symlink is refused)")
	}
	requirements, start, end, spanSHA256, err := lrfrepo.IntentRequirements(data)
	if err != nil {
		return nil, refuse("--intent-overlay is not an intent document: " + err.Error())
	}
	obligations := make([]any, 0, len(requirements))
	for _, requirement := range requirements {
		obligations = append(obligations, map[string]any{"requirement": requirement, "state": "UNKNOWN"})
	}
	sum := sha256.Sum256(data)
	return map[string]any{
		"authority":     "reviewer-supplied",
		"authoritative": false,
		"intent_sha256": hex.EncodeToString(sum[:]),
		"span":          map[string]any{"start": start, "end": end, "sha256": spanSHA256},
		"obligations":   obligations,
		"note":          intentOverlayNote,
	}, nil
}
