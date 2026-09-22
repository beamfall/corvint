// Copyright 2026 Russell Lewis
// Licensed under the Apache License, Version 2.0.

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/frontier"
	"github.com/Beamfall/corvint/internal/lrf"
)

// The canaries below are the CF-V0-024 privacy probes: distinctive strings
// planted in a source body, a diff body, a command body, a report body, a
// sibling path and an argv element. None of them may reach stdout or stderr in
// either the valid or the failing run. They mirror
// conformance/frontier-v0/fixtures/privacy-canaries/case.json.
var frontierCanaries = []string{
	"CORVINT-CANARY-SOURCE-BODY-3f1a",
	"CORVINT-CANARY-DIFF-BODY-c40e",
	"CORVINT-CANARY-COMMAND-BODY-4c19",
	"CORVINT-CANARY-REPORT-BODY-9d02",
	"CORVINT-CANARY-SIBLING-PATH-6a83",
	"CORVINT-CANARY-EXCEPTION-TEXT-7e44",
}

func frontierHex(seed string) string {
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:])
}

func frontierOID(seed string) string { return frontierHex(seed)[:40] }

// stubFrontierVerifier stands in for the repository-backed shared CEM/OCM
// verifier while the Git adapter lands separately.
type stubFrontierVerifier struct {
	universe frontier.VerifiedUniverse
	err      error
}

func (stub stubFrontierVerifier) Verify(_ context.Context, _ frontier.VerifyRequest) (frontier.VerifiedUniverse, error) {
	return stub.universe, stub.err
}

type stubFrontierTCQ struct{ id string }

func (stub stubFrontierTCQ) Recompute(_ frontier.TCQRequest) (frontier.TCQResult, error) {
	return frontier.TCQResult{ID: stub.id}, nil
}

// frontierUniverse builds the smallest verified universe that reaches item
// projection. `hunks` of zero yields the CF-V0-004 EMPTY state; one `unknown`
// hunk yields exactly one HUNK_BASIS item and the OPEN state. Both bodies and
// the hunk path carry canaries, which CF-V0-024 keeps out of every rendering.
func frontierUniverse(hunks int) frontier.VerifiedUniverse {
	intentPath := "docs/specs/change-frontier-v0.md"
	span := frontier.Span{Start: 0, End: 512}
	cemMap := &wire.Map{
		Spec:         wire.Spec02,
		BaseRevision: frontierOID("base"),
		PatchSha256:  frontierHex("patch"),
		ExcludedPath: frontier.ExcludedPath,
	}
	for index := 0; index < hunks; index++ {
		cemMap.Hunks = append(cemMap.Hunks, wire.Hunk{
			ID:          wire.HunkPrefix + frontierHex("hunk"),
			Path:        "src/CORVINT-CANARY-SOURCE-BODY-3f1a.go",
			Disposition: frontier.CEMUnknown,
			Reason:      "no-evidence",
		})
	}
	return frontier.VerifiedUniverse{
		CEM:            cemMap,
		BaseRevision:   frontierOID("base"),
		TargetRevision: frontierOID("target"),
		ObjectFormat:   "sha1",
		PatchSHA256:    frontierHex("patch"),
		Intent: frontier.IntentScope{
			BlobOID: frontierOID("intent-blob"), Path: intentPath,
			Span: span, SpanSHA256: frontierHex("intent-span"),
		},
		LRFRequest: lrf.Request{Context: lrf.Context{
			CEMSpec:          wire.Spec02,
			CEMMapSHA256:     frontierHex("cem-map"),
			PatchSource:      "canonical-derived",
			BaseRevision:     frontierOID("base"),
			TargetRevision:   frontierPointer(frontierOID("target")),
			PatchSHA256:      frontierHex("patch"),
			ExcludedPath:     frontierPointer(frontier.ExcludedPath),
			OCMSpec:          frontierPointer(lrf.OCMSpec),
			OCMMapSHA256:     frontierPointer(frontierHex("ocm-map")),
			IntentPath:       frontierPointer(intentPath),
			IntentBlobOID:    frontierPointer(frontierOID("intent-blob")),
			IntentStart:      frontierPointer(span.Start),
			IntentEnd:        frontierPointer(span.End),
			IntentSpanSHA256: frontierPointer(frontierHex("intent-span")),
		}},
	}
}

