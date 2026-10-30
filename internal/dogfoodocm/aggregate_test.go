package dogfoodocm

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/lrfrepo"
)

func TestOCMV0013AggregateHappyPath(t *testing.T) {
	manifest := []byte("docs/specs/a.md\ndocs/specs/b.md\n")
	scopes := []verifiedScope{
		fixtureScope("docs/specs/a.md", []string{"A-V0-001", "A-V0-002"}),
		fixtureScope("docs/specs/b.md", []string{"B-V0-001"}),
	}
	result, err := aggregateVerified(manifest, manifest, scopes)
	if err != nil {
		t.Fatal(err)
	}
	if result.Aggregate.Coverage.Total != 3 || len(result.Worklist) != 3 {
		t.Fatalf("unexpected aggregate: %+v", result)
	}
	if got := []string{result.Worklist[0].ID, result.Worklist[1].ID, result.Worklist[2].ID}; strings.Join(got, ",") != "A-V0-001,A-V0-002,B-V0-001" {
		t.Fatalf("worklist order = %v", got)
	}
	entries := []scopeSetEntry{
		{MapSHA256: sha256Hex(scopes[0].after), Path: "docs/specs/a.md"},
		{MapSHA256: sha256Hex(scopes[1].after), Path: "docs/specs/b.md"},
	}
	canonical := canonicalScopeSet(entries)
	if string(canonical) != `[{"mapSha256":"`+entries[0].MapSHA256+`","path":"docs/specs/a.md"},{"mapSha256":"`+entries[1].MapSHA256+`","path":"docs/specs/b.md"}]` {
		t.Fatalf("canonical scope set = %s", canonical)
	}
	preimage := append(append([]byte(scopeSetDomain), 0), canonical...)
	digest := sha256.Sum256(preimage)
	if result.ScopeSetSHA256 != hex.EncodeToString(digest[:]) {
		t.Fatalf("scopeSetSha256 = %s", result.ScopeSetSHA256)
	}
}

// TestAggregateFindsNoLinkedRequirements covers OCM-V0-016: an aggregate that
// links none of its declared requirements carries the no-requirements-linked
// finding and keeps its verdict; one linked requirement removes the finding.
func TestAggregateFindsNoLinkedRequirements(t *testing.T) {
	manifest := []byte("docs/specs/a.md\ndocs/specs/b.md\n")
	cases := []struct {
		name     string
		linked   []int
		findings string
	}{
		{name: "OCM-V0-016 zero of N linked", linked: []int{0, 0}, findings: `"findings":[{"code":"no-requirements-linked","message":"0 of 3 declared requirements are linked to the change"}],`},
		{name: "OCM-V0-016 some of N linked", linked: []int{1, 0}},
		{name: "OCM-V0-016 N of N linked", linked: []int{2, 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			scopes := []verifiedScope{
				fixtureScope("docs/specs/a.md", []string{"A-V0-001", "A-V0-002"}),
				fixtureScope("docs/specs/b.md", []string{"B-V0-001"}),
			}
			for index, linked := range tc.linked {
				scopes[index].coverage.Linked = linked
				scopes[index].coverage.Unknown -= linked
			}
			result, err := aggregateVerified(manifest, manifest, scopes)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(result.Aggregate)
			if err != nil {
				t.Fatal(err)
			}
			coverage, _ := json.Marshal(result.Aggregate.Coverage)
			want := `{"coverage":` + string(coverage) + `,` + tc.findings + `"state":"ready-for-review"}`
			if string(encoded) != want {
				t.Fatalf("aggregate = %s, want %s", encoded, want)
			}
		})
	}
}

func TestOCMV0013FailsClosedOnMissingScope(t *testing.T) {
	_, err := aggregateVerified([]byte("docs/specs/a.md\n"), []byte("docs/specs/a.md\n"), nil)
	assertCode(t, err, "missing-intent-scope")
	for _, raw := range [][]byte{nil, []byte(""), []byte("docs/specs/a.md"), []byte("docs/specs/b.md\ndocs/specs/a.md\n")} {
		_, err := parseManifest(raw)
		assertCode(t, err, "missing-intent-scope")
	}
}

func TestOCMV0013FailsClosedOnScopeAndMapDrift(t *testing.T) {
	manifest := []byte("docs/specs/a.md\n")
	scope := fixtureScope("docs/specs/a.md", []string{"A-V0-001"})
	_, err := aggregateVerified(manifest, []byte("docs/specs/b.md\n"), []verifiedScope{scope})
	assertCode(t, err, "intent-scope-drift")
	assertMessageNames(t, err, ".corvint/change.ocm-intents")
	scope.document.IntentScope.Path = "docs/specs/b.md"
	_, err = aggregateVerified(manifest, manifest, []verifiedScope{scope})
	assertCode(t, err, "intent-scope-drift")
	scope = fixtureScope("docs/specs/a.md", []string{"A-V0-001"})
	scope.after = append(scope.after, ' ')
	_, err = aggregateVerified(manifest, manifest, []verifiedScope{scope})
	assertCode(t, err, "intent-scope-drift")
	assertMessageNames(t, err, ".corvint/change.ocm.001.json")
}

