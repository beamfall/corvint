package sessiontest

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// twoShardManifest has one unit on shard 0, one on shard 1 and an empty
// shard 2.
func twoShardManifest(t *testing.T) (Manifest, string) {
	t.Helper()
	m := Manifest{
		Checks: []string{}, Incomplete: []string{}, PartitionDigest: "sha256:p", Profile: ManifestProfile,
		Scope: ScopeNarrowed, ShardCount: 3, Source: SourceIdentity{Head: "h", Paths: []PathDigest{}, Tree: "t"},
		Units: []Unit{
			{Adapter: AdapterGo, Shard: 0, Slice: SliceWhole, Unit: "go:m/a"},
			{Adapter: AdapterGo, Shard: 1, Slice: "0/2", Unit: "go:m/s"},
		},
	}
	var err error
	if m.SourceDigest, err = Digest(m.Source); err != nil {
		t.Fatal(err)
	}
	digest, err := Digest(m)
	if err != nil {
		t.Fatal(err)
	}
	return m, digest
}

func shardReceipt(m Manifest, digest string, shard int, outcomes ...Outcome) Receipt {
	receipt := NewReceipt(m, digest, shard, "corvint test")
	if len(outcomes) != 0 {
		receipt.Invocations = []Invocation{{Adapter: AdapterGo, AdapterManifest: AdapterManifestProfile, Argv: []string{"go"}, Env: []string{}, Outcomes: outcomes}}
	}
	return Finish(receipt, m)
}

// STR-V0-009: verify-shards accepts only the exact union.
func TestSTRV0009VerifyShardsMatrix(t *testing.T) {
	m, digest := twoShardManifest(t)
	a := Outcome{Outcome: OutcomePass, Slice: SliceWhole, Unit: "go:m/a"}
	s := Outcome{Outcome: OutcomePass, Slice: "0/2", Unit: "go:m/s"}
	good := func() []Receipt {
		return []Receipt{shardReceipt(m, digest, 0, a), shardReceipt(m, digest, 1, s), shardReceipt(m, digest, 2)}
	}
	if result, reasons := Verify(m, digest, good(), nil); result != ResultPass || len(reasons) != 0 {
		t.Fatalf("exact union: %s %v", result, reasons)
	}
	cases := []struct {
		name   string
		edit   func([]Receipt) []Receipt
		result string
		reason string
	}{
		{"dropped shard", func(r []Receipt) []Receipt { return r[:2] }, ResultIncomplete, "shard 2 has no accepted receipt"},
		{"missing empty shard", func(r []Receipt) []Receipt { return []Receipt{r[0], r[1]} }, ResultIncomplete, "shard 2 has no accepted receipt"},
		{"duplicated shard", func(r []Receipt) []Receipt { return append(r, r[2]) }, ResultIncomplete, "duplicate shard"},
		{"foreign manifest", func(r []Receipt) []Receipt { r[2].ManifestDigest = "sha256:other"; return r }, ResultIncomplete, "binds another manifest"},
		{"changed source", func(r []Receipt) []Receipt { r[2].SourceDigest = "sha256:x"; return r }, ResultIncomplete, "source, partition or shard count differs"},
		{"out of range", func(r []Receipt) []Receipt { r[2].ShardIndex = 3; return r }, ResultIncomplete, "shard index out of range"},
		{"changed source identity under the original digest", func(r []Receipt) []Receipt {
			r[1].Source.Tree = "other"
			return r
		}, ResultIncomplete, "receipt 1 (shard 1): the source identity does not match its digest or the manifest"},
		{"changed revision", func(r []Receipt) []Receipt { r[1].Revision = "other"; return r }, ResultIncomplete, "does not match its digest or the manifest"},
		{"extra unit", func(r []Receipt) []Receipt {
			r[2].Invocations = []Invocation{{Outcomes: []Outcome{{Outcome: OutcomePass, Slice: SliceWhole, Unit: "go:m/z"}}}}
			return r
		}, ResultIncomplete, "unplanned unit go:m/z whole"},
		{"unit on the wrong shard", func(r []Receipt) []Receipt {
			r[2].Invocations = []Invocation{{Outcomes: []Outcome{a}}}
			return r
		}, ResultIncomplete, "belongs to shard 0"},
		{"dropped unit", func(r []Receipt) []Receipt { r[1].Invocations = []Invocation{}; return r }, ResultIncomplete, "unit go:m/s 0/2 has no outcome"},
		{"duplicated unit", func(r []Receipt) []Receipt {
			r[1].Invocations = append(r[1].Invocations, r[1].Invocations[0])
			return r
		}, ResultIncomplete, "duplicate unit go:m/s 0/2"},
		{"incomplete after drift with passing outcomes", func(r []Receipt) []Receipt {
			r[0].Incomplete = []string{"source drift after the last adapter launch"}
			r[0] = Finish(r[0], m)
			return r
		}, ResultIncomplete, "result INCOMPLETE"},
		{"failed unit", func(r []Receipt) []Receipt {
			r[0].Invocations[0].Outcomes[0].Outcome = OutcomeFail
			r[0] = Finish(r[0], m)
			return r
		}, ResultFail, "unit go:m/a whole FAIL"},
		{"nonzero exit with passing outcomes", func(r []Receipt) []Receipt {
			r[0].Invocations[0].Exit = 2
			r[0] = Finish(r[0], m)
			return r
		}, ResultFail, "go exited 2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, reasons := Verify(m, digest, tc.edit(good()), nil)
			if result != tc.result || !strings.Contains(strings.Join(reasons, "\n"), tc.reason) {
				t.Fatalf("got %s %v, want %s naming %q", result, reasons, tc.result, tc.reason)
			}
		})
	}
	if result, reasons := Verify(m, digest, good(), []string{"r.json: unreadable"}); result != ResultIncomplete || reasons[0] != "r.json: unreadable" {
		t.Fatalf("unreadable receipt: %s %v", result, reasons)
	}
}

