package localauthority

import (
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T) (Receipt, Enrollment, PolicySnapshot, ed25519.PrivateKey) {
	t.Helper()
	key := ed25519.NewKeyFromSeed(make([]byte, 32))
	d := strings.Repeat("a", 64)
	checks := []Check{{"test:TestCanonicalOutput/case:ple-v0-canonical-vector", "wp3codec-canonical-v0", CheckProfile, d, "tools/local-authority/driver.go", "CheckOutput", "wp3codec-wasip1-go1.27-v0", "internal/wp3codec/codec.go"}}
	selection, _ := Digest(checks)
	e := Enrollment{RepositoryID: d, PolicySHA256: d, Profile: Profile, Nonce: d, Audience: "fixture-only", RootID: "unadmitted-fixture", Epoch: "1", Generation: "2", IssuedAt: "1000", ExpiresAt: "2000", Selection: SelectionMode, Checks: checks, Binding: Binding{strings.Repeat("b", 40), strings.Repeat("c", 40), strings.Repeat("d", 40), d, d, selection, d, d, d, d}}
	r := Receipt{Payload: Payload{e, "1100", "COMPLETE", "0", []Row{{checks[0], "PASS"}}}}
	p := PolicySnapshot{RepositoryID: d, PolicySHA256: d, Current: true, Fixture: true, RootID: e.RootID, PublicKey: key.Public().(ed25519.PublicKey), Epoch: "1", Generation: "2", MinimumGeneration: "2", Audience: e.Audience, Checks: checks, TerminalSHA256: map[string]string{}}
	seal(t, &r, &p, key)
	return r, e, p, key
}
func seal(t *testing.T, r *Receipt, p *PolicySnapshot, key ed25519.PrivateKey) {
	t.Helper()
	raw, err := SigningBytes(r.Payload)
	if err != nil {
		t.Fatal(err)
	}
	r.Signature = hex.EncodeToString(ed25519.Sign(key, raw))
	digest, err := Digest(*r)
	if err != nil {
		t.Fatal(err)
	}
	p.TerminalSHA256[r.Payload.Enrollment.Nonce] = digest
}
func TestFixtureNeverAdmitted(t *testing.T) {
	r, e, p, _ := fixture(t)
	if err := VerifyFixture(r, e, p, time.Unix(1200, 0)); err != nil {
		t.Fatal(err)
	}
	p.Accepted = true
	p.Current = true
	if err := Verify(r, e, p, time.Unix(1200, 0)); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("fixture admitted: %v", err)
	}
}
func TestSignedSemanticSubstitutionsRefused(t *testing.T) {
	t.Run("PLE-V0-006 fixture signatures cannot excuse semantic substitution", func(t *testing.T) {
		mutations := map[string]func(*Receipt, *Enrollment, *PolicySnapshot){
			"repository":  func(r *Receipt, e *Enrollment, p *PolicySnapshot) { p.RepositoryID = strings.Repeat("e", 64) },
			"policy":      func(r *Receipt, e *Enrollment, p *PolicySnapshot) { p.PolicySHA256 = strings.Repeat("e", 64) },
			"revoked":     func(r *Receipt, e *Enrollment, p *PolicySnapshot) { p.Revoked = true },
			"not-current": func(r *Receipt, e *Enrollment, p *PolicySnapshot) { p.Current = false },
			"target": func(r *Receipt, e *Enrollment, p *PolicySnapshot) {
				r.Payload.Enrollment.Binding.Target = strings.Repeat("e", 40)
			},
			"tree": func(r *Receipt, e *Enrollment, p *PolicySnapshot) {
				r.Payload.Enrollment.Binding.Tree = strings.Repeat("e", 40)
			},
			"cem": func(r *Receipt, e *Enrollment, p *PolicySnapshot) {
				r.Payload.Enrollment.Binding.CEMSHA256 = strings.Repeat("e", 64)
			},
			"ocm": func(r *Receipt, e *Enrollment, p *PolicySnapshot) {
				r.Payload.Enrollment.Binding.OCMSHA256 = strings.Repeat("e", 64)
			},
			"source": func(r *Receipt, e *Enrollment, p *PolicySnapshot) {
				r.Payload.Enrollment.Binding.SourceSHA256 = strings.Repeat("e", 64)
			},
			"wasm": func(r *Receipt, e *Enrollment, p *PolicySnapshot) {
				r.Payload.Enrollment.Binding.WasmSHA256 = strings.Repeat("e", 64)
			},
			"worker": func(r *Receipt, e *Enrollment, p *PolicySnapshot) {
				r.Payload.Enrollment.Binding.WorkerSHA256 = strings.Repeat("e", 64)
			},
			"recipe": func(r *Receipt, e *Enrollment, p *PolicySnapshot) {
				r.Payload.Enrollment.Binding.RecipeSHA256 = strings.Repeat("e", 64)
			},
			"nonce": func(r *Receipt, e *Enrollment, p *PolicySnapshot) {
				r.Payload.Enrollment.Nonce = strings.Repeat("e", 64)
			},
			"audience":           func(r *Receipt, e *Enrollment, p *PolicySnapshot) { p.Audience = "other" },
			"epoch":              func(r *Receipt, e *Enrollment, p *PolicySnapshot) { p.Epoch = "2" },
			"rollback":           func(r *Receipt, e *Enrollment, p *PolicySnapshot) { p.MinimumGeneration = "3" },
			"cleanup":            func(r *Receipt, e *Enrollment, p *PolicySnapshot) { r.Payload.Cleanup = "UNKNOWN" },
			"exit":               func(r *Receipt, e *Enrollment, p *PolicySnapshot) { r.Payload.ExitCode = "1" },
			"unsupported-status": func(r *Receipt, e *Enrollment, p *PolicySnapshot) { r.Payload.Rows[0].Status = "SKIP" },
			"drop-row":           func(r *Receipt, e *Enrollment, p *PolicySnapshot) { r.Payload.Rows = []Row{} },
			"duplicate-row": func(r *Receipt, e *Enrollment, p *PolicySnapshot) {
				r.Payload.Rows = append(r.Payload.Rows, r.Payload.Rows[0])
			},
			"unknown-profile": func(r *Receipt, e *Enrollment, p *PolicySnapshot) { r.Payload.Enrollment.Profile = "tcq/0" },
			"driver": func(r *Receipt, e *Enrollment, p *PolicySnapshot) {
				r.Payload.Rows[0].Check.DriverSHA256 = strings.Repeat("e", 64)
			},
		}
		for name, mutate := range mutations {
			t.Run(name, func(t *testing.T) {
				r, e, p, key := fixture(t)
				mutate(&r, &e, &p)
				seal(t, &r, &p, key)
				if VerifyFixture(r, e, p, time.Unix(1200, 0)) == nil {
					t.Fatal("accepted substitution")
				}
			})
		}

	})
}
func TestJournalFreshnessAndStrictWire(t *testing.T) {
	t.Run("PLE-V0-007 fixture freshness and journal replay refusal", func(t *testing.T) {
		r, e, p, _ := fixture(t)
		if VerifyFixture(r, e, p, time.Unix(2001, 0)) == nil {
			t.Fatal("stale receipt accepted")
		}
		delete(p.TerminalSHA256, e.Nonce)
		if VerifyFixture(r, e, p, time.Unix(1200, 0)) == nil {
			t.Fatal("unjournaled replay accepted")
		}
		raw, _ := Canonical(r)
		var decoded Receipt
		if err := Decode(raw, &decoded); err != nil {
			t.Fatal(err)
		}
		for _, bad := range [][]byte{append(append([]byte{}, raw...), 10), []byte(strings.Replace(string(raw), "{", "{\"extra\":true,", 1)), []byte(strings.Replace(string(raw), "\"cleanup\":\"COMPLETE\"", "\"cleanup\":\"COMPLETE\",\"cleanup\":\"COMPLETE\"", 1))} {
			if Decode(bad, &decoded) == nil {
				t.Fatal("noncanonical/unknown accepted")
			}
		}

	})
}

func TestCompleteFailureAuthenticatesWithoutPassing(t *testing.T) {
	r, e, p, key := fixture(t)
	r.Payload.Rows[0].Status = "FAIL"
	seal(t, &r, &p, key)
	if err := VerifyFixture(r, e, p, time.Unix(1200, 0)); err != nil {
		t.Fatalf("completed failure did not authenticate: %v", err)
	}
	if r.Payload.Rows[0].Status == "PASS" {
		t.Fatal("failure relabelled as pass")
	}
	r.Payload.ExitCode = "1"
	seal(t, &r, &p, key)
	if err := VerifyFixture(r, e, p, time.Unix(1200, 0)); err == nil {
		t.Fatal("infrastructure failure authenticated")
	}
}
