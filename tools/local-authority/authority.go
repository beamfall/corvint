package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/authoritystore"
	"github.com/Beamfall/corvint/internal/localauthority"
)

type enrolledInput struct {
	root       authoritystore.RootDocument
	floor      authoritystore.GenerationFloor
	policy     executionPolicy
	enrollment localauthority.Enrollment
	raw        map[string][]byte
	source     []byte
	capsule    capsule
}

func decodeHex(s string, n int) ([]byte, error) {
	b, e := hex.DecodeString(s)
	if e != nil || len(b) != n || strings.ToLower(s) != s {
		return nil, errors.New("hex identity")
	}
	return b, nil
}
func safeRead(path string, limit int, owner uint32) ([]byte, error) {
	relative, e := filepath.Rel(authoritystore.RootPath, path)
	if e != nil || strings.HasPrefix(relative, "../") || relative == ".." {
		return nil, errors.New("outside protected store")
	}
	return authoritystore.ReadExecutionFile(relative, owner, limit)
}
func sourceIdentity(source []byte) string {
	manifest := struct{ Codec, Wrapper, Module string }{digest(source), digest(wrapper), digest([]byte(moduleText))}
	return digest(mustJSONLine(manifest))
}
func loadEnrolled(handle string) (enrolledInput, error) {
	var in enrolledInput
	if _, e := decodeHex(handle, 32); e != nil {
		return in, e
	}
	root, floor, policy, e := loadExecution()
	if e != nil {
		return in, e
	}
	if e = requireAuthority(root); e != nil {
		return in, e
	}
	uid := uint32(os.Geteuid())
	in.root = root
	in.floor = floor
	in.policy = policy
	in.raw = map[string][]byte{}
	for _, name := range []string{"enrollment.json", "objects.json", "cem.json", "ocm.json", "selection.json", "target.json"} {
		limit := 128 << 10
		if name == "objects.json" {
			limit = 24 << 20
		}
		if name == "cem.json" || name == "ocm.json" {
			limit = 4 << 20
		}
		raw, e := safeRead(filepath.Join(authoritystore.RootPath, "private", "enrollments", handle, name), limit, uid)
		if e != nil {
			return in, e
		}
		in.raw[name] = raw
	}
	if e = localauthority.Decode(in.raw["enrollment.json"], &in.enrollment); e != nil {
		return in, e
	}
	enrollment := in.enrollment
	if e = localauthority.ValidateEnrollment(enrollment); e != nil {
		return in, e
	}
	actual, e := localauthority.Digest(enrollment)
	if e != nil || actual != handle {
		return in, errors.New("enrollment handle")
	}
	if enrollment.RootID != root.RootID || enrollment.RepositoryID != root.RepositoryID || enrollment.PolicySHA256 != root.PolicySHA256 || enrollment.Audience != root.Audience || enrollment.Epoch != root.Epoch || enrollment.Generation != root.Generation || !samePolicy(enrollment.Checks, root.Checks) {
		return in, errors.New("enrollment policy")
	}
	if len(enrollment.Checks) != 1 {
		return in, errors.New("unsupported check floor")
	}
	check := enrollment.Checks[0]
	if check.ID != CheckID || check.DriverPath != "tools/local-authority/driver.go" || check.DriverUnit != "TestCanonicalOutput" || check.ClaimSelector != "test:TestCanonicalOutput/case:ple-v0-canonical-vector" || check.Invocation != InvocationID || check.Subject != "internal/wp3codec/codec.go" || check.DriverSHA256 != digest(driverSource) {
		return in, errors.New("driver policy")
	}
	if enrollment.Binding.WorkerSHA256 != policy.Worker.SHA256 || enrollment.Binding.RecipeSHA256 != policy.RecipeSHA256 {
		return in, errors.New("build policy")
	}
	if digest(in.raw["cem.json"]) != enrollment.Binding.CEMSHA256 || digest(in.raw["selection.json"]) != enrollment.Binding.OCMSHA256 {
		return in, errors.New("artifact binding")
	}
	var target authoritystore.Target
	if e = localauthority.Decode(in.raw["target.json"], &target); e != nil {
		return in, e
	}
	if target.Profile != authoritystore.TargetProfile || target.RepositoryID != root.RepositoryID || target.RepositoryRoot != root.RepositoryRoot || target.Base != enrollment.Binding.Base || target.Target != enrollment.Binding.Target || target.Tree != enrollment.Binding.Tree {
		return in, errors.New("target binding")
	}
	var c capsule
	if e = decodeCapsule(in.raw["objects.json"], &c); e != nil {
		return in, e
	}
	c, objects, e := completeCapsule(c, target.Base, target.Target, target.Tree)
	in.capsule = c
	if e != nil {
		return in, e
	}
	if e = objects.requireEvidencePaths(in); e != nil {
		return in, e
	}
	tree, e := objects.tree(target.Target)
	if e != nil || tree != target.Tree {
		return in, errors.New("target tree")
	}
	if e = objects.onlyCodecPackage(target.Target); e != nil {
		return in, e
	}
	source, e := objects.path(target.Target, check.Subject)
	if e != nil {
		return in, e
	}
	if e = validateSource(source); e != nil {
		return in, e
	}
	if sourceIdentity(source) != enrollment.Binding.SourceSHA256 {
		return in, errors.New("source manifest")
	}
	in.source = source
	before, e := objects.path(target.Base, check.DriverPath)
	if e != nil {
		return in, e
	}
	after, e := objects.path(target.Target, check.DriverPath)
	if e != nil {
		return in, e
	}
	if !bytes.Equal(before, after) || digest(after) != check.DriverSHA256 {
		return in, errors.New("driver source identity")
	}
	return in, nil
}
func authorityPhase(phase, handle string) error {
	in, e := loadEnrolled(handle)
	if e != nil {
		return e
	}
	nonce := in.enrollment.Nonce
	journal := filepath.Join(authoritystore.RootPath, "private", "journal", nonce)
	switch phase {
	case "authority-prepare":
		if e = exclusiveFile(journal+".pending", []byte(handle), 0600); e != nil {
			return fmt.Errorf("nonce already used or journal unavailable: %w", e)
		}
		return writeWork(work{Nonce: nonce, Mode: "build", Go: in.policy.Go.Path, Source: in.source})
	case "authority-built":
		if e = pending(journal, handle); e != nil {
			return e
		}
		t, e := readTerminal(nonce)
		if e != nil {
			return e
		}
		if digest(t.Wasm) != in.enrollment.Binding.WasmSHA256 {
			return errors.New("compiled artifact mismatch")
		}
		if e = exclusiveFile(journal+".built", []byte(in.enrollment.Binding.WasmSHA256), 0600); e != nil {
			return e
		}
		return writeWork(work{Nonce: nonce, Mode: "execute", Wasm: t.Wasm, Input: driverInput()})
	case "authority-finish":
		if e = pending(journal, handle); e != nil {
			return e
		}
		built, e := safeRead(journal+".built", 64, uint32(os.Geteuid()))
		if e != nil || string(built) != in.enrollment.Binding.WasmSHA256 {
			return errors.New("missing build lifecycle")
		}
		t, e := readTerminal(nonce)
		if e != nil {
			return e
		}
		if t.ModuleSHA256 != in.enrollment.Binding.WasmSHA256 {
			return errors.New("executed artifact mismatch")
		}
		status := "PASS"
		if e = TestCanonicalOutput(t.Output); e != nil {
			status = "FAIL"
		}
		return signAndPublish(handle, journal, in, status)
	}
	return errors.New("unknown authority phase")
}
func readTerminal(nonce string) (terminal, error) {
	var t terminal
	raw, e := readBound(os.Stdin, 48<<20)
	if e != nil {
		return t, e
	}
	if e = strictDecode(raw, &t); e != nil {
		return t, e
	}
	if t.Nonce != nonce || t.Phase != "reaped" || t.Error != "" {
		return t, errors.New("unsuccessful worker lifecycle")
	}
	return t, nil
}
func pending(journal, handle string) error {
	raw, e := safeRead(journal+".pending", 64, uint32(os.Geteuid()))
	if e != nil || string(raw) != handle {
		return errors.New("nonce not pending")
	}
	if _, e = os.Lstat(journal + ".terminal"); !os.IsNotExist(e) {
		return errors.New("nonce terminal already exists")
	}
	return nil
}
func writeWork(w work) error { _, e := os.Stdout.Write(mustJSONLine(w)); return e }
func exclusiveFile(path string, raw []byte, mode os.FileMode) error {
	f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if e != nil {
		return e
	}
	if _, e = f.Write(raw); e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e != nil {
		return e
	}
	if closeErr != nil {
		return closeErr
	}
	d, e := os.Open(filepath.Dir(path))
	if e != nil {
		return e
	}
	defer d.Close()
	return d.Sync()
}
func signAndPublish(handle, journal string, in enrolledInput, status string) error {
	// Reread protected current policy after all worker cleanup, before touching the key.
	root, floor, policy, e := loadExecution()
	if e != nil {
		return e
	}
	if !samePolicy(root, in.root) || !samePolicy(floor, in.floor) || !samePolicy(policy, in.policy) {
		return errors.New("policy drift during execution")
	}
	key, e := safeRead(filepath.Join(authoritystore.RootPath, "private", "signing-key"), ed25519.PrivateKeySize, uint32(os.Geteuid()))
	if e != nil || len(key) != ed25519.PrivateKeySize {
		return errors.New("signing key unavailable")
	}
	defer clear(key)
	public, e := decodeHex(root.PublicKey, ed25519.PublicKeySize)
	if e != nil || !bytes.Equal(ed25519.PrivateKey(key).Public().(ed25519.PublicKey), public) {
		return errors.New("key not independently admitted")
	}
	rows := []localauthority.Row{}
	for _, check := range in.enrollment.Checks {
		rows = append(rows, localauthority.Row{Check: check, Status: status})
	}
	result := localauthority.Receipt{Payload: localauthority.Payload{Enrollment: in.enrollment, CompletedAt: strconv.FormatInt(time.Now().Unix(), 10), Cleanup: "COMPLETE", ExitCode: "0", Rows: rows}}
	signing, e := localauthority.SigningBytes(result.Payload)
	if e != nil {
		return e
	}
	result.Signature = hex.EncodeToString(ed25519.Sign(ed25519.PrivateKey(key), signing))
	hash, e := localauthority.Digest(result)
	if e != nil {
		return e
	}
	snapshot, e := currentPolicy(root, floor, hash, in.enrollment.Nonce)
	if e != nil {
		return e
	}
	if e = localauthority.Verify(result, in.enrollment, snapshot, time.Now()); e != nil {
		return e
	}
	terminal := authoritystore.Terminal{Profile: authoritystore.TerminalProfile, Nonce: in.enrollment.Nonce, State: "COMPLETE", ReceiptSHA256: hash}
	terminalBytes, e := localauthority.Canonical(terminal)
	if e != nil {
		return e
	}
	// The durable private terminal precedes public visibility. A crash can leave a
	// complete private terminal without publication; it can never expose partial success.
	if e = exclusiveFile(journal+".terminal", terminalBytes, 0600); e != nil {
		return e
	}
	resultBytes, e := localauthority.Canonical(result)
	if e != nil {
		return e
	}
	files := map[string][]byte{}
	for _, name := range []string{"enrollment.json", "cem.json", "ocm.json", "selection.json", "target.json"} {
		files[name] = in.raw[name]
	}
	view, e := publishEvidence(in, handle)
	if e != nil {
		return e
	}
	viewBytes, e := localauthority.Canonical(view)
	if e != nil {
		return e
	}
	files["evidence.json"] = viewBytes
	files["receipt.json"] = resultBytes
	files["terminal.json"] = terminalBytes
	return publish(handle, files)
}
func publish(handle string, files map[string][]byte) error {
	return publishAt(filepath.Join(authoritystore.RootPath, "public"), handle, files)
}
func publishAt(root, handle string, files map[string][]byte) error {
	stage, e := os.MkdirTemp(root, ".pending-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(stage)
	for name, raw := range files {
		if e = exclusiveFile(filepath.Join(stage, name), raw, 0444); e != nil {
			return e
		}
	}
	if e = os.Chmod(stage, 0755); e != nil {
		return e
	}
	destination := filepath.Join(root, handle)
	if _, e = os.Lstat(destination); !os.IsNotExist(e) {
		return errors.New("publication already exists")
	}
	if e = os.Rename(stage, destination); e != nil {
		return e
	}
	if e = publishActive(root, handle); e != nil {
		return e
	}
	d, e := os.Open(root)
	if e != nil {
		return e
	}
	defer d.Close()
	return d.Sync()
}

func publishActive(root, handle string) error {
	active := struct {
		Profile          string `json:"profile"`
		EnrollmentHandle string `json:"enrollmentHandle"`
	}{"corvint-protected-active-enrollment/0", handle}
	raw, e := localauthority.Canonical(active)
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(root, ".active-")
	if e != nil {
		return e
	}
	name := f.Name()
	defer os.Remove(name)
	if _, e = f.Write(raw); e == nil {
		e = f.Chmod(0444)
	}
	if e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e != nil {
		return e
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(name, filepath.Join(root, "active-enrollment.json"))
}
