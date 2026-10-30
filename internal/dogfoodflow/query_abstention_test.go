package dogfoodflow

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func queryFixture(t *testing.T) (*change, *check, string) {
	t.Helper()
	root := t.TempDir()
	evidence := filepath.Join(root, "evidence")
	if err := os.Mkdir(evidence, 0700); err != nil {
		t.Fatal(err)
	}
	f := flow{ctx: context.Background(), root: root, base: strings.Repeat("a", 40), target: strings.Repeat("b", 40), stderr: io.Discard}
	task := "Deliver the original workflow\nincluding its unchanged bytes."
	runner := Runner{Path: "/never-execute-recorded-path", Run: func(_ context.Context, gotRoot string, args []string, stdout, stderr io.Writer) int {
		if gotRoot != root || strings.Join(args, "\x00") != strings.Join([]string{"query", "--task", task, "--limit", "1"}, "\x00") {
			t.Fatalf("unexpected replay: %q %q", gotRoot, args)
		}
		io.WriteString(stderr, queryAbstentionEnvelope)
		return 2
	}}
	c := &change{flow: f, evidence: evidence, options: ChangeOptions{Steps: runner}}
	c.coordinationQuery(task)
	if !c.complete() || c.queryAbstentionSHA == "" {
		t.Fatal("abstention did not qualify")
	}
	k := &check{flow: f, evidence: evidence, options: CheckOptions{BaseVerifier: runner, TreeVerifier: runner}}
	return c, k, task
}

func queryReport(c *change) []byte {
	rows := []map[string]string{}
	for _, row := range c.rows {
		rows = append(rows, map[string]string{"name": row.name, "status": row.status, "reason": row.reason})
	}
	raw, _ := json.Marshal(map[string]any{"steps": rows, "queryAbstentionEvidenceSha256": "sha256:" + c.queryAbstentionSHA})
	return raw
}

func queryCheckCode(k *check, report []byte, replay bool) int {
	code, _ := boundary(context.Background(), func() int {
		task, claimed := k.checkQueryAbstention(report)
		if replay && claimed {
			k.verifyQueryAbstention(task)
		}
		return 0
	}, func() {})
	return code
}

func TestQueryAbstentionReplayAndTransitions(t *testing.T) {
	for _, next := range []struct {
		name           string
		status         int
		stdout, stderr string
	}{
		{"success", 0, "packet\n", ""}, {"other failure", 2, "", "{\"code\": \"unsupported-query-trace-state\", \"error\": \"trace commit is unreachable\", \"ok\": false}\n"},
	} {
		t.Run(next.name, func(t *testing.T) {
			c, k, _ := queryFixture(t)
			if code := queryCheckCode(k, queryReport(c), true); code != 0 {
				t.Fatalf("valid replay %d", code)
			}
			c.options.Steps.Run = func(_ context.Context, _ string, _ []string, stdout, stderr io.Writer) int {
				io.WriteString(stdout, next.stdout)
				io.WriteString(stderr, next.stderr)
				return next.status
			}
			c.rows = nil
			c.coordinationQuery("same task")
			if c.queryAbstentionSHA != "" || exists(c.evidence+"/"+queryStep+"-abstention.json") {
				t.Fatal("stale abstention survived")
			}
			if c.complete() != (next.status == 0) {
				t.Fatal("incorrect completion")
			}
		})
	}
}

func TestQueryAbstentionRejectsNoncanonicalRefusals(t *testing.T) {
	for _, tc := range []struct {
		name           string
		status         int
		stdout, stderr string
	}{
		{"extra stderr", 2, "", queryAbstentionEnvelope + "extra\n"}, {"nonempty stdout", 2, "packet", queryAbstentionEnvelope}, {"wrong status", 1, "", queryAbstentionEnvelope},
		{"malformed", 2, "", "unsupported-query-trace-state\n"}, {"unreachable trace", 2, "", strings.Replace(queryAbstentionEnvelope, "requires an absent clean-tree local trace store", "refuses an unreachable trace", 1)},
		{"corrupt trace", 2, "", strings.Replace(queryAbstentionEnvelope, "requires an absent clean-tree local trace store", "cannot read corrupt trace store", 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _, task := queryFixture(t)
			c.rows = nil
			c.options.Steps.Run = func(_ context.Context, _ string, _ []string, stdout, stderr io.Writer) int {
				io.WriteString(stdout, tc.stdout)
				io.WriteString(stderr, tc.stderr)
				return tc.status
			}
			c.coordinationQuery(task)
			if c.complete() || c.queryAbstentionSHA != "" || exists(c.evidence+"/"+queryStep+"-abstention.json") {
				t.Fatal("invalid refusal qualified")
			}
		})
	}
}

