package runidentity

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

var closureA = "sha256:" + strings.Repeat("a", 64)

// PWR-V0-071: caller-declared closure digests are parsed strictly and printed under rule DECLARED.
func TestPWRV0071_ClosureDigests(t *testing.T) {
	got, err := ParseClosureDigests([]byte(`{"cart.spec.ts":"` + closureA + `"}`))
	if err != nil || len(got) != 1 || got["cart.spec.ts"] != closureA {
		t.Fatalf("ParseClosureDigests = %v, %v", got, err)
	}
	bad := map[string]string{
		"not object":  `["x"]`,
		"duplicate":   `{"a":"` + closureA + `","a":"` + closureA + `"}`,
		"short":       `{"a":"sha256:abc"}`,
		"upper hex":   `{"a":"sha256:` + strings.Repeat("A", 64) + `"}`,
		"no prefix":   `{"a":"` + strings.Repeat("a", 64) + `"}`,
		"non string":  `{"a":1}`,
		"trailing":    `{"a":"` + closureA + `"} {}`,
		"truncated":   `{"a":"` + closureA + `"`,
		"not json":    `nope`,
		"nested":      `{"a":{"b":"` + closureA + `"}}`,
		"empty input": ``,
	}
	for name, raw := range bad {
		var coded *Error
		if _, err := ParseClosureDigests([]byte(raw)); !errors.As(err, &coded) || coded.Reason != ReasonClosureInvalid {
			t.Errorf("%s: err = %v, want %s", name, err, ReasonClosureInvalid)
		}
	}

	root := repo(t)
	config := filepath.Join(root, "e2e", "playwright.config.ts")
	plain := produce(t, Options{Config: config})
	if plain.CorvintRunIdentity.TestClosureRule != "" || plain.CorvintRunIdentity.TestClosureDigests != nil || plain.Counts.ClosureDigests != 0 {
		t.Fatalf("closure declared without --closure-digests: %+v", plain)
	}
	key := ""
	for k := range plain.CorvintRunIdentity.TestFileBlobs {
		if strings.HasSuffix(k, "cart.spec.ts") {
			key = k
		}
	}
	doc := produce(t, Options{Config: config, ClosureDigests: map[string]string{key: closureA}})
	d := doc.CorvintRunIdentity
	if d.TestClosureRule != ClosureRuleDeclared || key == "" || d.TestClosureDigests[key] != closureA || len(d.TestClosureDigests) != 1 || doc.Counts.ClosureDigests != 1 {
		t.Fatalf("declared closure = %+v counts %+v", d, doc.Counts)
	}
	if d.ConfigDigest != plain.CorvintRunIdentity.ConfigDigest || d.TestFileBlobs[key] != plain.CorvintRunIdentity.TestFileBlobs[key] {
		t.Fatal("a closure declaration changed the other inputs")
	}
	var coded *Error
	if _, err := Produce(context.Background(), Options{Config: config, ClosureDigests: map[string]string{"pages/login.ts": closureA}}); !errors.As(err, &coded) || coded.Reason != ReasonClosureInvalid {
		t.Fatalf("undeclared closure key: err = %v", err)
	}
}
