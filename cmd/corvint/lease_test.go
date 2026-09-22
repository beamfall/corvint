package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func leaseRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("create repository marker: %v", err)
	}
	return root
}

func runLeaseCase(t *testing.T, arguments ...string) (int, map[string]any, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runLease(arguments, &stdout, &stderr)
	payload := map[string]any{}
	if stdout.Len() != 0 {
		if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
			t.Fatalf("stdout is not one JSON object: %v (%q)", err, stdout.String())
		}
	}
	return code, payload, stderr.String()
}

// SCL-V0-010
func TestLeaseCommandAcquireRefusesOverlapAndListsWithoutWriting(t *testing.T) {
	t.Parallel()
	root := leaseRoot(t)
	code, payload, stderr := runLeaseCase(t, "--root", root, "lease", "acquire",
		"--holder", "agent-a", "--ttl", "30m", "--path", "internal/**", "--ticket", "AT-42")
	if code != 0 {
		t.Fatalf("acquire exit = %d, stderr=%s", code, stderr)
	}
	lease, _ := payload["lease"].(map[string]any)
	identifier, _ := lease["lease_id"].(string)
	if identifier == "" || payload["mutates"] != true || lease["state"] != "live" {
		t.Fatalf("acquire payload = %v", payload)
	}

	code, payload, _ = runLeaseCase(t, "--root", root, "lease", "acquire",
		"--holder", "agent-b", "--ttl", "30m", "--path", "internal/scopelease/lease.go")
	if code != 1 || payload["ok"] != false {
		t.Fatalf("overlapping acquire exit = %d payload = %v", code, payload)
	}
	conflicts, _ := payload["conflicts"].([]any)
	if len(conflicts) != 1 {
		t.Fatalf("conflict rows = %v", conflicts)
	}
	first, _ := conflicts[0].(map[string]any)
	if first["holder"] != "agent-a" || first["lease_id"] != identifier || first["scope"] != "internal/**" {
		t.Fatalf("conflict row = %v", first)
	}

	code, payload, stderr = runLeaseCase(t, "--root", root, "lease", "list")
	if code != 0 || payload["mutates"] != false {
		t.Fatalf("list exit = %d payload = %v stderr=%s", code, payload, stderr)
	}
	if leases, _ := payload["leases"].([]any); len(leases) != 1 {
		t.Fatalf("list leases = %v", leases)
	}

	code, _, stderr = runLeaseCase(t, "--root", root, "lease", "release",
		"--id", identifier, "--holder", "agent-b")
	if code != 1 {
		t.Fatalf("release by another holder exit = %d, stderr=%s", code, stderr)
	}
	code, payload, stderr = runLeaseCase(t, "--root", root, "lease", "release",
		"--id", identifier, "--holder", "agent-a")
	if code != 0 || payload["action"] != "release" {
		t.Fatalf("release exit = %d payload = %v stderr=%s", code, payload, stderr)
	}
}

// SCL-V0-010: several live leases list in one deterministic order, by lease id,
// so two runs over one directory render the same bytes.
func TestLeaseListIsSortedByIdentifier(t *testing.T) {
	t.Parallel()
	root := leaseRoot(t)
	identifiers := make([]string, 0, 3)
	for _, scope := range []string{"internal/scopelease/**", "internal/necessity/**", "cmd/corvint/lease.go"} {
		code, payload, stderr := runLeaseCase(t, "--root", root, "lease", "acquire",
			"--holder", "agent-a", "--ttl", "30m", "--path", scope)
		if code != 0 {
			t.Fatalf("acquire %s exit = %d, stderr=%s", scope, code, stderr)
		}
		lease, _ := payload["lease"].(map[string]any)
		identifier, _ := lease["lease_id"].(string)
		if identifier == "" {
			t.Fatalf("acquire %s payload = %v", scope, payload)
		}
		identifiers = append(identifiers, identifier)
	}
	sort.Strings(identifiers)

	code, payload, stderr := runLeaseCase(t, "--root", root, "lease", "list")
	if code != 0 {
		t.Fatalf("list exit = %d, stderr=%s", code, stderr)
	}
	rows, _ := payload["leases"].([]any)
	if len(rows) != len(identifiers) {
		t.Fatalf("list leases = %v, want %d rows", rows, len(identifiers))
	}
	for position, row := range rows {
		entry, _ := row.(map[string]any)
		if entry["lease_id"] != identifiers[position] {
			t.Fatalf("row %d = %v, want lease_id %s", position, entry, identifiers[position])
		}
	}
}

// SCL-V0-010
// TestLeaseHelpMatchesCommandSurface pins the scope-lease command surface in
// `lease --help`: --ttl is a required Go duration for acquire and renew (the
// parser refuses a bare `60`), and --path is optional beside --ticket.
func TestLeaseHelpMatchesCommandSurface(t *testing.T) {
	t.Parallel()
	if _, err := parseLeaseFlags("acquire", []string{"--holder", "h", "--ttl", "60"}); err == nil {
		t.Fatal("bare seconds accepted as --ttl; the help contract below would be stale")
	}
	for _, want := range []string{
		"lease acquire --holder NAME --ttl DURATION [--path GLOB]...",
		"lease renew --holder NAME --id ID --ttl DURATION",
	} {
		if !strings.Contains(leaseHelp, want) {
			t.Errorf("lease help lacks %q:\n%s", want, leaseHelp)
		}
	}
	for _, stale := range []string{"SECONDS", "[--ttl"} {
		if strings.Contains(leaseHelp, stale) {
			t.Errorf("lease help still advertises %q", stale)
		}
	}
}

func TestLeaseCommandRejectsMissingArguments(t *testing.T) {
	t.Parallel()
	root := leaseRoot(t)
	code, _, stderr := runLeaseCase(t, "--root", root, "lease", "acquire", "--holder", "agent-a")
	if code != 2 || stderr == "" {
		t.Fatalf("missing --ttl exit = %d stderr = %q", code, stderr)
	}
	code, _, stderr = runLeaseCase(t, "--root", root, "lease", "inspect")
	if code != 2 || stderr == "" {
		t.Fatalf("unknown action exit = %d stderr = %q", code, stderr)
	}
}

// SCL-V0-010: a request scopelease refuses before touching state is an
// argument error, not an internal one.
func TestLeaseCommandLabelsRequestRefusalsAsInvalidArguments(t *testing.T) {
	t.Parallel()
	root := leaseRoot(t)
	cases := map[string][]string{
		"no scope":     {"--root", root, "lease", "acquire", "--holder", "agent-a", "--ttl", "1m"},
		"ttl over cap": {"--root", root, "lease", "acquire", "--holder", "agent-a", "--ttl", "48h", "--path", "a"},
		"malformed id": {"--root", root, "lease", "status", "--id", "x"},
	}
	for name, arguments := range cases {
		code, _, stderr := runLeaseCase(t, arguments...)
		if code != 2 || !strings.Contains(stderr, `"code": "invalid-arguments"`) {
			t.Errorf("%s: exit = %d stderr = %q", name, code, stderr)
		}
	}
}

// SCL-V0-010: argparse consumes the action positional before it reports an
// unrecognized option, so an invalid action is named first.
func TestLeaseCommandReportsAnInvalidActionBeforeItsFlags(t *testing.T) {
	t.Parallel()
	root := leaseRoot(t)
	code, _, stderr := runLeaseCase(t, "--root", root, "lease", "bogus", "--x")
	if code != 2 || !strings.Contains(stderr, "argument action: invalid choice: 'bogus'") {
		t.Fatalf("exit = %d stderr = %q", code, stderr)
	}
}
