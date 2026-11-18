package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/json/jsontext"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/lineanchor"
	"github.com/Beamfall/corvint/internal/mcp/protocol"
	"github.com/Beamfall/corvint/internal/repoenvelope"
)

// The RCF-V1-007 frozen query set and its committed digest.
const (
	rcfQuerySetPath    = "testdata/rcf-v1-qualification/queries.json"
	rcfQuerySetDigest  = "testdata/rcf-v1-qualification/queries.sha256"
	rcfQuerySetSchema  = "corvint-rcf-qualification-queries/1"
	rcfReportSchema    = "corvint-rcf-qualification-report/1"
	rcfQualifyEnv      = "CORVINT_RCF_QUALIFY"
	rcfDigestOnly      = "digest-only"
	rcfMaxQueryBytes   = 1024
	rcfQueriesPerRepo  = 25
	rcfRepositoryCount = 3
)

type rcfQuerySet struct {
	Schema       string          `json:"schema"`
	Requirement  string          `json:"requirement"`
	Frozen       string          `json:"frozen"`
	Limit        int             `json:"limit"`
	Repositories []rcfRepository `json:"repositories"`
}

type rcfRepository struct {
	Name     string     `json:"name"`
	URL      string     `json:"url"`
	Commit   string     `json:"commit"`
	Evidence string     `json:"evidence"`
	Scopes   []string   `json:"scopes"`
	Queries  []rcfQuery `json:"queries"`
}

type rcfQuery struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Query string `json:"query"`
}

// rcfAnswer is one query's raw output on each surface: the CLI's stdout and
// exit code, and the corvint.docs_search response line of `corvint-docs mcp`.
type rcfAnswer struct {
	CLI      []byte
	CLICode  int
	MCPFrame []byte
}

// rcfFailure names a failed query and, for a passage failure, the passage.
// A digest-only repository names the passage by digest, never by path.
type rcfFailure struct {
	Query   string `json:"query"`
	Passage string `json:"passage,omitempty"`
	Reason  string `json:"reason"`
}

// rcfRecord is the digest-and-count evidence one query leaves in the report.
type rcfRecord struct {
	Query        string `json:"query"`
	RecordSHA256 string `json:"record_sha256"`
	Results      int    `json:"results"`
	Passages     int    `json:"passages"`
}

type rcfRepositoryReport struct {
	Name             string       `json:"name"`
	Commit           string       `json:"commit"`
	Evidence         string       `json:"evidence"`
	Verdict          string       `json:"verdict"`
	Reason           string       `json:"reason,omitempty"`
	ArtifactSHA256   string       `json:"artifact_sha256,omitempty"`
	StatusBefore     string       `json:"status_before_sha256,omitempty"`
	StatusAfter      string       `json:"status_after_sha256,omitempty"`
	Queries          int          `json:"queries"`
	PassagesVerified int          `json:"passages_verified"`
	Unverified       int          `json:"unverified"`
	Failures         []rcfFailure `json:"failures"`
	Records          []rcfRecord  `json:"records"`
}

type rcfReport struct {
	Schema         string                `json:"schema"`
	QuerySetSHA256 string                `json:"query_set_sha256"`
	Verdict        string                `json:"verdict"`
	Repositories   []rcfRepositoryReport `json:"repositories"`
}

// rcfLoadQuerySet reads the committed query set and checks it against its
// committed digest (RCF-V1-007), returning the set and the digest.
func rcfLoadQuerySet(t *testing.T) (rcfQuerySet, string) {
	t.Helper()
	raw, err := os.ReadFile(rcfQuerySetPath)
	if err != nil {
		t.Fatal(err)
	}
	pinned, err := os.ReadFile(rcfQuerySetDigest)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	digest := hex.EncodeToString(sum[:])
	if want := digest + "  queries.json\n"; string(pinned) != want {
		t.Fatalf("RCF-V1-007: query set digest %s does not match %s %q; an edit starts a new set", digest, rcfQuerySetDigest, pinned)
	}
	var set rcfQuerySet
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&set); err != nil {
		t.Fatal(err)
	}
	return set, digest
}

