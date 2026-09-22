package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/appflows"
)

func flowRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	manifest := `{"profile":"application-flow-intent/0","application":"fixture","origin":"http://127.0.0.1:12345","sources":["app.js"],"tests":[],"backendSource":"app.js","fixture":"seed","identityPath":"/identity","resetPath":"/reset","server":["node","app.js"],"scenarios":[{"id":"home","role":"reader","path":"/","basis":"inferred","actions":[],"checks":[{"id":"visible","kind":"visible","selector":"#home","want":true}]}]}`
	for p, b := range map[string]string{"flows.json": manifest, "app.js": "fixture"} {
		if err := os.WriteFile(filepath.Join(root, p), []byte(b), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "."}, {"-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "fixture"}} {
		c := exec.Command("git", args...)
		c.Dir = root
		if b, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v %s", err, b)
		}
	}
	return root
}

func flowSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	r := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		b, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		r[p] = fmt.Sprintf("%x", sha256.Sum256(b))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// AFU-V0-001 AFU-V0-009 AFU-V0-012
func TestFlowsCLIReadPurity(t *testing.T) {
	root := flowRepo(t)
	before := flowSnapshot(t, root)
	var out, diagnostic bytes.Buffer
	code := runContext(context.Background(), []string{"--root", root, "flows", "--manifest", "flows.json"}, bytes.NewReader(nil), &out, &diagnostic)
	if code != 0 {
		t.Fatalf("%d %s", code, diagnostic.String())
	}
	var r appflows.Report
	if err := json.Unmarshal(out.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	if r.Profile != appflows.ReportProfile || r.Complete || r.Flows[0].Runtime != "unobserved" {
		t.Fatal("unsupported confirmation", out.String())
	}
	if !reflect.DeepEqual(before, flowSnapshot(t, root)) {
		t.Fatal("flow read changed repository bytes")
	}
	out.Reset()
	diagnostic.Reset()
	code = runContext(context.Background(), []string{"--root", root, "flows", "--manifest", "missing.json"}, bytes.NewReader(nil), &out, &diagnostic)
	if code != 2 {
		t.Fatal(code)
	}
	if !reflect.DeepEqual(before, flowSnapshot(t, root)) {
		t.Fatal("failed read wrote state")
	}
}

func TestFlowsHelpDoesNotRequireRepository(t *testing.T) {
	for _, args := range [][]string{{"help", "flows"}, {"flows", "--help"}, {"flows", "record", "--help"}} {
		var out, diagnostic bytes.Buffer
		if code := runContext(context.Background(), args, bytes.NewReader(nil), &out, &diagnostic); code != 0 {
			t.Fatal(args, code, diagnostic.String())
		}
		if !bytes.Contains(out.Bytes(), []byte("application-flow-report/0")) {
			t.Fatal("missing help")
		}
	}
}
