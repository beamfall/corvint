package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/flowdocs"
)

func TestFlowDocsCLIEndToEnd(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		out, e := cmd.CombinedOutput()
		if e != nil {
			t.Fatalf("git: %v %s", e, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	git("config", "user.name", "Fixture")
	git("config", "user.email", "fixture@example.invalid")
	if err = os.Mkdir(filepath.Join(root, "app"), 0700); err != nil {
		t.Fatal(err)
	}
	ruby := "class PaymentsController\n  def create\n    authorize(user)\n    payment.save!\n    raise PaymentError\n  end\nend\n"
	files := map[string]string{"app/controller.rb": ruby, "app/routes.rb": "get '/payments', to: 'payments#create'\n", "app/view.tsx": "export function View() {\n  fetch('/payments');\n  return ready && <div>Ready</div>;\n}\n", "app/template.html": "<div\n id='view'\n ng-if='ready'>Visible</div>\n", "app/widget.js": "angular.module('app').controller('Widget', function() {\n});\n"}
	for p, v := range files {
		if err = os.WriteFile(filepath.Join(root, p), []byte(v), 0600); err != nil {
			t.Fatal(err)
		}
	}
	git("add", ".")
	git("commit", "-qm", "source")
	source := git("rev-parse", "HEAD")
	run := func(args ...string) (int, []byte) {
		t.Helper()
		var out, stderr bytes.Buffer
		code := runContext(context.Background(), append([]string{"--root", root, "docs", "flows"}, args...), strings.NewReader(""), &out, &stderr)
		if code == 2 {
			t.Fatalf("CLI refused: %s", stderr.String())
		}
		return code, out.Bytes()
	}
	destination := filepath.Join(root, "generated")
	_, raw := run("generate", "--revision", source, "--scope", "app", "--output-dir", destination)
	var m flowdocs.Manifest
	if err = json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Flows) < 4 {
		t.Fatalf("missing multilang declarations: %d", len(m.Flows))
	}
	for _, f := range m.Flows {
		dir := strings.TrimPrefix(f.ID, "flowdocs:flow:")
		entries, e := os.ReadDir(filepath.Join(destination, dir))
		if e != nil || len(entries) != 8 {
			t.Fatalf("eight files: %d %v", len(entries), e)
		}
	}
	if len(m.Provider.Claims) < 4 {
		t.Fatalf("missing behavioral observations: %+v", m.Provider.Claims)
	}
	previous := filepath.Join(destination, "generation.json")
	if code, _ := run("check", "--revision", source, "--previous", previous, "--output-dir", destination); code != 0 {
		t.Fatal("new output failed regeneration")
	}
	git("add", "generated")
	git("commit", "-qm", "provider")
	provider := git("rev-parse", "HEAD")
	_, manifest := run("finalize", "--previous", previous, "--provider-revision", provider, "--provider-path", "generated/provider.json")
	var corpusManifest doccorpus.Manifest
	if err = json.Unmarshal(manifest, &corpusManifest); err != nil {
		t.Fatal(err)
	}
	artifact, err := doccorpus.Build(context.Background(), root, corpusManifest)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := doccorpus.Encode(artifact)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = doccorpus.Open(context.Background(), root, encoded); err != nil {
		t.Fatal(err)
	}
	if corpusManifest.Repository.Revision != source || corpusManifest.Providers[len(corpusManifest.Providers)-1].Revision != provider {
		t.Fatal("circular source/provider binding")
	}
	if err = os.WriteFile(filepath.Join(root, "app/controller.rb"), []byte("# unrelated insertion\n"+strings.Replace(ruby, "authorize(user)", "authorize(admin)", 1)), 0600); err != nil {
		t.Fatal(err)
	}
	git("add", "app")
	git("commit", "-qm", "changed anchor")
	code, checked := run("check", "--revision", git("rev-parse", "HEAD"), "--previous", previous, "--output-dir", destination)
	if code != 1 {
		t.Fatal("changed anchor not stale")
	}
	var check flowdocs.Check
	if err = json.Unmarshal(checked, &check); err != nil {
		t.Fatal(err)
	}
	stale, fresh := 0, 0
	for _, b := range check.Bindings {
		switch b.State {
		case "stale":
			stale++
		case "fresh":
			fresh++
		}
	}
	// The changed authorize(...) line is the claim, its paragraph and, since
	// V1-0461 binds each flow's static call inventory (AFU-V1-045), the
	// create flow whose diagram renders that call.
	if stale != 3 || fresh < 30 || len(check.Added) != 0 || len(check.Retired) != 0 || len(check.OutputDifferences) != 0 {
		t.Fatalf("per-item freshness: %+v", check)
	}
}

// TestFlowDocsCLIPredecessorManifestTypedStale (AFU-V1-045, V1-0461): the CLI
// reports a generator version 1 manifest with the typed
// flowdocs-generator-stale code and exit 2.
func TestFlowDocsCLIPredecessorManifestTypedStale(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	previous := filepath.Join(root, "generation.json")
	if err = os.WriteFile(previous, []byte(`{"schema":"`+flowdocs.Schema+`","version":"1"}`), 0600); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	code := runContext(context.Background(), []string{"--root", root, "docs", "flows", "check", "--revision", strings.Repeat("a", 40), "--previous", previous}, strings.NewReader(""), &out, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), `"code": "`+flowdocs.StaleGeneratorCode+`"`) {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
}

// TestAFUV1125FlowLimitCLICode (AFU-V1-125, V1-2414): the CLI reports an
// over-limit scope with the owned flowdocs-flow-limit-exceeded code and exit 2.
func TestAFUV1125FlowLimitCLICode(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) string {
		t.Helper()
		out, e := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
		if e != nil {
			t.Fatalf("git: %v %s", e, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	git("config", "user.name", "Fixture")
	git("config", "user.email", "fixture@example.invalid")
	if err = os.Mkdir(filepath.Join(root, "app"), 0700); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for i := 0; i <= flowdocs.MaxFlows; i++ {
		b.WriteString("function f" + strconv.Itoa(i) + "() {\n  load();\n}\n")
	}
	if err = os.WriteFile(filepath.Join(root, "app", "a.js"), []byte(b.String()), 0600); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-qm", "source")
	rev := git("rev-parse", "HEAD")
	var out, stderr bytes.Buffer
	code := runContext(context.Background(), []string{"--root", root, "docs", "flows", "generate", "--revision", rev, "--scope", "app", "--output-dir", filepath.Join(root, "generated")}, strings.NewReader(""), &out, &stderr)
	var envelope struct {
		Code string `json:"code"`
	}
	if code != 2 || json.Unmarshal(stderr.Bytes(), &envelope) != nil || envelope.Code != flowdocs.FlowLimitCode {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
}