// RCF-V1-007: the committed query set matches its digest, holds 25 queries
// for each of Corvint, Beamfall and one public proof-set repository, pins
// each to a full commit, and mixes file, symbol and prose queries, each
// within the docs_search query bound.
func TestRCFV1007FrozenQuerySet(t *testing.T) {
	set, _ := rcfLoadQuerySet(t)
	if set.Schema != rcfQuerySetSchema || set.Requirement != "RCF-V1-007" || set.Limit < 1 || set.Limit > doccorpus.MaxResults {
		t.Fatalf("RCF-V1-007: header %+v", set)
	}
	names := []string{}
	ids := map[string]bool{}
	for _, repo := range set.Repositories {
		names = append(names, repo.Name)
		if len(repo.Commit) != 40 || strings.Trim(repo.Commit, "0123456789abcdef") != "" {
			t.Errorf("RCF-V1-007: %s is not pinned to a full commit: %q", repo.Name, repo.Commit)
		}
		if repo.URL == "" || len(repo.Scopes) == 0 || (repo.Evidence != "full" && repo.Evidence != rcfDigestOnly) {
			t.Errorf("RCF-V1-007: %s header %+v", repo.Name, repo)
		}
		if len(repo.Queries) != rcfQueriesPerRepo {
			t.Errorf("RCF-V1-007: %s has %d queries, want %d", repo.Name, len(repo.Queries), rcfQueriesPerRepo)
		}
		kinds := map[string]int{}
		for _, q := range repo.Queries {
			kinds[q.Kind]++
			if ids[q.ID] || !strings.HasPrefix(q.ID, repo.Name+"-") {
				t.Errorf("RCF-V1-007: query id %q duplicated or outside %s", q.ID, repo.Name)
			}
			ids[q.ID] = true
			if strings.TrimSpace(q.Query) == "" || len(q.Query) > rcfMaxQueryBytes {
				t.Errorf("RCF-V1-007: %s query outside the bound", q.ID)
			}
		}
		for _, kind := range []string{"file", "symbol", "prose"} {
			if kinds[kind] < rcfQueriesPerRepo/4 {
				t.Errorf("RCF-V1-007: %s has %d %s queries", repo.Name, kinds[kind], kind)
			}
		}
		if len(kinds) != 3 {
			t.Errorf("RCF-V1-007: %s query kinds %v", repo.Name, kinds)
		}
	}
	if !slices.Equal(names, []string{"corvint", "beamfall", "prometheus"}) {
		t.Fatalf("RCF-V1-007: repositories %v", names)
	}
	if set.Repositories[1].Evidence != rcfDigestOnly {
		t.Fatal("RCF-V1-008: Beamfall evidence must be digest-only")
	}
}

// rcfCollect asks every query on both surfaces against one corpus artifact:
// the CLI `docs corpus search` in process, and corvint.docs_search on one real
// `corvint-docs mcp` session in a child of this test binary.
func rcfCollect(t *testing.T, root, artifact string, limit int, queries []rcfQuery) (map[string]rcfAnswer, error) {
	t.Helper()
	answers := map[string]rcfAnswer{}
	requests := []string{}
	for i, q := range queries {
		code, out, _ := corpusCLI(t, root, "docs", "corpus", "search", "--artifact", artifact, "--query", q.Query, "--limit", strconv.Itoa(limit))
		answers[q.ID] = rcfAnswer{CLI: []byte(out), CLICode: code}
		arguments, err := json.Marshal(map[string]any{"query": q.Query, "limit": limit, "receipt": "full"})
		if err != nil {
			return nil, err
		}
		requests = append(requests, fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"tools/call","params":{"name":"corvint.docs_search","arguments":%s}}`, i+1, arguments))
	}
	frames, err := rcfMCPSession(root, artifact, requests)
	if err != nil {
		return nil, err
	}
	for i, q := range queries {
		a := answers[q.ID]
		a.MCPFrame = frames[strconv.Itoa(i+1)]
		answers[q.ID] = a
	}
	return answers, nil
}

// rcfMCPSession runs one `corvint-docs mcp` session (the TestMTTV1004DocsMCPHelper
// child) and returns each response line by its request id, unparsed.
func rcfMCPSession(root, artifact string, requests []string) (map[string][]byte, error) {
	binary, err := os.Executable()
	if err != nil {
		return nil, err
	}
	lines := append([]string{
		`{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"rcf","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
	}, requests...)
	child := exec.Command(binary, "-test.run=^TestMTTV1004DocsMCPHelper$", "--", "--mtt-docs-mcp", "--root", root, "--corpus-artifact", artifact)
	child.Stdin = strings.NewReader(strings.Join(lines, "\n") + "\n")
	var stdout, stderr bytes.Buffer
	child.Stdout, child.Stderr = &stdout, &stderr
	if err := child.Run(); err != nil {
		return nil, fmt.Errorf("corvint-docs mcp: %v: %s", err, stderr.Bytes())
	}
	frames := map[string][]byte{}
	scanner := bufio.NewScanner(&stdout)
	scanner.Buffer(nil, 4*protocol.MaxMessageBytes)
	for scanner.Scan() {
		var head struct {
			ID jsontext.Value `json:"id"`
		}
		line := slices.Clone(scanner.Bytes())
		if err := json.Unmarshal(line, &head); err != nil {
			return nil, err
		}
		frames[string(head.ID)] = line
	}
	return frames, scanner.Err()
}

