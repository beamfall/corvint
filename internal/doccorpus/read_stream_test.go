package doccorpus

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"sync"
	"testing"
	"testing/iotest"
)

// streamCases are the refusal shapes TestDCPV1414 and TestDCPV1419 cover, applied to data.
func streamCases(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	insert := func(after, text string) []byte {
		at := bytes.Index(data, []byte(after))
		if at < 0 {
			t.Fatalf("no %q in fixture", after)
		}
		at += len(after)
		return append(append(bytes.Clone(data[:at]), text...), data[at:]...)
	}
	replace := func(old, new string) []byte {
		if !bytes.Contains(data, []byte(old)) {
			t.Fatalf("no %q in fixture", old)
		}
		return bytes.Replace(data, []byte(old), []byte(new), 1)
	}
	return map[string][]byte{
		"space after claims bracket":  insert(`"claims":[`, " "),
		"space between elements":      replace(`},{"id"`, `}, {"id"`),
		"space before closing":        replace(`}],"relations"`, `} ],"relations"`),
		"space inside element":        replace(`"subject":`, `"subject": `),
		"unknown top-level member":    insert(`{`, `"extra":1,`),
		"unknown element member":      insert(`"claims":[{`, `"extra":1,`),
		"duplicate member":            insert(`{`, `"tree":"x",`),
		"null records":                replace(`"journeys":[]`, `"journeys":null`),
		"renamed member":              replace(`"schema":`, `"schemz":`),
		"trailing value":              append(bytes.Clone(data[:len(data)-1]), []byte("{}\n")...),
		"missing trailing newline":    data[:len(data)-1],
		"element encodes differently": replace(`"start_line":1`, `"start_line":1.0`),
		"unterminated string":         data[:bytes.LastIndex(data, []byte(`"`))],
		"unbalanced element":          replace(`"start_line":1`, `"start_line":[1`),
		"trailing bytes":              append(bytes.Clone(data), 'x'),
		"stray comma":                 replace(`"claims":[`, `"claims":[,`),
		"empty":                       {},
		"truncated":                   data[:len(data)/2],
	}
}

// TestDCPV1431StreamedDecodeMatchesInMemory: the streamed decode admits exactly what the in-memory
// decode admits, with the same records and digest tail, and refuses every shape the closed decoder
// refuses, whichever window and carry room splits the bytes, so every value crosses a window
// boundary at some size and carries both fit and overflow the room before a block.
func TestDCPV1431StreamedDecodeMatchesInMemory(t *testing.T) {
	t.Parallel()
	_, a, _ := builtFixture(t)
	data := rehashed(t, a, func(c *Artifact) {
		c.Claims = append([]Claim{}, c.Claims...)
		c.Claims[0].Text = `quote " backslash \ pair \\" brackets ]},{"id":"x"} [ { end\`
		c.Claims[1].Text = `\"],"relations":[` + "\u2028\x7f"
	})
	want, err := decodeArtifactParallel(data, nil)
	if err != nil {
		t.Fatal(err)
	}
	wantTail := hashArtifactTail(data)
	if err := wantTail.verify(want); err != nil {
		t.Fatal(err)
	}
	cases := streamCases(t, data)
	for _, window := range []int{1, 2, 3, 7, 61, 512, 1 << 20} {
		for _, headroom := range []int{0, 1, 16, 4096} {
			var header *Artifact
			got, tail, err := decodeArtifactBlocks(bytes.NewReader(data), window, headroom, func(h *Artifact) { header = h }, nil, nil)
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("window %d room %d: streamed decode differs: %v", window, headroom, err)
			}
			if tail.digest != wantTail.digest || !bytes.Equal(tail.tail, wantTail.tail) || tail.verify(got) != nil {
				t.Fatalf("window %d room %d: streamed digest tail differs", window, headroom)
			}
			if header == nil || header.Schema != want.Schema || header.Manifest.Repository != want.Manifest.Repository || header.Claims != nil {
				t.Fatalf("window %d room %d: header members %+v", window, headroom, header)
			}
			if window > 7 && window < 1<<20 {
				for name, bad := range cases {
					_, closed := ParseArtifact(bad)
					decoded, tail, err := decodeArtifactBlocks(bytes.NewReader(bad), window, headroom, nil, nil, nil)
					if err == nil {
						err = tail.verify(decoded)
					}
					if err == nil || closed == nil {
						t.Errorf("window %d room %d %s: streamed=%v closed=%v; both must refuse", window, headroom, name, err, closed)
					}
				}
			}
		}
	}
}

