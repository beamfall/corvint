package doccorpus

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// mentionClaimText names tokens that resolve in the cited span (`Count()`), to a provider subject
// (`adapter:count`), to native symbol and file subjects (`showCount`, `src/report.py`), and to
// nothing (`missingThing`).
const mentionClaimText = "`Count()` returns one for `adapter:count`; see `showCount` and `src/report.py`, not `missingThing`."

func mentionEdit(p *ProviderRecord) { p.Claims[0].Text = mentionClaimText }

// DCX-V1-340: the shared extractor's token rules are deterministic and documented.
func TestDCXV1340ExtractCodeTokens(t *testing.T) {
	t.Parallel()
	cases := []struct {
		text string
		want string
	}{
		{"`Count()` and `x := 1`", "Count():identifier `x := 1`:code"},
		{"Calls Foo::Bar and User#save then os.Exit.", "Foo::Bar:scope User#save:member os.Exit:member"},
		{"Reads internal/doccorpus/compile.go and `go.mod`", "internal/doccorpus/compile.go:path go.mod:path"},
		{"Serves GET /v1/items, then POST /v1/items/{id}.", "GET /v1/items:route POST /v1/items/{id}:route"},
		{"Uses report_count, showCount, ClaimMention and render() here", "report_count:identifier showCount:identifier ClaimMention:identifier render():identifier"},
		{"Plain prose: Count is one. See https://example.invalid/a.go and e.g. 3.5 or Count.", ""},
		{"`showCount` then showCount again", "showCount:identifier"},
		{"unterminated `showCount", "showCount:identifier"},
		{"`os.stat`, `main.go`, `app.config.json`, `os.stat()`, `a.go` and `ab.cd`", "os.stat:member main.go:path app.config.json:path os.stat():member a.go:path ab.cd:member"},
		{"Calls (renderAll) and f(x)) then [list]] and Map<K>>.", "renderAll:identifier"},
	}
	for _, c := range cases {
		tokens, omitted := ExtractCodeTokens(c.text)
		got := []string{}
		for _, tok := range tokens {
			token := tok.Token
			if strings.Contains(token, " ") && tok.Kind == "code" {
				token = "`" + token + "`"
			}
			got = append(got, token+":"+tok.Kind)
		}
		if strings.Join(got, " ") != c.want || omitted != 0 {
			t.Fatalf("%q = %q (omitted %d), want %q", c.text, strings.Join(got, " "), omitted, c.want)
		}
	}
	many := []string{}
	for i := 0; i < MaxClaimMentions+3; i++ {
		many = append(many, fmt.Sprintf("name_%d", i))
	}
	tokens, omitted := ExtractCodeTokens(strings.Join(many, " "))
	if len(tokens) != MaxClaimMentions || omitted != 3 || tokens[0].Token != "name_0" {
		t.Fatalf("bound: %d tokens, %d omitted", len(tokens), omitted)
	}
	if tokens, _ := ExtractCodeTokens("`" + strings.Repeat("a", MaxMentionBytes+1) + "`"); len(tokens) != 0 {
		t.Fatal("over-long token admitted")
	}
	// Adversarial closing-bracket runs trim in linear time: a quadratic rescan of a 1 MiB chunk
	// would not finish within the bound.
	for _, c := range []struct{ chunk, want string }{
		{"x" + strings.Repeat(")", 65536), "x"},
		{"f(" + strings.Repeat(")", 65536), "f(" + strings.Repeat(")", 65536)},
		{"'(" + strings.Repeat("])}>.", 13107) + "'", ""},
	} {
		if got := trimChunk(c.chunk); got != c.want {
			t.Fatalf("trimChunk(%.12q...) = %.12q... (%d bytes), want %d bytes", c.chunk, got, len(got), len(c.want))
		}
	}
	start := time.Now()
	if tokens, _ := ExtractCodeTokens("run_all" + strings.Repeat(")", 1<<20)); len(tokens) != 1 || tokens[0].Token != "run_all" {
		t.Fatalf("closing-bracket run tokens = %+v", tokens)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("1 MiB closing-bracket run took %s; trimming is not linear", elapsed)
	}
}

func mentionCheckFor(t *testing.T, a *Artifact, id string) ClaimMentionCheck {
	t.Helper()
	check, ok := claimMentionCheck(a, id)
	if !ok {
		t.Fatalf("no mention check for %s in %+v", id, a.ClaimMentions)
	}
	return check
}