// rcfCanonical is the RFC 8785 canonical form of one JSON value.
func rcfCanonical(raw []byte) ([]byte, error) {
	value := jsontext.Value(slices.Clone(bytes.TrimSpace(raw)))
	if err := value.Canonicalize(); err != nil {
		return nil, err
	}
	return value, nil
}

// rcfVerifier checks passages against the repository's pinned Git objects,
// reading each tree entry and blob once.
type rcfVerifier struct {
	root    string
	entries map[string]string
	blobs   map[string][]byte
}

func (v *rcfVerifier) git(args ...string) ([]byte, error) {
	command := exec.Command("git", append([]string{"-C", v.root}, args...)...)
	command.Env = fixtureGitEnvironment()
	return command.Output()
}

// verify returns "" when the anchor's blob is the one its revision holds at
// its path, its whole-file digest matches, and its line span's digest
// matches; otherwise it returns why not.
func (v *rcfVerifier) verify(a lineanchor.Anchor) string {
	if a.Revision == "" || a.Path == "" || a.Blob == "" || a.SHA256 == "" {
		return "passage is not pinned to a revision, path, blob and digest"
	}
	key := a.Revision + ":" + a.Path
	blob, ok := v.entries[key]
	if !ok {
		out, err := v.git("rev-parse", "--verify", "--quiet", key)
		blob = strings.TrimSpace(string(out))
		if err != nil {
			blob = ""
		}
		v.entries[key] = blob
	}
	if blob != a.Blob {
		return "blob is not the one the pinned revision holds at the path"
	}
	data, ok := v.blobs[blob]
	if !ok {
		out, err := v.git("cat-file", "blob", blob)
		if err != nil {
			return "pinned blob cannot be read"
		}
		data = out
		v.blobs[blob] = data
	}
	if doccorpus.Digest(data) != a.SHA256 {
		return "blob digest does not match"
	}
	if a.SpanSHA256 == "" {
		return "line span digest is missing"
	}
	lines := bytes.SplitAfter(data, []byte("\n"))
	if len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	if a.Start < 1 || a.End < a.Start || a.End > len(lines) {
		return "line span is outside the pinned blob"
	}
	span := bytes.Join(lines[a.Start-1:a.End], nil)
	if len(bytes.TrimSpace(span)) == 0 || len(span) > lineanchor.MaxSpanBytes {
		return "line span is empty or exceeds the excerpt bound"
	}
	if doccorpus.Digest(span) != a.SpanSHA256 {
		return "line span digest does not match the pinned blob"
	}
	return ""
}

// rcfAnchors collects anchors from the receipt's defined locations, even
// when required fields are missing: citations, result evidence, rank match
// anchors and envelope citations. verify rejects incomplete anchors.
func rcfAnchors(value any, out *[]lineanchor.Anchor) error {
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			if key == "citations" || key == "anchors" || key == "match_anchor" {
				raw, err := json.Marshal(child)
				if err != nil {
					return err
				}
				if key == "match_anchor" {
					var a lineanchor.Anchor
					if err := json.Unmarshal(raw, &a); err != nil {
						return err
					}
					*out = append(*out, a)
				} else {
					var anchors []lineanchor.Anchor
					if err := json.Unmarshal(raw, &anchors); err != nil {
						return err
					}
					*out = append(*out, anchors...)
				}
				continue
			}
			if err := rcfAnchors(child, out); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range v {
			if err := rcfAnchors(child, out); err != nil {
				return err
			}
		}
	}
	return nil
}

func rcfDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// rcfPassageName names a passage by path and span, or by digest only.
func rcfPassageName(a lineanchor.Anchor, digestOnly bool) string {
	name := fmt.Sprintf("%s:%d-%d", a.Path, a.Start, a.End)
	if digestOnly {
		return "sha256:" + rcfDigest([]byte(a.Revision+"\x00"+name))
	}
	return name
}