func frontierPointer[T any](value T) *T { return &value }

// registerFrontierAdapters returns a context whose invocations use the given
// seam, so a test never writes frontierAdapters and never leaks an adapter
// into another.
func registerFrontierAdapters(t *testing.T, verifier frontier.Verifier, recomputer frontier.TCQRecomputer) context.Context {
	t.Helper()
	return context.WithValue(t.Context(), frontierAdaptersKey{}, frontierSeam(func(context.Context, string) (frontier.Verifier, frontier.TCQRecomputer) {
		return verifier, recomputer
	}))
}

// clearFrontierAdapters returns a context whose invocations see no registered
// seam, which is the state the command must survive before any
// repository-backed adapter exists.
func clearFrontierAdapters(t *testing.T) context.Context {
	t.Helper()
	return context.WithValue(t.Context(), frontierAdaptersKey{}, frontierSeam(nil))
}

// frontierRepository writes the artifacts the command reads. Every body
// carries a canary so a leak through the reader is visible.
func frontierRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"cem.json":         `{"spec":"cem/0.2","body":"CORVINT-CANARY-DIFF-BODY-c40e"}`,
		"ocm.json":         `{"spec":"ocm/0.1-experimental","cem":{"spec":"cem/0.2"},"body":"CORVINT-CANARY-SOURCE-BODY-3f1a"}`,
		"command.json":     `{"body":"CORVINT-CANARY-COMMAND-BODY-4c19"}`,
		"observation.json": `{"body":"CORVINT-CANARY-EXCEPTION-TEXT-7e44"}`,
		"report.xml":       `<testsuite name="CORVINT-CANARY-REPORT-BODY-9d02"/>`,
	}
	planted := ""
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		planted += body
	}
	// A canary probe that proves nothing is worse than no probe, so the
	// fixture asserts every body canary is actually planted before any test
	// asserts it is absent from the output. The sibling-path canary is planted
	// in argv instead, by the invocations that use it.
	for _, canary := range frontierCanaries {
		if canary == "CORVINT-CANARY-SIBLING-PATH-6a83" {
			continue
		}
		if !strings.Contains(planted, canary) {
			t.Fatalf("fixture never planted canary %s", canary)
		}
	}
	return root
}

func frontierArguments(root string, extra ...string) []string {
	return append([]string{
		"--root", root, "frontier",
		"--cem", "cem.json", "--ocm", "ocm.json",
		"--expected-base", frontierOID("base"), "--target", frontierOID("target"),
	}, extra...)
}

func assertNoFrontierCanary(t *testing.T, label, text string) {
	t.Helper()
	for _, canary := range frontierCanaries {
		if strings.Contains(text, canary) {
			t.Errorf("CF-V0-024: %s leaked canary %s", label, canary)
		}
	}
}

// CF-V0-004: exit 0 is a VALID empty frontier and exit 1 is a VALID open one.
// Exit 1 must never be produced by an error path, and exit 2 is reserved for
// operational failure alone.
func TestRunFrontierExitCodesSeparateValidStatesFromFailure(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		hunks int
		state string
		exit  int
	}{
		{"empty is a successful zero", 0, "EMPTY", 0},
		{"open is a successful one", 1, "OPEN", 1},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			ctx := registerFrontierAdapters(t,
				stubFrontierVerifier{universe: frontierUniverse(testCase.hunks)},
				stubFrontierTCQ{id: "tcq:sha256:" + frontierHex("tcq")})
			root := frontierRepository(t)
			stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
			exit := runContext(ctx, frontierArguments(root, "--json"), strings.NewReader(""), stdout, stderr)
			if exit != testCase.exit {
				t.Fatalf("exit = %d, want %d (stderr %q)", exit, testCase.exit, stderr.String())
			}
			if !strings.Contains(stdout.String(), `"frontierState":"`+testCase.state+`"`) {
				t.Fatalf("stdout does not declare %s: %s", testCase.state, stdout.String())
			}
			if stderr.Len() != 0 {
				t.Errorf("stderr must stay empty on a valid result: %q", stderr.String())
			}
			assertNoFrontierCanary(t, "valid stdout", stdout.String())
		})
	}
}

