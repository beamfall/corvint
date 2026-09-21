package companionrelease

import (
	"bytes"
	"context"
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// buildTimeout bounds one `go build` invocation. A companion component is a
// small module-relative package; ten minutes is generous headroom.
const buildTimeout = 10 * time.Minute

// BuiltBinary is one component binary this package has verified twice: two
// independent, cold-cache builds produced byte-identical output, and its
// on-disk buildinfo matches the pinned toolchain and target exactly.
type BuiltBinary struct {
	Name       string
	Path       string
	SHA256     string
	Size       int64
	ModulePath string
}

// buildComponentTwice runs `go build` for pkgPath (relative to moduleRoot)
// twice, each with its own cold GOCACHE/GOMODCACHE/GOPATH under scratch, and
// requires the two binaries to be byte-identical before accepting either.
// It then verifies the retained binary's embedded buildinfo against the
// pinned Go version and requested target.
func buildComponentTwice(ctx context.Context, moduleRoot, pkgPath, name, target, scratch string) (BuiltBinary, error) {
	return buildComponentTwiceWithFlags(ctx, moduleRoot, pkgPath, name, target, scratch, nil)
}

func buildComponentTwiceWithFlags(ctx context.Context, moduleRoot, pkgPath, name, target, scratch string, flags []string) (BuiltBinary, error) {
	goos, arch, err := splitTarget(target)
	if err != nil {
		return BuiltBinary{}, err
	}
	outA := filepath.Join(scratch, "build-a-"+name)
	outB := filepath.Join(scratch, "build-b-"+name)
	homeA := filepath.Join(scratch, "home-a-"+name)
	homeB := filepath.Join(scratch, "home-b-"+name)
	for _, dir := range []string{outA, outB, homeA, homeB} {
		if err := mkdirScratchDir(dir); err != nil {
			return BuiltBinary{}, err
		}
	}
	binA := filepath.Join(outA, name)
	binB := filepath.Join(outB, name)

	goPath, err := goBinary()
	if err != nil {
		return BuiltBinary{}, err
	}
	if err := runGoBuild(ctx, goPath, moduleRoot, pkgPath, binA, closedGoEnv(homeA, target), flags); err != nil {
		return BuiltBinary{}, fmt.Errorf("%s: primary build: %w", name, err)
	}
	if err := runGoBuild(ctx, goPath, moduleRoot, pkgPath, binB, closedGoEnv(homeB, target), flags); err != nil {
		return BuiltBinary{}, fmt.Errorf("%s: independent build: %w", name, err)
	}

	dataA, err := os.ReadFile(binA)
	if err != nil {
		return BuiltBinary{}, err
	}
	dataB, err := os.ReadFile(binB)
	if err != nil {
		return BuiltBinary{}, err
	}
	if !bytes.Equal(dataA, dataB) {
		return BuiltBinary{}, fmt.Errorf("%s: two independent builds produced different bytes (%d vs %d)", name, len(dataA), len(dataB))
	}

	if err := verifyBuildInfo(binA, goos, arch); err != nil {
		return BuiltBinary{}, fmt.Errorf("%s: %w", name, err)
	}

	sum := sha256.Sum256(dataA)
	return BuiltBinary{
		Name:   name,
		Path:   binA,
		SHA256: hex.EncodeToString(sum[:]),
		Size:   int64(len(dataA)),
	}, nil
}

// stageBuildSource writes export's hash-verified files into a fresh dir and
// re-verifies the staged tree against the export — exactly the export's file
// set, each file's bytes matching its Git object id — before it is used as a
// module root. The binary is therefore built from the same content its source
// archive ships, not from the live checkout.
func stageBuildSource(export Export, dir string) (string, error) {
	if err := os.RemoveAll(dir); err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	if err := materializeExport(export, dir); err != nil {
		return "", err
	}
	if err := verifyStagedTree(export, dir); err != nil {
		return "", err
	}
	return dir, nil
}

func verifyStagedTree(export Export, dir string) error {
	want := make(map[string]string, len(export.Files))
	for _, f := range export.Files {
		want[f.Path] = f.OID
	}
	staged := 0
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		oid, ok := want[filepath.ToSlash(rel)]
		if !ok {
			return fmt.Errorf("staged build tree has a file the export lacks: %s", rel)
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("staged build tree entry %s is not a regular file", rel)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := verifyBlobDigest(oid, data); err != nil {
			return fmt.Errorf("staged build tree %s: %w", rel, err)
		}
		staged++
		return nil
	})
	if err != nil {
		return err
	}
	if staged != len(want) {
		return fmt.Errorf("staged build tree has %d files, export has %d", staged, len(want))
	}
	return nil
}

func runGoBuild(ctx context.Context, goPath, moduleRoot, pkgPath, out string, env, flags []string) error {
	arguments := []string{"build", "-trimpath", "-buildvcs=false"}
	arguments = append(arguments, flags...)
	arguments = append(arguments, "-o", out, pkgPath)
	_, stderr, err := runCaptured(ctx, moduleRoot, env, buildTimeout,
		append([]string{goPath}, arguments...)...)
	if err != nil {
		return fmt.Errorf("%w (stderr=%s)", err, trimForError(stderr))
	}
	if info, statErr := os.Stat(out); statErr != nil || info.Size() == 0 {
		return fmt.Errorf("go build produced no output at %s", out)
	}
	return nil
}

func sourceBuildNumber(ctx context.Context, gitPath, root, scratch string) (string, error) {
	stdout, _, err := runCaptured(ctx, root, closedGitEnv(scratch), subprocessTimeout,
		gitPath, "rev-list", "--first-parent", "--count", "HEAD")
	if err != nil {
		return "", fmt.Errorf("resolve first-parent build number: %w", err)
	}
	build := strings.TrimSpace(string(stdout))
	n, err := strconv.ParseUint(build, 10, 64)
	if err != nil || n == 0 || strconv.FormatUint(n, 10) != build {
		return "", fmt.Errorf("invalid first-parent build number %q", build)
	}
	return build, nil
}

// verifyBuildInfo requires the exact pinned Go version, target, and the
// closed build flags (-trimpath, buildvcs=false, cgo disabled) to be present
// in the binary's own embedded build info, rather than trusting the
// invocation that produced it.
func verifyBuildInfo(path, goos, arch string) error {
	info, err := buildinfo.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read buildinfo: %w", err)
	}
	if info.GoVersion != requiredGoVersion {
		return fmt.Errorf("buildinfo Go version %q != required %q", info.GoVersion, requiredGoVersion)
	}
	settings := map[string]string{}
	for _, s := range info.Settings {
		settings[s.Key] = s.Value
	}
	if settings["GOOS"] != goos || settings["GOARCH"] != arch {
		return fmt.Errorf("buildinfo target %s/%s != requested %s/%s", settings["GOOS"], settings["GOARCH"], goos, arch)
	}
	if settings["CGO_ENABLED"] != "0" {
		return fmt.Errorf("buildinfo CGO_ENABLED=%q, want \"0\"", settings["CGO_ENABLED"])
	}
	if settings["-trimpath"] != "true" {
		return fmt.Errorf("buildinfo -trimpath=%q, want \"true\"", settings["-trimpath"])
	}
	if v, ok := settings["vcs"]; ok && v != "" {
		return fmt.Errorf("buildinfo recorded vcs info %q despite -buildvcs=false", v)
	}
	return nil
}

func goBinary() (string, error) {
	return exec.LookPath("go")
}