func TestQueryAbstentionRejectsEvidenceDrift(t *testing.T) {
	cases := []string{"missing artifact", "duplicate member", "extra member", "malformed artifact", "stale base", "stale target", "task digest", "argv task", "argv root", "argv shape", "argv NUL", "argv limit", "stderr extra", "stdout tamper", "artifact digest", "duplicate query row", "duplicate digest", "wrong step", "failed local outcome", "failed CEM", "failed OCM", "missing digest"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			c, k, _ := queryFixture(t)
			stem := c.evidence + "/" + queryStep
			artifact := readFile(stem + "-abstention.json")
			changeFile := func(suffix string, data []byte) {
				t.Helper()
				if err := os.WriteFile(stem+suffix, data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			switch name {
			case "missing artifact":
				os.Remove(stem + "-abstention.json")
			case "duplicate member":
				artifact = bytes.Replace(artifact, []byte(`"exitStatus":"2"`), []byte(`"exitStatus":"2","exitStatus":"2"`), 1)
			case "extra member":
				artifact = bytes.Replace(artifact, []byte(`"exitStatus":"2"`), []byte(`"exitStatus":"2","extra":true`), 1)
			case "malformed artifact":
				artifact = []byte("{broken")
			case "stale base":
				k.base = strings.Repeat("c", 40)
			case "stale target":
				k.target = strings.Repeat("c", 40)
			case "task digest":
				artifact = bytes.Replace(artifact, []byte(`"taskSha256":"sha256:`), []byte(`"taskSha256":"sha256:0`), 1)
			case "argv task":
				changeFile(".argv", bytes.Replace(readFile(stem+".argv"), []byte("original"), []byte("changed"), 1))
			case "argv root":
				changeFile(".argv", bytes.Replace(readFile(stem+".argv"), []byte(c.root), []byte("/elsewhere"), 1))
			case "argv shape":
				changeFile(".argv", append(readFile(stem+".argv"), []byte("extra\x00")...))
			case "argv NUL":
				raw := readFile(stem + ".argv")
				changeFile(".argv", raw[:len(raw)-1])
			case "argv limit":
				changeFile(".argv", bytes.Replace(readFile(stem+".argv"), []byte("--limit\x001\x00"), []byte("--limit\x002\x00"), 1))
			case "stderr extra":
				changeFile(".stderr", []byte(queryAbstentionEnvelope+"extra\n"))
			case "stdout tamper":
				changeFile(".json", []byte("packet"))
			case "artifact digest":
				c.queryAbstentionSHA = strings.Repeat("0", 64)
			case "duplicate query row":
				c.rows = append(c.rows, c.rows[0])
			case "wrong step":
				c.rows[0].name = "cem-status"
			case "failed local outcome":
				c.rows = append(c.rows, step{"local-outcome", "NOT_PRODUCED", "record-index-failed"})
			case "failed CEM":
				c.rows = append(c.rows, step{"cem-status", "NOT_PRODUCED", "not-ready"})
			case "failed OCM":
				c.rows = append(c.rows, step{"ocm-status", "NOT_PRODUCED", "not-ready"})
			}
			if !bytes.Equal(artifact, readFile(stem+"-abstention.json")) && name != "missing artifact" {
				changeFile("-abstention.json", artifact)
				c.queryAbstentionSHA = sha256Hex(artifact)
			}
			report := queryReport(c)
			if name == "duplicate digest" {
				report = bytes.Replace(report, []byte(`{"queryAbstentionEvidenceSha256":`), []byte(`{"queryAbstentionEvidenceSha256":null,"queryAbstentionEvidenceSha256":`), 1)
			}
			if name == "missing digest" {
				var obj map[string]any
				json.Unmarshal(report, &obj)
				delete(obj, "queryAbstentionEvidenceSha256")
				report, _ = json.Marshal(obj)
			}
			if code := queryCheckCode(k, report, false); code != 1 {
				t.Fatalf("drift accepted: %d %s", code, report)
			}
		})
	}
}

