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
	"strings"
	"sync"
	"testing"
	"testing/iotest"
)

// streamCases are the refusal shapes TestDCRV1414 and TestDCRV1419 cover, applied to data.
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
		"mismatch then undecodable":   bytes.Replace(replace(`"start_line":1`, `"start_line":1.0`), []byte(`"end_line":`), []byte(`"end_line":"x",`), 1),
		"tail past the held bytes":    append(bytes.Clone(data[:len(data)-2]), append(bytes.Repeat([]byte(" "), 5000), "}\n"...)...),
	}
}

// TestDCRV1431StreamedDecodeMatchesInMemory: the streamed decode admits exactly what the in-memory
// decode admits, with the same records and digest tail, and refuses every shape the closed decoder
// refuses, whichever window and carry room splits the bytes, so every value crosses a window
// boundary at some size and carries both fit and overflow the room before a block.
func TestDCRV1431StreamedDecodeMatchesInMemory(t *testing.T) {
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
			for name, bad := range cases {
				_, closed := ParseArtifact(bad)
				decoded, err := decodeArtifactParallel(bad, nil)
				if err == nil {
					err = hashArtifactTail(bad).verify(decoded)
				}
				decoded, tail, streamed := decodeArtifactBlocks(bytes.NewReader(bad), window, headroom, nil, nil, nil)
				if streamed == nil {
					streamed = tail.verify(decoded)
				}
				if streamed == nil || closed == nil || err == nil || streamed.Error() != err.Error() {
					t.Errorf("window %d room %d %s: streamed=%v in memory=%v closed=%v; all must refuse, the two decoders alike", window, headroom, name, streamed, err, closed)
				}
			}
		}
	}
}

// TestDCRV1431StreamedLongValue: a value much longer than a window (one claim of 3 MiB) decodes
// as in memory; the window grows geometrically while the value is scanned.
func TestDCRV1431StreamedLongValue(t *testing.T) {
	t.Parallel()
	_, a, _ := builtFixture(t)
	data := rehashed(t, a, func(c *Artifact) {
		c.Claims = append([]Claim{}, c.Claims...)
		c.Claims[0].Text = strings.Repeat("long paragraph text ", 3<<20/20)
	})
	want, err := decodeArtifactParallel(data, nil)
	if err != nil {
		t.Fatal(err)
	}
	var stats blockStats
	got, _, err := decodeArtifactBlocks(bytes.NewReader(data), 4096, 1024, nil, nil, &stats)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("streamed long value differs: %v", err)
	}
}

// TestDCRV1431StreamedReadReusesBlocks: a streamed read of many blocks reuses block buffers
// instead of allocating one per block, so the artifact is never held whole.
func TestDCRV1431StreamedReadReusesBlocks(t *testing.T) {
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

// TestDCRV1431OpenReadFile: the streamed file read admits and refuses as OpenRead of the file's
// bytes does, a read error is not reported as a refusal of the bytes read before it, and a missing
// or unsafe path is the read's unavailable refusal.
func TestDCRV1431OpenReadFile(t *testing.T) {
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
	for name, prefix := range map[string][]byte{"valid prefix": data[:len(data)/3], "refused prefix": append([]byte(`{"extra":1,`), data[1:len(data)/3]...)} {
		reader := io.MultiReader(bytes.NewReader(prefix), iotest.ErrReader(broken))
		if _, _, err := decodeArtifactBlocks(reader, 512, 64, nil, nil, nil); !errors.Is(err, broken) {
			t.Fatalf("%s: read error reported as %v", name, err)
		}
	}
	// Bytes past the 128 MiB bound during the read refuse with the bound, whatever they are.
	endless := io.MultiReader(strings.NewReader("{"), spaces{})
	if _, _, err := decodeArtifactBlocks(endless, 1<<20, 64, nil, nil, nil); err == nil || err.Error() != fail("input bound exceeded").Error() {
		t.Fatalf("over-bound stream: %v", err)
	}
}

// spaces is an endless reader of spaces.
type spaces struct{}

func (spaces) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = ' '
	}
	return len(p), nil
}

// TestDCRV1434DecodeTimeClaimTermsMatchIndex: claim terms derived on the decode workers while a
// read decodes give the query index the reference derivation stores, whichever batches the
// window splits the claims into; blocks that do not cover every claim are not used.
func TestDCRV1434DecodeTimeClaimTermsMatchIndex(t *testing.T) {
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
				b := deriveClaimTerms(chunk, lo, nil)
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
				decoded.claimTerms = &claimTermHandoff{blocks: pre, of: decoded.Claims}
				index, err := BuildQueryIndex(ctx, decoded)
				if err != nil || !reflect.DeepEqual(index, reference) || decoded.claimTerms.blocks != nil {
					t.Fatalf("%s window %d (%d blocks): index from decode-time terms differs: %v", name, window, len(pre), err)
				}
			}
		}
	}
}

