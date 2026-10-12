package main

// CEM-V1-084 portable conformance: the public receipt-authentication case
// under protocol/cem-1.0/stable/receipt-authentication verifies with the exact
// public expected result, its authentication axis follows only the consumer
// trusted-keys file, and every public refusal fails at its recorded stage and
// code in both portable commands.

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

type receiptAuthPortableMutation struct {
	Name           string                     `json:"name"`
	Set            map[string]json.RawMessage `json:"set"`
	TrustedKeys    string                     `json:"trustedKeys"`
	Exit           int                        `json:"exit"`
	Outcome        string                     `json:"outcome"`
	Stage          string                     `json:"stage"`
	Code           string                     `json:"code"`
	IssueCode      string                     `json:"issueCode"`
	Authentication string                     `json:"authentication"`
}

type receiptAuthPortableCase struct {
	dir    string
	raw    []byte
	repo   string
	base   string
	target string
	expect struct {
		BaseRevision   string `json:"baseRevision"`
		TargetRevision string `json:"targetRevision"`
		Accept         struct {
			TrustedKeys string         `json:"trustedKeys"`
			Exit        int            `json:"exit"`
			Result      map[string]any `json:"resultExceptRuntime"`
		} `json:"accept"`
		AxisCases []receiptAuthPortableMutation `json:"axisCases"`
		Refusals  []receiptAuthPortableMutation `json:"refusals"`
	}
}