// TestDCPV1431StreamedReadReusesBlocks: a streamed read of many blocks reuses block buffers
// instead of allocating one per block, so the artifact is never held whole.
func TestDCPV1431StreamedReadReusesBlocks(t *testing.T) {
	t.Parallel()
	_, a, _ := builtFixture(t)
	data := rehashed(t, a, func(c *Artifact) {
		claims := []Claim{}
		for i := 0; len(claims)*600 < 2<<20; i++ {
			for _, claim := range a.Claims {
				claim.ID = fmt.Sprintf("%s~%06d", claim.ID, i)
				claims = append(claims, claim)
			}
		}
		sort.Slice(claims, func(i, j int) bool { return claims[i].ID < claims[j].ID })
		c.Claims = claims
	})
	want, err := decodeArtifactParallel(data, nil)
	if err != nil {
		t.Fatal(err)
	}
	var stats blockStats
	got, _, err := decodeArtifactBlocks(bytes.NewReader(data), 4096, 1024, nil, nil, &stats)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("streamed decode differs: %v", err)
	}
	if stats.blocks < 400 || stats.allocated*2 > stats.blocks {
		t.Fatalf("%d blocks allocated %d buffers; want reuse", stats.blocks, stats.allocated)
	}
}

// TestDCPV1431OpenReadFile: the streamed file read admits and refuses as OpenRead of the file's
// bytes does, a read error is not reported as a refusal of the bytes read before it, and a missing
// or unsafe path is the read's unavailable refusal.
func TestDCPV1431OpenReadFile(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	root, _, data := builtFixture(t)
	if err := os.WriteFile(filepath.Join(root, "corpus.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	want, err := OpenRead(ctx, root, data, ReadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var timings ReadTimings
	got, err := OpenReadFile(ctx, root, "corpus.json", ReadOptions{Timings: &timings})
	if err != nil || !reflect.DeepEqual(got, want) || timings.Validation != ValidationVerified {
		t.Fatalf("OpenReadFile differs from OpenRead: %v %+v", err, timings)
	}
	edited := bytes.Replace(data, []byte(`"start_line":1`), []byte(`"start_line":2`), 1)
	if err := os.WriteFile(filepath.Join(root, "edited.json"), edited, 0o600); err != nil {
		t.Fatal(err)
	}
	_, inMemory := OpenRead(ctx, root, edited, ReadOptions{})
	_, streamed := OpenReadFile(ctx, root, "edited.json", ReadOptions{})
	if inMemory == nil || streamed == nil || inMemory.Error() != streamed.Error() {
		t.Fatalf("edited artifact: in memory %v, streamed %v", inMemory, streamed)
	}
	if err := os.Symlink(root, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"missing.json", "link/corpus.json"} {
		var coded *Error
		if _, err := OpenReadFile(ctx, root, path, ReadOptions{}); !errors.As(err, &coded) || coded.Code != "corpus-input-unavailable" {
			t.Fatalf("%s: %v", path, err)
		}
	}
	broken := errors.New("disk gone")
	reader := io.MultiReader(bytes.NewReader(data[:len(data)/3]), iotest.ErrReader(broken))
	if _, _, err := decodeArtifactBlocks(reader, 512, 64, nil, nil, nil); !errors.Is(err, broken) {
		t.Fatalf("read error reported as %v", err)
	}
}

// TestDCPV1434DecodeTimeClaimTermsMatchIndex: claim terms derived on the decode workers while a
// read decodes give the query index the reference derivation stores, whichever batches the
// window splits the claims into; blocks that do not cover every claim are not used.
func TestDCPV1434DecodeTimeClaimTermsMatchIndex(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	_, built, _ := builtFixture(t)
	root, m := adoptionFixture(t, nil)
	adoption, err := Build(ctx, root, m)
	if err != nil {
		t.Fatal(err)
	}
	padded, err := Build(ctx, root, m)
	if err != nil {
		t.Fatal(err)
	}
	padIndexCapacity(padded, 3000, 9000, 2000)
	for name, a := range map[string]*Artifact{"built": built, "adoption": adoption, "padded": padded} {
		data, err := Encode(a)
		if err != nil {
			t.Fatal(err)
		}
		reference, err := buildQueryIndexReference(ctx, a)
		if err != nil {
			t.Fatalf("%s: reference: %v", name, err)
		}
		for _, window := range []int{97, 4096, 1 << 20} {
			var mu sync.Mutex
			var blocks []*claimTermBlock
			decoded, _, err := decodeArtifactBlocks(bytes.NewReader(data), window, 64, nil, func(lo int, chunk []Claim) {
				b := deriveClaimTerms(chunk, lo)
				mu.Lock()
				blocks = append(blocks, b)
				mu.Unlock()
			}, nil)
			if err != nil {
				t.Fatalf("%s window %d: %v", name, window, err)
			}
			sort.Slice(blocks, func(i, j int) bool { return blocks[i].lo < blocks[j].lo })
			if len(a.Claims) > 0 && !claimTermBlocksCover(blocks, len(decoded.Claims)) {
				t.Fatalf("%s window %d: blocks do not cover the claims", name, window)
			}
			for _, pre := range [][]*claimTermBlock{blocks, blocks[min(1, len(blocks)):]} {
				decoded.claimTerms = pre
				index, err := BuildQueryIndex(ctx, decoded)
				if err != nil || !reflect.DeepEqual(index, reference) || decoded.claimTerms != nil {
					t.Fatalf("%s window %d (%d blocks): index from decode-time terms differs: %v", name, window, len(pre), err)
				}
			}
		}
	}
}
