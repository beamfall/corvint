package companionrelease

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstalledQualificationRefusesOutputBeforeScratchWrite(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "tool")
	if err := os.WriteFile(executable, []byte("tool"), 0o700); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, "result.json")
	if err := os.WriteFile(output, []byte("owned"), 0o600); err != nil {
		t.Fatal(err)
	}
	scratch := filepath.Join(root, "scratch")
	_, err := RunInstalledQualification(t.Context(), InstalledOptions{SourceRoot: root, BundleDirectory: root, Scratch: scratch, OutputPath: output, NPMCache: root, BrowserCache: root, NodePath: executable, PythonPath: executable, ExpectedPythonSHA256: hex64, ExpectedCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ExpectedTree: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"})
	if err == nil {
		t.Fatal("existing output accepted")
	}
	if _, err := os.Lstat(scratch); !os.IsNotExist(err) {
		t.Fatalf("scratch written before refusal: %v", err)
	}
	body, _ := os.ReadFile(output)
	if string(body) != "owned" {
		t.Fatalf("existing output changed: %q", body)
	}
}

func TestCopyBoundedTreeRejectsSymlink(t *testing.T) {
	source := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(source, "link")); err != nil {
		t.Fatal(err)
	}
	if err := copyBoundedTree(t.Context(), source, filepath.Join(t.TempDir(), "copy"), false); err == nil {
		t.Fatal("symlinked npm cache accepted")
	}
}

func TestCopyBoundedTreePreservesOnlyInternalBrowserSymlink(t *testing.T) {
	source := t.TempDir()
	if err := os.Mkdir(filepath.Join(source, "Versions"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "Versions", "browser"), []byte("browser"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("Versions/browser", filepath.Join(source, "Current")); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "copy")
	if err := copyBoundedTree(t.Context(), source, target, true); err != nil {
		t.Fatal(err)
	}
	if link, err := os.Readlink(filepath.Join(target, "Current")); err != nil || link != "Versions/browser" {
		t.Fatalf("copied link=%q err=%v", link, err)
	}
	if info, err := os.Stat(filepath.Join(target, "Versions", "browser")); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("executable mode not preserved: %v %v", info, err)
	}
	escaping := t.TempDir()
	if err := os.Symlink("../outside", filepath.Join(escaping, "escape")); err != nil {
		t.Fatal(err)
	}
	if err := copyBoundedTree(t.Context(), escaping, filepath.Join(t.TempDir(), "bad"), true); err == nil {
		t.Fatal("escaping browser symlink accepted")
	}
}

func TestCopyBoundedTreeHonorsCancellation(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "file"), []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := copyBoundedTree(ctx, source, filepath.Join(t.TempDir(), "copy"), false); err == nil {
		t.Fatal("cancelled copy succeeded")
	}
}

func TestCopyBoundedTreeLaunchesPinnedBrowser(t *testing.T) {
	source := os.Getenv("CORVINT_TEST_BROWSER_CACHE")
	if source == "" {
		t.Skip("set CORVINT_TEST_BROWSER_CACHE for installed browser proof")
	}
	target := filepath.Join(t.TempDir(), "copy")
	if err := copyBoundedTree(t.Context(), source, target, true); err != nil {
		t.Fatal(err)
	}
	browser := filepath.Join(target, "chromium_headless_shell-1243", "chrome-headless-shell-mac-arm64", "chrome-headless-shell")
	sourceBrowser := filepath.Join(source, "chromium_headless_shell-1243", "chrome-headless-shell-mac-arm64", "chrome-headless-shell")
	sourceSHA, err := hashRegular(sourceBrowser)
	if err != nil {
		t.Fatal(err)
	}
	copySHA, err := hashRegular(browser)
	if err != nil {
		t.Fatal(err)
	}
	if copySHA != sourceSHA {
		t.Fatalf("copied browser digest %s differs from source %s", copySHA, sourceSHA)
	}
	output, err := exec.Command(browser, "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("copied browser launch: %v: %s", err, output)
	}
	if len(output) == 0 {
		t.Fatal("copied browser omitted version")
	}
	t.Logf("copied browser sha256=%s version=%s", copySHA, strings.TrimSpace(string(output)))
}

func TestWriteExclusiveAtomicNeverOverwrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "result.json")
	if err := os.WriteFile(path, []byte("first"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeExclusiveAtomic(path, []byte("second")); err == nil {
		t.Fatal("overwrite accepted")
	}
	body, _ := os.ReadFile(path)
	if string(body) != "first" {
		t.Fatalf("result changed: %q", body)
	}
}

func TestInstalledAdmissionRejectsInputOutputOverlap(t *testing.T) {
	root := t.TempDir()
	tool := filepath.Join(root, "tool")
	if err := os.WriteFile(tool, []byte("tool"), 0o700); err != nil {
		t.Fatal(err)
	}
	base := InstalledOptions{SourceRoot: root, BundleDirectory: root, NPMCache: root, BrowserCache: root, NodePath: tool, PythonPath: tool, ExpectedPythonSHA256: hex64, ExpectedCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ExpectedTree: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}
	base.Scratch, base.OutputPath = filepath.Join(root, "scratch"), filepath.Join(t.TempDir(), "result")
	if err := admitInstalledOptions(base); err == nil {
		t.Fatal("source-overlapping scratch accepted")
	}
	base.Scratch, base.OutputPath = filepath.Join(t.TempDir(), "scratch"), filepath.Join(root, "result")
	if err := admitInstalledOptions(base); err == nil {
		t.Fatal("source-overlapping result accepted")
	}
}

func TestRehashRetainedRejectsEquivalentSidecarRewrite(t *testing.T) {
	dir := makeRetainedFixture(t)
	verified, err := VerifyRetainedBundle(dir)
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(verified.SmokePath)
	if err != nil {
		t.Fatal(err)
	}
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		t.Fatal(err)
	}
	rewritten, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(verified.SmokePath, append(rewritten, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := rehashRetained(verified); err == nil {
		t.Fatal("rewritten smoke sidecar accepted")
	}
}

func TestSelectProfileRequiresOnePassingRecord(t *testing.T) {
	good := []byte("noise\n{\"profile\":\"wanted/0\",\"status\":\"PASS\",\"value\":1}\n")
	if got, err := selectProfile(good, "wanted/0"); err != nil || len(got) == 0 {
		t.Fatalf("select profile: %s %v", got, err)
	}
	if _, err := selectProfile(append(good, good...), "wanted/0"); err == nil {
		t.Fatal("duplicate profile accepted")
	}
	if _, err := selectProfile([]byte(`{"profile":"wanted/0","status":"FAIL"}`), "wanted/0"); err == nil {
		t.Fatal("failed profile accepted")
	}
}

func TestInstalledEvidenceSerializationPreservesRawBytes(t *testing.T) {
	raw := "{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{}}\n"
	report := InstalledReport{Docs: []InstalledEvidence{{Name: "mcp.json", SHA256: sha256Hex([]byte(raw)), Raw: raw}}}
	body, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateSerializedEvidence(body); err != nil {
		t.Fatal(err)
	}
	var decoded InstalledReport
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Docs[0].Raw != raw {
		t.Fatal("raw frame changed across result serialization")
	}
	decoded.Docs[0].Raw += " "
	tampered, _ := json.Marshal(decoded)
	if err := validateSerializedEvidence(tampered); err == nil {
		t.Fatal("tampered retained frame accepted")
	}
}