func loadReceiptAuthPortableCase(t *testing.T) *receiptAuthPortableCase {
	t.Helper()
	if !stableOwnerSupported() {
		t.Skip("NOT_RUN: process containment is not qualified on " + runtime.GOOS)
	}
	c := &receiptAuthPortableCase{dir: stableCaseFiles(t, "receipt-authentication")}
	raw, err := os.ReadFile(filepath.Join(c.dir, "EXPECTED.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &c.expect); err != nil {
		t.Fatal(err)
	}
	if c.raw, err = os.ReadFile(filepath.Join(c.dir, "stable.json")); err != nil {
		t.Fatal(err)
	}
	c.repo, c.base, c.target = neutralRepository(t, c.dir)
	if c.base != c.expect.BaseRevision || c.target != c.expect.TargetRevision {
		t.Fatalf("recipe commits %s..%s, want %s..%s", c.base, c.target, c.expect.BaseRevision, c.expect.TargetRevision)
	}
	return c
}

func (c *receiptAuthPortableCase) trusted(t *testing.T, name string) map[string]ed25519.PublicKey {
	t.Helper()
	if name == "" {
		return nil
	}
	raw, err := os.ReadFile(filepath.Join(c.dir, name))
	if err != nil {
		t.Fatal(err)
	}
	keys, ok := parseStableTrustedKeys(raw)
	if !ok {
		t.Fatalf("%s is not a trusted-keys file", name)
	}
	return keys
}

func (c *receiptAuthPortableCase) mutate(t *testing.T, set map[string]json.RawMessage) []byte {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(c.raw, &m); err != nil {
		t.Fatal(err)
	}
	for k, v := range set {
		m[k] = v
	}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// check asserts one mutation against both portable verifiers.
func (c *receiptAuthPortableCase) check(t *testing.T, m receiptAuthPortableMutation) {
	t.Helper()
	raw := c.mutate(t, m.Set)
	trusted := c.trusted(t, m.TrustedKeys)
	runs := map[string]func() (stableResult, int){
		"canonical": func() (stableResult, int) {
			return runStableCanonicalTrusted(context.Background(), c.repo, raw, c.base, c.target, c.dir, nil, trusted)
		},
		"stable": func() (stableResult, int) {
			return runStableTrusted(context.Background(), c.repo, raw, c.base, c.target, c.dir, trusted)
		},
	}
	for name, run := range runs {
		r, exit := run()
		issue := ""
		if len(r.IssueCodes) > 0 {
			issue = r.IssueCodes[0]
		}
		if exit != m.Exit || r.Outcome != m.Outcome || r.Axes["authentication"] != m.Authentication || strOf(r.Code) != m.Code || issue != m.IssueCode {
			b, _ := json.Marshal(r)
			t.Fatalf("%s exit=%d result=%s, want exit %d %s code %q issue %q authentication %s", name, exit, b, m.Exit, m.Outcome, m.Code, m.IssueCode, m.Authentication)
		}
		if m.Stage != "" && r.Stage != m.Stage {
			t.Fatalf("%s stage %s, want %s", name, r.Stage, m.Stage)
		}
		if m.Exit == 0 && r.Axes["executionAtCommit"] != "NOT_OBSERVED" {
			t.Fatalf("%s executionAtCommit %s, want NOT_OBSERVED", name, r.Axes["executionAtCommit"])
		}
	}
}

// TestCEMV1084PortableReceiptAuthenticationPublicCase is the portable side of
// the public receipt-authentication case; the native verifier asserts the
// same EXPECTED.json.
func TestCEMV1084PortableReceiptAuthenticationPublicCase(t *testing.T) {
	c := loadReceiptAuthPortableCase(t)
	trusted := c.trusted(t, c.expect.Accept.TrustedKeys)
	r, exit := runStableCanonicalTrusted(context.Background(), c.repo, c.raw, c.base, c.target, c.dir, nil, trusted)
	if got := neutralResultMap(t, r); exit != c.expect.Accept.Exit || !reflect.DeepEqual(got, c.expect.Accept.Result) {
		b, _ := json.Marshal(r)
		t.Fatalf("runStableCanonicalTrusted exit=%d result=%s, want the public expected ACCEPT", exit, b)
	}
	if r.Axes["authentication"] != "VERIFIED" || r.Axes["executionAtCommit"] != "NOT_OBSERVED" {
		t.Fatalf("axes = %v, want VERIFIED authentication and NOT_OBSERVED executionAtCommit", r.Axes)
	}
	r, exit = runStableTrusted(context.Background(), c.repo, c.raw, c.base, c.target, c.dir, trusted)
	if exit != 0 || r.Axes["authentication"] != "VERIFIED" || r.Axes["executionAtCommit"] != "NOT_OBSERVED" {
		b, _ := json.Marshal(r)
		t.Fatalf("runStableTrusted exit=%d result=%s, want VERIFIED", exit, b)
	}
	if len(c.expect.AxisCases) != 5 || len(c.expect.Refusals) != 13 {
		t.Fatalf("public axis cases %d refusals %d, want 5 and 13", len(c.expect.AxisCases), len(c.expect.Refusals))
	}
	for _, m := range append(append([]receiptAuthPortableMutation{}, c.expect.AxisCases...), c.expect.Refusals...) {
		t.Run(m.Name, func(t *testing.T) { c.check(t, m) })
	}
}

// TestCEMV1084PortableTrustedKeysArgument proves both portable commands take
// --trusted-keys only as an absolute path to a closed trusted-keys file and
// refuse anything else as invalid-arguments before reading the map.
func TestCEMV1084PortableTrustedKeysArgument(t *testing.T) {
	c := loadReceiptAuthPortableCase(t)
	scratch := t.TempDir()
	mapPath := filepath.Join(scratch, "stable.json")
	if err := os.WriteFile(mapPath, c.raw, 0o644); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(scratch, "bad-keys.json")
	if err := os.WriteFile(bad, []byte(`{"profile":"corvint-trusted-keys/1","keys":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	commands := map[string]func(context.Context, []string) (stableResult, int){"stable": executeStableCLI, "canonical": executeStableCanonicalCLI}
	for name, command := range commands {
		args := func(keys string) []string {
			return []string{"--repository", c.repo, "--map", mapPath, "--expected-base", c.base, "--target", c.target, "--artifacts", c.dir, "--trusted-keys", keys}
		}
		for _, keys := range []string{"trusted-keys.json", bad, filepath.Join(scratch, "missing.json"), c.dir} {
			r, exit := command(context.Background(), args(keys))
			if exit != 2 || r.Stage != "arguments" || strOf(r.Code) != "invalid-arguments" || r.MapSHA256 != nil {
				t.Fatalf("%s --trusted-keys %s: exit=%d stage=%s code=%s", name, keys, exit, r.Stage, strOf(r.Code))
			}
		}
		for keys, want := range map[string]string{"trusted-keys.json": "VERIFIED", "other-trusted-keys.json": "UNTRUSTED_KEY"} {
			r, exit := command(context.Background(), args(filepath.Join(c.dir, keys)))
			if exit != 0 || r.Axes["authentication"] != want {
				t.Fatalf("%s --trusted-keys %s: exit=%d authentication=%s, want %s", name, keys, exit, r.Axes["authentication"], want)
			}
		}
	}
}
