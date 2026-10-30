package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/requestpath"
)

// requestPathRepo commits a token-exchange route fixture into a fresh repository.
func requestPathRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"src/app.js": "const express = require('express');\nconst { exchange } = require('./exchange');\nconst { validate } = require('./validate');\n" +
			"const app = express();\napp.post('/token', exchange, validate, (req, res) => res.json(req.user));\n" +
			"app.post('/token2', validate, exchange, (req, res) => res.json(req.user));\n",
		"src/exchange.js": "const axios = require('axios');\nexports.exchange = async (req, res, next) => {\n" +
			"  req.user = (await axios.post(process.env.IDP_TOKEN_ENDPOINT, { iss: process.env.OIDC_ISSUER })).data;\n  next();\n};\n",
		"src/validate.js": "exports.validate = (req, res, next) => (req.body && req.body.code ? next() : res.sendStatus(400));\n",
	}
	for name, text := range files {
		if err := os.MkdirAll(filepath.Join(root, filepath.Dir(name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}, {"-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", "fixture"}} {
		cmd := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
		cmd.Dir, cmd.Env = root, append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	// An uncommitted edit is never read.
	if err := os.WriteFile(filepath.Join(root, "src/validate.js"), []byte("exports.validate = () => fetch(process.env.WORKTREE_ONLY);\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// AMAP-V0-131 AMAP-V0-133 AMAP-V0-135 AMAP-V0-136 AMAP-V0-138: the CLI reads committed source,
// reports keys and the outbound guard, flags the order, and fails closed on an empty profile
// without printing a value.
func TestAMAPV0138RequestPathCLI(t *testing.T) {
	root := requestPathRepo(t)
	code, out, diagnostic := runFlowsCLI(root, "request-path", "--file", "src/app.js", "--route", "post /token")
	if code != 0 {
		t.Fatalf("query: %d %s", code, diagnostic)
	}
	var r requestpath.Report
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatal(err)
	}
	if r.Schema != requestpath.Schema || len(r.Revision) != 40 || len(r.EnvKeys) != 2 || r.EnvKeys[0].Name != "IDP_TOKEN_ENDPOINT" ||
		r.EnvKeys[1].Name != "OIDC_ISSUER" || len(r.OutboundCalls) != 1 || r.OutboundCalls[0].Callee != "axios.post" ||
		len(r.GuardOrder) != 1 || r.GuardOrder[0].OutboundGuard != 1 || strings.Contains(out, "WORKTREE_ONLY") {
		t.Fatalf("report = %s", out)
	}
	code, out, diagnostic = runFlowsCLI(root, "request-path", "--file", "src/app.js", "--route", "POST /token2")
	if code != 0 || strings.Contains(out, requestpath.FindingOutboundGuardFirst) {
		t.Fatalf("reordered: %d %s %s", code, out, diagnostic)
	}

	profile := filepath.Join(t.TempDir(), "staging.env")
	if err := os.WriteFile(profile, []byte("OIDC_ISSUER=\nIDP_TOKEN_ENDPOINT=\nAPI_SECRET=do-not-print\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, diagnostic = runFlowsCLI(root, "request-path", "--file", "src/app.js", "--route", "POST /token", "--env-profile", profile)
	if code != 1 || !strings.Contains(out, `"batch":"DO_NOT_SCHEDULE"`) || !strings.Contains(diagnostic, "BLOCKED") ||
		strings.Contains(out+diagnostic, "do-not-print") || strings.Contains(out+diagnostic, "API_SECRET") {
		t.Fatalf("fail closed: %d %s %s", code, out, diagnostic)
	}
	code, out, _ = runFlowsCLI(root, "request-path", "--file", "src/app.js", "--route", "POST /token", "--env-profile", profile, "--warn")
	if code != 0 || !strings.Contains(out, `"batch":"SCHEDULE_WITH_WARNINGS"`) {
		t.Fatalf("warn: %d %s", code, out)
	}

	for _, tc := range []struct {
		args []string
		code string
	}{
		{[]string{"--file", "src/app.js", "--route", "GET /missing"}, "request-path-route-not-found"},
		{[]string{"--file", "src/app.js", "--handler", "nope"}, "request-path-handler-not-found"},
		{[]string{"--file", "src/app.js", "--route", "POST /token", "--revision", "no-such-rev"}, "request-path-invalid-revision"},
		{[]string{"--file", "src/app.js", "--route", "POST /token", "--env-profile", filepath.Join(root, "absent.env")}, "request-path-profile-invalid"},
	} {
		if code, _, diagnostic := runFlowsCLI(root, append([]string{"request-path"}, tc.args...)...); code != 2 || !strings.Contains(diagnostic, tc.code) {
			t.Fatalf("%v: %d %s", tc.args, code, diagnostic)
		}
	}
	if code, _, _ := runFlowsCLI(root, "request-path", "--file", "src/app.js", "--route", "POST /token", "--handler", "x"); code != 2 {
		t.Fatalf("both selectors: %d", code)
	}
}