func TestOCMV0013FailsClosedOnCrossScopeDuplicate(t *testing.T) {
	manifest := []byte("docs/specs/a.md\ndocs/specs/b.md\n")
	scopes := []verifiedScope{
		fixtureScope("docs/specs/a.md", []string{"SHARED-V0-001"}),
		fixtureScope("docs/specs/b.md", []string{"SHARED-V0-001"}),
	}
	_, err := aggregateVerified(manifest, manifest, scopes)
	assertCode(t, err, "duplicate-requirement")
}

func TestOCMV0013DriftPrecedesDuplicate(t *testing.T) {
	manifest := []byte("docs/specs/a.md\ndocs/specs/b.md\n")
	scopes := []verifiedScope{
		fixtureScope("docs/specs/a.md", []string{"SHARED-V0-001"}),
		fixtureScope("docs/specs/b.md", []string{"SHARED-V0-001"}),
	}
	scopes[1].document.TargetRevision = strings.Repeat("b", 40)
	_, err := aggregateVerified(manifest, manifest, scopes)
	assertCode(t, err, "intent-scope-drift")
}

// TestOCMV0021BoundRefusalsNameTotalsBoundAndScopes covers OCM-V0-021: each
// aggregate bound refusal names the observed total, the fixed bound and every
// scope's share, keeps bytes-then-claims-then-obligations precedence, and an
// aggregate at exactly the bounds still succeeds.
func TestOCMV0021BoundRefusalsNameTotalsBoundAndScopes(t *testing.T) {
	manifest := []byte("docs/specs/a.md\ndocs/specs/b.md\n")
	identities := func(prefix string, count int) []string {
		result := make([]string, count)
		for index := range result {
			result[index] = fmt.Sprintf("%s-V0-%03d", prefix, index+1)
		}
		return result
	}
	claims := func(count int) []json.RawMessage {
		result := make([]json.RawMessage, count)
		for index := range result {
			result[index] = json.RawMessage(`{}`)
		}
		return result
	}
	pad := func(scope *verifiedScope, size int) {
		scope.after = append(scope.after, make([]byte, size-len(scope.after))...)
		scope.before = append([]byte(nil), scope.after...)
	}
	cases := []struct {
		name  string
		setup func(a, b *verifiedScope)
		code  string
		want  string
	}{
		{name: "OCM-V0-021 obligations", setup: func(a, b *verifiedScope) {
			*a = fixtureScope("docs/specs/a.md", identities("A", 2049))
			*b = fixtureScope("docs/specs/b.md", identities("B", 2048))
		}, code: "too-many-obligations", want: "aggregate OCM obligations exceed the limit: observed 4097, bound 4096; per scope 1=2049, 2=2048" + boundNarrowing},
		{name: "OCM-V0-021 claims", setup: func(a, b *verifiedScope) {
			a.document.Claims = claims(4100)
			b.document.Claims = claims(4093)
		}, code: "too-many-claims", want: "aggregate OCM claims exceed the limit: observed 8193, bound 8192; per scope 1=4100, 2=4093" + boundNarrowing},
		{name: "OCM-V0-021 map bytes precede claims and obligations", setup: func(a, b *verifiedScope) {
			*a = fixtureScope("docs/specs/a.md", identities("A", 2049))
			*b = fixtureScope("docs/specs/b.md", identities("B", 2048))
			a.document.Claims = claims(8193)
			pad(a, 1<<19)
			pad(b, 1<<19+1)
		}, code: "map-too-large", want: "aggregate OCM map bytes exceed the limit: observed 1048577, bound 1048576; per scope 1=524288, 2=524289" + boundNarrowing},
		{name: "OCM-V0-021 claims precede obligations", setup: func(a, b *verifiedScope) {
			*a = fixtureScope("docs/specs/a.md", identities("A", 4097))
			a.document.Claims = claims(8193)
		}, code: "too-many-claims", want: "aggregate OCM claims exceed the limit: observed 8193, bound 8192; per scope 1=8193, 2=0" + boundNarrowing},
		{name: "OCM-V0-021 exactly at every bound succeeds", setup: func(a, b *verifiedScope) {
			*a = fixtureScope("docs/specs/a.md", identities("A", 2048))
			*b = fixtureScope("docs/specs/b.md", identities("B", 2048))
			a.document.Claims = claims(4096)
			b.document.Claims = claims(4096)
			pad(a, 1<<19)
			pad(b, 1<<19)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := fixtureScope("docs/specs/a.md", []string{"A-V0-001"})
			b := fixtureScope("docs/specs/b.md", []string{"B-V0-001"})
			b.document.Obligations = nil
			tc.setup(&a, &b)
			result, err := aggregateVerified(manifest, manifest, []verifiedScope{a, b})
			if tc.code == "" {
				if err != nil || !result.OK {
					t.Fatalf("at-bound aggregate refused: %v", err)
				}
				return
			}
			assertCode(t, err, tc.code)
			if message := err.(*lrfrepo.Error).Message; message != tc.want {
				t.Fatalf("message = %q, want %q", message, tc.want)
			}
		})
	}
}