// CF-V0-026: JSON and human are two renderings of ONE computation, and
// CF-V0-025 requires the human rendering to carry the assertion boundary.
func TestRunFrontierHumanRenderingCarriesTheAssertionBoundary(t *testing.T) {
	t.Parallel()
	ctx := registerFrontierAdapters(t,
		stubFrontierVerifier{universe: frontierUniverse(1)},
		stubFrontierTCQ{id: "tcq:sha256:" + frontierHex("tcq")})
	root := frontierRepository(t)
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	exit := runContext(ctx, frontierArguments(root), strings.NewReader(""), stdout, stderr)
	if exit != 1 {
		t.Fatalf("exit = %d, want 1 (stderr %q)", exit, stderr.String())
	}
	rendered := stdout.String()
	for _, phrase := range []string{
		"asserts only that no qualifying relation was accepted",
		"not proof that evidence does not exist",
		"that the repository was exhaustively searched",
	} {
		if !strings.Contains(rendered, phrase) {
			t.Errorf("human rendering omits the CF-V0-025 boundary phrase %q", phrase)
		}
	}
	if strings.Contains(rendered, `"frontierState"`) {
		t.Error("human rendering must not be the JSON document")
	}
	assertNoFrontierCanary(t, "human stdout", rendered)
}

// CF-V0-022: an operational failure emits NO stdout, exit 2, and exactly one
// bounded stderr object plus one LF. CF-V0-024 keeps every canary — including
// one planted in argv — out of it.
func TestRunFrontierOperationalFailuresEmitOnlyTheBoundedEnvelope(t *testing.T) {
	t.Parallel()
	root := frontierRepository(t)
	cases := []struct {
		name      string
		adapters  bool
		arguments []string
		code      string
	}{
		{
			name:      "no repository adapter is an unsupported context",
			adapters:  false,
			arguments: frontierArguments(root, "--json"),
			code:      frontier.CodeUnsupportedContext,
		},
		{
			name:      "an unknown flag is refused without echoing argv",
			adapters:  true,
			arguments: frontierArguments(root, "--json", "--CORVINT-CANARY-SIBLING-PATH-6a83", "1"),
			code:      frontier.CodeInvalidInput,
		},
		{
			name:      "an unreadable artifact is refused without echoing the path",
			adapters:  true,
			arguments: []string{"--root", root, "frontier", "--json", "--cem", "CORVINT-CANARY-SIBLING-PATH-6a83.json"},
			code:      frontier.CodeInvalidInput,
		},
		{
			// CF-V0-003: the dynamic tuple is all-or-none, and the library —
			// not the wrapper — owns the verdict.
			name:      "a partial dynamic tuple is invalid input",
			adapters:  true,
			arguments: frontierArguments(root, "--json", "--command", "command.json"),
			code:      frontier.CodeInvalidInput,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			ctx := clearFrontierAdapters(t)
			if testCase.adapters {
				ctx = registerFrontierAdapters(t,
					stubFrontierVerifier{universe: frontierUniverse(1)},
					stubFrontierTCQ{id: "tcq:sha256:" + frontierHex("tcq")})
			}
			stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
			exit := runContext(ctx, testCase.arguments, strings.NewReader(""), stdout, stderr)
			if exit != 2 {
				t.Fatalf("exit = %d, want 2 (stdout %q stderr %q)", exit, stdout.String(), stderr.String())
			}
			if stdout.Len() != 0 {
				t.Errorf("CF-V0-022 forbids stdout on operational failure: %q", stdout.String())
			}
			want := `{"code":"` + testCase.code + `","profile":"frontier-error/0"}` + "\n"
			if stderr.String() != want {
				t.Errorf("stderr = %q, want %q", stderr.String(), want)
			}
			assertNoFrontierCanary(t, "error stderr", stderr.String())
		})
	}
}

