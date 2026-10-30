package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/authoritystore"
	"github.com/Beamfall/corvint/internal/localauthority"
)

type executionPolicy struct {
	Profile             string               `json:"profile"`
	Worker              authoritystore.Image `json:"worker"`
	Go                  authoritystore.Image `json:"go"`
	GoRootSHA256        string               `json:"goRootSHA256"`
	DriverSHA256        string               `json:"driverSHA256"`
	RecipeSHA256        string               `json:"recipeSHA256"`
	RuntimeModule       string               `json:"runtimeModule"`
	RuntimeVersion      string               `json:"runtimeVersion"`
	RuntimeSum          string               `json:"runtimeSum"`
	BuildDeadlineMillis string               `json:"buildDeadlineMillis"`
	RunDeadlineMillis   string               `json:"runDeadlineMillis"`
	MemoryPages         string               `json:"memoryPages"`
	WorkerRSSKiB        string               `json:"workerRSSKiB"`
	BuildRSSKiB         string               `json:"buildRSSKiB"`
}

func policyConstants(p executionPolicy) bool {
	return p.Profile == "corvint-protected-execution-policy/0" && p.DriverSHA256 == digest(driverSource) && p.RecipeSHA256 == digest([]byte(recipe)) && p.RuntimeModule == "github.com/tetratelabs/wazero" && p.RuntimeVersion == "v1.12.0" && p.RuntimeSum == "h1:DuWcpNu/FzgEXgGBDp8J1Spc+CWOvvtvVyjKlaZopYU=" && p.BuildDeadlineMillis == "60000" && p.RunDeadlineMillis == "5000" && p.MemoryPages == "8192" && p.WorkerRSSKiB == "786432" && p.BuildRSSKiB == "2097152"
}
func loadExecution() (authoritystore.RootDocument, authoritystore.GenerationFloor, executionPolicy, error) {
	root, floor, e := authoritystore.LoadExecutionRoot()
	if e != nil {
		return root, floor, executionPolicy{}, e
	}
	raw, e := authoritystore.ReadExecutionPolicy()
	if e != nil {
		return root, floor, executionPolicy{}, e
	}
	var p executionPolicy
	if e = localauthority.Decode(raw, &p); e != nil {
		return root, floor, p, e
	}
	if localauthority.BytesDigest(raw) != root.PolicySHA256 || !policyConstants(p) {
		return root, floor, p, errors.New("execution policy identity")
	}
	exe, e := os.Executable()
	if e != nil {
		return root, floor, p, e
	}
	if exe != p.Worker.Path {
		return root, floor, p, errors.New("worker path")
	}
	if !strings.HasPrefix(exe, authoritystore.RootPath+"/versions/") {
		return root, floor, p, errors.New("release path")
	}
	binary, e := safeRead(exe, 64<<20, 0)
	if e != nil {
		return root, floor, p, e
	}
	if digest(binary) != p.Worker.SHA256 {
		return root, floor, p, errors.New("worker image")
	}
	tool, e := safeRead(p.Go.Path, 64<<20, 0)
	if e != nil {
		return root, floor, p, e
	}
	if digest(tool) != p.Go.SHA256 {
		return root, floor, p, errors.New("toolchain image")
	}
	if p.Go.Path != filepath.Join(filepath.Dir(exe), "go", "bin", "go") {
		return root, floor, p, errors.New("toolchain location")
	}
	tree, e := treeDigest(filepath.Dir(filepath.Dir(p.Go.Path)), true)
	if e != nil {
		return root, floor, p, e
	}
	if tree != p.GoRootSHA256 {
		return root, floor, p, errors.New("toolchain tree")
	}
	return root, floor, p, nil
}
func treeDigest(root string, protected bool) (string, error) {
	type entry struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
	}
	entries := []entry{}
	total := int64(0)
	e := filepath.WalkDir(root, func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.Type()&os.ModeSymlink != 0 {
			return errors.New("toolchain symlink")
		}
		if d.IsDir() {
			return nil
		}
		info, e := d.Info()
		if e != nil {
			return e
		}
		if !info.Mode().IsRegular() {
			return errors.New("toolchain special file")
		}
		total += info.Size()
		if total > 2<<30 || len(entries) >= 100000 {
			return errors.New("toolchain bound")
		}
		var b []byte
		if protected {
			b, e = safeRead(path, 64<<20, 0)
		} else {
			b, e = os.ReadFile(path)
		}
		if e != nil {
			return e
		}
		relative, e := filepath.Rel(root, path)
		if e != nil {
			return e
		}
		entries = append(entries, entry{filepath.ToSlash(relative), digest(b)})
		return nil
	})
	if e != nil {
		return "", e
	}
	return localauthority.Digest(entries)
}
func currentPolicy(root authoritystore.RootDocument, floor authoritystore.GenerationFloor, receiptHash, nonce string) (localauthority.PolicySnapshot, error) {
	key, e := decodeHex(root.PublicKey, 32)
	if e != nil {
		return localauthority.PolicySnapshot{}, e
	}
	return localauthority.PolicySnapshot{Accepted: root.Admission == "OPERATOR_ACCEPTED", Current: true, Fixture: root.KeyClass != "PRODUCTION", Revoked: root.Revoked, RootID: root.RootID, RepositoryID: root.RepositoryID, PolicySHA256: root.PolicySHA256, PublicKey: key, Epoch: root.Epoch, Generation: root.Generation, MinimumGeneration: floor.Generation, Audience: root.Audience, Checks: root.Checks, TerminalSHA256: map[string]string{nonce: receiptHash}}, nil
}
func requireAuthority(root authoritystore.RootDocument) error {
	uid, e := strconv.Atoi(root.AuthorityUID)
	if e != nil || uid == 0 || uid != os.Geteuid() {
		return errors.New("dedicated authority identity required")
	}
	return nil
}
func samePolicy(a, b any) bool { return reflect.DeepEqual(a, b) }
