package typescript

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	json "encoding/json/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const discoveryFixtureRevision = "1111111111111111111111111111111111111111"

// Input membership is provided by an independent fixture oracle, never selector output.
func discoveryFixtureBytes(t *testing.T, root string, units []PlaywrightDiscoveryUnit) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, "playwright.config.ts"))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	digest, err := ObservePlaywrightSources(root, "playwright.config.ts")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(PlaywrightDiscovery{Profile: "playwright-discovery/0", Revision: discoveryFixtureRevision, Config: PlaywrightConfigIdentity{Path: "playwright.config.ts", SHA256: hex.EncodeToString(sum[:])}, SourceDigest: digest, Units: units}, json.Deterministic(true))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func smallDiscoveryUnits() []PlaywrightDiscoveryUnit {
	return []PlaywrightDiscoveryUnit{
		{Project: "angular", Test: "tests/login.spec.ts"}, {Project: "angular", Test: "tests/other.spec.ts"},
		{Project: "chromium", Test: "tests/login.spec.ts"}, {Project: "chromium", Test: "tests/other.spec.ts"},
		{Project: "cleanup", Test: "tests/global.teardown.ts"},
		{Project: "react", Test: "tests/login.spec.ts"}, {Project: "react", Test: "tests/other.spec.ts"},
		{Project: "setup", Test: "tests/global.setup.ts"},
	}
}

