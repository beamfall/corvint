// SPDX-License-Identifier: Apache-2.0

package gateselect

import (
	"math/rand/v2"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// AFP-V0-012: preserve the regexp attribution lexicon, including its
// deliberately broader handling of short path components. Each run keeps its
// written spelling beside the closing-tag, root-anchored and parent-backslash
// spellings the planner also reads (AFP-V0-2242).
func TestPathTokensMatchOriginalRegexp(t *testing.T) {
	pattern := regexp.MustCompile(`[A-Za-z0-9._~@+/-]+`)
	values := []string{"", "</script> &LT;/script&gt;", `%03d.go %s/a %% ..\x`, "example.test/m/a/b", " s/a /a/b/ éa/b中", `foo..\notes`}
	random := rand.New(rand.NewPCG(4726, 12))
	for range 1000 {
		body := make([]byte, 256)
		for i := range body {
			body[i] = byte(random.UintN(256))
		}
		values = append(values, string(body))
	}
	spellings := func(want []string, item, module string) []string {
		want = append(want, item)
		anchored := item
		if item == module {
			anchored = "/"
		} else if rest, ok := strings.CutPrefix(item, module+"/"); ok {
			anchored = "/" + rest
		}
		if anchored != item {
			want = append(want, anchored)
		}
		return want
	}
	spelled := func(literal, module string) []string {
		var want []string
		for _, span := range pattern.FindAllStringIndex(literal, -1) {
			start, end := span[0], span[1]
			want = spellings(want, literal[start:end], module)
			if literal[start] == '/' && end > start+1 && (start > 0 && literal[start-1] == '<' || start >= 4 && strings.EqualFold(literal[start-4:start], "&lt;")) {
				want = spellings(want, literal[start+1:end], module)
			}
		}
		return want
	}
	for _, value := range values {
		for _, module := range []string{"", "example.test/m"} {
			literal := printfVerb.ReplaceAllString(value, " ")
			want := spelled(literal, module)
			seen := map[string]bool{}
			for _, item := range want {
				seen[item] = true
			}
			for _, item := range spelled(strings.ReplaceAll(literal, `..\`, `../`), module) {
				if !seen[item] {
					seen[item] = true
					want = append(want, item)
				}
			}
			if got := pathTokens(value, module); !reflect.DeepEqual(got, want) {
				t.Fatalf("tokens(%q, %q) = %q, want %q", value, module, got, want)
			}
		}
	}
}

func TestScanSourceTokenHistoryKeepsCommentBoundary(t *testing.T) {
	for _, tc := range []struct {
		body string
		want []string
	}{
		{"var _ = alias.Getwd()", []string{"os.Getwd"}},
		{"var _ = alias /* gap */ .Getwd()", nil},
		{"var _ = alias. /* gap */ Getwd()", []string{"os.Getwd (dot import)"}},
		{"var _ = other.Getwd()", nil},
		{"var _ = Getwd()", []string{"os.Getwd (dot import)"}},
	} {
		got := scanSource([]byte(tc.body), map[string]string{"alias": "os"}, map[string]bool{"os": true})
		if got.err != nil || !reflect.DeepEqual(got.calls, tc.want) {
			t.Fatalf("scan(%q) calls = %v, error = %v; want %v", tc.body, got.calls, got.err, tc.want)
		}
	}
}
