package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/stepverify"
)

func stepCLIInputs(t *testing.T) (string, string, string) {
	t.Helper()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("unsupported native observation platform")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	hostRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	affectedGit(t, root, "init", "-q")
	writeFixtureFile(t, root, "allowed", "before")
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "fixture")
	d := stepverify.Declaration{Profile: "corvint-step-declaration/0", SessionID: "session", CapabilityID: "authoring", Author: stepverify.Checkout{ID: "author", Root: root}, ReadOnly: []stepverify.Checkout{}, WritePaths: []string{"allowed"}, GuardPaths: []string{}, Environment: []stepverify.EnvironmentKey{}}
	binary, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "darwin" {
		data, e := exec.Command("/usr/bin/xcrun", "--find", "git").Output()
		if e != nil {
			t.Fatal(e)
		}
		binary = strings.TrimSpace(string(data))
	}
	binary, err = filepath.EvalSymlinks(binary)
	if err != nil {
		t.Fatal(err)
	}
	data, err := stepverify.ReadInput(context.Background(), binary, &d)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	env := []stepverify.EnvironmentKey{}
	h := stepverify.Host{Profile: "corvint-step-host/0", ObserverID: "observer", SessionID: d.SessionID, CapabilityID: d.CapabilityID, DeclarationDigest: stepverify.DeclarationDigest(d), MetadataProtected: true, FilesystemConfined: true, GitBinary: binary, GitBinaryDigest: hex.EncodeToString(sum[:]), Environment: &env}
	dp, hp := hostRoot+"/declaration.json", hostRoot+"/host.json"
	if err := os.WriteFile(dp, stepverify.Encode(d), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hp, stepverify.Encode(h), 0600); err != nil {
		t.Fatal(err)
	}
	return root, dp, hp
}
func stepInvoke(t *testing.T, args ...string) (int, []byte, []byte) {
	t.Helper()
	options, handled, err := parseStepInvocation(args)
	var out, stderr bytes.Buffer
	if !handled {
		t.Fatal("not handled")
	}
	if err != nil {
		return stepError(&stderr, err), out.Bytes(), stderr.Bytes()
	}
	exit := runStep(context.Background(), options, &out, &stderr)
	return exit, out.Bytes(), stderr.Bytes()
}
func TestStepCLI(t *testing.T) {
	t.Run("ASS-V0-006 read only snapshot verify and env preflight", func(t *testing.T) {
		root, dp, hp := stepCLIInputs(t)
		indexBefore, err := os.ReadFile(root + "/.git/index")
		if err != nil {
			t.Fatal(err)
		}
		statusBefore := affectedGit(t, root, "status", "--porcelain=v1")
		exit, data, stderr := stepInvoke(t, "step", "snapshot", "--declaration", dp, "--host", hp)
		if exit != 0 || len(stderr) != 0 {
			t.Fatalf("snapshot %d %s %s", exit, data, stderr)
		}
		before := filepath.Dir(dp) + "/before.json"
		os.WriteFile(before, data, 0600)
		exit, data, stderr = stepInvoke(t, "step", "verify", "--declaration="+dp, "--host="+hp, "--before="+before)
		if exit != 0 || len(stderr) != 0 {
			t.Fatalf("verify %d %s %s", exit, data, stderr)
		}
		indexAfter, _ := os.ReadFile(root + "/.git/index")
		if !bytes.Equal(indexBefore, indexAfter) || statusBefore != affectedGit(t, root, "status", "--porcelain=v1") {
			t.Fatal("observer mutated checkout/index")
		}
		writeFixtureFile(t, root, "outside", "secret-value-not-for-output")
		exit, data, stderr = stepInvoke(t, "step", "verify", "--declaration", dp, "--host", hp, "--before", before)
		var receipt stepverify.Receipt
		if json.Unmarshal(data, &receipt) != nil || exit != 1 || receipt.WriteScopeVerdict != "FAIL" || len(stderr) != 0 || bytes.Contains(data, []byte("secret-value-not-for-output")) {
			t.Fatalf("finding %d %s %s", exit, data, stderr)
		}
		exit, data, stderr = stepInvoke(t, "step", "env-check", "--declaration", dp, "--host", hp)
		if exit != 0 || len(stderr) != 0 || !bytes.Contains(data, []byte(`"environment_verdict":"PASS"`)) {
			t.Fatalf("preflight %d %s %s", exit, data, stderr)
		}
	})
	t.Run("ASS-V0-004 host input failures stay closed", func(t *testing.T) {
		root, dp, hp := stepCLIInputs(t)
		data, _ := os.ReadFile(dp)
		inside := root + "/declaration.json"
		os.WriteFile(inside, data, 0600)
		for _, path := range []string{inside, filepath.Dir(dp) + "/missing"} {
			exit, out, stderr := stepInvoke(t, "step", "snapshot", "--declaration", path, "--host", hp)
			if exit != 2 || len(out) != 0 || !bytes.Contains(stderr, []byte("STEP_")) || bytes.Contains(stderr, []byte(path)) {
				t.Fatalf("failure %d %s %s", exit, out, stderr)
			}
		}
		os.WriteFile(dp, []byte(`{"secret-parse-error":"hidden-value"}`), 0600)
		exit, out, stderr := stepInvoke(t, "step", "snapshot", "--declaration", dp, "--host", hp)
		if exit != 2 || len(out) != 0 || bytes.Contains(stderr, []byte("hidden-value")) {
			t.Fatal("parser leak")
		}
	})
	t.Run("ASS-V0-001 strict CLI options", func(t *testing.T) {
		for _, args := range [][]string{{"step"}, {"step", "other"}, {"step", "snapshot", "--host", "x"}, {"step", "snapshot", "--declaration", "x", "--host", "y", "--before", "z"}, {"step", "verify", "--declaration", "x", "--host", "y"}, {"step", "env-check", "--declaration", "x", "--host", "y", "--host", "z"}, {"step", "snapshot", "--declaration", "x", "--host", "y", "--raw", "secret"}} {
			_, handled, err := parseStepInvocation(args)
			if !handled || err == nil {
				t.Fatalf("accepted %v", args)
			}
		}
		if _, handled, _ := parseStepInvocation([]string{"query"}); handled {
			t.Fatal("stole other command")
		}
	})
}

