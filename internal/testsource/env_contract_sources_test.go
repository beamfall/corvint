package testsource

import (
	"context"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func envVariableIn(t *testing.T, report EnvContractReport, name, service string) EnvVariable {
	t.Helper()
	for _, v := range report.Variables {
		if v.Name == name && v.Service == service {
			return v
		}
	}
	t.Fatalf("variable %s (service %q) missing: %+v", name, service, report.Variables)
	return EnvVariable{}
}

func sourcesFixture() map[string]string {
	return map[string]string{
		"scripts/bootstrap-ci.sh": `#!/usr/bin/env bash
set -a
. ./.env
set +a
PORT=${PORT:-3000}
docker compose -f e2e/stack.yml up -d
npx playwright test
`,
		".env": "API_URL=http://localhost:4100/api\n",
		"e2e/stack.yml": `services:
  e2e:
    image: mcr.microsoft.com/playwright:v1.48.0
    command: npx playwright test
    env_file: ../config/e2e.vars
    environment:
      BASE_URL: http://app:${PORT}
      PORT:
  db:
    image: postgres:16
    environment:
      - POSTGRES_DB=app
`,
		"config/e2e.vars": "# e2e settings\nRETRIES=2\n",
		".github/workflows/e2e.yml": `name: e2e
on: push
env:
  NODE_ENV: test
jobs:
  e2e:
    runs-on: ubuntu-latest
    env:
      PORT: "4100"
    steps:
      - uses: actions/checkout@v4
      - name: run
        run: scripts/bootstrap-ci.sh
        env:
          SECRET: ${{ secrets.TOKEN }}
`,
	}
}

// TestEnvContractSources: TSA-V0-350..352, TSA-V0-354 — a sourced dotenv file, the workflow env of
// the step that runs the profile script and a launched compose file's environment and env_file
// each contribute variables with their source cited; the env_file outside the first read's
// filter is read in a further round.
func TestEnvContractSources(t *testing.T) {
	root := t.TempDir()
	commitFiles(t, root, sourcesFixture())
	report, err := EnvContract(OpenRepo(context.Background(), root), EnvContractOptions{Profile: "ci", Revision: "HEAD"})
	if err != nil {
		t.Fatal(err)
	}
	api := envVariable(t, report, "API_URL")
	if api.SourceKind != SourceDotenv || api.Source != ".env" || api.Line != 1 || !api.PassedToTests || valueOf(api) != "http://localhost:4100/api" {
		t.Fatalf("API_URL = %+v", api)
	}
	node := envVariable(t, report, "NODE_ENV")
	if node.SourceKind != SourceWorkflow || node.Source != ".github/workflows/e2e.yml" || node.Line != 4 || !node.PassedToTests || valueOf(node) != "test" {
		t.Fatalf("NODE_ENV = %+v", node)
	}
	port := envVariableIn(t, report, "PORT", "")
	if port.Source != "scripts/bootstrap-ci.sh" || port.Line != 5 || valueOf(port) != "4100" || port.Derivation.Kind != DerivationDefaulted {
		t.Fatalf("PORT = %+v", port)
	}
	secret := envVariable(t, report, "SECRET")
	if secret.Value != nil || secret.UnknownReason != envWorkflowExpression || secret.SourceKind != SourceWorkflow {
		t.Fatalf("SECRET = %+v", secret)
	}
	retries := envVariableIn(t, report, "RETRIES", "e2e")
	if retries.SourceKind != SourceComposeEnvFile || retries.Source != "config/e2e.vars" || retries.Line != 2 || !retries.PassedToTests || valueOf(retries) != "2" {
		t.Fatalf("RETRIES = %+v", retries)
	}
	base := envVariableIn(t, report, "BASE_URL", "e2e")
	if base.SourceKind != SourceComposeEnvironment || base.Source != "e2e/stack.yml" || base.Line != 7 || valueOf(base) != "http://app:4100" || !base.Endpoint {
		t.Fatalf("BASE_URL = %+v", base)
	}
	if passthrough := envVariableIn(t, report, "PORT", "e2e"); valueOf(passthrough) != "4100" || passthrough.Line != 8 {
		t.Fatalf("compose PORT = %+v", passthrough)
	}
	if db := envVariableIn(t, report, "POSTGRES_DB", "db"); db.PassedToTests || db.Reliable || valueOf(db) != "app" || db.Line != 12 {
		t.Fatalf("POSTGRES_DB = %+v", db)
	}
	if len(report.UnreadSources) != 0 {
		t.Fatalf("unread = %+v", report.UnreadSources)
	}
}

// TestEnvContractSourceNotes: TSA-V0-353, TSA-V0-354 — the profile's own compose.NAME file stays
// unread even when launched, a missing or unparseable file is noted rather than guessed, and
// callers that set a variable differently make it caller-dependent.
func TestEnvContractSourceNotes(t *testing.T) {
	files := sourcesFixture()
	files["scripts/bootstrap-ci.sh"] = strings.Replace(files["scripts/bootstrap-ci.sh"], "-f e2e/stack.yml", "-f docker-compose.ci.yml -f e2e/missing.yml -f e2e/bad.yml", 1)
	files["docker-compose.ci.yml"] = "services:\n  e2e:\n    environment:\n      HIDDEN: x\n"
	files["e2e/bad.yml"] = "base: &base\n  a: 1\nservices:\n  e2e: *base\n"
	files[".github/workflows/anchors.yml"] = "env: &shared\n  X: 1\njobs: {}\n# scripts/bootstrap-ci.sh\n"
	files[".github/workflows/nightly.yml"] = "env:\n  NODE_ENV: production\njobs:\n  n:\n    steps:\n      - run: ./scripts/bootstrap-ci.sh\n"
	files[".github/workflows/unrelated.yml"] = "env:\n  NODE_ENV: other\njobs:\n  n:\n    steps:\n      - run: make lint\n"
	report := ReadEnvContract(files, "ci")
	got := []string{}
	for _, n := range report.UnreadSources {
		got = append(got, n.Path+"="+n.Reason)
	}
	want := ".github/workflows/anchors.yml=" + envWorkflowUnparsed + " docker-compose.ci.yml=" + envComposeNotRead +
		" e2e/bad.yml=" + envComposeUnparsed + " e2e/missing.yml=" + envNotCommitted
	if strings.Join(got, " ") != want {
		t.Fatalf("unread = %v\nwant %s", got, want)
	}
	for _, v := range report.Variables {
		if v.Name == "HIDDEN" {
			t.Fatalf("profile compose file read: %+v", v)
		}
	}
	node := envVariable(t, report, "NODE_ENV")
	if node.Value != nil || node.UnknownReason != envCallerValuesDiffer || strings.Join(node.Derivation.Alternatives, ",") != "test,production" {
		t.Fatalf("NODE_ENV = %+v", node)
	}
	// Set by one caller only: the other caller leaves it unset.
	if port := envVariableIn(t, report, "PORT", ""); port.Value != nil || port.UnknownReason != envCallerValuesDiffer {
		t.Fatalf("PORT = %+v", port)
	}
	// Without a calling workflow the self-default stays unknown, as before (TSA-V0-216).
	delete(files, ".github/workflows/e2e.yml")
	delete(files, ".github/workflows/nightly.yml")
	if port := envVariableIn(t, ReadEnvContract(files, "ci"), "PORT", ""); port.Value != nil || port.UnknownReason != "reference-cycle" {
		t.Fatalf("default PORT = %+v", port)
	}
}

const branchScript = `#!/usr/bin/env bash
PORT=3000
if [ -n "$CI" ]; then
  PORT=4000
elif [ -n "$STAGING" ]; then
  PORT=5000
fi
[ -n "$CI" ] && HOST=ci-host
case "$MODE" in
  fast) RETRIES=0 ;;
  *) RETRIES=3 ;;
esac
setup() {
  TOKEN=abc
}
DONE=x
[ -n "$CI" ] && DONE=y
DONE=z
{ GROUPED=1; }
export APP_URL="http://localhost:${PORT}"
npx playwright test
`

// TestEnvContractBranchDependent: TSA-V0-361 — an assignment under if/elif, after && or ||, in a
// case arm or in a function is branch-dependent: UNKNOWN with every value it may take; a later
// unconditional assignment settles it and a { } group is not a branch.
func TestEnvContractBranchDependent(t *testing.T) {
	report := ReadEnvContract(map[string]string{"scripts/bootstrap-e2e.sh": branchScript}, "e2e")
	for name, want := range map[string]string{"PORT": "3000,4000,5000", "HOST": "ci-host", "RETRIES": "0,3", "TOKEN": "abc"} {
		v := envVariable(t, report, name)
		if v.Value != nil || v.UnknownReason != envBranchDependent || v.Derivation.Kind != DerivationUnknown || strings.Join(v.Derivation.Alternatives, ",") != want {
			t.Fatalf("%s = %+v", name, v)
		}
	}
	for name, want := range map[string]string{"DONE": "z", "GROUPED": "1"} {
		if v := envVariable(t, report, name); valueOf(v) != want || v.Derivation.Kind != DerivationLiteral || len(v.Derivation.Alternatives) != 0 {
			t.Fatalf("%s = %+v", name, v)
		}
	}
	if app := envVariable(t, report, "APP_URL"); app.Value != nil || app.UnknownReason != envBranchDependent {
		t.Fatalf("APP_URL = %+v", app)
	}
	// A sourced dotenv file inside a branch is branch-dependent too.
	sourced := ReadEnvContract(map[string]string{"scripts/bootstrap-e2e.sh": "if [ -f .env.ci ]; then . ./.env.ci; fi\nexport A\nnpx playwright test\n", ".env.ci": "A=1\n"}, "e2e")
	if a := envVariable(t, sourced, "A"); a.Value != nil || a.UnknownReason != envBranchDependent || a.SourceKind != SourceDotenv {
		t.Fatalf("sourced A = %+v", a)
	}
}

const handlerStub = `const express = require('express');
const app = express();
app.post('/__control/faults', (req, res) => {
  faults[req.body.testId] = req.body.status;
  res.sendStatus(204);
});
function audit(req) {
  return req.body.auditField;
}
app.get('/__control/requests', listRequests);
const unrelated = (req) => req.query.leaked;
function listRequests(req, res) {
  res.json(log.filter(r => r.testId === req.query.testId));
}
app.post('/__control/reset', (req, res) => res.sendStatus(204));
`

// TestEnvContractHandlerBounded: TSA-V0-360 — a route's fields come from its registration call and
// its named handler, never from unrelated code between registrations.
func TestEnvContractHandlerBounded(t *testing.T) {
	report := ReadEnvContract(map[string]string{
		"scripts/bootstrap-e2e.sh": "PORT=4110 node stubs/control/server.js &\nnpx playwright test\n",
		"stubs/control/server.js":  handlerStub,
	}, "e2e")
	if len(report.StandIns) != 1 || len(report.StandIns[0].Routes) != 3 {
		t.Fatalf("stand-ins = %+v", report.StandIns)
	}
	got := []string{}
	for _, r := range report.StandIns[0].Routes {
		got = append(got, r.Method+" "+r.Path+" p="+strings.Join(r.Payload, ",")+" q="+strings.Join(r.Query, ","))
	}
	want := "POST /__control/faults p=status,testId q= | GET /__control/requests p= q=testId | POST /__control/reset p= q="
	if strings.Join(got, " | ") != want {
		t.Fatalf("routes = %v\nwant %s", got, want)
	}
}

func languageFixture() map[string]string {
	return map[string]string{
		"scripts/bootstrap-e2e.sh": `#!/usr/bin/env bash
java -jar wiremock.jar --root-dir stubs/wiremock &
PAY_PORT=7001 go run ./stubs/payments &
flask --app stubs/flaskpay/app run &
ruby stubs/sinatra/app.rb &
PORT=7004 foreman start -d stubs/rails &
gradle -p stubs/kotlin run &
npx playwright test
`,
		"stubs/wiremock/mappings/fault.json": `{
  "mappings": [
    {
      "request": {
        "method": "POST",
        "urlPath": "/__control/faults",
        "bodyPatterns": [{"matchesJsonPath": "$.testId"}, {"matchesJsonPath": {"expression": "$.delayMs"}}]
      },
      "response": {"status": 204}
    },
    {
      "request": {"urlPathPattern": "/charges/.*", "queryParameters": {"currency": {"equalTo": "usd"}}},
      "response": {"status": 200}
    }
  ]
}
`,
		"stubs/payments/main.go": "package main\n\nimport (\n\t\"encoding/json\"\n\t\"net/http\"\n\t\"os\"\n)\n\n" +
			"func main() {\n\tmux := http.NewServeMux()\n\tmux.HandleFunc(\"POST /control/faults\", injectFault)\n\tmux.HandleFunc(\"/charges\", charge)\n" +
			"\thttp.ListenAndServe(\":\"+os.Getenv(\"PAY_PORT\"), mux)\n}\n\n" +
			"func audit(r *http.Request) string { return r.FormValue(\"auditField\") }\n\n" +
			"func injectFault(w http.ResponseWriter, r *http.Request) {\n\tvar f struct {\n\t\tTestID string `json:\"testId\"`\n\t\tStatus int    `json:\"status\"`\n\t}\n" +
			"\t_ = json.NewDecoder(r.Body).Decode(&f)\n}\n\n" +
			"func charge(w http.ResponseWriter, r *http.Request) {\n\t_ = r.URL.Query().Get(\"currency\")\n}\n",
		"stubs/payments/main_test.go": "package main\n\nfunc TestX() { mux.HandleFunc(\"/ignored\", nil) }\n",
		"stubs/flaskpay/app.py": `from flask import Flask, request

app = Flask(__name__)


@app.route("/__control/reset", methods=["POST"])
def reset():
    return "", 204


@app.post("/__control/delay")
def delay():
    data = request.get_json()
    return str(data["delayMs"]), 204


@app.route("/payments/<int:id>")
def payment(id):
    return {"id": id, "expand": request.args.get("expand")}
`,
		"stubs/sinatra/app.rb": `require 'sinatra'

post '/__control/faults' do
  code = params[:status]
  scope = params['testId']
end

get '/accounts/:id' do
  params[:verbose]
end
`,
		"stubs/rails/config/routes.rb": `Rails.application.routes.draw do
  post '/faults', to: 'faults#create'
  delete 'reset', to: 'faults#reset'
end
`,
		"stubs/rails/app/controllers/faults_controller.rb": `class FaultsController < ApplicationController
  def create
    params.require(:fault).permit(:testId, :status)
  end

  def reset
    head :no_content
  end
end
`,
		"stubs/kotlin/Server.kt": "fun main() {}\n",
	}
}

// TestEnvContractStandInLanguages: TSA-V0-355..359 — WireMock mappings and Go, Flask, Sinatra and
// Rails stand-ins launched by directory, module or file contribute routes with their fields; a
// stand-in in a language the reader does not read is a named abstention.
func TestEnvContractStandInLanguages(t *testing.T) {
	root := t.TempDir()
	commitFiles(t, root, languageFixture())
	report, err := EnvContract(OpenRepo(context.Background(), root), EnvContractOptions{Profile: "e2e", Revision: "HEAD"})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]string{}
	abstentions := map[string]string{}
	for _, s := range report.StandIns {
		for _, r := range s.Routes {
			got[s.Source] = append(got[s.Source], strings.Join([]string{r.Method, r.Path, r.Source, strconv.Itoa(r.Line), r.Kind, r.Scope,
				"p=" + strings.Join(r.Payload, ","), "q=" + strings.Join(r.Query, ","), "t=" + strings.Join(r.Teardown, ",")}, " "))
		}
		for _, a := range s.Abstentions {
			abstentions[s.Source] += a.Path + "=" + a.Reason
		}
	}
	want := map[string][]string{
		"stubs/wiremock": {
			"POST /__control/faults stubs/wiremock/mappings/fault.json 4 FAULT_INJECTION KEYED:testId p=delayMs,testId q= t=",
			"ALL /charges/.* stubs/wiremock/mappings/fault.json 12 SERVICE  p= q=currency t=",
		},
		"stubs/payments": {
			"POST /control/faults stubs/payments/main.go 11 FAULT_INJECTION KEYED:testId p=status,testId q= t=",
			"ALL /charges stubs/payments/main.go 12 SERVICE  p= q=currency t=",
		},
		"stubs/flaskpay/app.py": {
			"POST /__control/reset stubs/flaskpay/app.py 6 RESET SERVICE_WIDE p= q= t=",
			"POST /__control/delay stubs/flaskpay/app.py 11 FAULT_INJECTION SERVICE_WIDE p=delayMs q= t=POST /__control/reset",
			"GET /payments/<int:id> stubs/flaskpay/app.py 17 SERVICE  p= q=expand t=",
		},
		"stubs/sinatra/app.rb": {
			"POST /__control/faults stubs/sinatra/app.rb 3 FAULT_INJECTION KEYED:testId p=status,testId q= t=",
			"GET /accounts/:id stubs/sinatra/app.rb 8 SERVICE  p= q=verbose t=",
		},
		"stubs/rails": {
			"POST /faults stubs/rails/config/routes.rb 2 FAULT_INJECTION KEYED:testId p=status,testId q= t=DELETE /reset",
			"DELETE /reset stubs/rails/config/routes.rb 3 RESET SERVICE_WIDE p= q= t=",
		},
	}
	for source, routes := range want {
		if strings.Join(got[source], "\n") != strings.Join(routes, "\n") {
			t.Errorf("%s routes:\n%s\nwant\n%s", source, strings.Join(got[source], "\n"), strings.Join(routes, "\n"))
		}
	}
	if len(report.StandIns) != 6 || abstentions["stubs/kotlin"] != "stubs/kotlin/Server.kt="+envLanguageNotRead || len(got["stubs/kotlin"]) != 0 {
		t.Fatalf("stand-ins = %+v", report.StandIns)
	}
	for _, s := range report.StandIns {
		if s.Source == "stubs/rails" && (s.URL == nil || *s.URL != "http://localhost:7004") {
			t.Fatalf("rails URL = %+v", s)
		}
	}
}

