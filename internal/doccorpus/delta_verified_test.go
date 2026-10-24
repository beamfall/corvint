package doccorpus

import (
	"bytes"
	"context"
	"os"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// TestDCRV1460DeltaVerifiedReadMatchesRederived: over a pair with carry-forward, moved, changed,
// removed, unresolved and uncovered entries, the verified current side and canonical previous
// side give byte-identical paged receipts and jsonl streams to the --verify-rederive path and to
// the earlier ParseArtifact and OpenDeltaCurrent path, in every view; the verified path runs no
// build and the rederived path does (DCR-V1-460, DCR-V1-461). Not parallel: it counts builds.
func TestDCRV1460DeltaVerifiedReadMatchesRederived(t *testing.T) {
	ctx := context.Background()
	root, previous, m := deltaFixture(t)
	current, err := Build(ctx, root, m)
	if err != nil {
		t.Fatal(err)
	}
	previousData, err := Encode(previous)
	if err != nil {
		t.Fatal(err)
	}
	currentData, err := Encode(current)
	if err != nil {
		t.Fatal(err)
	}
	type sides struct{ previous, current *Artifact }
	open := map[string]func() sides{
		"verified": func() sides {
			before := BuildsForTest()
			p, err := OpenDeltaPrevious(previousData, nil)
			if err != nil {
				t.Fatal(err)
			}
			c, err := OpenDeltaRead(ctx, root, currentData, ReadOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if got := BuildsForTest() - before; got != 0 || c.validation != ValidationVerified {
				t.Fatalf("verified delta sides ran %d builds, validation %q", got, c.validation)
			}
			return sides{p, c}
		},
		"rederived": func() sides {
			before := BuildsForTest()
			p, err := OpenDeltaPrevious(previousData, nil)
			if err != nil {
				t.Fatal(err)
			}
			c, err := OpenDeltaRead(ctx, root, currentData, ReadOptions{Rederive: true})
			if err != nil {
				t.Fatal(err)
			}
			if BuildsForTest() == before || c.validation != ValidationRederived {
				t.Fatalf("--verify-rederive delta side did not rederive: validation %q", c.validation)
			}
			return sides{p, c}
		},
		"earlier": func() sides {
			p, err := ParseArtifact(previousData)
			if err != nil {
				t.Fatal(err)
			}
			c, err := OpenDeltaCurrent(ctx, root, currentData)
			if err != nil {
				t.Fatal(err)
			}
			return sides{p, c}
		},
	}
	views := []DeltaOptions{{}, {Uncovered: DeltaUncoveredChanged}, {Anchorless: DeltaAnchorlessList}, {Offset: 3, Limit: 2}}
	outputs := map[string][]byte{}
	for _, name := range []string{"verified", "rederived", "earlier"} {
		s := open[name]()
		var out []byte
		for _, view := range views {
			view.Timings = &ReadTimings{}
			stream, err := DeltaStream(ctx, root, s.previous, s.current, "artifact", view)
			if err != nil {
				t.Fatal(err)
			}
			receipt, err := DeltaWith(ctx, root, s.previous, s.current, "artifact", view)
			if err != nil {
				t.Fatal(err)
			}
			paged, err := Encode(receipt)
			if err != nil {
				t.Fatal(err)
			}
			if view.Timings.Compare <= 0 || view.Timings.Encode <= 0 {
				t.Fatalf("%s delta timings not recorded: %+v", name, view.Timings)
			}
			out = append(append(out, stream...), paged...)
			if name == "verified" && view == (DeltaOptions{Timings: view.Timings}) {
				for _, class := range []string{DeltaCarryForward, DeltaMoved, DeltaChanged, DeltaRemoved, DeltaUnresolved, DeltaUncovered} {
					if receipt.Counts[class] == 0 {
						t.Fatalf("fixture lacks a %s entry: %+v", class, receipt.Counts)
					}
				}
			}
		}
		outputs[name] = out
	}
	if !bytes.Equal(outputs["verified"], outputs["rederived"]) || !bytes.Equal(outputs["verified"], outputs["earlier"]) {
		t.Fatal("verified delta output differs from the rederived output")
	}
}

// TestDCRV1460DeltaVerifiedReadRefusesClosed: a current side the verified read refuses is
// refused by the delta, never served from its unverified bytes, and a previous side keeps the
// closed decoder's admission (DCR-V1-460, DCR-V1-461).
func TestDCRV1460DeltaVerifiedReadRefusesClosed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	root, a, data := builtFixture(t)
	for name, bad := range map[string][]byte{
		"builder": rehashed(t, a, func(b *Artifact) {
			b.Builder.Revision = strings.Repeat("0", 40)
			b.Builder.Engine = "native-contextindex/" + Schema + "/" + runtime.Version()
		}),
		"tree":         rehashed(t, a, func(b *Artifact) { b.Tree = strings.Repeat("0", 40) }),
		"digest":       bytes.Replace(data, []byte(`"sha256":"`+a.SHA256), []byte(`"sha256":"`+strings.Repeat("0", 64)), 1),
		"noncanonical": append([]byte(" "), data...),
	} {
		if _, err := OpenDeltaRead(ctx, root, bad, ReadOptions{}); err == nil {
			t.Errorf("%s: verified delta current side admitted", name)
		}
		_, closed := ParseArtifact(bad)
		if _, err := OpenDeltaPrevious(bad, nil); (err == nil) != (closed == nil) {
			t.Errorf("%s: previous side admission %v differs from ParseArtifact %v", name, err, closed)
		}
	}
	if _, err := OpenDeltaPrevious(data, nil); err != nil {
		t.Fatal(err)
	}
}

// TestDCRV1461PreviousOverDefaultBound: a schema /2 previous artifact larger than the default
// 4 MiB bound is admitted by OpenDeltaPrevious exactly as by ParseArtifact; the parallel decode
// reads the schema before it applies the bound even without a header callback (DCR-V1-461).
func TestDCRV1461PreviousOverDefaultBound(t *testing.T) {
	t.Parallel()
	_, a, _ := builtFixture(t)
	data := rehashed(t, a, func(b *Artifact) {
		b.Schema = SchemaV2
		b.Subjects = append([]Subject(nil), b.Subjects...)
		b.Subjects[0].Name = strings.Repeat("x", MaxBytes)
	})
	if len(data) <= MaxBytes {
		t.Fatalf("fixture artifact is %d bytes, not over the default bound", len(data))
	}
	closed, err := ParseArtifact(data)
	if err != nil {
		t.Fatal(err)
	}
	got, err := OpenDeltaPrevious(data, nil)
	if err != nil {
		t.Fatalf("previous side refused what ParseArtifact admits: %v", err)
	}
	if !EncodedEqual(got, data) || got.SHA256 != closed.SHA256 {
		t.Fatal("previous side decoded a different artifact")
	}
}

// TestDCRV1460DeltaVerifiedReadUnavailableCheckout: on the default verified branch, an
// unavailable additional checkout still takes the DCX-V1-425 path, observed from the header
// members, and an available input that does not name its blob is still refused; an invalid
// manifest is refused on both branches before any checkout is examined (DCR-V1-460).
func TestDCRV1460DeltaVerifiedReadUnavailableCheckout(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newDeltaAddRepoFixture(t)
	encode := func(a Artifact) []byte {
		t.Helper()
		a.SHA256 = ""
		a.SHA256 = testHash(t, a)
		data, err := Encode(a)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	retained, err := Encode(f.previous)
	if err != nil {
		t.Fatal(err)
	}
	timings := &ReadTimings{}
	if a, err := OpenDeltaRead(ctx, f.root, retained, ReadOptions{Timings: timings}); err != nil || a.deltaUnverified != nil || timings.Validation != ValidationVerified {
		t.Fatalf("available checkout not verified: %v %q", err, timings.Validation)
	}
	invalid := *f.previous
	invalid.Manifest.Profile.Format = "bogus"
	if err := os.Rename(f.lib, f.lib+"-gone"); err != nil {
		t.Fatal(err)
	}
	timings = &ReadTimings{}
	a, err := OpenDeltaRead(ctx, f.root, retained, ReadOptions{Timings: timings})
	if err != nil || len(a.deltaUnverified) != 1 || !strings.Contains(a.deltaUnverified[0], f.extra.ID) || timings.Validation != ValidationUnrederived {
		t.Fatalf("unavailable checkout on the verified branch: %v %v %q", err, a, timings.Validation)
	}
	rederived, err := OpenDeltaCurrent(ctx, f.root, retained)
	if err != nil {
		t.Fatal(err)
	}
	for _, current := range []*Artifact{a, rederived} {
		receipt, err := DeltaWith(ctx, f.root, f.previous, current, "artifact", DeltaOptions{Limit: MaxResults})
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Contains(receipt.Limitations, a.deltaUnverified[0]) || entriesByID(receipt)["adapter:lib"].Class != DeltaUnresolved {
			t.Fatalf("unavailable checkout not disclosed: %+v", receipt)
		}
	}
	tampered := *f.previous
	tampered.Manifest.Inputs = slices.Clone(tampered.Manifest.Inputs)
	for i, in := range tampered.Manifest.Inputs {
		if in.Revision == tampered.Manifest.Repository.Revision {
			tampered.Manifest.Inputs[i].Blob = f.extra.ID
			break
		}
	}
	for _, rederive := range []bool{false, true} {
		if _, err := OpenDeltaRead(ctx, f.root, encode(tampered), ReadOptions{Rederive: rederive}); err == nil || !strings.Contains(err.Error(), "does not name the blob its path holds") {
			t.Fatalf("rederive=%t: tampered primary input admitted: %v", rederive, err)
		}
		if _, err := OpenDeltaRead(ctx, f.root, encode(invalid), ReadOptions{Rederive: rederive}); err == nil || !strings.Contains(err.Error(), "unsupported render profile") {
			t.Fatalf("rederive=%t: invalid manifest not refused first: %v", rederive, err)
		}
	}
}

// TestDCRV1464DeltaVerifiedReadChecksInputBindings: with every checkout available, a current
// artifact whose changed input names the previous blob and SHA-256, with its manifest and artifact
// digests recomputed, is refused by the verified delta current side before any classification, as
// by the rederivation, and never classified carry-forward (DCR-V1-464).
func TestDCRV1464DeltaVerifiedReadChecksInputBindings(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	root, previous, m := deltaFixture(t)
	current, err := Build(ctx, root, m)
	if err != nil {
		t.Fatal(err)
	}
	old := map[string]Input{}
	for _, in := range previous.Manifest.Inputs {
		old[in.Path] = in
	}
	swapped := ""
	data := rehashed(t, current, func(c *Artifact) {
		c.Manifest.Inputs = slices.Clone(c.Manifest.Inputs)
		for i, in := range c.Manifest.Inputs {
			if prior, ok := old[in.Path]; ok && in.Path == "src/report.py" && prior.Blob != in.Blob {
				c.Manifest.Inputs[i].Blob, c.Manifest.Inputs[i].SHA256 = prior.Blob, prior.SHA256
				swapped = in.Path
			}
		}
		c.ManifestSHA256 = testHash(t, c.Manifest)
	})
	if swapped == "" {
		t.Fatal("fixture lacks a changed input to swap")
	}
	if _, err := OpenDeltaRead(ctx, root, data, ReadOptions{}); err == nil || !strings.Contains(err.Error(), "input "+swapped+" at ") || !strings.Contains(err.Error(), "does not name the blob its path holds") {
		t.Fatalf("verified delta current side admitted a swapped input blob: %v", err)
	}
	if _, err := OpenDeltaRead(ctx, root, data, ReadOptions{Rederive: true}); err == nil {
		t.Fatal("rederived delta current side admitted a swapped input blob")
	}
}
