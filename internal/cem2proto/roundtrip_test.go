package cem2proto

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// CEM2-V0-001, -016: nothing outside this package imports it, and the label is
// not the unclaimed cem/2.0.
func TestCEM2V0001PrototypeIsUnwired(t *testing.T) {
	if ProtoSpec == "cem/2.0" || !strings.HasSuffix(ProtoSpec, "-proto") {
		t.Fatalf("label %q must stay non-final", ProtoSpec)
	}
	out, err := exec.Command("git", "-C", "../..", "grep", "-l", "-F", `"github.com/Beamfall/corvint/internal/cem2proto"`, "--", "*.go", ":!internal/cem2proto/").CombinedOutput()
	var exit *exec.ExitError
	if err != nil && !(errors.As(err, &exit) && exit.ExitCode() == 1) {
		t.Fatalf("git grep: %v\n%s", err, out)
	}
	if len(bytes.TrimSpace(out)) > 0 {
		t.Fatalf("cem2proto is imported outside itself:\n%s", out)
	}
}

// CEM2-V0-003: the header is closed, canonical and typed.
func TestCEM2V0003HeaderIsClosed(t *testing.T) {
	f := newFixture(t, nil)
	header, _ := f.encode(t, false)
	if _, err := DecodeHeader(header); err != nil {
		t.Fatal(err)
	}
	for name, raw := range map[string]string{
		"unknown member":  strings.Replace(string(header), `{"base"`, `{"alg":"x","base"`, 1),
		"missing member":  strings.Replace(string(header), `"critical":[],`, ``, 1),
		"non-canonical":   strings.Replace(string(header), `,"critical"`, `, "critical"`, 1),
		"trailing LF":     string(header) + "\n",
		"wrong type":      strings.Replace(string(header), `"parents":[]`, `"parents":{}`, 1),
		"bad role":        strings.Replace(string(header), `"role":"change"`, `"role":"other"`, 1),
		"non-string list": strings.Replace(string(header), `"critical":[]`, `"critical":[1]`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if raw == string(header) {
				t.Fatal("tamper did not apply")
			}
			_, err := DecodeHeader([]byte(raw))
			wantCode(t, err, CodeMalformed)
		})
	}
	if bytes.Contains(header, []byte(f.target)) {
		t.Fatal("the target commit must stay external to the header")
	}
}

// CEM2-V0-005: fixture maps project back byte-identically, in derived and
// pinned mode, with every role and with provenance.
func TestCEM2V0005FixtureRoundTrip(t *testing.T) {
	provenance := &wire.StableProvenance{
		SourceSpec:    "cem/0.3",
		SourceBlobOid: strings.Repeat("a", 40),
		SourceSha256:  strings.Repeat("b", 64),
		BindCommit:    strings.Repeat("c", 40),
		Converter:     "corvint-cem-convert/1",
	}
	cases := map[string]func(*wire.StableMap){
		"change":     nil,
		"bind-chore": func(m *wire.StableMap) { m.Role = wire.StableRoleBindChore },
		"seal-chore": func(m *wire.StableMap) { m.Role = wire.StableRoleSealChore },
		"provenance": func(m *wire.StableMap) { m.Provenance = provenance },
	}
	for name, mutate := range cases {
		for _, pinned := range []bool{false, true} {
			t.Run(name+map[bool]string{false: "/derived", true: "/pinned"}[pinned], func(t *testing.T) {
				f := newFixture(t, mutate)
				header, body := f.encode(t, pinned)
				got, err := f.project(t, header, body)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(got, f.original) {
					t.Fatalf("projection differs:\n got %s\nwant %s", got, f.original)
				}
				if len(header)+len(body) >= len(f.original) {
					t.Fatalf("2.0 form (%d B) is not smaller than 1.0 (%d B)", len(header)+len(body), len(f.original))
				}
			})
		}
	}
}

// CEM2-V0-006: claims bind by patch ordinal. Reordered rows break the body
// digest; resealed, they are a different map with permuted claims.
func TestCEM2V0006HunkReorderPermutesClaims(t *testing.T) {
	f := newFixture(t, nil)
	header, body := f.encode(t, false)
	b := decoded(t, body)
	b.Hunks[0], b.Hunks[2] = b.Hunks[2], b.Hunks[0]
	tampered, err := EncodeBody(b)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.project(t, header, tampered)
	wantCode(t, err, CodeBodySha256Mismatch)

	header, tampered = reseal(t, header, b)
	got, err := f.project(t, header, tampered)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(got, f.original) {
		t.Fatal("a reordered body must not be equivalent")
	}
	projected, err := wire.ParseStable(got)
	if err != nil {
		t.Fatal(err)
	}
	for i, h := range projected.Change.Hunks {
		original := f.stable.Change.Hunks[i]
		if h.ID != original.ID || h.Path != original.Path || h.OldRange != original.OldRange {
			t.Fatalf("hunk %d identity moved with its claim", i)
		}
	}
	if projected.Change.Hunks[0].Reason != f.stable.Change.Hunks[2].Reason || projected.Change.Hunks[2].Reason != f.stable.Change.Hunks[0].Reason {
		t.Fatal("claims did not permute by ordinal")
	}
}

// CEM2-V0-020: maps the prototype cannot reproduce get UNSUPPORTED, no bytes.
func TestCEM2V0020EncoderAbstains(t *testing.T) {
	base := newFixture(t, nil)
	cases := map[string]func(*wire.StableMap){
		"criteria":           func(m *wire.StableMap) { m.Criteria = []wire.StableCriterion{{}} },
		"receipts":           func(m *wire.StableMap) { m.Receipts = []wire.StableReceipt{{}} },
		"links":              func(m *wire.StableMap) { m.Links = []wire.StableLink{{}} },
		"artifacts":          func(m *wire.StableMap) { m.Artifacts = []wire.StableArtifact{{}} },
		"coverage witness":   func(m *wire.StableMap) { m.Change.Hunks[0].Coverage = &wire.CoverageWitness{} },
		"mutation witness":   func(m *wire.StableMap) { m.Change.Hunks[0].Discriminates = &wire.DiscriminationWitness{} },
		"explicit role":      func(m *wire.StableMap) { m.Role = wire.StableRoleChange },
		"retired spec":       func(m *wire.StableMap) { m.Change.Spec = "cem/0.3" },
		"other excludedPath": func(m *wire.StableMap) { m.Change.ExcludedPath = "other.json" },
		"forged evidence id": func(m *wire.StableMap) { m.Change.Evidence[0].ID = "evidence:sha256:" + strings.Repeat("0", 64) },
		"repeated evidence":  func(m *wire.StableMap) { m.Change.Evidence = append(m.Change.Evidence, m.Change.Evidence[0]) },
		"disposition drift":  func(m *wire.StableMap) { m.Change.Hunks[2].Disposition = "supported" },
		"unknown relation":   func(m *wire.StableMap) { m.Change.Hunks[0].Basis[0].Relation = "rumour" },
		"dangling basis": func(m *wire.StableMap) {
			m.Change.Hunks[0].Basis[0].EvidenceID = "evidence:sha256:" + strings.Repeat("1", 64)
		},
		"unknown reason code": func(m *wire.StableMap) { m.Change.Hunks[2].Reason = "vibes" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			m := cloneMap(base.stable)
			mutate(m)
			header, body, err := Encode(m, false)
			var typed *Error
			if !errors.As(err, &typed) || !typed.Unsupported() || header != nil || body != nil {
				t.Fatalf("Encode = %v, %d+%d bytes; want UNSUPPORTED and no bytes", err, len(header), len(body))
			}
		})
	}
}
