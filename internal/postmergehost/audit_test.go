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
	reconcile := template(t, "reconcile.yml")
	changeCase := "          case \"${#REQUESTED_CHANGE}\" in\n"
	replayCase := "            case \"${#change}\" in\n"
	authorEnv := "      CORVINT_PM_STEP: authoring\n"
	stepEnv := "        env:\n          CORVINT_PM_AGENT_MODEL_KEY: ${{ secrets.CORVINT_PM_AGENT_MODEL_KEY }}\n"
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
		{"local action with commit suffix", pipeline, "actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e", "./config/evil@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e", "UNPINNED_ACTION"},
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
		{"line-oriented change check", pipeline, changeCase, "          printf '%s' \"$REQUESTED_CHANGE\" | grep -Eqx '[0-9a-f]{40}|[0-9a-f]{64}'\n" + changeCase, "LINE_ORIENTED_VALIDATION"},
		{"here-string replay check", reconcile, replayCase, "            grep -Eqx '[0-9a-f]{40}' <<< \"$change\"\n" + replayCase, "LINE_ORIENTED_VALIDATION"},
		{"custom step shell", pipeline, "      - name: Record the pending delta step\n", "      - name: Record the pending delta step\n        shell: python {0}\n", "UNMODELLED_KEY"},
		{"expression in with.script", pipeline, "          go-version: \"1.27.1\"\n", "          go-version: \"1.27.1\"\n          script: ${{ inputs.change }}\n", "RUN_EXPRESSION_INTERPOLATION"},
		{"workflow default shell", pipeline, "permissions: {}\n", "permissions: {}\ndefaults:\n  run:\n    shell: python {0}\n", "CUSTOM_SHELL"},
		{"job default shell", pipeline, "    timeout-minutes: 5\n    permissions: {}\n    env:\n      CORVINT_PM_STEP: delta\n", "    timeout-minutes: 5\n    defaults:\n      run:\n        shell: python {0}\n    permissions: {}\n    env:\n      CORVINT_PM_STEP: delta\n", "CUSTOM_SHELL"},
		{"unmodelled defaults key", pipeline, "permissions: {}\n", "permissions: {}\ndefaults:\n  run:\n    working-directory: config\n  other: x\n", "UNMODELLED_KEY"},
		{"empty trigger branches", trigger, "    branches: [main]\n", "    branches: []\n", "SOURCE_TRIGGER_BRANCHES"},
		{"wildcard trigger branch", trigger, "    branches: [main]\n", "    branches: [\"**\"]\n", "SOURCE_TRIGGER_BRANCHES"},
		{"tag-only push trigger", trigger, "    branches: [main]\n", "    tags: [v1]\n", "SOURCE_TRIGGER_BRANCHES"},
		{"path-filtered push trigger", trigger, "    branches: [main]\n", "    branches: [main]\n    paths: [src]\n", "SOURCE_TRIGGER_BRANCHES"},
		{"capitalised with.Script expression", pipeline, "          go-version: \"1.27.1\"\n", "          go-version: \"1.27.1\"\n          Script: ${{ github.event.client_payload.change }}\n", "RUN_EXPRESSION_INTERPOLATION"},
		{"non-lowercase with key", pipeline, "          go-version: \"1.27.1\"\n", "          Go-Version: \"1.27.1\"\n", "NON_LOWERCASE_INPUT"},
		{"unmodelled defaults.run key", pipeline, "permissions: {}\n", "permissions: {}\ndefaults:\n  run:\n    working-directory: config\n    other: x\n", "UNMODELLED_KEY"},
		{"expression in defaults working-directory", pipeline, "permissions: {}\n", "permissions: {}\ndefaults:\n  run:\n    working-directory: ${{ inputs.change }}\n", "WORKING_DIRECTORY"},
		{"expression in step working-directory", pipeline, "      - name: Record the pending delta step\n", "      - name: Record the pending delta step\n        working-directory: ${{ inputs.change }}\n", "WORKING_DIRECTORY"},
		{"non-scalar step working-directory", pipeline, "      - name: Record the pending delta step\n", "      - name: Record the pending delta step\n        working-directory: [a, b]\n", "WORKING_DIRECTORY"},
		{"workflow BASH_ENV", pipeline, "  CORVINT_PM_WORKFLOW: pipeline\n", "  CORVINT_PM_WORKFLOW: pipeline\n  BASH_ENV: config/x.sh\n", "STARTUP_ENV"},
		{"job LD_PRELOAD", pipeline, authorEnv, authorEnv + "      LD_PRELOAD: /tmp/x.so\n", "STARTUP_ENV"},
		{"step ENV", pipeline, stepEnv, stepEnv + "          ENV: config/x.sh\n", "STARTUP_ENV"},
		{"non-mapping step env", pipeline, stepEnv, "        env: ${{ fromJSON(inputs.change) }}\n", "UNMODELLED_KEY"},
		{"script writes GITHUB_ENV", pipeline, `echo "delta NOT_PRODUCED corvint-delta-not-yet-published"`, `echo "BASH_ENV=x" >> "$GITHUB_ENV"`, "RUNNER_ENV_FILE"},
		{"script writes GITHUB_PATH", pipeline, `echo "delta NOT_PRODUCED corvint-delta-not-yet-published"`, `echo "$RUNNER_TEMP" >> "$GITHUB_PATH"`, "RUNNER_ENV_FILE"},
		{"expression as step with", pipeline, "        with:\n          go-version: \"1.27.1\"\n          cache: false\n", "        with: ${{ fromJSON(inputs.change) }}\n", "UNMODELLED_KEY"},
		{"sequence as step with", pipeline, "        with:\n          go-version: \"1.27.1\"\n          cache: false\n", "        with: [a]\n", "UNMODELLED_KEY"},
		{"dotless-i with key", pipeline, "          go-version: \"1.27.1\"\n", "          go-version: \"1.27.1\"\n          scr\u0131pt: echo hi\n", "NON_LOWERCASE_INPUT"},
		{"lowercase step ld_preload", pipeline, stepEnv, stepEnv + "          ld_preload: /tmp/x.so\n", "STARTUP_ENV"},
		{"step HOME", pipeline, stepEnv, stepEnv + "          HOME: config\n", "STARTUP_ENV"},
		{"step CC", pipeline, stepEnv, stepEnv + "          CC: config/cc.sh\n", "STARTUP_ENV"},
		{"step GOTOOLCHAIN", pipeline, stepEnv, stepEnv + "          GOTOOLCHAIN: go1.99.0\n", "STARTUP_ENV"},
		{"step JAVA_TOOL_OPTIONS", pipeline, stepEnv, stepEnv + "          JAVA_TOOL_OPTIONS: -javaagent:x.jar\n", "STARTUP_ENV"},
		{"step GIT_CONFIG_GLOBAL", pipeline, stepEnv, stepEnv + "          GIT_CONFIG_GLOBAL: config/gitconfig\n", "STARTUP_ENV"},
		{"workflow ACTIONS_ALLOW_UNSECURE_COMMANDS", pipeline, "  CORVINT_PM_WORKFLOW: pipeline\n", "  CORVINT_PM_WORKFLOW: pipeline\n  ACTIONS_ALLOW_UNSECURE_COMMANDS: \"true\"\n", "STARTUP_ENV"},
		{"step ACTIONS_ALLOW_USE_UNSECURE_NODE_VERSION", pipeline, stepEnv, stepEnv + "          ACTIONS_ALLOW_USE_UNSECURE_NODE_VERSION: \"true\"\n", "STARTUP_ENV"},
		{"job FORCE_JAVASCRIPT_ACTIONS_TO_NODE24", pipeline, authorEnv, authorEnv + "      FORCE_JAVASCRIPT_ACTIONS_TO_NODE24: \"true\"\n", "STARTUP_ENV"},
		{"script prints ::set-env", pipeline, `echo "delta NOT_PRODUCED corvint-delta-not-yet-published"`, `echo "::set-env name=BASH_ENV::x"`, "RUNNER_ENV_FILE"},
		{"script prints ::add-path", pipeline, `echo "delta NOT_PRODUCED corvint-delta-not-yet-published"`, `echo "::add-path::/tmp/x"`, "RUNNER_ENV_FILE"},
		{"script prints ::SET-ENV", pipeline, `echo "delta NOT_PRODUCED corvint-delta-not-yet-published"`, `echo "::SET-ENV name=BASH_ENV::x"`, "RUNNER_ENV_FILE"},
		{"script prints ::ADD-PATH", pipeline, `echo "delta NOT_PRODUCED corvint-delta-not-yet-published"`, `echo "::ADD-PATH::/tmp/x"`, "RUNNER_ENV_FILE"},
		{"script prints legacy set-env", pipeline, `echo "delta NOT_PRODUCED corvint-delta-not-yet-published"`, `echo "##[set-env name=BASH_ENV;]x"`, "RUNNER_ENV_FILE"},
		{"script prints legacy add-path", pipeline, `echo "delta NOT_PRODUCED corvint-delta-not-yet-published"`, `echo "##[add-path]/tmp/x"`, "RUNNER_ENV_FILE"},
		{"script prints legacy SET-ENV", pipeline, `echo "delta NOT_PRODUCED corvint-delta-not-yet-published"`, `echo "##[SET-ENV name=BASH_ENV;]x"`, "RUNNER_ENV_FILE"},
		{"script prints legacy ADD-PATH", pipeline, `echo "delta NOT_PRODUCED corvint-delta-not-yet-published"`, `echo "##[ADD-PATH]/tmp/x"`, "RUNNER_ENV_FILE"},
		{"workflow env merge key", pipeline, "  CORVINT_PM_WORKFLOW: pipeline\n", "  CORVINT_PM_WORKFLOW: pipeline\n  <<: x\n", "UNMODELLED_KEY"},
		{"job env merge key", pipeline, authorEnv, authorEnv + "      <<: x\n", "UNMODELLED_KEY"},
		{"step env merge key", pipeline, stepEnv, stepEnv + "          <<: x\n", "UNMODELLED_KEY"},
		{"step env merge key hiding LD_PRELOAD", pipeline, stepEnv, stepEnv + "          <<:\n            LD_PRELOAD: /tmp/x.so\n", "UNMODELLED_KEY"},
		{"workflow env mapping value", pipeline, "  CORVINT_PM_WORKFLOW: pipeline\n", "  CORVINT_PM_WORKFLOW: pipeline\n  X:\n    LD_PRELOAD: /tmp/x.so\n", "UNMODELLED_KEY"},
		{"job env sequence value", pipeline, authorEnv, authorEnv + "      X: [a]\n", "UNMODELLED_KEY"},
		{"step env mapping value", pipeline, stepEnv, stepEnv + "          X:\n            LD_PRELOAD: /tmp/x.so\n", "UNMODELLED_KEY"},
		{"non-scalar with value", pipeline, "          cache: false\n", "          cache: [false]\n", "UNMODELLED_KEY"},
		{"non-scalar run", pipeline, "        run: config/postmerge/install-pinned.sh corvint\n", "        run: [config/postmerge/install-pinned.sh corvint]\n", "UNMODELLED_KEY"},
		{"capitalised checkout persists credentials", pipeline, "        uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0 # v7.0.0\n        with:\n          path: config\n          persist-credentials: false\n", "        uses: Actions/Checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0 # v7.0.0\n        with:\n          path: config\n", "CHECKOUT_PERSISTS_CREDENTIALS"},
		{"sub-path checkout persists credentials", pipeline, "        uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0 # v7.0.0\n        with:\n          path: config\n          persist-credentials: false\n", "        uses: actions/checkout/.@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0 # v7.0.0\n        with:\n          path: config\n", "CHECKOUT_PERSISTS_CREDENTIALS"},
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
	hex40 := strings.Repeat("a", 40)
	for _, commit := range []string{"main", hex40 + "\nmain", hex40 + "\n", strings.Repeat("A", 40), hex40 + "0"} {
		cmd := exec.Command("sh", script, "corvint")
		cmd.Env = append(os.Environ(), "CORVINT_SOURCE_COMMIT="+commit, "CORVINT_PINS="+script)
		if out, err := cmd.CombinedOutput(); err == nil || !strings.Contains(string(out), "full 40-hex commit") {
			t.Fatalf("commit %q accepted: %v %s", commit, err, out)
		}
	}
	// Names are checked before any fetch, so these refusals need no network.
	for _, name := range []string{"corvint-../x", "corvint-a/b", "corvint-x.y", "corvint-", "corvintx", "corvint-a\nb"} {
		cmd := exec.Command("sh", script, name)
		cmd.Env = append(os.Environ(), "CORVINT_SOURCE_COMMIT="+hex40, "CORVINT_PINS="+script, "RUNNER_TEMP="+t.TempDir())
		if out, err := cmd.CombinedOutput(); err == nil || !strings.Contains(string(out), "is not a Corvint command") {
			t.Fatalf("name %q accepted: %v %s", name, err, out)
		}
	}
}