// TestOCMV0021AggregateBoundsDeriveFromScopeCount (V1-2971): each scope is a
// map bounded at 256 obligations and 512 claims, so the aggregate bounds are
// 16 times those; three scopes whose 271 obligations exceed one map's bound
// bind, and the refusal detail names the derived bound.
func TestOCMV0021AggregateBoundsDeriveFromScopeCount(t *testing.T) {
	if maxObligations != 4096 || maxClaims != 8192 || MaxObligations != maxObligations {
		t.Fatalf("aggregate bounds %d obligations, %d claims, not derived from %d scopes", maxObligations, maxClaims, maxScopes)
	}
	manifest := []byte("docs/specs/a.md\ndocs/specs/b.md\ndocs/specs/c.md\n")
	identities := func(prefix string, count int) []string {
		result := make([]string, count)
		for index := range result {
			result[index] = fmt.Sprintf("%s-V0-%03d", prefix, index+1)
		}
		return result
	}
	scopes := []verifiedScope{
		fixtureScope("docs/specs/a.md", identities("A", 151)),
		fixtureScope("docs/specs/b.md", identities("B", 105)),
		fixtureScope("docs/specs/c.md", identities("C", 15)),
	}
	for index := range scopes {
		scopes[index].document.Claims = make([]json.RawMessage, 300)
	}
	result, err := aggregateVerified(manifest, manifest, scopes)
	if err != nil || !result.OK || result.Aggregate.Coverage.Total != 271 || len(result.Worklist) != 271 {
		t.Fatalf("271 obligations and 900 claims over three scopes did not bind: %v %+v", err, result.Aggregate)
	}
}

func fixtureScope(path string, identities []string) verifiedScope {
	document := mapDocument{TargetRevision: strings.Repeat("a", 40)}
	document.CEM.MapSHA256 = strings.Repeat("1", 64)
	document.CEM.PatchSHA256 = strings.Repeat("2", 64)
	document.IntentScope.Path = path
	for _, identity := range identities {
		row := struct {
			ClaimIDs    []string `json:"claimIds"`
			Disposition string   `json:"disposition"`
			HunkIDs     []string `json:"hunkIds"`
			ID          string   `json:"id"`
			Reason      string   `json:"reason"`
		}{ClaimIDs: []string{}, Disposition: "unknown", HunkIDs: []string{}, ID: identity, Reason: "insufficient-evidence"}
		document.Obligations = append(document.Obligations, row)
	}
	raw, _ := json.Marshal(document)
	raw = append(raw, '\n')
	return verifiedScope{
		declaredPath: path, mapPath: ".corvint/change.ocm.001.json",
		before: raw, after: append([]byte(nil), raw...), document: document,
		coverage: Coverage{Total: len(identities), Unknown: len(identities)},
	}
}

func assertMessageNames(t *testing.T, err error, path string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), path) {
		t.Fatalf("err = %v, want the stale path %q named", err, path)
	}
}

func assertCode(t *testing.T, err error, expected string) {
	t.Helper()
	if lrfrepo.CodeOf(err) != expected {
		t.Fatalf("code = %q, err = %v", lrfrepo.CodeOf(err), err)
	}
}

// TestOCMV0024BoundRefusalNamesNarrowingRoute pins the fixed narrowing route
// (V1-1068) and keeps the worst-case message inside finish's 512-byte detail.
func TestOCMV0024BoundRefusalNamesNarrowingRoute(t *testing.T) {
	const want = "; narrow: list fewer intent scopes in the manifest and bind the rest as a separate staged change"
	if boundNarrowing != want {
		t.Fatalf("narrowing route = %q, want %q", boundNarrowing, want)
	}
	shares := make([]int, maxScopes)
	for index := range shares {
		shares[index] = maxMapBytes
	}
	message := boundMessage("aggregate OCM map bytes exceed the limit", maxMapBytes*maxScopes, maxMapBytes, shares)
	if !strings.HasSuffix(message, want) || len(message) > 512 || strings.Contains(message, "/") {
		t.Fatalf("worst-case message (%d bytes) = %q", len(message), message)
	}
}