// CF-V0-022 permits ONE static message per code in human mode and forbids
// unverified values in it, so the human failure names the code and nothing
// drawn from the invocation.
func TestRunFrontierHumanFailureIsOneStaticMessage(t *testing.T) {
	t.Parallel()
	ctx := clearFrontierAdapters(t)
	root := frontierRepository(t)
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	exit := runContext(ctx, frontierArguments(root), strings.NewReader(""), stdout, stderr)
	if exit != 2 {
		t.Fatalf("exit = %d, want 2", exit)
	}
	if stdout.Len() != 0 {
		t.Errorf("CF-V0-022 forbids stdout on operational failure: %q", stdout.String())
	}
	want := "frontier context is not supported by this profile (" + frontier.CodeUnsupportedContext + ")\n"
	if stderr.String() != want {
		t.Errorf("stderr = %q, want %q", stderr.String(), want)
	}
}

// CF-V0-033: a Frontier refusal is read-only in either rendering, including
// the otherwise permitted self-observation ledger.
func TestRunFrontierUnsupportedRefusalLeavesRepositoryUnchanged(t *testing.T) {
	t.Parallel()
	ctx := clearFrontierAdapters(t)
	for _, extra := range [][]string{nil, {"--json"}} {
		root := frontierRepository(t)
		if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".corvint", ".gitignore"), []byte("*\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		before := repositoryBytesDigest(t, root)
		stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
		if exit := runContext(ctx, frontierArguments(root, extra...), strings.NewReader(""), stdout, stderr); exit != 2 {
			t.Fatalf("%v: exit = %d, stderr = %q", extra, exit, stderr.String())
		}
		if after := repositoryBytesDigest(t, root); after != before {
			t.Fatalf("%v: refusal wrote to the repository", extra)
		}
	}
}

// The surface only ships once the repository-backed seam is registered: with a
// nil seam every invocation is an unsupported context, which is correct but
// useless. This is the test that would fail if frontier_adapters.go were lost.
func TestFrontierAdaptersAreRegistered(t *testing.T) {
	t.Parallel()
	if frontierAdapters == nil {
		t.Fatal("no repository-backed frontier adapter is registered")
	}
	verifier, recomputer := frontierAdapters(t.Context(), t.TempDir())
	if verifier == nil || recomputer == nil {
		t.Fatal("the registered seam returned no verifier or no recomputer")
	}
}

// End to end through the REAL adapter: an artifact that is not a `cem/0.2`
// document must reach the cascade and come back as one bounded envelope, not
// as a panic and not as a message-carrying error from the shared emitError.
func TestRunFrontierReachesTheRegisteredAdapter(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for name, body := range map[string]string{
		"cem.json": `{"spec":"cem/0.1"}`,
		"ocm.json": `{"spec":"ocm/0.1-experimental"}`,
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	exit := runContext(t.Context(), frontierArguments(root, "--json"), strings.NewReader(""), stdout, stderr)
	if exit != 2 {
		t.Fatalf("exit = %d, want 2 (stdout %q)", exit, stdout.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("CF-V0-022 forbids stdout on operational failure: %q", stdout.String())
	}
	// CF-V0-001: CEM 0.1 is an unsupported context, and the Frontier-owned
	// profile rejection runs before the repository is ever opened.
	want := `{"code":"` + frontier.CodeUnsupportedContext + `","profile":"frontier-error/0"}` + "\n"
	if stderr.String() != want {
		t.Errorf("stderr = %q, want %q", stderr.String(), want)
	}
}

// The interception must recognise `frontier` exactly where witness recognises
// its own verb, and must not claim any other invocation.
func TestParseFrontierInvocation(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		arguments []string
		root      string
		rest      []string
		claimed   bool
	}{
		{"bare verb", []string{"frontier", "--json"}, "", []string{"--json"}, true},
		{"separate root", []string{"--root", "/tmp", "frontier"}, "/tmp", []string{}, true},
		{"inline root", []string{"--root=/tmp", "frontier", "--cem", "a"}, "/tmp", []string{"--cem", "a"}, true},
		{"another verb", []string{"witness", "--base", "x"}, "", nil, false},
		{"dangling root", []string{"--root"}, "", nil, false},
		{"no arguments", []string{}, "", nil, false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root, rest, claimed := parseFrontierInvocation(testCase.arguments)
			if claimed != testCase.claimed {
				t.Fatalf("claimed = %v, want %v", claimed, testCase.claimed)
			}
			if !claimed {
				return
			}
			if root != testCase.root {
				t.Errorf("root = %q, want %q", root, testCase.root)
			}
			if strings.Join(rest, " ") != strings.Join(testCase.rest, " ") {
				t.Errorf("rest = %v, want %v", rest, testCase.rest)
			}
		})
	}
}

