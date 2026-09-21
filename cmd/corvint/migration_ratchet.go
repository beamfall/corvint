package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/Beamfall/corvint/internal/migrationratchet"
)

func parseMigrationRatchetInvocation(arguments []string) (string, bool, error) {
	position := commandPositionAfterRoots(arguments)
	if position < 0 || arguments[position] != "migration-ratchet" {
		return "", false, nil
	}
	rest := arguments[position+1:]
	if len(rest) != 2 || rest[0] != "--profile" || strings.TrimSpace(rest[1]) == "" {
		return "", true, argumentError("usage: corvint migration-ratchet --profile FILE")
	}
	return rest[1], true, nil
}

func runMigrationRatchet(_ context.Context, path string, stdout, stderr io.Writer) int {
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
	receipt, err := migrationratchet.Compare(profile)
	if err != nil {
		emitMigrationRatchetError(stderr, err.Error())
		return 2
	}
	encoded, err := migrationratchet.Encode(receipt)
	if err != nil {
		emitMigrationRatchetError(stderr, "receipt-encoding-failed")
		return 2
	}
	if _, err := fmt.Fprintf(stdout, "%s\n", encoded); err != nil {
		emitMigrationRatchetError(stderr, "receipt-write-failed")
		return 2
	}
	if receipt.Verdict != "pass" {
		return 1
	}
	return 0
}

func emitMigrationRatchetError(stderr io.Writer, reason string) {
	_, _ = fmt.Fprintf(stderr, "{\"error\":%s,\"ok\":false,\"tool\":\"migration-ratchet\"}\n", pythonJSONString(reason))
}
