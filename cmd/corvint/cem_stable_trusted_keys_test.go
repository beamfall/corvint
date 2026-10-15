package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// TestCEMV1084VerifyStableTrustedKeysFlag proves cem verify-stable accepts
// --trusted-keys only as an absolute closed corvint-trusted-keys/1 file and
// refuses any other value as invalid-arguments before the map is read.
func TestCEMV1084VerifyStableTrustedKeysFlag(t *testing.T) {
	dir := t.TempDir()
	public := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, ed25519.SeedSize)).Public().(ed25519.PublicKey)
	good := filepath.Join(dir, "trusted-keys.json")
	doc := `{"profile":"corvint-trusted-keys/1","keys":[{"keyid":"` + wire.Ed25519KeyID(public) + `","ed25519PublicKey":"` + base64.StdEncoding.EncodeToString(public) + `"}]}`
	if err := os.WriteFile(good, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(dir, "bad-keys.json")
	if err := os.WriteFile(bad, []byte(strings.Replace(doc, "corvint-trusted-keys/1", "corvint-trusted-keys/2", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	run := func(keys string) (int, map[string]any) {
		var out, errout bytes.Buffer
		exit := runContext(context.Background(), []string{"cem", "verify-stable", "--repository", filepath.Join(dir, "repo"), "--map", filepath.Join(dir, "missing-map.json"), "--trusted-keys", keys}, nil, &out, &errout)
		var result map[string]any
		if e := json.Unmarshal(out.Bytes(), &result); e != nil || (errout.Len() != 0 && !relativePathFixLine(result, errout.String())) {
			t.Fatalf("--trusted-keys %s: %v %s %s", keys, e, out.String(), errout.String())
		}
		return exit, result
	}
	for _, keys := range []string{"trusted-keys.json", bad, filepath.Join(dir, "absent.json"), dir} {
		if exit, result := run(keys); exit != 2 || result["stage"] != "arguments" || result["code"] != "invalid-arguments" || result["mapSha256"] != nil {
			t.Fatalf("--trusted-keys %s: exit %d result %v, want an arguments refusal", keys, exit, result)
		}
	}
	if exit, result := run(good); exit != 2 || result["stage"] == "arguments" {
		t.Fatalf("--trusted-keys %s: exit %d result %v, want the flag accepted and the missing map refused later", good, exit, result)
	}
}

// TestCEMV1084HelpNamesTrustedKeys proves cem help documents verify-stable
// and the consumer --trusted-keys file without claiming execution.
func TestCEMV1084HelpNamesTrustedKeys(t *testing.T) {
	for _, want := range []string{"cem verify-stable --repository PATH --map MAP", "[--trusted-keys FILE]", "corvint-trusted-keys/1", "UNTRUSTED_KEY", "never raises executionAtCommit"} {
		if !strings.Contains(cemHelp, want) {
			t.Errorf("cemHelp lacks %q", want)
		}
	}
	var out, errout bytes.Buffer
	if exit := runContext(context.Background(), []string{"cem", "--help"}, nil, &out, &errout); exit != 0 || !strings.Contains(out.String(), "--trusted-keys FILE") {
		t.Fatalf("cem --help exit %d output lacks --trusted-keys", exit)
	}
}
