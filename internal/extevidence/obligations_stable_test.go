package extevidence

import (
	"bytes"
	"context"
	"errors"
	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/stablefixture"
	"github.com/Beamfall/corvint/internal/gokernel"
)

type stableObligationsFixture struct {
	change stablefixture.Change
	stable stablefixture.Stable
}

func newStableObligationsFixture(t *testing.T) stableObligationsFixture {
	t.Helper()
	change := stablefixture.NewChange(t)
	return stableObligationsFixture{change: change, stable: change.Stable(t)}
}

func (f stableObligationsFixture) options() StableObligationsOptions {
	return StableObligationsOptions{Repository: f.change.Root, ExpectedBase: f.change.Base, Target: f.stable.Target, ArtifactRoot: f.stable.ArtifactRoot}
}

// stableObligationsReceipt is the frozen test receipt joined to the fixture's
// changed path.
func stableObligationsReceipt() []byte {
	return bytes.ReplaceAll(obligationsReceipt(false), []byte("pkg/main.go"), []byte("src/app.txt"))
}

// TestCEMV1130StableObligationsAccepts (CEM-V1-130, CEM-V1-131, CEM-V1-133): a
// verified cem/1.0 map yields the stable sidecar bound to the exact map, patch
// and target; the hunk keeps its associations and the declared criterion link
// is listed apart from them with no satisfaction.
func TestCEMV1130StableObligationsAccepts(t *testing.T) {
	f := newStableObligationsFixture(t)
	document, err := StableObligations(context.Background(), f.stable.Map, stableObligationsReceipt(), 64, f.options())
	if err != nil {
		t.Fatal(err)
	}
	if document["schema"] != StableObligationsSchema || !strings.HasPrefix(document["id"].(string), "obligations-stable:sha256:") {
		t.Fatalf("schema %v id %v", document["schema"], document["id"])
	}
	binding := document["binding"].(map[string]any)
	for key, want := range map[string]any{
		"cem_spec": "cem/1.0", "cem_sha256": stablefixture.Digest(f.stable.Map), "base_revision": f.change.Base,
		"target_revision": f.stable.Target, "verification": "ACCEPT", "impact_sha256": digest(stableObligationsReceipt()),
	} {
		if binding[key] != want {
			t.Fatalf("binding.%s = %v, want %v", key, binding[key], want)
		}
	}
	hunks := obligationsHunks(t, document)
	if len(hunks) != 1 || hunks[0]["hunk"] != f.change.HunkID || hunks[0]["path"] != "src/app.txt" {
		t.Fatalf("hunks = %v", hunks)
	}
	kinds := map[string]int{}
	for _, row := range associationsOf(hunks[0]) {
		kinds[row["kind"].(string)]++
		if _, closed := row["criterion"]; closed {
			t.Fatalf("association carries a criterion: %v", row)
		}
	}
	if kinds["entity"] != 2 || kinds["obligation"] != 1 || kinds["test"] != 1 {
		t.Fatalf("association kinds = %v", kinds)
	}
	criteria := document["criteria"].([]any)
	if len(criteria) != 1 {
		t.Fatalf("criteria = %v", criteria)
	}
	link := criteria[0].(map[string]any)
	if link["criterion"] != f.stable.CriterionID || link["satisfaction"] != "NOT_OBSERVED" || link["ticket"] != "ticket:beamfall:corvint:V1-0632" {
		t.Fatalf("criterion link = %v", link)
	}
	encoded, err := gokernel.CanonicalJSON(document)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte(`"external-frontier-obligations/0"`)) {
		t.Fatal("stable sidecar relabels the frozen schema")
	}
}

// TestCEMV1131StableObligationsKeepUnknowns (CEM-V1-131): a receipt without an
// external section keeps the hunk's explicit unknown; nothing closes it.
func TestCEMV1131StableObligationsKeepUnknowns(t *testing.T) {
	f := newStableObligationsFixture(t)
	receipt := []byte(`{"ok":true,"tool":"impact","context":{"profile":"impact"}}`)
	document, err := StableObligations(context.Background(), f.stable.Map, receipt, 64, f.options())
	if err != nil {
		t.Fatal(err)
	}
	rows := associationsOf(obligationsHunks(t, document)[0])
	if document["state"] != ObligationsUnknown || len(rows) != 1 || rows[0]["code"] != "no-external-section" {
		t.Fatalf("state %v rows %v", document["state"], rows)
	}
}