// The raw `--root` is returned unresolved on purpose: resolution failures must
// leave through the CF-V0-022 envelope, not the shared message-carrying one.
func TestParseFrontierOptions(t *testing.T) {
	t.Parallel()
	options, err := parseFrontierOptions([]string{
		"--cem", "a.json", "--ocm=b.json", "--expected-base", "abc", "--target", "def",
		"--command", "c.json", "--observation", "o.json", "--report", "r.xml", "--json",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := frontierOptions{
		cem: "a.json", ocm: "b.json", expectedBase: "abc", target: "def",
		command: "c.json", observation: "o.json", report: "r.xml", json: true,
	}
	if options != want {
		t.Fatalf("options = %+v, want %+v", options, want)
	}
	for name, arguments := range map[string][]string{
		"value for a valueless flag": {"--json=1"},
		"unknown flag":               {"--depth", "2"},
		"missing value":              {"--cem"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseFrontierOptions(arguments); frontier.CodeOf(err) != frontier.CodeInvalidInput {
				t.Errorf("code = %q, want %q", frontier.CodeOf(err), frontier.CodeInvalidInput)
			}
		})
	}
}

// The command must be discoverable without reading its source: it appears in
// the root help, and `help frontier` / `frontier --help` reach a topic that
// names every flag plus the CF-V0-004 exit law.
func TestFrontierHelpIsDiscoverable(t *testing.T) {
	t.Parallel()
	for _, required := range []string{"frontier", "corvint help ", "|frontier|"} {
		if !strings.Contains(rootHelp, required) {
			t.Errorf("root help omits %q", required)
		}
	}
	for _, required := range []string{
		"--cem MAP", "--ocm MAP", "--expected-base REV", "--target REV",
		"--command FILE", "--observation FILE", "--report FILE", "--json",
		"0 is a valid empty frontier",
		"1 is a\nvalid open frontier",
		"2 is operational failure",
		"Exit 1 is success.",
		"all-or-none",
		// CF-V0-025 travels with the human-facing text, not only the renderer.
		"asserts only that no qualifying relation was accepted",
		"the repository was exhaustively searched",
		// CF-V0-004 and CF-V0-027: no stop decision, no hook authority.
		"no stop decision",
	} {
		if !strings.Contains(frontierHelp, required) {
			t.Errorf("frontier help omits %q", required)
		}
	}
}

// A help invocation must never reach the frontier cascade: it is not a
// frontier computation, so it exits 0 with usage on stdout rather than
// entering the CF-V0-022 failure envelope. Static usage text echoes no
// unverified path, ID, digest, count, or argv element, so CF-V0-024 is not
// engaged by it.
func TestRunFrontierHelpDoesNotEnterTheCascade(t *testing.T) {
	t.Parallel()
	for _, arguments := range [][]string{{"frontier", "--help"}, {"help", "frontier"}} {
		t.Run(strings.Join(arguments, " "), func(t *testing.T) {
			t.Parallel()
			ctx := clearFrontierAdapters(t)
			stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
			exit := runContext(ctx, arguments, strings.NewReader(""), stdout, stderr)
			if exit != 0 {
				t.Fatalf("exit = %d, want 0 (stderr %q)", exit, stderr.String())
			}
			if stdout.String() != frontierHelp || stderr.Len() != 0 {
				t.Errorf("stdout = %q stderr = %q", stdout.String(), stderr.String())
			}
		})
	}
}