// STR-V0-010, STR-V0-011: result precedence and summary wording.
func TestSTRV0011FinishResultAndSummary(t *testing.T) {
	m, digest := twoShardManifest(t)
	pass := shardReceipt(m, digest, 0, Outcome{Outcome: OutcomePass, Slice: SliceWhole, Unit: "go:m/a"})
	if pass.Result != ResultPass || pass.Summary != "selected units passed: 1 of 1; the full suite runs under make gate and the required CI checks" {
		t.Fatalf("narrowed pass: %s %q", pass.Result, pass.Summary)
	}
	if empty := shardReceipt(m, digest, 2); empty.Result != ResultPass || empty.Summary != "selected units passed: 0 of 0; the full suite runs under make gate and the required CI checks" {
		t.Fatalf("empty shard: %s %q", empty.Result, empty.Summary)
	}
	missing := shardReceipt(m, digest, 0)
	if missing.Result != ResultIncomplete || !reflect.DeepEqual(missing.Incomplete, []string{"1 execution units have no terminal outcome"}) {
		t.Fatalf("missing outcome: %s %v", missing.Result, missing.Incomplete)
	}
	notRun := shardReceipt(m, digest, 0, Outcome{Outcome: OutcomeNotRun, Slice: SliceWhole, Unit: "go:m/a"})
	if notRun.Result != ResultIncomplete {
		t.Fatalf("NOT_RUN: %s", notRun.Result)
	}
	m.Scope = ScopeFull
	full := shardReceipt(m, digest, 0, Outcome{Outcome: OutcomeBuildFailed, Slice: SliceWhole, Unit: "go:m/a"})
	if full.Result != ResultFail || full.Summary != "FULL units passed: 0 of 1" {
		t.Fatalf("full fail: %s %q", full.Result, full.Summary)
	}
	m.Scope, m.Units = ScopeNothing, []Unit{}
	if nothing := shardReceipt(m, digest, 0); nothing.Result != ResultPass || !strings.HasPrefix(nothing.Summary, "no execution unit selected; the full suite runs under make gate") {
		t.Fatalf("nothing: %s %q", nothing.Result, nothing.Summary)
	}
}

// STR-V0-004: package-level terminal events decide unit outcomes; output
// text streams through.
func TestSTRV0004ParseTestEvents(t *testing.T) {
	stream := strings.Join([]string{
		`{"Action":"output","Package":"m/a","Output":"ok m/a\n"}`,
		`{"Action":"pass","Package":"m/a"}`,
		`{"Action":"skip","Package":"m/n"}`,
		`{"Action":"fail","Package":"m/b","Test":"TestX"}`,
		`{"Action":"fail","Package":"m/b"}`,
		`{"Action":"fail","Package":"m/c","FailedBuild":"m/c"}`,
		`{"Action":"output","Package":"m/d","Output":"panic: test timed out after 1s\n"}`,
		`{"Action":"fail","Package":"m/d"}`,
		`# m/e build output`,
		`{"Action":"pass","Package":"m/f","Output":"` + strings.Repeat("x", maxEventLine) + `"}`,
		`{"Action":"pass","Package":"m/g"}`,
	}, "\n") + "\n"
	var out bytes.Buffer
	got := parseTestEvents(strings.NewReader(stream), &out)
	want := map[string]string{"m/a": OutcomePass, "m/n": OutcomeNoTests, "m/b": OutcomeFail, "m/c": OutcomeBuildFailed, "m/d": OutcomeTimedOut, "m/g": OutcomePass}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("outcomes %v", got)
	}
	if !strings.HasPrefix(out.String(), "ok m/a\npanic: test timed out after 1s\n# m/e build output\n") {
		t.Fatalf("streamed %q", out.String()[:80])
	}
}

// STR-V0-010, STR-V0-018: manifests and receipts round-trip canonically and
// a non-canonical, unknown-member or wrong-profile document is refused.
func TestSTRV0018ManifestStrictRoundTrip(t *testing.T) {
	m, digest := twoShardManifest(t)
	dir := t.TempDir()
	name := filepath.Join(dir, "m.json")
	data, err := Encode(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(name, data); err != nil {
		t.Fatal(err)
	}
	read, readDigest, err := ReadManifest(name)
	if err != nil || readDigest != digest || !reflect.DeepEqual(read, m) {
		t.Fatalf("round trip: %v %s", err, readDigest)
	}
	receipt := shardReceipt(m, digest, 2)
	encoded, _ := Encode(receipt)
	receiptName := filepath.Join(dir, "r.json")
	if err := WriteFile(receiptName, encoded); err != nil {
		t.Fatal(err)
	}
	if back, err := ReadReceipt(receiptName); err != nil || !reflect.DeepEqual(back, receipt) {
		t.Fatalf("receipt round trip: %v", err)
	}
	for label, body := range map[string]string{
		"spaced":        strings.Replace(string(data), `":`, `": `, 1),
		"unknown":       strings.Replace(string(data), `{"base"`, `{"extra":1,"base"`, 1),
		"trailing":      string(data) + "{}\n",
		"wrong profile": strings.Replace(string(data), ManifestProfile, "other/0", 1),
	} {
		bad := filepath.Join(dir, "bad.json")
		if err := os.WriteFile(bad, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, _, err := ReadManifest(bad); err == nil {
			t.Errorf("%s manifest accepted", label)
		}
	}
	if _, err := ReadReceipt(name); err == nil {
		t.Fatal("a manifest was read as a receipt")
	}
}
