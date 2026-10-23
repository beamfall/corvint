package doccorpus

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// TestDCPV1460DeltaVerifiedReadMatchesRederived: over a pair with carry-forward, moved, changed,
// removed, unresolved and uncovered entries, the verified current side and canonical previous
// side give byte-identical paged receipts and jsonl streams to the --verify-rederive path and to
// the earlier ParseArtifact and OpenDeltaCurrent path, in every view; the verified path runs no
// build and the rederived path does (DCP-V1-460, DCP-V1-461). Not parallel: it counts builds.
func TestDCPV1460DeltaVerifiedReadMatchesRederived(t *testing.T) {
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

// TestDCPV1460DeltaVerifiedReadRefusesClosed: a current side the verified read refuses is
// refused by the delta, never served from its unverified bytes, and a previous side keeps the
// closed decoder's admission (DCP-V1-460, DCP-V1-461).
func TestDCPV1460DeltaVerifiedReadRefusesClosed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	root, a, data := builtFixture(t)
	for name, bad := range map[string][]byte{
		"builder":      rehashed(t, a, func(b *Artifact) { b.Builder.Revision = strings.Repeat("0", 40) }),
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