// TestRoleLocatorPerTestMounts: TSA-V0-362 — a locator is checked against the components its own
// test mounts, not every component the file mounts; a mount outside every test counts for all.
func TestRoleLocatorPerTestMounts(t *testing.T) {
	spec := `import ErrorBanner from '../src/ErrorBanner';
import Alert from '../src/Alert';

test('banner', async ({ mount }) => {
  const c = await mount(<ErrorBanner message="boom" />);
  await expect(c.getByRole('alert')).toBeVisible();
});

test('alert', async ({ mount }) => {
  const c = await mount(<Alert />);
  await expect(c.getByRole('alert')).toBeVisible();
});
`
	files := map[string]string{"tests/two.spec.tsx": spec, "src/ErrorBanner.tsx": plainBanner,
		"src/Alert.tsx": `export default function Alert() { return <div role="alert">x</div>; }`}
	report := CheckRoleLocators(files)
	if report.Checked != 2 || len(report.Findings) != 1 || len(report.Abstentions) != 0 {
		t.Fatalf("report = %+v", report)
	}
	if f := report.Findings[0]; f.Line != 6 || strings.Join(f.Components, ",") != "src/ErrorBanner.tsx" {
		t.Fatalf("finding = %+v", f)
	}
	// A shared mount outside every test reaches each test's locators.
	shared := strings.Replace(spec, "test('banner'", "const shared = () => mount(<Alert />);\ntest('banner'", 1)
	if r := CheckRoleLocators(map[string]string{"tests/two.spec.tsx": shared, "src/ErrorBanner.tsx": plainBanner, "src/Alert.tsx": files["src/Alert.tsx"]}); len(r.Findings) != 0 {
		t.Fatalf("shared = %+v", r)
	}
}