// TestCEMV1132StableObligationsRefusals (CEM-V1-132): a wrong base, edited map
// bytes, a missing artifact root and a tampered artifact each refuse with the
// stable verifier's retained result and no sidecar.
func TestCEMV1132StableObligationsRefusals(t *testing.T) {
	f := newStableObligationsFixture(t)
	edited := append(bytes.TrimSuffix(append([]byte(nil), f.stable.Map...), []byte("\n")), ' ', '\n')
	cases := map[string]func(*[]byte, *StableObligationsOptions){
		"wrong base":            func(_ *[]byte, o *StableObligationsOptions) { o.ExpectedBase = f.change.ProducerTarget },
		"edited map bytes":      func(raw *[]byte, _ *StableObligationsOptions) { *raw = edited },
		"missing artifact root": func(_ *[]byte, o *StableObligationsOptions) { o.ArtifactRoot = "" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			raw, options := f.stable.Map, f.options()
			mutate(&raw, &options)
			document, err := StableObligations(context.Background(), raw, stableObligationsReceipt(), 64, options)
			requireStableObligationsRefusal(t, document, err)
		})
	}
	t.Run("tampered artifact", func(t *testing.T) {
		path := filepath.Join(f.stable.ArtifactRoot, filepath.FromSlash(f.stable.ArtifactPaths[0]))
		if err := os.WriteFile(path, []byte(`{"fixture":"tampered"}`+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		document, err := StableObligations(context.Background(), f.stable.Map, stableObligationsReceipt(), 64, f.options())
		requireStableObligationsRefusal(t, document, err)
	})
}

func requireStableObligationsRefusal(t *testing.T, document map[string]any, err error) {
	t.Helper()
	var refusal *StableObligationsRefusal
	var kernelErr *gokernel.Error
	if document != nil || !errors.As(err, &refusal) || refusal.Result.Outcome == "ACCEPT" || !errors.As(err, &kernelErr) || kernelErr.Code == "" {
		t.Fatalf("document=%v err=%v, want a retained stable refusal", document != nil, err)
	}
}

// TestCEMV1134ObligationRoutesStayDisjoint (CEM-V1-134, CEM-V1-210,
// CEM-V1-217): an invocation without the stable context refuses cem/1.0 as
// the retired frozen route, and the stable sidecar refuses a retired
// cem/0.2 map with unsupported-spec and the recovery line before any
// verification.
func TestCEMV1134ObligationRoutesStayDisjoint(t *testing.T) {
	f := newStableObligationsFixture(t)
	for name, test := range map[string]struct {
		run  func() (map[string]any, error)
		code string
	}{
		"frozen over cem/1.0": {func() (map[string]any, error) {
			return StableObligations(context.Background(), f.stable.Map, stableObligationsReceipt(), 64, StableObligationsOptions{})
		}, "invalid-obligations-input"},
		"stable over cem/0.2": {func() (map[string]any, error) {
			return StableObligations(context.Background(), []byte(`{"spec":"cem/0.2","hunks":[]}`), stableObligationsReceipt(), 64, f.options())
		}, cemcode.UnsupportedSpec},
	} {
		document, err := test.run()
		var kernelErr *gokernel.Error
		var refusal *StableObligationsRefusal
		if document != nil || !errors.As(err, &kernelErr) || kernelErr.Code != test.code || errors.As(err, &refusal) {
			t.Fatalf("%s: document %v err %v", name, document != nil, err)
		}
		if test.code == cemcode.UnsupportedSpec && !strings.Contains(kernelErr.Message, wire.RetiredSpecFix) {
			t.Fatalf("%s: refusal lacks the recovery line: %v", name, err)
		}
	}
}