// TestDCRV1431StreamedTruncationAtWindowEnd: input that ends exactly at a window boundary, in the
// middle of a value or right after a scalar, refuses as a truncated artifact instead of panicking,
// whatever the window is; the refill at the end of the input moves the window without reading.
func TestDCRV1431StreamedTruncationAtWindowEnd(t *testing.T) {
	t.Parallel()
	_, _, data := builtFixture(t)
	cases := map[string][]byte{"schema scalar": []byte(`{"schema":1`), "schema scalar with space": []byte(`{"schema": 12`)}
	// A scalar of the artifact truncated where the window ends.
	at := bytes.Index(data, []byte(`"schema":`)) + len(`"schema":`)
	cases["artifact scalar"] = data[:at+1]
	for name, input := range cases {
		for _, window := range []int{len(input), len(input) - 1, 1, 64} {
			for _, headroom := range []int{0, 1, 64} {
				_, _, err := decodeArtifactBlocks(bytes.NewReader(input), window, headroom, nil, nil, nil)
				if err == nil || err.Error() != fail("invalid closed JSON input").Error() {
					t.Fatalf("%s window %d room %d: %v", name, window, headroom, err)
				}
			}
		}
	}
}

// TestDCRV1434ConcurrentQueriesShareClaimTerms: concurrent queries of one verified artifact
// derive their query indexes without racing on the decode-time claim terms; exactly one takes
// them, the others tokenize, and every receipt is the serial one. Run under -race.
func TestDCRV1434ConcurrentQueriesShareClaimTerms(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	root, _, data := builtFixture(t)
	serial, err := OpenRead(ctx, root, data, ReadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	requests := []Request{{Operation: "claims", Query: "cache"}, {Operation: "concept", Query: "cache"}, {Operation: "gaps"}, {Operation: "flow", Query: "cache"}}
	want := make([]Receipt, len(requests))
	for i, request := range requests {
		if want[i], err = QueryContext(ctx, serial, request, "fresh", nil); err != nil {
			t.Fatalf("%s: %v", request.Operation, err)
		}
	}
	a, err := OpenRead(ctx, root, data, ReadOptions{Index: true})
	if err != nil {
		t.Fatal(err)
	}
	if a.claimTerms == nil || a.claimTerms.blocks == nil {
		t.Fatal("the indexed read derived no claim terms")
	}
	var wg sync.WaitGroup
	got := make([]Receipt, 4*len(requests))
	errs := make([]error, len(got))
	for i := range got {
		wg.Go(func() {
			if i%2 == 0 {
				_, errs[i] = BuildQueryIndex(ctx, a)
			}
			if errs[i] == nil {
				got[i], errs[i] = QueryContext(ctx, a, requests[i%len(requests)], "fresh", nil)
			}
		})
	}
	wg.Wait()
	for i := range got {
		if errs[i] != nil || !reflect.DeepEqual(got[i], want[i%len(requests)]) {
			t.Fatalf("query %d (%s): %v; receipt differs from the serial one", i, requests[i%len(requests)].Operation, errs[i])
		}
	}
	if a.claimTerms.blocks != nil || a.RuntimeIndex != nil {
		t.Fatal("the claim terms were not taken, or a query changed the shared artifact")
	}
}

// TestDCRV1554UndecodableHeaderMemberRefuses: a header member that scans but does not decode
// refuses as the in-memory decode refuses, whichever window splits it, and the header callback
// never runs on it (DCR-V1-554).
func TestDCRV1554UndecodableHeaderMemberRefuses(t *testing.T) {
	t.Parallel()
	_, _, data := builtFixture(t)
	at := bytes.Index(data, []byte(`"tree":"`))
	if at < 0 {
		t.Fatal("no tree member in fixture")
	}
	open := at + len(`"tree":`)
	end := open + 1 + bytes.IndexByte(data[open+1:], '"')
	bad := append(append(append(bytes.Clone(data[:open]), '['), data[open:end+1]...), append([]byte{']'}, data[end+1:]...)...)
	_, want := decodeArtifactParallel(bad, nil)
	if want == nil {
		t.Fatal("in-memory decode admits an undecodable header member")
	}
	for _, window := range []int{1, 7, 512, 1 << 20} {
		called := false
		_, _, err := decodeArtifactBlocks(bytes.NewReader(bad), window, 16, func(*Artifact) { called = true }, nil, nil)
		if err == nil || err.Error() != want.Error() {
			t.Fatalf("window %d: streamed refusal %v, want %v", window, err, want)
		}
		if called {
			t.Fatalf("window %d: header callback ran on an undecodable header member", window)
		}
	}
}
