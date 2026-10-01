package postmergehost

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const hostDir = "../../protocol/postmerge-host"

var templates = []string{"postmerge.yml", "source-trigger.yml", "reconcile.yml"}

func loadGraph(t *testing.T) *Graph {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(hostDir, "workflow-graph.json"))
	if err != nil {
		t.Fatal(err)
	}
	g, err := ParseGraph(data)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func template(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(hostDir, "github-actions", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// PCH-V0-001: the shipped graph is valid; malformed graphs refuse.
func TestGraphContract(t *testing.T) {
	g := loadGraph(t)
	if g.Step("authoring").Attestation != "required" {
		t.Fatal("authoring attestation")
	}
	if got := g.Step("delta").PendingCommands; len(got) != 1 || got[0] != "corvint delta" {
		t.Fatalf("delta pending commands %v", got)
	}
	original, err := os.ReadFile(filepath.Join(hostDir, "workflow-graph.json"))
	if err != nil {
		t.Fatal(err)
	}
	for name, edit := range map[string][2]string{
		"authoring write class": {`"credentials": ["agent-model"], "commands": ["corvint step env-check"`, `"credentials": ["agent-model", "forge-write"], "commands": ["corvint step env-check"`},
		"unknown field":         {`"stepMarker"`, `"extra": 1, "stepMarker"`},
		"forward dependency":    {`"id": "intake", "order": 2, "after": ["trigger"]`, `"id": "intake", "order": 2, "after": ["delta"]`},
		"unsorted list":         {`["corvint-intake reader-check", "corvint-intake validate"`, `["corvint-intake validate", "corvint-intake reader-check"`},
		"undocumented shape":    {`"corvint-postmerge-metrics report"`, `"corvint-postmerge-metrics report --all"`},
		"overlapping prefix":    {`"CORVINT_PM_FORGE_WRITE_"`, `"CORVINT_PM_FORGE_"`},
		"attestation dropped":   {`"attestation": "required"`, `"attestation": "none"`},
	} {
		mutated := strings.Replace(string(original), edit[0], edit[1], 1)
		if mutated == string(original) {
			t.Fatalf("%s: mutation did not apply", name)
		}
		if _, err := ParseGraph([]byte(mutated)); err == nil {
			t.Errorf("%s: graph accepted", name)
		}
	}
}

// PCH-V0-003, -004, -005, -006, -007: every reference template audits clean.
func TestReferenceTemplatesAuditClean(t *testing.T) {
	g := loadGraph(t)
	for _, name := range templates {
		if findings := Audit(g, name, []byte(template(t, name))); len(findings) != 0 {
			t.Errorf("%s: %v", name, findings)
		}
	}
}

// PCH-V0-007: the authoring job's environment holds no write credential, and
// it is isolated from every job that performs any other step.
func TestAuthoringEnvironmentHasNoWriteCredential(t *testing.T) {
	g := loadGraph(t)
	root, err := ParseYAML([]byte(template(t, "postmerge.yml")))
	if err != nil {
		t.Fatal(err)
	}
	jobs := root.Get("jobs")
	authoring := 0
	for _, id := range jobs.Keys {
		job := jobs.Map[id]
		if scalar(job.Get("env").Get(g.StepMarker)) != "authoring" {
			continue
		}
		authoring++
		a := &auditor{g: g}
		if held := a.permissions(id, job.Get("permissions"), false); len(held) != 0 {
			t.Errorf("authoring token holds %v", held)
		}
		for _, s := range scalars(job) {
			for _, ref := range a.credentialRefs(s) {
				class, ok := g.SecretClass(strings.TrimPrefix(ref, "secrets."))
				if !ok || g.Class(class).OutwardWrite {
					t.Errorf("authoring references %s", ref)
				}
			}
		}
	}
	if authoring != 1 {
		t.Fatalf("authoring jobs = %d", authoring)
	}
}

func TestAuditRefusesUnsafeTemplates(t *testing.T) {
	g := loadGraph(t)
	pipeline := template(t, "postmerge.yml")
	trigger := template(t, "source-trigger.yml")
	authorEnv := "      CORVINT_PM_STEP: authoring\n"
	for _, tc := range []struct {
		name, base, old, new, code string
	}{
		{"write secret in authoring", pipeline, authorEnv, authorEnv + "      TOKEN: ${{ secrets.CORVINT_PM_FORGE_WRITE_TOKEN }}\n", "AUTHORING_WRITE_CREDENTIAL"},
		{"tracker write secret in authoring", pipeline, authorEnv, authorEnv + "      TOKEN: ${{ secrets['CORVINT_PM_TRACKER_WRITE_TOKEN'] }}\n", "UNCLASSIFIED_SECRET"},
		{"github token in authoring", pipeline, authorEnv, authorEnv + "      GH_TOKEN: ${{ github.token }}\n", "AUTHORING_WRITE_CREDENTIAL"},
		{"write permission in authoring", pipeline, "    permissions:\n      contents: read\n    env:\n" + authorEnv, "    permissions:\n      contents: write\n    env:\n" + authorEnv, "AUTHORING_WRITE_CREDENTIAL"},
		{"authoring merged with trusted steps", pipeline, authorEnv, "      CORVINT_PM_STEP: authoring,findings\n", "AUTHORING_NOT_ISOLATED"},
		{"unclassified secret", pipeline, authorEnv, authorEnv + "      KEY: ${{ secrets.OPENAI_KEY }}\n", "UNCLASSIFIED_SECRET"},
		{"workflow-level secret", pipeline, "  CORVINT_PM_WORKFLOW: pipeline\n", "  CORVINT_PM_WORKFLOW: pipeline\n  KEY: ${{ secrets.CORVINT_PM_AGENT_MODEL_KEY }}\n", "WORKFLOW_LEVEL_SECRET"},
		{"workflow write permission", pipeline, "permissions: {}\n", "permissions:\n  contents: write\n", "WORKFLOW_WRITE_PERMISSION"},
		{"write-all", pipeline, "permissions: {}\n", "permissions: write-all\n", "WRITE_PERMISSION"},
		{"missing job permissions", pipeline, "    timeout-minutes: 5\n    permissions: {}\n    env:\n      CORVINT_PM_STEP: delta\n", "    timeout-minutes: 5\n    env:\n      CORVINT_PM_STEP: delta\n", "MISSING_PERMISSIONS"},
		{"unpinned action", pipeline, "actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e", "actions/setup-go@v7", "UNPINNED_ACTION"},
		{"persisted checkout credentials", pipeline, "          path: config\n          persist-credentials: false\n", "          path: config\n", "CHECKOUT_PERSISTS_CREDENTIALS"},
		{"expression in run", pipeline, `echo "delta NOT_PRODUCED corvint-delta-not-yet-published"`, `echo "${{ inputs.change }}"`, "RUN_EXPRESSION_INTERPOLATION"},
		{"pending command", pipeline, `echo "delta NOT_PRODUCED corvint-delta-not-yet-published"`, `config/postmerge/install-pinned.sh corvint && corvint delta --base x`, "UNDOCUMENTED_COMMAND"},
		{"command outside step", pipeline, "corvint-postmerge-connect plan --experimental", "corvint-intake validate --experimental", "UNDOCUMENTED_COMMAND"},
		{"binary before install", pipeline, "        run: config/postmerge/install-pinned.sh corvint\n", "        run: echo skipped\n", "UNPINNED_BINARY"},
		{"missing step marker", pipeline, "      CORVINT_PM_STEP: delta\n", "      STEP: delta\n", "MISSING_STEP_MARKER"},
		{"order violation", pipeline, "    needs: [resolve, intake, delta]\n", "    needs: [resolve, intake]\n", "ORDER_VIOLATION"},
		{"unknown need", pipeline, "    needs: [resolve, intake, delta]\n", "    needs: [resolve, intake, delta, lint]\n", "UNKNOWN_NEED"},
		{"reusable workflow", pipeline, "    runs-on: ubuntu-latest\n    timeout-minutes: 5\n    permissions: {}\n    env:\n      CORVINT_PM_STEP: delta\n", "    uses: org/repo/.github/workflows/x.yml@main\n    permissions: {}\n    env:\n      CORVINT_PM_STEP: delta\n", "UNMODELLED_KEY"},
		{"cancelling concurrency", pipeline, "  cancel-in-progress: false\n", "  cancel-in-progress: true\n", "NO_CHANGE_CONCURRENCY"},
		{"live mode option", pipeline, "        options: [dry-run, recording]\n", "        options: [dry-run, recording, live]\n", "MODE_INPUT"},
		{"optional change", pipeline, "        required: true\n        type: string\n", "        required: false\n        type: string\n", "NO_REPLAY_BY_CHANGE"},
		{"missing role", pipeline, "  CORVINT_PM_WORKFLOW: pipeline\n", "", "MISSING_WORKFLOW_ROLE"},
		{"change-request trigger", trigger, "on:\n  push:\n", "on:\n  pull_request_target:\n  push:\n", "CHANGE_REQUEST_TRIGGER"},
		{"pre-merge trigger only", trigger, "on:\n  push:\n    branches: [main]\n", "on:\n  workflow_dispatch: {}\n", "SOURCE_TRIGGER_NOT_POST_MERGE"},
		{"blocking trigger", trigger, "    continue-on-error: true\n", "", "SOURCE_TRIGGER_CAN_FAIL"},
		{"unbounded trigger", trigger, "    timeout-minutes: 5\n", "    timeout-minutes: 360\n", "SOURCE_TRIGGER_UNBOUNDED"},
		{"trigger doing work", trigger, "      CORVINT_PM_STEP: trigger\n", "      CORVINT_PM_STEP: trigger,intake\n", "SOURCE_TRIGGER_STEP"},
		{"anchor", pipeline, "permissions: {}\n", "permissions: &p {}\n", "UNSUPPORTED_YAML"},
		{"duplicate key", pipeline, "permissions: {}\n", "permissions: {}\npermissions: {}\n", "UNSUPPORTED_YAML"},
		{"tab", pipeline, "permissions: {}\n", "permissions:\t{}\n", "UNSUPPORTED_YAML"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mutated := strings.Replace(tc.base, tc.old, tc.new, 1)
			if mutated == tc.base {
				t.Fatal("mutation did not apply")
			}
			findings := Audit(g, tc.name, []byte(mutated))
			for _, f := range findings {
				if f.Code == tc.code {
					return
				}
			}
			t.Fatalf("want %s, got %v", tc.code, findings)
		})
	}
}

func TestParseYAMLSubset(t *testing.T) {
	root, err := ParseYAML([]byte("a:\n  - b: \"x # y\"\n    c: |\n      one\n\n      two\n  - [p, 'q']\nd: plain # note\n"))
	if err != nil {
		t.Fatal(err)
	}
	items := root.Get("a").Items
	if len(items) != 2 || items[0].Get("b").Text != "x # y" || items[0].Get("c").Text != "one\n\ntwo\n" {
		t.Fatalf("items %+v", items)
	}
	if got := scalars(items[1]); len(got) != 2 || got[1] != "q" {
		t.Fatalf("flow %v", got)
	}
	if root.Get("d").Text != "plain" {
		t.Fatalf("comment %q", root.Get("d").Text)
	}
	for _, bad := range []string{"a: *x\n", "a: !!str x\n", "a: >\n  x\n", "a: {b: c}\n", "---\na: 1\n---\nb: 2\n", "a: b: c\n", "a: x\ry\n", "a: [[x]]\n"} {
		if _, err := ParseYAML([]byte(bad)); !errors.Is(err, ErrUnsupportedYAML) {
			t.Errorf("%q: err = %v", bad, err)
		}
	}
}

func TestShellCommands(t *testing.T) {
	got := shellCommands("set -eu\nX=1 corvint --root \"$R\" step verify \\\n  --before b && echo ok\nif [ -f x ]; then corvint-intake validate; fi\n")
	want := [][]string{{"set", "-eu"}, {"corvint", "--root", "$R", "step", "verify", "--before", "b"}, {"echo", "ok"}, {"[", "-f", "x", "]"}, {"corvint-intake", "validate"}, {"fi"}}
	if len(got) != len(want) {
		t.Fatalf("got %q", got)
	}
	for i := range want {
		if strings.Join(got[i], " ") != strings.Join(want[i], " ") {
			t.Fatalf("command %d: got %q want %q", i, got[i], want[i])
		}
	}
	commands := map[string]bool{"corvint step verify": true}
	if !documented("corvint", []string{"--root", "/r", "step", "verify", "--before", "b"}, commands) {
		t.Fatal("documented global-flag form")
	}
	if documented("corvint", []string{"step", "snapshot"}, commands) {
		t.Fatal("accepted undocumented subcommand")
	}
}

// PCH-V0-006: the installer is valid POSIX shell and refuses an unpinned binary.
func TestInstallPinnedSyntax(t *testing.T) {
	script := filepath.Join(hostDir, "install-pinned.sh")
	if out, err := exec.Command("sh", "-n", script).CombinedOutput(); err != nil {
		t.Fatalf("sh -n: %v %s", err, out)
	}
	data, err := os.ReadFile(script)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"sha256sum -c", "has no pin", "rev-parse HEAD", "-trimpath -buildvcs=false"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("installer lacks %q", want)
		}
	}
	cmd := exec.Command("sh", script, "corvint")
	cmd.Env = append(os.Environ(), "CORVINT_SOURCE_COMMIT=main", "CORVINT_PINS="+script)
	if out, err := cmd.CombinedOutput(); err == nil || !strings.Contains(string(out), "full 40-hex commit") {
		t.Fatalf("short commit accepted: %v %s", err, out)
	}
}

// PCH-V0-002: every hook and marker the templates rely on is documented.
func TestReadmeDocumentsTemplateHooks(t *testing.T) {
	readme, err := os.ReadFile(filepath.Join(hostDir, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range templates {
		for _, hook := range []string{"export-context.sh", "reader.sh", "step-host.sh", "author.sh", "validate.sh"} {
			if strings.Contains(template(t, name), "hooks/"+hook) && !strings.Contains(string(readme), "`"+hook) {
				t.Errorf("%s uses undocumented hook %s", name, hook)
			}
		}
	}
	for _, want := range []string{"CORVINT_PM_STEP", "CORVINT_PM_WORKFLOW", "persist-credentials: false", "Porting to another host", "NOT_RUN"} {
		if !strings.Contains(string(readme), want) {
			t.Errorf("README lacks %q", want)
		}
	}
}
