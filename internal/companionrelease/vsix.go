package companionrelease

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	requiredNodeVersion       = "v22.23.2"
	requiredNPMVersion        = "10.9.8"
	requiredTypeScriptVersion = "Version 5.9.3"
	requiredVSCEVersion       = "3.9.2"
	vsixBuildTimeout          = 10 * time.Minute
	maxVSIXBytes              = 8 << 20
)

var vsixMembers = []string{
	"[Content_Types].xml", "extension.vsixmanifest", "extension/LICENSE.txt", "extension/changelog.md",
	"extension/dist/src/configuration.js", "extension/dist/src/executable.js", "extension/dist/src/extension.js", "extension/dist/src/json.js",
	"extension/dist/src/lifecycle.js", "extension/dist/src/liveTestProtocol.js", "extension/dist/src/liveTests.js",
	"extension/dist/src/mcp.js", "extension/dist/src/model.js", "extension/dist/src/observations.js",
	"extension/dist/src/presentation.js", "extension/dist/src/process.js", "extension/dist/src/testing.js",
	"extension/dist/src/testvalidity.js", "extension/media/corvint.svg", "extension/package.json", "extension/readme.md",
}

type builtVSIX struct {
	Path  string
	Data  []byte
	Tools VSIXToolchainManifest
}