// runStep runs one template step's script the way the hosted runner does
// (bash -e) and under POSIX sh, calling check after each run.
func runStep(t *testing.T, name, job string, step int, check func(shell string, err error), env ...string) {
	t.Helper()
	root, err := ParseYAML([]byte(template(t, name)))
	if err != nil {
		t.Fatal(err)
	}
	steps := root.Get("jobs").Get(job).Get("steps")
	if steps == nil || len(steps.Items) <= step {
		t.Fatalf("%s: no step %d in job %s", name, step, job)
	}
	script := steps.Items[step].Get("run").Text
	for _, shell := range [][]string{{"bash", "--noprofile", "--norc", "-e", "-c"}, {"sh", "-e", "-c"}} {
		if _, err := exec.LookPath(shell[0]); err != nil {
			t.Fatalf("%s: %v", shell[0], err)
		}
		cmd := exec.Command(shell[0], append(shell[1:], script)...)
		cmd.Env = append(os.Environ(), env...)
		check(shell[0], cmd.Run())
	}
}

// PCH-V0-006: the pipeline accepts a dispatched change id only as one whole
// 40- or 64-hex string, so a newline cannot inject a second output line.
func TestResolveRefusesInjectedChange(t *testing.T) {
	hex40 := strings.Repeat("0123456789abcdef", 3)[:40]
	hex64 := strings.Repeat("0123456789abcdef", 4)
	for _, tc := range []struct {
		change, mode string
		ok           bool
	}{
		{hex40, "dry-run", true},
		{hex64, "recording", true},
		{hex40 + "\nchange=refs/pull/1/head", "dry-run", false},
		{"refs/pull/1/head\n" + hex40, "dry-run", false},
		{hex40 + "\n", "dry-run", false},
		{hex40[:39] + "\n", "dry-run", false},
		{strings.ToUpper(hex40), "dry-run", false},
		{hex40[:39] + "g", "dry-run", false},
		{hex40[:39], "dry-run", false},
		{"", "dry-run", false},
		{hex40, "dry-run\nchange=refs/pull/1/head", false},
	} {
		dir := t.TempDir()
		output := filepath.Join(dir, "output")
		runStep(t, "postmerge.yml", "resolve", 0, func(shell string, err error) {
			data, _ := os.ReadFile(output)
			_ = os.Remove(output)
			want := ""
			if tc.ok {
				want = "change=" + tc.change + "\nmode=" + tc.mode + "\n"
			}
			if (err == nil) != tc.ok || string(data) != want {
				t.Errorf("%s change %q mode %q: err %v output %q", shell, tc.change, tc.mode, err, data)
			}
		}, "REQUESTED_CHANGE="+tc.change, "REQUESTED_MODE="+tc.mode, "GITHUB_OUTPUT="+output)
	}
}

