package doccorpus

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// scanPlans are the segmentations the DCP-V1-551 and DCP-V1-552 tests compare with the
// sequential scan: one byte segments so a split falls at every byte, and coarser ones.
var scanPlans = []boundaryScan{{1, 2}, {1, 3}, {1, 5}, {1, 7}, {1, 16}, {3, 64}, {17, 9}, {}}

// sequentialScan is the boundary scan with one segment, which never runs in parallel.
var sequentialScan = boundaryScan{1, 1}

// scanElements is the sequential scan's run of object and array elements from data[at], each
// directly followed by one comma, and where it stops: the elements parallelElements may return.
func scanElements(data []byte, at int) []windowSpan {
	var spans []windowSpan
	for at < len(data) && (data[at] == '{' || data[at] == '[') {
		end, state := scanValue(data, at)
		if state != scanOK || end >= len(data) || data[end] != ',' {
			break
		}
		spans = append(spans, windowSpan{at, end})
		at = end + 1
	}
	return spans
}

// scanTexts are claim texts that put a string, an escape or a bracket at every kind of split
// point: escaped quotes, long backslash runs, the separator and brackets inside strings, a value
// longer than many segments, and non-ASCII bytes.
func scanTexts() []string {
	texts := []string{
		`plain`, `quote " inside`, `\`, `\\`, `\"`, `"\`, `},{"id":"x"}`, `,{"id":`, `],[`, `[[[{{{`,
		`}}}]]]`, `","`, `\",\"`, `{"a":"b"}`, "\u2028\x7f\u00e9\U0001f600", `]},{"id":"c9","text":"`, ``,
		`,{`, strings.Repeat(`\"`, 9), strings.Repeat(`x\"{`, 300),
	}
	for _, n := range []int{31, 64, 65, 131} {
		texts = append(texts, strings.Repeat(`\`, n), strings.Repeat(`\`, n)+`"`+strings.Repeat(`\`, n))
	}
	return texts
}

// scanArray is a split array of n elements holding the claim texts. Its elements are canonical,
// objects that start with the same member name, unless hostile, when some are arrays, start with
// another name or nest an array of objects that start with that name.
func scanArray(t *testing.T, n int, hostile bool, seed uint64) []byte {
	t.Helper()
	r := rand.New(rand.NewPCG(seed, 547))
	texts := scanTexts()
	var b bytes.Buffer
	b.WriteByte('[')
	for i := range n {
		if i > 0 {
			b.WriteByte(',')
		}
		text, err := json.Marshal(texts[r.IntN(len(texts))])
		if err != nil {
			t.Fatal(err)
		}
		shape := r.IntN(2)
		if hostile && i > 0 {
			shape = r.IntN(5)
		}
		switch shape {
		case 0:
			fmt.Fprintf(&b, `{"id":"c%d","text":%s}`, i, text)
		case 1:
			fmt.Fprintf(&b, `{"id":"c%d","n":[1,[2,{"t":%s}],{}],"text":%s}`, i, text, text)
		case 2:
			fmt.Fprintf(&b, `[%s,{"k":[[],{}]},%d]`, text, i)
		case 3:
			fmt.Fprintf(&b, `{"k":"c%d","text":%s}`, i, text)
		default:
			fmt.Fprintf(&b, `{"id":"c%d","n":[{"id":1},{"id":%s}]}`, i, text)
		}
	}
	b.WriteByte(']')
	return b.Bytes()
}

// parallelSpans runs parallelElements over data from data[at] as one in-memory window.
func parallelSpans(data []byte, at int, plan boundaryScan) []windowSpan {
	d := &artifactDecoder{artifactStream: &artifactStream{buf: data, pos: at, eof: true, boundary: plan}}
	return d.parallelElements()
}

// TestDCPV1551ParallelElementsMatchScan: on a canonical split array the parallel scan finds every
// element the sequential scan finds, whichever segmentation splits its strings, escapes and
// nesting; on a hostile one, whose separators mislead the guesses, it finds a prefix of them.
func TestDCPV1551ParallelElementsMatchScan(t *testing.T) {
	t.Parallel()
	for _, hostile := range []bool{false, true} {
		for seed := range uint64(8) {
			data := scanArray(t, 40, hostile, seed)
			want := scanElements(data, 1)
			if len(want) != 39 {
				t.Fatalf("seed %d: sequential scan found %d elements", seed, len(want))
			}
			for _, plan := range scanPlans {
				if plan.segments == 0 {
					continue // the default segments are larger than the array
				}
				got := parallelSpans(data, 1, plan)
				if !slices.Equal(got, want[:len(got)]) || !hostile && len(got) != len(want) {
					t.Fatalf("hostile %v seed %d plan %+v: parallel scan found %d of %d elements or others", hostile, seed, plan, len(got), len(want))
				}
			}
		}
	}
}

// TestDCPV1551ParallelElementsCorrupt: on arrays with bytes replaced, inserted or cut, the
// parallel scan returns only elements the sequential scan finds, in order from the scan position,
// so a wrong guess or an anomaly can only shorten the run it hands over.
func TestDCPV1551ParallelElementsCorrupt(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewPCG(547, 1))
	sharp := []byte(`"\{}[],x 0` + "\x00")
	for trial := range 4000 {
		bad := scanArray(t, 12, trial%2 == 1, uint64(trial))
		for range 1 + r.IntN(3) {
			if len(bad) < 3 {
				break
			}
			at := 1 + r.IntN(len(bad)-1)
			c := sharp[r.IntN(len(sharp))]
			switch r.IntN(4) {
			case 0, 1:
				bad[at] = c
			case 2:
				bad = append(bad[:at], append([]byte{c}, bad[at:]...)...)
			default:
				bad = bad[:at]
			}
		}
		if len(bad) < 2 {
			continue
		}
		want := scanElements(bad, 1)
		for _, plan := range scanPlans[:7] {
			got := parallelSpans(bad, 1, plan)
			if len(got) > len(want) || !slices.Equal(got, want[:len(got)]) {
				t.Fatalf("trial %d plan %+v: parallel scan found elements the sequential scan does not: %v, want a prefix of %v", trial, plan, got, want)
			}
		}
	}
}

// decodeOutcome is everything a decode returns that its callers can observe.
type decodeOutcome struct {
	artifact *Artifact
	tail     artifactTail
	err      string
}

func decodeWith(data []byte, plan boundaryScan, window, headroom int) decodeOutcome {
	var (
		a    *Artifact
		tail artifactTail
		err  error
	)
	if window == 0 {
		a, tail, err = decodeArtifactStream(&artifactStream{buf: data, eof: true, boundary: plan}, nil, false, nil)
	} else {
		a, tail, err = decodeArtifactBlocksScan(bytes.NewReader(data), window, headroom, plan, nil, nil, nil)
	}
	out := decodeOutcome{artifact: a, tail: tail}
	if err != nil {
		out.err = err.Error()
	}
	return out
}

// TestDCPV1552ParallelScanMatchesSequential: the decode with the parallel boundary scan returns
// the same records, digest tail and refusal as with the sequential scan, in memory and streamed,
// for the committed fixture, adversarial claim texts, every refusal shape of DCP-V1-414 and
// DCP-V1-419, and the claims member truncated or corrupted at up to 1500 split points.
func TestDCPV1552ParallelScanMatchesSequential(t *testing.T) {
	t.Parallel()
	_, a, base := builtFixture(t)
	adversarial := rehashed(t, a, func(c *Artifact) {
		texts := scanTexts()
		c.Claims = append([]Claim{}, c.Claims...)
		for i := range c.Claims {
			c.Claims[i].Text = texts[i%len(texts)]
		}
	})
	at := bytes.Index(adversarial, []byte(`"claims":[`)) + len(`"claims":[`)
	if got, want := parallelSpans(adversarial, at, boundaryScan{1, 7}), scanElements(adversarial, at); len(want) < 2 || !reflect.DeepEqual(got, want) {
		t.Fatalf("parallel scan of the claims found %d of %d elements", len(got), len(want))
	}
	inputs := map[string][]byte{"fixture": base, "adversarial": adversarial}
	for name, bad := range streamCases(t, adversarial) {
		inputs[name] = bad
	}
	for name, data := range inputs {
		for _, window := range []int{0, 7, 61, 512, 1 << 20} {
			want := decodeWith(data, sequentialScan, window, 16)
			if (name == "fixture" || name == "adversarial") && want.err != "" {
				t.Fatalf("%s window %d: sequential decode refused: %s", name, window, want.err)
			}
			for _, plan := range scanPlans {
				if got := decodeWith(data, plan, window, 16); !reflect.DeepEqual(got, want) {
					t.Fatalf("%s window %d plan %+v: parallel decode %q differs from sequential %q", name, window, plan, got.err, want.err)
				}
			}
		}
	}
	// Every truncation, and a quote, backslash, bracket or comma at every offset, refuses alike.
	claims := bytes.Index(adversarial, []byte(`"claims":[`))
	end := bytes.Index(adversarial, []byte(`],"relations"`)) + 2
	stride := max(1, (end-claims)/1500)
	for at := claims; at < end; at += stride {
		variants := [][]byte{adversarial[:at]}
		for _, c := range []byte(`"\]},`) {
			bad := bytes.Clone(adversarial)
			bad[at] = c
			variants = append(variants, bad)
		}
		for _, bad := range variants {
			want := decodeWith(bad, sequentialScan, 0, 0)
			for _, plan := range scanPlans[:5] {
				if got := decodeWith(bad, plan, 0, 0); !reflect.DeepEqual(got, want) {
					t.Fatalf("offset %d plan %+v: parallel decode %q differs from sequential %q", at, plan, got.err, want.err)
				}
			}
		}
	}
}