// TestSourceValidityEnv: TSA-V0-363 — `tests validity --profile` lists the profile's warnings on
// the named files, each with the id of the test that contains it; without a profile there is no
// projection, and the projection changes no test's hygiene.
func TestSourceValidityEnv(t *testing.T) {
	root := t.TempDir()
	commitFiles(t, root, envFixture())
	repo := OpenRepo(context.Background(), root)
	plain, err := SourceValidity(repo, SourceValidityOptions{Revision: "HEAD", Files: []string{"tests/pay.spec.ts"}})
	if err != nil || plain.Env != nil {
		t.Fatalf("plain = %+v, %v", plain.Env, err)
	}
	report, err := SourceValidity(repo, SourceValidityOptions{Revision: "HEAD", Files: []string{"tests/pay.spec.ts"}, Profile: "e2e"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(report.Tests, plain.Tests) || !reflect.DeepEqual(report.Files, plain.Files) {
		t.Fatalf("projection changed hygiene: %+v vs %+v", report.Tests, plain.Tests)
	}
	if report.Env == nil || report.Env.Profile != "e2e" || len(report.Env.Warnings) != 2 {
		t.Fatalf("env = %+v", report.Env)
	}
	tests := map[string]bool{}
	for _, test := range plain.Tests {
		tests[test.Test] = true
	}
	offset, literal := report.Env.Warnings[0], report.Env.Warnings[1]
	if offset.Rule != RuleEnvHardCodedOffset || offset.Test != "" || literal.Rule != RuleEnvHardCodedAddress || literal.Test != "tests/pay.spec.ts:4" || !tests[literal.Test] {
		t.Fatalf("warnings = %+v (tests %v)", report.Env.Warnings, tests)
	}
	other, err := SourceValidity(repo, SourceValidityOptions{Revision: "HEAD", Files: []string{"stubs/payments/server.js"}, Profile: "e2e"})
	if err != nil || other.Env == nil || len(other.Env.Warnings) != 0 {
		t.Fatalf("other file = %+v, %v", other.Env, err)
	}
	missing, err := SourceValidity(repo, SourceValidityOptions{Revision: "HEAD", Files: []string{"tests/pay.spec.ts"}, Profile: "staging"})
	if err != nil || missing.Env == nil || missing.Env.Reason != envProfileNotFound {
		t.Fatalf("missing profile = %+v, %v", missing.Env, err)
	}
	if _, err := SourceValidity(repo, SourceValidityOptions{Revision: "HEAD", Files: []string{"tests/pay.spec.ts"}, Profile: "../x"}); err == nil {
		t.Fatal("invalid profile accepted")
	}
}

// TestEnvContractReaderEdges: TSA-V0-350, TSA-V0-352..354, TSA-V0-358, TSA-V0-360 and TSA-V0-361 —
// edge shapes that must fail closed or stay bounded rather than guess.
func TestEnvContractReaderEdges(t *testing.T) {
	if _, ok := readYAML("env:\n  \"\": x\n"); ok {
		t.Fatal("an empty YAML key read")
	}
	run := "jobs:\n  e:\n    steps:\n      - run: scripts/bootstrap-e2e.sh\n"
	script := "PORT=${PORT:-3000}\nnpx playwright test\n"
	// A workflow value naming the variable is a literal, and never recurses without bound.
	looped := ReadEnvContract(map[string]string{"scripts/bootstrap-e2e.sh": script,
		".github/workflows/e.yml": "env:\n  PORT: \"$PORT's\"\n" + run}, "e2e")
	if port := envVariableIn(t, looped, "PORT", ""); valueOf(port) != "$PORT's" || !port.PassedToTests {
		t.Fatalf("PORT = %+v", port)
	}
	// A caller value is evaluated once, so a self-naming caller value ends as a cycle.
	self := envResolver{assignments: map[string]shellAssignment{"PORT": {name: "PORT", raw: "${PORT:-3000}"}},
		caller: map[string]shellAssignment{"PORT": {name: "PORT", raw: "$PORT", kind: SourceWorkflow}}, memo: map[string]resolved{}}
	if got := self.resolve("PORT", map[string]bool{}); got.value != nil || got.reason != envCycle {
		t.Fatalf("self-naming caller = %+v", got)
	}
	// A list continued across lines keeps its condition.
	split := ReadEnvContract(map[string]string{"scripts/bootstrap-e2e.sh": "PORT=3000\ntest -n \"$CI\" &&\n  PORT=4000\nHOST=a\ntest -n \"$CI\" ||\n  HOST=b\nexport PORT HOST\nnpx playwright test\n"}, "e2e")
	for name, want := range map[string]string{"PORT": "3000,4000", "HOST": "a,b"} {
		if v := envVariableIn(t, split, name, ""); v.Value != nil || v.UnknownReason != envBranchDependent || strings.Join(v.Derivation.Alternatives, ",") != want {
			t.Fatalf("%s = %+v", name, v)
		}
	}
	// The profile's own dotenv stays unread when compose names it as an env_file.
	own := ReadEnvContract(map[string]string{"scripts/bootstrap-e2e.sh": "docker compose -f e2e/stack.yml up\n",
		"e2e/stack.yml": "services:\n  t:\n    command: npx playwright test\n    env_file: ../.env.e2e\n", ".env.e2e": "SECRET=1\n"}, "e2e")
	for _, v := range own.Variables {
		if v.Name == "SECRET" {
			t.Fatalf("profile dotenv read through env_file: %+v", v)
		}
	}
	// Block scalars and duplicate flow keys in an env map are outside the subset.
	for name, files := range map[string]map[string]string{
		".github/workflows/e.yml": {".github/workflows/e.yml": "env:\n  PORT: |\n    4000\n" + run},
		".github/workflows/f.yml": {".github/workflows/f.yml": "env: {PORT: 3000, PORT: 4000}\n" + run},
		"e2e/stack.yml": {"e2e/stack.yml": "services:\n  t:\n    environment:\n      PORT: >\n        4000\n",
			"scripts/bootstrap-e2e.sh": "docker compose -f e2e/stack.yml up\n"},
	} {
		if _, ok := files["scripts/bootstrap-e2e.sh"]; !ok {
			files["scripts/bootstrap-e2e.sh"] = script
		}
		report := ReadEnvContract(files, "e2e")
		notes := []string{}
		for _, n := range report.UnreadSources {
			notes = append(notes, n.Path)
		}
		if !slices.Contains(notes, name) {
			t.Fatalf("%s not unread: %+v", name, report.UnreadSources)
		}
		for _, v := range report.Variables {
			if v.Name == "PORT" && v.Value != nil && *v.Value != "3000" {
				t.Fatalf("%s guessed PORT: %+v", name, v)
			}
		}
	}
	// An expression-bodied handler never borrows the next function's body.
	arrow := ReadEnvContract(map[string]string{"scripts/bootstrap-e2e.sh": "node stubs/x/server.js &\nnpx playwright test\n",
		"stubs/x/server.js": "const h = (req, res) => res.end();\nfunction other(req) { return req.body.testId; }\napp.post('/a', h);\nconst g = async (req, res) => { res.json(req.query.q); };\napp.get('/b', g);\n"}, "e2e")
	if len(arrow.StandIns) != 1 || len(arrow.StandIns[0].Routes) != 2 || len(arrow.StandIns[0].Routes[0].Payload) != 0 || strings.Join(arrow.StandIns[0].Routes[1].Query, ",") != "q" {
		t.Fatalf("arrow stand-ins = %+v", arrow.StandIns)
	}
	// A mapping with no route still advances the WireMock citation.
	mock := ReadEnvContract(map[string]string{"scripts/bootstrap-e2e.sh": "java -jar wiremock.jar --root-dir stubs/wiremock &\nnpx playwright test\n",
		"stubs/wiremock/mappings/m.json": "{\"mappings\": [\n  {\"request\": {}},\n  {\"request\": {\"method\": \"GET\", \"url\": \"/b\"}}\n]}\n"}, "e2e")
	if len(mock.StandIns) != 1 || len(mock.StandIns[0].Routes) != 1 || mock.StandIns[0].Routes[0].Line != 3 {
		t.Fatalf("wiremock stand-ins = %+v", mock.StandIns)
	}
}
