// SPDX-License-Identifier: Apache-2.0

package gateselect

import (
	"math/rand/v2"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// AFP-V0-012: preserve the original regexp attribution lexicon, including
// its deliberately broader handling of markup and short path components.
func TestPathTokensMatchOriginalRegexp(t *testing.T) {
	pattern := regexp.MustCompile(`[A-Za-z0-9._~@+/-]+`)
	values := []string{"", "</script> &LT;/script&gt;", `%03d.go %s/a %% ..\x`, "example.test/m/a/b", " s/a /a/b/ éa/b中"}
	random := rand.New(rand.NewPCG(4726, 12))
	for range 1000 {
		body := make([]byte, 256)
		for i := range body {
			body[i] = byte(random.UintN(256))
		}
		values = append(values, string(body))
	}
	for _, value := range values {
		for _, module := range []string{"", "example.test/m"} {
			var want []string
			for _, item := range pattern.FindAllString(printfVerb.ReplaceAllString(value, " "), -1) {
				if item == module {
					item = "/"
				} else if rest, ok := strings.CutPrefix(item, module+"/"); ok {
					item = "/" + rest
				}
				want = append(want, item)
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