func TestPlaywrightDiscoveryReconciliation(t *testing.T) {
	t.Run("TJAA-V0-012 discovered membership excludes helper argv", func(t *testing.T) {
		root := playwrightFixture(t)
		write(t, root, "tests/api.ts", `export const api = 1`)
		raw := discoveryFixtureBytes(t, root, smallDiscoveryUnits())
		plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, []string{"tests/pages/login.ts"}, raw)
		if err != nil {
			t.Fatal(err)
		}
		if plan.Discovery.State != "MATCHED" || len(plan.Selected) != 5 || len(plan.Excluded) != 3 || len(plan.FallbackArgv) != 0 {
			t.Fatalf("%+v", plan)
		}
		for _, unit := range plan.Selected {
			if !strings.Contains(unit.Test, ".spec.") && !strings.Contains(unit.Test, ".setup.") && !strings.Contains(unit.Test, ".teardown.") {
				t.Fatal("helper became runnable", unit)
			}
		}
		unrelated, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, []string{"tests/api.ts"}, raw)
		if err != nil || len(unrelated.Selected) != 0 || unrelated.Discovery.State != "MATCHED" {
			t.Fatal(unrelated, err)
		}
	})
	t.Run("TJAA-V0-015 unproven universe uses one suite command", func(t *testing.T) {
		root := playwrightFixture(t)
		valid := discoveryFixtureBytes(t, root, smallDiscoveryUnits())
		for _, row := range []struct {
			name  string
			edit  func([]byte) []byte
			state string
		}{
			{"absent", func([]byte) []byte { return nil }, "MISSING"},
			{"truncated", func(b []byte) []byte { return b[:len(b)-2] }, "MALFORMED"},
			{"duplicate-key", func(b []byte) []byte {
				return bytes.Replace(b, []byte(`"profile":`), []byte(`"profile":"playwright-discovery/0","profile":`), 1)
			}, "MALFORMED"},
			{"unknown-field", func(b []byte) []byte { return append([]byte(`{"extra":true,`), b[1:]...) }, "MALFORMED"},
			{"stale-revision", func(b []byte) []byte {
				return bytes.ReplaceAll(b, []byte(discoveryFixtureRevision), []byte(strings.Repeat("2", 40)))
			}, "BINDING_MISMATCH"},
			{"noncanonical-path", func(b []byte) []byte {
				return bytes.ReplaceAll(b, []byte("tests/login.spec.ts"), []byte("tests/../tests/login.spec.ts"))
			}, "MALFORMED"},
			{"profile", func(b []byte) []byte {
				return bytes.ReplaceAll(b, []byte("playwright-discovery/0"), []byte("playwright-discovery/1"))
			}, "MALFORMED"},
			{"bound", func([]byte) []byte { return bytes.Repeat([]byte("x"), PlaywrightDiscoveryMaxBytes+1) }, "MALFORMED"},
		} {
			t.Run(row.name, func(t *testing.T) {
				plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, []string{"tests/pages/login.ts"}, row.edit(valid))
				if err != nil {
					t.Fatal(err)
				}
				assertDiscoveryFallback(t, plan, row.state)
			})
		}
		for _, mutate := range []func(*PlaywrightDiscovery){
			func(r *PlaywrightDiscovery) { r.Config.SHA256 = strings.Repeat("0", 64) },
			func(r *PlaywrightDiscovery) { r.SourceDigest = "playwright-sources:sha256:" + strings.Repeat("0", 64) },
		} {
			var r PlaywrightDiscovery
			_ = json.Unmarshal(valid, &r)
			mutate(&r)
			raw, _ := json.Marshal(r, json.Deterministic(true))
			plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, nil, raw)
			if err != nil {
				t.Fatal(err)
			}
			assertDiscoveryFallback(t, plan, "BINDING_MISMATCH")
		}
	})
	t.Run("TJAA-V0-017 exact discovery differences and canonical repeated bytes", func(t *testing.T) {
		root := playwrightFixture(t)
		for _, units := range [][]PlaywrightDiscoveryUnit{
			smallDiscoveryUnits()[1:],
			append([]PlaywrightDiscoveryUnit{{Project: "angular", Test: "tests/api.ts"}}, smallDiscoveryUnits()...),
		} {
			raw := discoveryFixtureBytes(t, root, units)
			plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, nil, raw)
			if err != nil {
				t.Fatal(err)
			}
			assertDiscoveryFallback(t, plan, "UNIVERSE_MISMATCH")
			if len(plan.Discovery.OnlyInReceipt)+len(plan.Discovery.OnlyInStatic) != 1 {
				t.Fatal(plan.Discovery)
			}
			second, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, nil, raw)
			if err != nil {
				t.Fatal(err)
			}
			a, _ := plan.Canonical()
			b, _ := second.Canonical()
			if !bytes.Equal(a, b) {
				t.Fatal("unstable bytes")
			}
		}
		for _, units := range [][]PlaywrightDiscoveryUnit{append(smallDiscoveryUnits(), smallDiscoveryUnits()[0]), {smallDiscoveryUnits()[1], smallDiscoveryUnits()[0]}} {
			plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, nil, discoveryFixtureBytes(t, root, units))
			if err != nil {
				t.Fatal(err)
			}
			assertDiscoveryFallback(t, plan, "MALFORMED")
		}
	})
	t.Run("TJAA-V0-015 matched dynamic closure widens only to discovered files", func(t *testing.T) {
		root := playwrightFixture(t)
		write(t, root, "tests/pages/login.ts", `export const page = import(process.env.PAGE)`)
		plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, []string{"tests/pages/login.ts"}, discoveryFixtureBytes(t, root, smallDiscoveryUnits()))
		if err != nil {
			t.Fatal(err)
		}
		if plan.Scope != "UNKNOWN" || plan.Discovery.State != "MATCHED" || len(plan.Selected) != 8 || len(plan.Excluded) != 0 || len(plan.FallbackArgv) != 0 {
			t.Fatal(plan)
		}
	})
}

func assertDiscoveryFallback(t *testing.T, plan PlaywrightPlan, state string) {
	t.Helper()
	if plan.Discovery.State != state || plan.Scope != "UNKNOWN" || plan.Fallback != PlaywrightFallbackFullSuite || len(plan.Selected) != 0 || len(plan.Excluded) != 0 || !slices.Equal(plan.FallbackArgv, []string{"npx", "playwright", "test", "--config=playwright.config.ts"}) {
		t.Fatalf("%+v", plan)
	}
}