func TestQueryAbstentionVerifierDisagreement(t *testing.T) {
	for _, role := range []string{"base", "current", "override", "all-success", "all-other-failure"} {
		t.Run(role, func(t *testing.T) {
			c, k, _ := queryFixture(t)
			bad := Runner{Run: func(_ context.Context, _ string, _ []string, _ io.Writer, stderr io.Writer) int {
				if role == "all-other-failure" {
					io.WriteString(stderr, "other refusal\n")
					return 2
				}
				return 0
			}}
			switch role {
			case "base":
				k.options.BaseVerifier = bad
			case "current":
				k.options.TreeVerifier = bad
			case "override":
				k.options.Override = &bad
			default:
				k.options.BaseVerifier = bad
				k.options.TreeVerifier = bad
			}
			if code := queryCheckCode(k, queryReport(c), true); code != 1 {
				t.Fatalf("disagreement accepted %d", code)
			}
		})
	}
}

func TestQueryAbstentionCompatibilityAndCompletion(t *testing.T) {
	c, k, _ := queryFixture(t)
	for _, row := range []step{{queryStep, "NOT_PRODUCED", queryAbstentionReason}, {"other", "PRODUCED", queryAbstentionReason}} {
		c.rows = []step{row}
		c.queryAbstentionSHA = ""
		if c.complete() {
			t.Fatal("unbacked exemption")
		}
	}
	os.Remove(c.evidence + "/" + queryStep + "-abstention.json")
	for _, report := range []string{`{"steps":[{"name":"coordination-time-query","status":"PRODUCED","reason":"none"}]}`, `{"steps":[{"name":"coordination-time-query","status":"PRODUCED","reason":"none"}],"queryAbstentionEvidenceSha256":null}`} {
		if code := queryCheckCode(k, []byte(report), false); code != 0 {
			t.Fatalf("old report refused: %d", code)
		}
	}
}

// Recompute the artifact and report digests, so the shape checks (rather than
// a simple digest mismatch) are what refuse adversarial recorded commands.
func TestQueryAbstentionRejectsReboundArgvShape(t *testing.T) {
	for _, variant := range []string{"root", "NUL", "extra", "limit", "verb", "empty task", "task NUL"} {
		t.Run(variant, func(t *testing.T) {
			c, k, task := queryFixture(t)
			stem := c.evidence + "/" + queryStep
			argv := readFile(stem + ".argv")
			switch variant {
			case "root":
				argv = bytes.Replace(argv, []byte(c.root), []byte("/elsewhere"), 1)
			case "NUL":
				argv = argv[:len(argv)-1]
			case "extra":
				argv = append(argv, []byte("extra\x00")...)
			case "limit":
				argv = bytes.Replace(argv, []byte("--limit\x001\x00"), []byte("--limit\x002\x00"), 1)
			case "verb":
				argv = bytes.Replace(argv, []byte("\x00query\x00"), []byte("\x00record\x00"), 1)
			case "empty task":
				argv = bytes.Replace(argv, []byte(task), nil, 1)
				task = ""
			case "task NUL":
				argv = bytes.Replace(argv, []byte(task), []byte(task+"\x00injected"), 1)
				task += "\x00injected"
			}
			artifact := queryAbstentionArtifact(argv, task, c.base, c.target, nil, []byte(queryAbstentionEnvelope))
			if err := os.WriteFile(stem+".argv", argv, 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(stem+"-abstention.json", artifact, 0600); err != nil {
				t.Fatal(err)
			}
			c.queryAbstentionSHA = sha256Hex(artifact)
			if code := queryCheckCode(k, queryReport(c), false); code != 1 {
				t.Fatalf("rebound invalid argv accepted: %d", code)
			}
		})
	}
}

func TestQueryAbstentionEvidenceWriteFailureBlocks(t *testing.T) {
	c, _, task := queryFixture(t)
	stem := c.evidence + "/" + queryStep
	if err := os.Remove(stem + "-abstention.json"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(stem+"-abstention.json", 0700); err != nil {
		t.Fatal(err)
	}
	c.rows = nil
	c.coordinationQuery(task)
	if c.complete() || c.queryAbstentionSHA != "" || c.rows[0].reason != "query-abstention-evidence-failed" {
		t.Fatal("artifact clear failure qualified")
	}
}