// rcfCheck applies RCF-V1-008 to every query's answers: CLI success, a full
// MCP result, byte-equal canonical records, the result and frame bounds, the
// pinned source revision, and every returned passage verified. It returns the
// failures and fills the report's counts and per-query records.
func rcfCheck(root, commit string, limit int, digestOnly bool, queries []rcfQuery, answers map[string]rcfAnswer, report *rcfRepositoryReport) {
	verifier := &rcfVerifier{root: root, entries: map[string]string{}, blobs: map[string][]byte{}}
	fail := func(query, passage, reason string) {
		report.Failures = append(report.Failures, rcfFailure{Query: query, Passage: passage, Reason: reason})
	}
	for _, q := range queries {
		report.Queries++
		a, ok := answers[q.ID]
		if !ok {
			fail(q.ID, "", "query was not asked")
			continue
		}
		if a.CLICode != 0 {
			fail(q.ID, "", "docs corpus search exited "+strconv.Itoa(a.CLICode))
			continue
		}
		if len(a.MCPFrame) == 0 {
			fail(q.ID, "", "corvint.docs_search gave no response")
			continue
		}
		if len(a.MCPFrame) > protocol.MaxMessageBytes {
			fail(q.ID, "", "corvint.docs_search response exceeds the MCP message bound")
		}
		var frame struct {
			Result *struct {
				IsError           bool           `json:"isError"`
				StructuredContent jsontext.Value `json:"structuredContent"`
				Content           []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"result"`
		}
		if err := json.Unmarshal(a.MCPFrame, &frame); err != nil || frame.Result == nil || frame.Result.IsError || len(frame.Result.StructuredContent) == 0 {
			fail(q.ID, "", "corvint.docs_search did not answer with a receipt")
			continue
		}
		cli, cliErr := rcfCanonical(a.CLI)
		mcp, mcpErr := rcfCanonical(frame.Result.StructuredContent)
		if cliErr != nil || mcpErr != nil {
			fail(q.ID, "", "a record is not valid JSON")
			continue
		}
		if !bytes.Equal(cli, mcp) {
			fail(q.ID, "", "CLI and MCP records differ after canonical JSON encoding")
		}
		if len(frame.Result.Content) != 1 || frame.Result.Content[0].Type != "text" {
			fail(q.ID, "", "MCP text receipt is missing or has unexpected content blocks")
		} else {
			text := frame.Result.Content[0].Text
			payload := strings.TrimSuffix(strings.TrimPrefix(text, repoenvelope.Prefix), repoenvelope.Suffix)
			framed, err := repoenvelope.Frame(payload)
			if err != nil || framed != text {
				fail(q.ID, "", "MCP text receipt has an invalid repository envelope")
			} else if record, err := rcfCanonical([]byte(payload)); err != nil || !bytes.Equal(record, mcp) || !bytes.Equal(record, cli) {
				fail(q.ID, "", "MCP text and structured records differ after canonical JSON encoding")
			}
		}
		var receipt doccorpus.Receipt
		var tree any
		if err := json.Unmarshal(a.CLI, &receipt); err != nil {
			fail(q.ID, "", "CLI record is not a corpus receipt")
			continue
		}
		if err := json.Unmarshal(a.CLI, &tree); err != nil {
			fail(q.ID, "", "CLI record is not a corpus receipt")
			continue
		}
		if receipt.Operation != "search" || receipt.Repository.Revision != commit {
			fail(q.ID, "", "receipt is not a search of the pinned commit")
		}
		if len(receipt.Results) > limit {
			fail(q.ID, "", fmt.Sprintf("%d results exceed the limit %d", len(receipt.Results), limit))
		}
		var anchors []lineanchor.Anchor
		if err := rcfAnchors(tree, &anchors); err != nil {
			fail(q.ID, "", "receipt anchors cannot be read")
			continue
		}
		seen := map[lineanchor.Anchor]bool{}
		passages := 0
		for _, anchor := range anchors {
			if seen[anchor] {
				continue
			}
			seen[anchor] = true
			passages++
			if reason := verifier.verify(anchor); reason != "" {
				report.Unverified++
				fail(q.ID, rcfPassageName(anchor, digestOnly), reason)
				continue
			}
			report.PassagesVerified++
		}
		report.Records = append(report.Records, rcfRecord{Query: q.ID, RecordSHA256: rcfDigest(cli), Results: len(receipt.Results), Passages: passages})
	}
}

// rcfStatus is `git status --porcelain` of the repository, untracked files included.
func rcfStatus(root string) ([]byte, error) {
	command := exec.Command("git", "-C", root, "status", "--porcelain=v1", "--untracked-files=all", "--ignored=no")
	command.Env = fixtureGitEnvironment()
	return command.Output()
}

// rcfQualify runs one repository's queries on both surfaces and
// checks them (RCF-V1-008). `git status --porcelain` must not change.
func rcfQualify(t *testing.T, repo rcfRepository, limit int, root, artifact string, tamper func(map[string]rcfAnswer)) rcfRepositoryReport {
	t.Helper()
	report := rcfRepositoryReport{Name: repo.Name, Commit: repo.Commit, Evidence: repo.Evidence, Failures: []rcfFailure{}, Records: []rcfRecord{}}
	raw, err := doccorpus.ReadCorpusFile(root, artifact)
	if err != nil {
		report.Verdict, report.Reason = "FAIL", "corpus artifact cannot be read"
		return report
	}
	report.ArtifactSHA256 = rcfDigest(raw)
	before, err := rcfStatus(root)
	if err != nil {
		report.Verdict, report.Reason = "FAIL", "git status failed"
		return report
	}
	answers, err := rcfCollect(t, root, artifact, limit, repo.Queries)
	if err != nil {
		report.Verdict, report.Reason = "FAIL", "corvint-docs mcp session failed"
		return report
	}
	after, err := rcfStatus(root)
	if err != nil {
		report.Verdict, report.Reason = "FAIL", "git status failed"
		return report
	}
	report.StatusBefore, report.StatusAfter = rcfDigest(before), rcfDigest(after)
	if tamper != nil {
		tamper(answers)
	}
	rcfCheck(root, repo.Commit, limit, repo.Evidence == rcfDigestOnly, repo.Queries, answers, &report)
	if !bytes.Equal(before, after) {
		report.Failures = append(report.Failures, rcfFailure{Query: "*", Reason: "git status --porcelain changed"})
	}
	report.Verdict = "PASS"
	if len(report.Failures) > 0 {
		report.Verdict = "FAIL"
	}
	return report
}

// rcfVerdict is PASS only when every repository passed; a FAIL outranks a
// NOT_RUN, and neither is ever reported as a pass.
func rcfVerdict(repos []rcfRepositoryReport) string {
	verdict := "PASS"
	for _, r := range repos {
		if r.Verdict == "FAIL" {
			return "FAIL"
		}
		if r.Verdict != "PASS" {
			verdict = "NOT_RUN"
		}
	}
	return verdict
}

// rcfFixtureRepository is the typed corpus fixture as a one-repository query
// set: its pinned commit is the fixture's source revision.
func rcfFixtureRepository(t *testing.T, evidence string) (string, rcfRepository) {
	t.Helper()
	root, m := typedCorpusCLIFixture(t)
	return root, rcfRepository{Name: "fixture", Commit: m.Repository.Revision, Evidence: evidence, Queries: []rcfQuery{
		{ID: "fixture-01", Kind: "symbol", Query: "Checkout"},
		{ID: "fixture-02", Kind: "file", Query: "proof.go"},
		{ID: "fixture-03", Kind: "prose", Query: "how is checkout declared"},
	}}
}

// rcfRewrite decodes one JSON document, applies edit and re-encodes it.
func rcfRewrite(t *testing.T, raw []byte, edit func(map[string]any)) []byte {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	edit(doc)
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// rcfRewriteAnswer keeps all three receipt copies equal while injecting a
// passage defect, so transport parity cannot mask the verifier regression.
func rcfRewriteAnswer(t *testing.T, a rcfAnswer, edit func(map[string]any)) rcfAnswer {
	t.Helper()
	a.CLI = rcfRewrite(t, a.CLI, edit)
	a.MCPFrame = rcfRewrite(t, a.MCPFrame, func(frame map[string]any) {
		result := frame["result"].(map[string]any)
		edit(result["structuredContent"].(map[string]any))
		framed, err := repoenvelope.Frame(string(a.CLI))
		if err != nil {
			t.Fatal(err)
		}
		result["content"] = []any{map[string]any{"type": "text", "text": framed}}
	})
	return a
}

// RCF-V1-008: on the typed fixture every query answers on both surfaces with
// byte-equal canonical records, every passage verifies against its pinned
// blob, the bounds hold and the repository's status is unchanged.
func TestRCFV1008QualificationPassesOnAFixture(t *testing.T) {
	root, repo := rcfFixtureRepository(t, "full")
	report := rcfQualify(t, repo, 20, root, "corpus.json", nil)
	if report.Verdict != "PASS" || report.Queries != 3 || report.PassagesVerified == 0 || report.Unverified != 0 || len(report.Records) != 3 {
		t.Fatalf("RCF-V1-008: %+v", report)
	}
	if report.StatusBefore != report.StatusAfter {
		t.Fatal("RCF-V1-008: status changed", report)
	}
	if got := rcfVerdict([]rcfRepositoryReport{report}); got != "PASS" {
		t.Fatal(got)
	}
}

// RCF-V1-008 scenario "One unverified passage": a returned passage whose span
// digest does not match its pinned blob fails the run and names the query
// and the passage, even though both surfaces agree on it.
func TestRCFV1008UnverifiedPassageFailsTheRun(t *testing.T) {
	root, repo := rcfFixtureRepository(t, "full")
	report := rcfQualify(t, repo, 20, root, "corpus.json", func(answers map[string]rcfAnswer) {
		a := answers["fixture-01"]
		edit := func(doc map[string]any) {
			citations, _ := doc["citations"].([]any)
			if len(citations) == 0 {
				t.Fatal("fixture-01 returned no citation")
			}
			citations[0].(map[string]any)["span_sha256"] = doccorpus.Digest([]byte("tampered\n"))
		}
		answers["fixture-01"] = rcfRewriteAnswer(t, a, edit)
	})
	if report.Verdict != "FAIL" || report.Unverified != 1 || len(report.Failures) != 1 {
		t.Fatalf("RCF-V1-008: an unverified passage passed: %+v", report)
	}
	f := report.Failures[0]
	if f.Query != "fixture-01" || f.Passage != "proof.go:2-2" || !strings.Contains(f.Reason, "span digest") {
		t.Fatalf("RCF-V1-008: failure does not name the query and passage: %+v", f)
	}
	if got := rcfVerdict([]rcfRepositoryReport{report}); got != "FAIL" {
		t.Fatal(got)
	}
}

// RCF-V1-008: incomplete anchors at each defined location fail even when
// the CLI and both MCP receipt copies agree on the malformed object.
func TestRCFV1008IncompleteAnchorFailsTheRun(t *testing.T) {
	root, repo := rcfFixtureRepository(t, "full")
	locations := []struct {
		name string
		put  func(map[string]any, any)
	}{
		{"citations", func(doc map[string]any, a any) { doc["citations"] = []any{a} }},
		{"result evidence", func(doc map[string]any, a any) {
			doc["results"].([]any)[0].(map[string]any)["evidence"].(map[string]any)["anchors"] = []any{a}
		}},
		{"rank match", func(doc map[string]any, a any) {
			doc["result_ranks"] = map[string]any{"proof:flow": map[string]any{"match_anchor": a}}
		}},
		{"envelope citations", func(doc map[string]any, a any) {
			doc["trust_envelope"].(map[string]any)["citations"] = []any{a}
		}},
	}
	for _, location := range locations {
		for _, field := range []string{"blob", "path", "revision", "sha256"} {
			t.Run(location.name+"/"+field, func(t *testing.T) {
				report := rcfQualify(t, repo, 20, root, "corpus.json", func(answers map[string]rcfAnswer) {
					// First prove the same location with an intact anchor passes.
					answers["fixture-01"] = rcfRewriteAnswer(t, answers["fixture-01"], func(doc map[string]any) {
						location.put(doc, doc["citations"].([]any)[0])
					})
					var baseline rcfRepositoryReport
					rcfCheck(root, repo.Commit, 20, false, repo.Queries, answers, &baseline)
					if len(baseline.Failures) != 0 || baseline.PassagesVerified == 0 {
						t.Fatalf("RCF-V1-008: invalid baseline: %+v", baseline)
					}
					answers["fixture-01"] = rcfRewriteAnswer(t, answers["fixture-01"], func(doc map[string]any) {
						original := doc["citations"].([]any)[0].(map[string]any)
						anchor := map[string]any{}
						for key, value := range original {
							anchor[key] = value
						}
						delete(anchor, field)
						location.put(doc, anchor)
					})
				})
				if report.Verdict != "FAIL" || report.Unverified != 1 || len(report.Failures) != 1 {
					t.Fatalf("RCF-V1-008: incomplete anchor passed: %+v", report)
				}
				if f := report.Failures[0]; f.Query != "fixture-01" || !strings.Contains(f.Reason, "not pinned") {
					t.Fatalf("RCF-V1-008: failure does not name the query: %+v", f)
				}
			})
		}
	}
}

// RCF-V1-008 / DCP-V1-004: a matching whole-blob hash cannot excuse a
// missing, invalid, vacuous or oversized passage, even with a matching span hash.
func TestRCFV1008InvalidSpanFailsTheRun(t *testing.T) {
	root, repo := rcfFixtureRepository(t, "full")
	blank := " \t\r\n"
	oversized := strings.Repeat("x", lineanchor.MaxSpanBytes) + "\n"
	data := "valid\n" + blank + oversized
	// This commit is confined to the disposable fixture repository.
	cemWrite(t, root, "spans.txt", data)
	cemGit(t, root, "add", "spans.txt")
	cemGit(t, root, "commit", "-qm", "span verification fixture")
	revision := cemGit(t, root, "rev-parse", "HEAD")
	blob := cemGit(t, root, "rev-parse", revision+":spans.txt")
	for _, tc := range []struct {
		name   string
		start  int
		end    int
		digest string
		reason string
	}{
		{"missing span", 0, 0, "", "span digest is missing"},
		{"missing digest", 1, 1, "", "span digest is missing"},
		{"zero coordinates", 0, 0, doccorpus.Digest(nil), "outside the pinned blob"},
		{"negative start", -1, 1, doccorpus.Digest([]byte("valid\n")), "outside the pinned blob"},
		{"reversed coordinates", 2, 1, doccorpus.Digest(nil), "outside the pinned blob"},
		{"past last line", 1, 4, doccorpus.Digest([]byte(data)), "outside the pinned blob"},
		{"whitespace only", 2, 2, doccorpus.Digest([]byte(blank)), "empty or exceeds"},
		{"oversized", 3, 3, doccorpus.Digest([]byte(oversized)), "empty or exceeds"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			report := rcfQualify(t, repo, 20, root, "corpus.json", func(answers map[string]rcfAnswer) {
				answers["fixture-01"] = rcfRewriteAnswer(t, answers["fixture-01"], func(doc map[string]any) {
					a := doc["citations"].([]any)[0].(map[string]any)
					a["revision"], a["path"], a["blob"], a["sha256"] = revision, "spans.txt", blob, doccorpus.Digest([]byte(data))
					a["start_line"], a["end_line"], a["span_sha256"] = 1, 1, doccorpus.Digest([]byte("valid\n"))
				})
				var baseline rcfRepositoryReport
				rcfCheck(root, repo.Commit, 20, false, repo.Queries, answers, &baseline)
				if len(baseline.Failures) != 0 || baseline.PassagesVerified == 0 {
					t.Fatalf("RCF-V1-008: invalid baseline: %+v", baseline)
				}
				answers["fixture-01"] = rcfRewriteAnswer(t, answers["fixture-01"], func(doc map[string]any) {
					a := doc["citations"].([]any)[0].(map[string]any)
					a["start_line"], a["end_line"], a["span_sha256"] = tc.start, tc.end, tc.digest
				})
			})
			if report.Verdict != "FAIL" || report.Unverified != 1 || len(report.Failures) != 1 {
				t.Fatalf("RCF-V1-008: invalid span passed: %+v", report)
			}
			if f := report.Failures[0]; f.Query != "fixture-01" || !strings.Contains(f.Reason, tc.reason) {
				t.Fatalf("RCF-V1-008: failure does not name the query: %+v", f)
			}
		})
	}
}

// RCF-V1-008 / DCP-V1-016: corrupting only the model-facing text fails the
// run while the CLI and MCP structured receipts still agree.
func TestRCFV1008MCPTextMismatchFailsTheRun(t *testing.T) {
	root, repo := rcfFixtureRepository(t, "full")
	for _, mode := range []string{"missing", "different", "unframed", "invalid JSON", "terminator collision", "hidden character"} {
		t.Run(mode, func(t *testing.T) {
			reason := "invalid repository envelope"
			switch mode {
			case "missing":
				reason = "missing or has unexpected content blocks"
			case "different", "invalid JSON":
				reason = "records differ"
			}
			report := rcfQualify(t, repo, 20, root, "corpus.json", func(answers map[string]rcfAnswer) {
				a := answers["fixture-02"]
				a.MCPFrame = rcfRewrite(t, a.MCPFrame, func(frame map[string]any) {
					result := frame["result"].(map[string]any)
					text := result["content"].([]any)[0].(map[string]any)
					switch mode {
					case "missing":
						delete(result, "content")
					case "different":
						raw := rcfRewrite(t, a.CLI, func(doc map[string]any) { doc["freshness"] = "stale" })
						framed, err := repoenvelope.Frame(string(raw))
						if err != nil {
							t.Fatal(err)
						}
						text["text"] = framed
					case "unframed":
						text["text"] = string(a.CLI)
					case "invalid JSON":
						text["text"] = repoenvelope.Prefix + "{" + repoenvelope.Suffix
					case "terminator collision":
						text["text"] = repoenvelope.Prefix + `{"text":"` + repoenvelope.Terminator + `"}` + repoenvelope.Suffix
					case "hidden character":
						text["text"] = repoenvelope.Prefix + "{\"text\":\"\u2028\"}" + repoenvelope.Suffix
					}
				})
				answers["fixture-02"] = a
			})
			if report.Verdict != "FAIL" || len(report.Failures) != 1 || report.Unverified != 0 {
				t.Fatalf("RCF-V1-008: MCP text mismatch passed: %+v", report)
			}
			if f := report.Failures[0]; f.Query != "fixture-02" || !strings.Contains(f.Reason, reason) {
				t.Fatalf("RCF-V1-008: failure does not name the query: %+v", f)
			}
		})
	}
}

// RCF-V1-008 scenario "Surfaces disagree": a docs_search record that differs
// from the `docs corpus search` record for one query fails the run and names
// that query only.
func TestRCFV1008SurfaceMismatchFailsTheRun(t *testing.T) {
	root, repo := rcfFixtureRepository(t, "full")
	report := rcfQualify(t, repo, 20, root, "corpus.json", func(answers map[string]rcfAnswer) {
		a := answers["fixture-02"]
		a.MCPFrame = rcfRewrite(t, a.MCPFrame, func(frame map[string]any) {
			frame["result"].(map[string]any)["structuredContent"].(map[string]any)["freshness"] = "stale"
		})
		answers["fixture-02"] = a
	})
	if report.Verdict != "FAIL" || len(report.Failures) != 2 {
		t.Fatalf("RCF-V1-008: a surface mismatch passed: %+v", report)
	}
	for _, f := range report.Failures {
		if f.Query != "fixture-02" || f.Passage != "" || !strings.Contains(f.Reason, "records differ") {
			t.Fatalf("RCF-V1-008: failure does not name the query: %+v", f)
		}
	}
}

// RCF-V1-008: a digest-only repository's report keeps digests and counts
// only; no query text, path or passage text reaches it, even for a failure.
func TestRCFV1008DigestOnlyReport(t *testing.T) {
	root, repo := rcfFixtureRepository(t, rcfDigestOnly)
	report := rcfQualify(t, repo, 20, root, "corpus.json", func(answers map[string]rcfAnswer) {
		a := answers["fixture-01"]
		edit := func(doc map[string]any) {
			citations, _ := doc["citations"].([]any)
			citations[0].(map[string]any)["sha256"] = doccorpus.Digest([]byte("tampered"))
		}
		answers["fixture-01"] = rcfRewriteAnswer(t, a, edit)
	})
	if report.Verdict != "FAIL" || len(report.Failures) == 0 || !strings.HasPrefix(report.Failures[0].Passage, "sha256:") {
		t.Fatalf("RCF-V1-008: %+v", report)
	}
	raw, err := json.Marshal(rcfReport{Schema: rcfReportSchema, Verdict: rcfVerdict([]rcfRepositoryReport{report}), Repositories: []rcfRepositoryReport{report}})
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{"proof.go", "Checkout", "checkout", "func ", "package cache"} {
		if bytes.Contains(raw, []byte(leak)) {
			t.Fatalf("RCF-V1-008: digest-only report leaks %q: %s", leak, raw)
		}
	}
}

// rcfRunConfig names, for the opt-in run, each repository's checkout at its
// pinned commit and its corpus artifact relative to that root, and where the
// report is written.
type rcfRunConfig struct {
	Report       string `json:"report"`
	Repositories map[string]struct {
		Root     string `json:"root"`
		Artifact string `json:"artifact"`
	} `json:"repositories"`
}

// TestRCFV1008Qualification is the owner-run qualification (task 2.3): with
// CORVINT_RCF_QUALIFY naming a run configuration, it asks the frozen query
// set on both surfaces for every configured repository and writes a
// digest-and-count report. A repository left unconfigured is NOT_RUN, and the
// run passes only when every repository passes. Without the variable it skips.
func TestRCFV1008Qualification(t *testing.T) {
	path := os.Getenv(rcfQualifyEnv)
	if path == "" {
		t.Skip(rcfQualifyEnv + " is not set; the RCF-V1-008 qualification is owner-run")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var config rcfRunConfig
	if err := json.Unmarshal(raw, &config); err != nil {
		t.Fatal(err)
	}
	if config.Report == "" || !filepath.IsAbs(config.Report) {
		t.Fatal("RCF-V1-008: the run configuration needs an absolute report path")
	}
	set, digest := rcfLoadQuerySet(t)
	if len(set.Repositories) != rcfRepositoryCount {
		t.Fatalf("RCF-V1-007: %d repositories", len(set.Repositories))
	}
	report := rcfReport{Schema: rcfReportSchema, QuerySetSHA256: digest}
	for _, repo := range set.Repositories {
		target, ok := config.Repositories[repo.Name]
		if !ok || target.Root == "" || target.Artifact == "" {
			report.Repositories = append(report.Repositories, rcfRepositoryReport{Name: repo.Name, Commit: repo.Commit, Evidence: repo.Evidence, Verdict: "NOT_RUN", Reason: "repository not configured", Failures: []rcfFailure{}, Records: []rcfRecord{}})
			continue
		}
		report.Repositories = append(report.Repositories, rcfQualify(t, repo, set.Limit, target.Root, target.Artifact, nil))
	}
	report.Verdict = rcfVerdict(report.Repositories)
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.Report, append(encoded, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	if report.Verdict != "PASS" {
		var failures []string
		for _, r := range report.Repositories {
			failures = append(failures, fmt.Sprintf("%s %s %s %d failures", r.Name, r.Verdict, r.Reason, len(r.Failures)))
		}
		t.Fatalf("RCF-V1-008: qualification %s (report %s): %s", report.Verdict, config.Report, strings.Join(failures, "; "))
	}
}