// PCH-V0-005: a replay line is dispatched only when it is one whole change id.
func TestReplayRefusesMalformedChange(t *testing.T) {
	hex40 := strings.Repeat("ab", 20)
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte("#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$GH_LOG\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		set   string
		calls int
		ok    bool
	}{
		{"# replay\n" + hex40 + "\n\n" + hex40 + "\n", 2, true},
		{hex40 + " change=refs/pull/1/head\n", 0, false},
		{hex40 + "0\n", 0, false},
		{hex40 + "\n" + hex40, 2, true},
	} {
		dir := t.TempDir()
		set, log := filepath.Join(dir, "set"), filepath.Join(dir, "gh.log")
		if err := os.WriteFile(set, []byte(tc.set), 0o600); err != nil {
			t.Fatal(err)
		}
		runStep(t, "reconcile.yml", "dispatch", 1, func(shell string, err error) {
			data, _ := os.ReadFile(log)
			_ = os.Remove(log)
			calls := strings.Count(string(data), "-f change="+hex40+" -f mode=dry-run")
			if (err == nil) != tc.ok || calls != tc.calls || strings.Count(string(data), "\n") != tc.calls {
				t.Errorf("%s set %q: err %v calls %q", shell, tc.set, err, data)
			}
		}, "REPLAY_SET="+set, "MODE=dry-run", "RUNNER_TEMP="+dir, "GH_LOG="+log, "GITHUB_REPOSITORY=o/r",
			"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
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