func buildVSIXTwice(ctx context.Context, export Export, scratch, npmCache string) (builtVSIX, error) {
	if !filepath.IsAbs(npmCache) || filepath.Clean(npmCache) != npmCache {
		return builtVSIX{}, fmt.Errorf("npm cache must be an absolute normalized path")
	}
	info, err := os.Lstat(npmCache)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return builtVSIX{}, fmt.Errorf("npm cache is not a real directory: %v", err)
	}
	nodePath, err := exec.LookPath("node")
	if err != nil {
		return builtVSIX{}, fmt.Errorf("resolve node: %w", err)
	}
	npmPath, err := exec.LookPath("npm")
	if err != nil {
		return builtVSIX{}, fmt.Errorf("resolve npm: %w", err)
	}
	nodeDigest, err := fileSHA256(nodePath)
	if err != nil {
		return builtVSIX{}, err
	}
	if err := requireVersion(ctx, scratch, nodePath, []string{"--version"}, requiredNodeVersion); err != nil {
		return builtVSIX{}, err
	}
	if err := requireFileDigest(nodePath, nodeDigest); err != nil {
		return builtVSIX{}, err
	}
	npmDigest, err := fileSHA256(npmPath)
	if err != nil {
		return builtVSIX{}, err
	}
	if err := requireVersion(ctx, scratch, npmPath, []string{"--version"}, requiredNPMVersion); err != nil {
		return builtVSIX{}, err
	}
	if err := requireFileDigest(npmPath, npmDigest); err != nil {
		return builtVSIX{}, err
	}
	lock, ok := findExportFile(export, "extensions/vscode/package-lock.json")
	if !ok {
		return builtVSIX{}, fmt.Errorf("VSIX lockfile absent from export")
	}
	tools := VSIXToolchainManifest{NodeVersion: requiredNodeVersion, NodeSHA256: nodeDigest, NPMVersion: requiredNPMVersion, NPMSHA256: npmDigest, TypeScriptVersion: requiredTypeScriptVersion, VSCEVersion: requiredVSCEVersion, LockfileSHA256: sha256Hex(lock.Data)}

	build := func(side string) (map[string][]byte, string, string, error) {
		root, err := stageBuildSource(export, filepath.Join(scratch, "vsix-source-"+side))
		if err != nil {
			return nil, "", "", err
		}
		extensionRoot := filepath.Join(root, "extensions", "vscode")
		cacheParent := filepath.Join(scratch, "vsix-cache-"+side)
		if err := mkdirScratchDir(cacheParent); err != nil {
			return nil, "", "", err
		}
		cache := filepath.Join(cacheParent, "npm")
		if _, _, err := runCaptured(ctx, scratch, []string{"PATH=/usr/bin:/bin", "HOME=" + cacheParent, "LANG=C", "LC_ALL=C"}, vsixBuildTimeout, "/bin/cp", "-cR", npmCache, cache); err != nil {
			return nil, "", "", fmt.Errorf("clone npm cache %s: %w", side, err)
		}
		env := closedNodeEnv(cacheParent, cache, filepath.Dir(nodePath))
		if err := requireFileDigest(nodePath, nodeDigest); err != nil {
			return nil, "", "", err
		}
		if err := requireFileDigest(npmPath, npmDigest); err != nil {
			return nil, "", "", err
		}
		if _, _, err := runCaptured(ctx, extensionRoot, env, vsixBuildTimeout, npmPath, "ci", "--ignore-scripts", "--offline", "--no-audit", "--no-fund", "--cache", cache); err != nil {
			return nil, "", "", fmt.Errorf("npm ci %s: %w", side, err)
		}
		if err := requireFileDigest(nodePath, nodeDigest); err != nil {
			return nil, "", "", err
		}
		if err := requireFileDigest(npmPath, npmDigest); err != nil {
			return nil, "", "", err
		}
		tscPath := filepath.Join(extensionRoot, "node_modules", ".bin", "tsc")
		vscePath := filepath.Join(extensionRoot, "node_modules", ".bin", "vsce")
		tscDigest, err := fileSHA256(tscPath)
		if err != nil {
			return nil, "", "", err
		}
		vsceDigest, err := fileSHA256(vscePath)
		if err != nil {
			return nil, "", "", err
		}
		if err := requireVersionEnv(ctx, extensionRoot, env, tscPath, []string{"--version"}, requiredTypeScriptVersion); err != nil {
			return nil, "", "", err
		}
		if err := requireFileDigest(tscPath, tscDigest); err != nil {
			return nil, "", "", err
		}
		if err := requireVersionEnv(ctx, extensionRoot, env, vscePath, []string{"--version"}, requiredVSCEVersion); err != nil {
			return nil, "", "", err
		}
		if err := requireFileDigest(vscePath, vsceDigest); err != nil {
			return nil, "", "", err
		}
		if err := requireFileDigest(nodePath, nodeDigest); err != nil {
			return nil, "", "", err
		}
		if err := requireFileDigest(npmPath, npmDigest); err != nil {
			return nil, "", "", err
		}
		if err := requireFileDigest(filepath.Join(extensionRoot, "node_modules", ".bin", "tsc"), tscDigest); err != nil {
			return nil, "", "", err
		}
		if _, _, err := runCaptured(ctx, extensionRoot, env, vsixBuildTimeout, npmPath, "run", "compile"); err != nil {
			return nil, "", "", fmt.Errorf("compile VSIX %s: %w", side, err)
		}
		if err := requireFileDigest(nodePath, nodeDigest); err != nil {
			return nil, "", "", err
		}
		if err := requireFileDigest(npmPath, npmDigest); err != nil {
			return nil, "", "", err
		}
		if err := requireFileDigest(filepath.Join(extensionRoot, "node_modules", ".bin", "tsc"), tscDigest); err != nil {
			return nil, "", "", err
		}
		raw := filepath.Join(scratch, "corvint-vscode-"+side+".vsix")
		base := "https://github.com/Beamfall/corvint/blob/" + export.HeadCommit
		images := "https://github.com/Beamfall/corvint/raw/" + export.HeadCommit
		if err := requireFileDigest(nodePath, nodeDigest); err != nil {
			return nil, "", "", err
		}
		if err := requireFileDigest(vscePath, vsceDigest); err != nil {
			return nil, "", "", err
		}
		if _, _, err := runCaptured(ctx, extensionRoot, env, vsixBuildTimeout, vscePath, "package", "--no-dependencies", "--allow-missing-repository", "--baseContentUrl", base, "--baseImagesUrl", images, "--out", raw); err != nil {
			return nil, "", "", fmt.Errorf("package VSIX %s: %w", side, err)
		}
		if err := requireFileDigest(nodePath, nodeDigest); err != nil {
			return nil, "", "", err
		}
		if err := requireFileDigest(vscePath, vsceDigest); err != nil {
			return nil, "", "", err
		}
		data, err := os.ReadFile(raw)
		if err != nil {
			return nil, "", "", err
		}
		members, err := readVSIXMembers(data)
		return members, tscDigest, vsceDigest, err
	}
	a, tscA, vsceA, err := build("a")
	if err != nil {
		return builtVSIX{}, err
	}
	b, tscB, vsceB, err := build("b")
	if err != nil {
		return builtVSIX{}, err
	}
	if err := equalVSIXMembers(a, b); err != nil {
		return builtVSIX{}, err
	}
	if tscA != tscB || vsceA != vsceB {
		return builtVSIX{}, fmt.Errorf("VSIX tool executable digests disagree")
	}
	tools.TypeScriptSHA256, tools.VSCESHA256 = tscA, vsceA
	first, err := canonicalVSIX(a)
	if err != nil {
		return builtVSIX{}, err
	}
	second, err := canonicalVSIX(b)
	if err != nil {
		return builtVSIX{}, err
	}
	if !bytes.Equal(first, second) {
		return builtVSIX{}, fmt.Errorf("canonical VSIX assemblies disagree")
	}
	if got, err := readVSIXMembers(first); err != nil {
		return builtVSIX{}, err
	} else if err := equalVSIXMembers(a, got); err != nil {
		return builtVSIX{}, fmt.Errorf("canonical VSIX verification: %w", err)
	}
	return builtVSIX{Path: "extensions/corvint-vscode-0.1.0.vsix", Data: first, Tools: tools}, nil
}

func fileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return sha256Hex(data), nil
}

func requireFileDigest(path, want string) error {
	got, err := fileSHA256(path)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("tool identity changed at %s", path)
	}
	return nil
}

func closedNodeEnv(home, cache, nodeDir string) []string {
	return []string{"PATH=" + nodeDir + ":/usr/bin:/bin", "HOME=" + home, "npm_config_cache=" + cache,
		"npm_config_audit=false", "npm_config_fund=false", "npm_config_ignore_scripts=true", "npm_config_offline=true",
		"NO_UPDATE_NOTIFIER=1", "LANG=C", "LC_ALL=C", "TZ=UTC"}
}

func requireVersion(ctx context.Context, dir, command string, args []string, want string) error {
	return requireVersionEnv(ctx, dir, []string{"PATH=" + filepath.Dir(command) + ":/usr/bin:/bin", "HOME=" + dir, "LANG=C", "LC_ALL=C"}, command, args, want)
}

func requireVersionEnv(ctx context.Context, dir string, env []string, command string, args []string, want string) error {
	out, _, err := runCaptured(ctx, dir, env, subprocessTimeout, append([]string{command}, args...)...)
	if err != nil {
		return err
	}
	if got := strings.TrimSpace(string(out)); got != want {
		return fmt.Errorf("%s version %q != required %q", command, got, want)
	}
	return nil
}

func readVSIXMembers(data []byte) (map[string][]byte, error) {
	if len(data) == 0 || len(data) > maxVSIXBytes {
		return nil, fmt.Errorf("VSIX size outside 1..%d", maxVSIXBytes)
	}
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	want := make(map[string]bool, len(vsixMembers))
	for _, name := range vsixMembers {
		want[name] = true
	}
	got := make(map[string][]byte, len(r.File))
	for _, f := range r.File {
		if !want[f.Name] {
			return nil, fmt.Errorf("unexpected VSIX member %s", f.Name)
		}
		if _, exists := got[f.Name]; exists {
			return nil, fmt.Errorf("duplicate VSIX member %s", f.Name)
		}
		if f.FileInfo().IsDir() || !f.Mode().IsRegular() {
			return nil, fmt.Errorf("unsafe VSIX member %s", f.Name)
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		body, readErr := io.ReadAll(io.LimitReader(rc, maxVSIXBytes+1))
		closeErr := rc.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if len(body) > maxVSIXBytes {
			return nil, fmt.Errorf("VSIX member %s exceeds bound", f.Name)
		}
		got[f.Name] = body
	}
	if len(got) != len(want) {
		return nil, fmt.Errorf("VSIX has %d members, want %d", len(got), len(want))
	}
	return got, nil
}

func equalVSIXMembers(a, b map[string][]byte) error {
	if len(a) != len(b) {
		return fmt.Errorf("VSIX member counts disagree")
	}
	for name, body := range a {
		if !bytes.Equal(body, b[name]) {
			return fmt.Errorf("VSIX builds disagree at member %s", name)
		}
	}
	return nil
}

func canonicalVSIX(members map[string][]byte) ([]byte, error) {
	names := append([]string(nil), vsixMembers...)
	sort.Strings(names)
	var out bytes.Buffer
	w := zip.NewWriter(&out)
	for _, name := range names {
		body, ok := members[name]
		if !ok {
			return nil, fmt.Errorf("missing VSIX member %s", name)
		}
		h := &zip.FileHeader{Name: name, Method: zip.Store}
		h.Modified = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)
		h.SetMode(0o644)
		entry, err := w.CreateHeader(h)
		if err != nil {
			return nil, err
		}
		if _, err := entry.Write(body); err != nil {
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