// DCX-V1-341, DCX-V1-342: the build stores each non-native claim's mention check, resolving
// in the cited span first, then subject, indexed symbol and tracked path, deterministically.
func TestDCXV1341ClaimMentionCheckIsBuiltAndPinned(t *testing.T) {
	t.Parallel()
	root, m := providerFixture(t, mentionEdit)
	a, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	check := mentionCheckFor(t, a, "adapter:claim")
	got := []string{}
	for _, x := range check.Mentions {
		got = append(got, x.Token+"="+x.Resolution+":"+x.TargetKind+":"+x.Target)
	}
	want := []string{
		"Count()=resolved-in-span::",
		"adapter:count=resolved-elsewhere:subject:adapter:count",
		"showCount=resolved-elsewhere:subject:native:symbol:src/view.ts:1:showCount",
		"src/report.py=resolved-elsewhere:subject:native:file:src/report.py",
		"missingThing=unresolved::",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("mentions:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if check.State != MentionsPartiallyUnresolved || check.InSpan != 1 || check.Elsewhere != 3 || check.Unresolved != 1 || check.Subject != "adapter:count" {
		t.Fatalf("check = %+v", check)
	}
	for _, c := range a.ClaimMentions {
		if c.ClaimID != "adapter:claim" {
			t.Fatalf("native claim %s was checked", c.ClaimID)
		}
	}
	x := &mentionIndex{subjects: map[string]string{"Widget": "app:widget"}, symbols: map[string]string{"Widget": "src/w.go:1", "render": "src/a.go:3"}, paths: map[string]string{"a.go": "src/a.go", "src/a.go": "src/a.go"}}
	for _, c := range []struct {
		token CodeToken
		want  string
	}{
		{CodeToken{"Widget", "identifier"}, "subject app:widget"},
		{CodeToken{"View#render", "member"}, "symbol src/a.go:3"},
		{CodeToken{"render()", "identifier"}, "symbol src/a.go:3"},
		{CodeToken{"a.go", "path"}, "path src/a.go"},
		{CodeToken{"./src/a.go", "path"}, "path src/a.go"},
		{CodeToken{"a.go", "code"}, " "},
		{CodeToken{"render_all", "identifier"}, " "},
	} {
		if target, kind := x.resolve(c.token); kind+" "+target != c.want {
			t.Fatalf("%+v resolved to %s %s, want %s", c.token, kind, target, c.want)
		}
	}
	data, err := Encode(a)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	if other, _ := Encode(again); !bytes.Equal(data, other) {
		t.Fatal("mention check is nondeterministic")
	}
	if _, err := Open(context.Background(), root, data); err != nil {
		t.Fatalf("rederived artifact refused: %v", err)
	}
	// A tampered check with a recomputed digest parses, so only Open's rederivation refuses it.
	forged, err := ParseArtifact(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(forged.ClaimMentions) == 0 || forged.ClaimMentions[0].State == MentionsConsistent {
		t.Fatalf("fixture has no non-consistent stored state: %+v", forged.ClaimMentions)
	}
	forged.ClaimMentions[0].State = MentionsConsistent
	forged.SHA256 = ""
	if forged.SHA256, err = hashValue(*forged); err != nil {
		t.Fatal(err)
	}
	tampered, err := Encode(forged)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseArtifact(tampered); err != nil {
		t.Fatalf("re-digested tampered artifact does not parse: %v", err)
	}
	if _, err := Open(context.Background(), root, tampered); err == nil || !strings.Contains(err.Error(), "cannot be rederived") {
		t.Fatalf("tampered mention check: Open err = %v, want rederivation refusal", err)
	}
	plainRoot, plainManifest := providerFixture(t, nil)
	plain, err := Build(context.Background(), plainRoot, plainManifest)
	if err != nil {
		t.Fatal(err)
	}
	if plain.ClaimMentions != nil {
		t.Fatalf("token-free claims stored checks: %+v", plain.ClaimMentions)
	}
	if raw, _ := Encode(plain); bytes.Contains(raw, []byte("claim_mentions")) {
		t.Fatal("token-free corpus serialized claim_mentions")
	}
}

// DCX-V1-343: the mention check is served with the claim on v1 and v2 readers without changing
// the claim or its trust.
func TestDCXV1343ClaimMentionsAreServedWithTheClaim(t *testing.T) {
	t.Parallel()
	for _, fixture := range []struct {
		name  string
		build func(*testing.T, func(*ProviderRecord)) (string, Manifest)
	}{{"v1", providerFixture}, {"v2", adoptionFixture}} {
		root, m := fixture.build(t, mentionEdit)
		a, err := Build(context.Background(), root, m)
		if err != nil {
			t.Fatal(err)
		}
		claim := auditClaim(t, a, "adapter:claim")
		for _, op := range []Request{{Operation: "get", ID: "adapter:claim"}, {Operation: "claims", Path: "src/value.go"}, {Operation: "trace", ID: "adapter:claim"}} {
			r, err := Query(a, op, "fresh", nil)
			if err != nil {
				t.Fatal(err)
			}
			check, ok := r.ClaimMentions["adapter:claim"]
			if !ok || check.State != MentionsPartiallyUnresolved || len(check.Mentions) != 5 {
				t.Fatalf("%s %s: claim_mentions = %+v", fixture.name, op.Operation, r.ClaimMentions)
			}
			for _, x := range r.Results {
				if c, ok := x.(Claim); ok && c.ID == claim.ID && (c.Text != claim.Text || c.Evidence.Trust != claim.Evidence.Trust) {
					t.Fatalf("%s %s: claim changed: %+v", fixture.name, op.Operation, c)
				}
			}
			raw, _ := json.Marshal(r)
			if !bytes.Contains(raw, []byte(`"claim_mentions":{"adapter:claim":{`)) {
				t.Fatalf("%s %s: receipt JSON lacks claim_mentions", fixture.name, op.Operation)
			}
		}
		r, err := Query(a, Request{Operation: "search", Query: "nonexistentquokka"}, "fresh", nil)
		if err != nil || r.ClaimMentions != nil {
			t.Fatalf("%s: claim-free receipt carried claim_mentions: %+v %v", fixture.name, r.ClaimMentions, err)
		}
	}
}

// DCX-V1-344, DCX-V1-345: an unresolved mention is a typed gap on the claim, and coverage
// counts every checked claim by mention state.
func TestDCXV1344ClaimMentionGapsAndCoverage(t *testing.T) {
	t.Parallel()
	root, m := adoptionFixture(t, mentionEdit)
	a, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	r, err := Query(a, Request{Operation: "gaps", ID: "adapter:claim", Limit: MaxResults}, "fresh", nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, x := range r.Results {
		if g, ok := x.(Gap); ok && g.Kind == "claim-mentions-partially-unresolved" {
			found = g.Subject == "adapter:claim" && strings.HasSuffix(g.Reason, ": missingThing") && strings.HasPrefix(g.Reason, "1 of 5 ")
		}
	}
	if !found {
		t.Fatalf("gaps = %+v", r.Results)
	}
	filtered, err := Query(a, Request{Operation: "gaps", Kinds: []string{"claim-mentions-partially-unresolved"}, Limit: MaxResults}, "fresh", nil)
	if err != nil || len(filtered.Results) != 1 {
		t.Fatalf("kind filter = %+v %v", filtered.Results, err)
	}
	unresolved := []ClaimMentionCheck{{ClaimID: "c", Subject: "s", State: MentionsUnresolved, Unresolved: 9, Mentions: []ClaimMention{}}}
	for i := 0; i < 9; i++ {
		unresolved[0].Mentions = append(unresolved[0].Mentions, ClaimMention{Token: fmt.Sprintf("t%d", i), Resolution: MentionUnresolved})
	}
	gaps := claimMentionGaps(&Artifact{ClaimMentions: unresolved})
	if len(gaps) != 1 || gaps[0].Kind != "claim-mentions-unresolved" || !strings.HasSuffix(gaps[0].Reason, "t7, ...") {
		t.Fatalf("bounded reason: %+v", gaps)
	}
	rows := mentionCoverageRows(t, a)
	checked := checkedClaims(a)
	if checked < 1 {
		t.Fatalf("fixture checks %d claims", checked)
	}
	want := map[string]int{"claim_mentions:consistent": 0, "claim_mentions:no-mentions": checked - 1, "claim_mentions:partially-unresolved": 1, "claim_mentions:unresolved": 0}
	for metric, value := range want {
		row, ok := rows[metric]
		if !ok || fmt.Sprint(row["value"]) != fmt.Sprint(value) || fmt.Sprint(row["denominator"]) != fmt.Sprint(checked) || row["defined"] != true {
			t.Fatalf("%s = %+v", metric, row)
		}
	}
	plainRoot, plainManifest := adoptionFixture(t, nil)
	plain, err := Build(context.Background(), plainRoot, plainManifest)
	if err != nil {
		t.Fatal(err)
	}
	for metric := range mentionCoverageRows(t, plain) {
		if strings.HasPrefix(metric, "claim_mentions:") {
			t.Fatalf("token-free corpus reported %s", metric)
		}
	}
}

func mentionCoverageRows(t *testing.T, a *Artifact) map[string]map[string]any {
	t.Helper()
	r, err := Query(a, Request{Operation: "coverage", Limit: MaxResults}, "fresh", nil)
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]map[string]any{}
	for _, x := range r.Results {
		if row, ok := x.(map[string]any); ok {
			if metric, ok := row["metric"].(string); ok {
				rows[metric] = row
			}
		}
	}
	return rows
}

// DCX-V1-346: the opt-in unresolved-mention threshold refuses a corpus above it, naming the
// rate; unset or met thresholds accept.
func TestDCXV1346UnresolvedMentionThreshold(t *testing.T) {
	t.Parallel()
	a := &Artifact{ClaimMentions: []ClaimMentionCheck{{ClaimID: "a", Unresolved: 1}, {ClaimID: "b"}, {ClaimID: "c"}, {ClaimID: "d", Unresolved: 2}}}
	if n, d, rate := UnresolvedMentionRate(a); n != 2 || d != 4 || rate == nil || *rate != 0.5 {
		t.Fatalf("rate = %d/%d %v", n, d, rate)
	}
	half, below := 0.5, 0.49
	if err := CheckBuildThresholds(a, BuildThresholds{}); err != nil {
		t.Fatal(err)
	}
	if err := CheckBuildThresholds(a, BuildThresholds{MaxUnresolvedMentionRate: &half}); err != nil {
		t.Fatalf("rate equal to threshold refused: %v", err)
	}
	err := CheckBuildThresholds(a, BuildThresholds{MaxUnresolvedMentionRate: &below})
	var refused *Error
	if !errors.As(err, &refused) || refused.Code != "corpus-refused" || !strings.Contains(refused.Message, "unresolved-mention rate 2/4 = 0.5 exceeds 0.49") {
		t.Fatalf("threshold not refused: %v", err)
	}
	zero := 0.0
	if err := CheckBuildThresholds(&Artifact{}, BuildThresholds{MaxUnresolvedMentionRate: &zero}); err != nil {
		t.Fatalf("undefined rate refused: %v", err)
	}
	for _, good := range []string{"0", "1", "0.25", "1.0"} {
		if _, err := ParseRate(good); err != nil {
			t.Fatalf("%s refused", good)
		}
	}
	for _, bad := range []string{"", "-0.1", "1.01", "NaN", "1e-1", "0x1", "+0.5", "0_5", "half"} {
		if _, err := ParseRate(bad); err == nil {
			t.Fatalf("%q admitted", bad)
		}
	}
}

// DCX-V1-347: a dangling subject, relation, journey or observation reference refuses the build
// rather than entering the corpus.
func TestDCXV1347DanglingReferencesRefuse(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		want string
		edit func(*ProviderRecord)
	}{
		{"claim subject missing", func(p *ProviderRecord) { p.Claims[0].Subject = "adapter:missing" }},
		{"relationship source missing", func(p *ProviderRecord) {
			p.Relations = []Relation{{ID: "adapter:rel", From: "adapter:missing", To: "adapter:count", Type: "depends_on", Provider: "adapter", Evidence: p.Claims[0].Evidence}}
		}},
		{"relationship target missing", func(p *ProviderRecord) {
			p.Relations = []Relation{{ID: "adapter:rel", From: "adapter:count", To: "adapter:missing", Type: "depends_on", Provider: "adapter", Evidence: p.Claims[0].Evidence}}
		}},
		{"journey subject missing", func(p *ProviderRecord) { p.Journeys[0].Subject = "adapter:missing" }},
		{"observation must join a test subject", func(p *ProviderRecord) { p.Observations[0].Subject = "native:symbol:src/missing_test.go:1:TestMissing" }},
	} {
		root, m := providerFixture(t, c.edit)
		_, err := Build(context.Background(), root, m)
		tkRefused(t, err, c.want)
	}
}