func TestStepCLIMissingMarkerRetainsWrites(t *testing.T) {
	t.Run("ASS-V0-004 handler retains findings after administrative loss", func(t *testing.T) {
		root, dp, hp := stepCLIInputs(t)
		exit, data, stderr := stepInvoke(t, "step", "snapshot", "--declaration", dp, "--host", hp)
		if exit != 0 {
			t.Fatalf("%d %s %s", exit, data, stderr)
		}
		before := filepath.Dir(dp) + "/before.json"
		os.WriteFile(before, data, 0600)
		writeFixtureFile(t, root, "outside", "changed")
		if err := os.Rename(root+"/.git", root+"/.git-hidden"); err != nil {
			t.Fatal(err)
		}
		exit, data, stderr = stepInvoke(t, "step", "verify", "--declaration", dp, "--host", hp, "--before", before)
		var r stepverify.Receipt
		if json.Unmarshal(data, &r) != nil || exit != 2 || len(stderr) != 0 || r.AfterDigest != nil {
			t.Fatalf("%d %s %s", exit, data, stderr)
		}
		found := false
		for _, f := range r.Findings {
			found = found || f.Code == "OUT_OF_SCOPE_WRITE" && f.Path == "outside"
		}
		if !found {
			t.Fatalf("missing completed write: %s", data)
		}
	})
}

func TestStepPublicDispatchConformance(t *testing.T) {
	t.Run("ASS-V0-006 public namespace and help", func(t *testing.T) {
		for _, args := range [][]string{{"step", "--help"}, {"help", "step"}, {"step", "snapshot", "--help"}, {"--root", "/missing", "step", "verify", "--help"}, {"step", "env-check", "--help"}} {
			var out, stderr bytes.Buffer
			if exit := runContext(context.Background(), args, strings.NewReader(""), &out, &stderr); exit != 0 || out.String() != stepHelp || stderr.Len() != 0 {
				t.Fatalf("%v: %d %s %s", args, exit, &out, &stderr)
			}
		}
		root, dp, hp := stepCLIInputs(t)
		var out, stderr bytes.Buffer
		exit := runContext(context.Background(), []string{"step", "snapshot", "--declaration", dp, "--host", hp}, strings.NewReader(""), &out, &stderr)
		if exit != 0 || stderr.Len() != 0 || !bytes.Contains(out.Bytes(), []byte(`"corvint-step-state/0"`)) {
			t.Fatalf("public snapshot %d %s %s", exit, &out, &stderr)
		}
		_ = root
		out.Reset()
		stderr.Reset()
		if exit := runContext(context.Background(), []string{"step", "verify"}, strings.NewReader(""), &out, &stderr); exit != 2 || out.Len() != 0 || stderr.String() != "{\"ok\":false,\"code\":\"STEP_INPUT\"}\n" {
			t.Fatalf("public refusal %d %s %s", exit, &out, &stderr)
		}
	})
}
