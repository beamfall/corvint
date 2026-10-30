package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/breakagemap"
	"github.com/Beamfall/corvint/internal/gateagreement"
	"github.com/Beamfall/corvint/internal/migrationratchet"
)

const migrationRatchetUsage = "usage: corvint migration-ratchet --profile FILE " + gateAgreementUsage + " | schema | seal (--profile FILE | --snapshot FILE) | span-digest --revision COMMIT --path PATH --lines START:END"

// migrationRatchetInvocation is one parsed migration-ratchet form. The
// comparison stays the default and alone takes the uniform agreement options
// (GVA-V0-001); the helpers only read their named input.
type migrationRatchetInvocation struct {
	mode      string
	root      string
	profile   string
	snapshot  string
	revision  string
	path      string
	start     int
	end       int
	gate      gateAgreement
	arguments []string
}

func parseMigrationRatchetInvocation(arguments []string) (migrationRatchetInvocation, bool, error) {
	position := commandPositionAfterRoots(arguments)
	if position < 0 || arguments[position] != "migration-ratchet" {
		return migrationRatchetInvocation{}, false, nil
	}
	invocation := migrationRatchetInvocation{mode: "compare"}
	for index := 0; index < position; index++ {
		if value, ok := strings.CutPrefix(arguments[index], "--root="); ok {
			invocation.root = value
		} else if arguments[index] == "--root" {
			invocation.root = arguments[index+1]
			index++
		}
	}
	usage := argumentError(migrationRatchetUsage)
	gate, rest, gateErr := splitGateAgreement(arguments[position+1:])
	if gateErr != nil {
		return invocation, true, usage
	}
	invocation.gate, invocation.arguments = gate, rest
	if len(rest) > 0 && (rest[0] == "schema" || rest[0] == "seal" || rest[0] == "span-digest") {
		invocation.mode = rest[0]
		rest = rest[1:]
	}
	values := map[string]string{}
	for index := 0; index < len(rest); index += 2 {
		name := rest[index]
		if index+1 >= len(rest) || !strings.HasPrefix(name, "--") || values[name] != "" || strings.TrimSpace(rest[index+1]) == "" {
			return invocation, true, usage
		}
		values[name] = rest[index+1]
	}
	allowed := map[string][]string{"compare": {"--profile"}, "schema": {}, "seal": {"--profile", "--snapshot"}, "span-digest": {"--revision", "--path", "--lines"}}[invocation.mode]
	for name := range values {
		if !slices.Contains(allowed, name) {
			return invocation, true, usage
		}
	}
	invocation.profile, invocation.snapshot = values["--profile"], values["--snapshot"]
	if invocation.mode != "compare" && (gate.advisory || gate.log != "") {
		return invocation, true, usage
	}
	switch invocation.mode {
	case "compare":
		if invocation.profile == "" {
			return invocation, true, usage
		}
	case "seal":
		if (invocation.profile == "") == (invocation.snapshot == "") {
			return invocation, true, usage
		}
	case "span-digest":
		invocation.revision, invocation.path = values["--revision"], values["--path"]
		first, last, ok := strings.Cut(values["--lines"], ":")
		start, startErr := strconv.Atoi(first)
		end, endErr := strconv.Atoi(last)
		if invocation.revision == "" || invocation.path == "" || !ok || startErr != nil || endErr != nil || start < 1 || end < start {
			return invocation, true, usage
		}
		invocation.start, invocation.end = start, end
	}
	return invocation, true, nil
}

func runMigrationRatchetInvocation(ctx context.Context, invocation migrationRatchetInvocation, stdout, stderr io.Writer) int {
	switch invocation.mode {
	case "schema":
		return writeMigrationRatchetJSON(migrationratchet.Describe(), stdout, stderr)
	case "seal":
		return runMigrationRatchetSeal(invocation, stdout, stderr)
	case "span-digest":
		return runMigrationRatchetSpanDigest(ctx, invocation, stdout, stderr)
	default:
		return runMigrationRatchet(ctx, invocation, stdout, stderr)
	}
}

func runMigrationRatchet(ctx context.Context, invocation migrationRatchetInvocation, stdout, stderr io.Writer) int {
	path, gate := invocation.profile, invocation.gate
	input, err := readBoundedFile(path, migrationratchet.MaxBytes)
	if err != nil {
		emitMigrationRatchetError(stderr, "profile-unavailable")
		return 2
	}
	profile, err := migrationratchet.Decode(input)
	if err != nil {
		emitMigrationRatchetError(stderr, err.Error())
		return 2
	}
	// Record source spans are re-read from the checkout's Git objects
	// (MER-V0-018); a profile without sources never touches Git.
	// The source root is the analysed repository the --log guard protects
	// (GVA-V0-005); a sourceless profile reads no repository.
	var read migrationratchet.SpanReader
	var root string
	if migrationRatchetHasSources(profile) {
		if root, err = migrationRatchetRoot(invocation.root); err != nil {
			emitMigrationRatchetError(stderr, "source-root-unavailable")
			return 2
		}
	}
	if err := gate.prepare(root); err != nil {
		emitMigrationRatchetError(stderr, agreementReason(err))
		return 2
	}
	if root != "" {
		spans := breakagemap.NewSpanReader(ctx, root)
		read = func(commit, path string, start, end int) (string, error) {
			pin, err := spans.Read(commit, path, start, end)
			return pin.SHA256, err
		}
	}
	receipt, err := migrationratchet.CompareVerified(profile, read)
	if err != nil {
		emitMigrationRatchetError(stderr, err.Error())
		return 2
	}
	encoded, err := migrationratchet.Encode(receipt)
	if err != nil {
		emitMigrationRatchetError(stderr, "receipt-encoding-failed")
		return 2
	}
	status := 0
	if receipt.Verdict != "pass" {
		status = 1
	}
	gate.baseline = receipt.Baseline.Repository.Revision
	status, err = gate.finish("migration-ratchet", invocation.arguments, receipt.Candidate.Repository.Revision, nil, ratchetObserved(receipt), status)
	if err != nil {
		emitMigrationRatchetError(stderr, agreementReason(err))
		return 2
	}
	if _, err := fmt.Fprintf(stdout, "%s\n", encoded); err != nil {
		emitMigrationRatchetError(stderr, "receipt-write-failed")
		return 2
	}
	return status
}

