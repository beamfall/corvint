package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestPiClosedInput(t *testing.T) {
	t.Run("AHI-024 closed input refusal", func(t *testing.T) {
		for _, raw := range []string{`null`, `{"hostVersion":"0.85.1","input":{},"root":"/"}`, `{"hostVersion":"0.85.1","hostVersion":"0.85.1","input":{}}`, `{"hostVersion":null,"input":{}}`, `{"hostVersion":"0.85.1","input":{"task":null}}`, strings.Repeat(" ", piInputLimit+1)} {
			var out bytes.Buffer
			runPiAdapter(context.Background(), []string{"user-prompt"}, strings.NewReader(raw), &out)
			var v map[string]any
			if json.Unmarshal(out.Bytes(), &v) != nil || v["fault"] == nil || v["receiptId"] != nil || v["shouldContinue"] != false || len(out.Bytes()) > 8000 {
				t.Fatalf("bad refusal: %s", out.Bytes())
			}
		}
	})
}
func TestPiUnknownVersion(t *testing.T) {
	t.Run("AHI-024 unsupported version remains unknown", func(t *testing.T) {
		v := piAdapterResult(context.Background(), "stop", strings.NewReader(`{"hostVersion":"other","input":{}}`))
		if v["hostVersion"] != nil || v["fault"] != "unsupported-host-version" {
			t.Fatal(v)
		}
	})
}
func TestPiNativeStopReceipt(t *testing.T) {
	t.Run("AHI-024 native Stop receipt without continuation", func(t *testing.T) {
		root := t.TempDir()
		for _, args := range [][]string{{"init", "-q"}, {"-c", "user.name=Fixture", "-c", "user.email=f@example.invalid", "commit", "--allow-empty", "-qm", "fixture"}} {
			c := exec.Command("git", args...)
			c.Dir = root
			if out, e := c.CombinedOutput(); e != nil {
				t.Fatalf("git:%s %v", out, e)
			}
		}
		old, _ := os.Getwd()
		if e := os.Chdir(root); e != nil {
			t.Fatal(e)
		}
		defer os.Chdir(old)
		v := piAdapterResult(context.Background(), "stop", strings.NewReader(`{"hostVersion":"0.85.1","input":{}}`))
		if v["fault"] != nil || v["context"] != "" || v["host"] != "pi" || v["shouldContinue"] != false || !piReceipt.MatchString(v["receiptId"].(string)) {
			t.Fatal(v)
		}
	})
}

func TestPiInvalidOutcomeInput(t *testing.T) {
	for _, input := range []string{`{"outcome":"passed"}`, `{"unknown":true}`, `{"verification":[{"commandSha256":"bad","status":"passed"}]}`} {
		v := piAdapterResult(context.Background(), "session-end", strings.NewReader(`{"hostVersion":"0.85.1","input":`+input+`}`))
		if v["fault"] != "invalid-input" || v["receiptId"] != nil {
			t.Fatalf("AHI-024 invalid outcome: %v", v)
		}
	}
}