// agreementReason is the refusal code of an agreement error.
func agreementReason(err error) string {
	var agreementError *gateagreement.Error
	if errors.As(err, &agreementError) {
		return agreementError.Code
	}
	return "agreement-log-unavailable"
}

func migrationRatchetHasSources(profile migrationratchet.Profile) bool {
	for _, records := range [][]migrationratchet.Record{profile.Baseline.Records, profile.Candidate.Records} {
		for _, record := range records {
			if record.Source != nil {
				return true
			}
		}
	}
	return false
}

// runMigrationRatchetSeal fills self-digests and canonical order in one
// snapshot or profile and writes it to stdout; the input file is not changed.
func runMigrationRatchetSeal(invocation migrationRatchetInvocation, stdout, stderr io.Writer) int {
	path := invocation.profile
	if path == "" {
		path = invocation.snapshot
	}
	input, err := readBoundedFile(path, migrationratchet.MaxBytes)
	if err != nil {
		emitMigrationRatchetError(stderr, "seal-input-unavailable")
		return 2
	}
	if invocation.profile != "" {
		profile, err := migrationratchet.Decode(input)
		if err != nil {
			emitMigrationRatchetError(stderr, err.Error())
			return 2
		}
		for _, side := range []migrationratchet.Snapshot{profile.Baseline, profile.Candidate} {
			if err := migrationratchet.ValidateSealable(side); err != nil {
				emitMigrationRatchetError(stderr, err.Error())
				return 2
			}
		}
		return writeMigrationRatchetJSON(migrationratchet.SealProfile(profile), stdout, stderr)
	}
	snapshot, err := migrationratchet.DecodeSnapshot(input)
	if err != nil {
		emitMigrationRatchetError(stderr, err.Error())
		return 2
	}
	if err := migrationratchet.ValidateSealable(snapshot); err != nil {
		emitMigrationRatchetError(stderr, err.Error())
		return 2
	}
	return writeMigrationRatchetJSON(migrationratchet.SealSnapshot(snapshot), stdout, stderr)
}

// runMigrationRatchetSpanDigest prints the sub-file identity of one test body:
// the blob and line-span digest at an immutable commit, read from Git only.
func runMigrationRatchetSpanDigest(ctx context.Context, invocation migrationRatchetInvocation, stdout, stderr io.Writer) int {
	root, err := migrationRatchetRoot(invocation.root)
	if err != nil {
		emitMigrationRatchetError(stderr, "span-root-unavailable")
		return 2
	}
	pin, err := breakagemap.ReadSpan(ctx, root, invocation.revision, invocation.path, invocation.start, invocation.end)
	if err != nil {
		emitMigrationRatchetError(stderr, "migration-ratchet span refused: "+err.Error())
		return 2
	}
	return writeMigrationRatchetJSON(struct {
		Schema string `json:"schema"`
		breakagemap.SpanPin
	}{"corvint-migration-span-digest/1", pin}, stdout, stderr)
}

func migrationRatchetRoot(value string) (string, error) {
	if value == "" {
		// Without --root the working directory must lie in a Git checkout, so a profile with
		// sources outside one refuses with source-root-unavailable (MER-V0-099).
		dir, err := os.Getwd()
		if err != nil {
			return "", err
		}
		for current := dir; ; current = filepath.Dir(current) {
			if _, statErr := os.Stat(filepath.Join(current, ".git")); statErr == nil {
				return dir, nil
			}
			if filepath.Dir(current) == current {
				return "", errors.New("not inside a Git checkout")
			}
		}
	}
	return normalizeRoot(value)
}

func writeMigrationRatchetJSON(value any, stdout, stderr io.Writer) int {
	encoded, err := migrationratchet.Encode(value)
	if err != nil {
		emitMigrationRatchetError(stderr, "output-encoding-failed")
		return 2
	}
	if _, err := fmt.Fprintf(stdout, "%s\n", encoded); err != nil {
		emitMigrationRatchetError(stderr, "output-write-failed")
		return 2
	}
	return 0
}

func emitMigrationRatchetError(stderr io.Writer, reason string) {
	_, _ = fmt.Fprintf(stderr, "{\"error\":%s,\"ok\":false,\"tool\":\"migration-ratchet\"}\n", pythonJSONString(reason))
}
