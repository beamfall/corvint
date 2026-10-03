// Copyright 2026 Corvint contributors.
// SPDX-License-Identifier: AGPL-3.0-or-later
package postmergehost

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"errors"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/intake"
	"github.com/Beamfall/corvint/internal/procgroup"
	"github.com/Beamfall/corvint/internal/stepverify"
)

const HostWireLimit = 4 << 20

var ErrHostInput = errors.New("invalid paired host input")

type HostFileRef struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}
type HostEnvironment struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Class string `json:"class"`
}
type HostImplementation struct {
	SourceCommit    string `json:"source_commit"`
	SourceTree      string `json:"source_tree"`
	LauncherSHA256  string `json:"launcher_sha256"`
	GuestShimSHA256 string `json:"guest_shim_sha256"`
}
type HostEngine struct {
	Context       string `json:"context"`
	ClientSHA256  string `json:"client_sha256"`
	ServerVersion string `json:"server_version"`
	Platform      string `json:"platform"`
}
type HostImage struct {
	ManifestDigest  string            `json:"manifest_digest"`
	ConfigDigest    string            `json:"config_digest"`
	Environment     []HostEnvironment `json:"environment"`
	HoldEnvironment []HostEnvironment `json:"hold_environment"`
}
type HostAdmission struct {
	ProductBase         string `json:"product_base"`
	ProductMerge        string `json:"product_merge"`
	ProductTree         string `json:"product_tree"`
	DeclarationSHA256   string `json:"declaration_sha256"`
	AdmittedInputSHA256 string `json:"admitted_input_sha256"`
	AuthorSHA256        string `json:"author_sha256"`
}
type HostCommand struct {
	Executable       string            `json:"executable"`
	Argv             []string          `json:"argv"`
	WorkingDirectory string            `json:"working_directory"`
	Environment      []HostEnvironment `json:"environment"`
}
type HostLimits struct {
	CPUsNanos             int64 `json:"cpus_nanos"`
	MemoryBytes           int64 `json:"memory_bytes"`
	SwapBytes             int64 `json:"swap_bytes"`
	PIDs                  int64 `json:"pids"`
	ShmBytes              int64 `json:"shm_bytes"`
	PrivateTmpfsBytes     int64 `json:"private_tmpfs_bytes"`
	AuthorTimeoutSeconds  int64 `json:"author_timeout_seconds"`
	EngineTimeoutSeconds  int64 `json:"engine_timeout_seconds"`
	CleanupTimeoutSeconds int64 `json:"cleanup_timeout_seconds"`
	OutputBytes           int64 `json:"output_bytes"`
}
type HostProfile struct {
	Profile        string             `json:"profile"`
	Mode           string             `json:"mode"`
	Implementation HostImplementation `json:"implementation"`
	Engine         HostEngine         `json:"engine"`
	Image          HostImage          `json:"image"`
	Admission      HostAdmission      `json:"admission"`
	Command        HostCommand        `json:"command"`
	Limits         HostLimits         `json:"limits"`
}
type HostProduct struct {
	Root  string `json:"root"`
	Base  string `json:"base"`
	Merge string `json:"merge"`
	Tree  string `json:"tree"`
}
type HostObserver struct {
	ID        string      `json:"id"`
	GitBinary HostFileRef `json:"git_binary"`
}
type HostRequest struct {
	Profile          string       `json:"profile"`
	RunID            string       `json:"run_id"`
	OperatorManifest HostFileRef  `json:"operator_manifest"`
	Product          HostProduct  `json:"product"`
	Declaration      HostFileRef  `json:"declaration"`
	Observer         HostObserver `json:"observer"`
	AdmittedInput    HostFileRef  `json:"admitted_input"`
	Author           HostFileRef  `json:"author"`
	ExecShim         HostFileRef  `json:"exec_shim"`
	OutputRoot       string       `json:"output_root"`
}
type HostControl struct {
	Profile       string       `json:"profile"`
	Operation     string       `json:"operation"`
	RunID         string       `json:"run_id"`
	RequestSHA256 string       `json:"request_sha256"`
	ProfileSHA256 string       `json:"profile_sha256"`
	Host          *HostFileRef `json:"host"`
	Before        *HostFileRef `json:"before"`
	Preflight     *HostFileRef `json:"preflight"`
	Reason        *string      `json:"reason"`
}
type HostFact struct {
	Kind   string      `json:"kind"`
	Record HostFileRef `json:"record"`
}
type HostAuthorExit struct {
	Code     *int    `json:"code"`
	Signal   *string `json:"signal"`
	TimedOut bool    `json:"timed_out"`
}
type HostEvent struct {
	Profile       string          `json:"profile"`
	Event         string          `json:"event"`
	RunID         string          `json:"run_id"`
	RequestSHA256 string          `json:"request_sha256"`
	ProfileSHA256 string          `json:"profile_sha256"`
	ContainerID   *string         `json:"container_id"`
	Facts         []HostFact      `json:"facts"`
	AuthorExit    *HostAuthorExit `json:"author_exit"`
	Cleanup       string          `json:"cleanup"`
	Reasons       []string        `json:"reasons"`
}

var hostSHA = regexp.MustCompile(`^[0-9a-f]{64}$`)
var hostOID = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
var hostRun = regexp.MustCompile(`^[0-9a-f]{32}$`)
var hostName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)
var hostEnvKey = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,127}$`)

func hostDigest(data []byte) string { h := sha256.Sum256(data); return hex.EncodeToString(h[:]) }
func hostSafe(s string) bool {
	return utf8.ValidString(s) && !strings.ContainsFunc(s, unicode.IsControl)
}
func hostPath(s string) bool {
	return len(s) <= 512 && hostSafe(s) && filepath.IsAbs(s) && filepath.Clean(s) == s && !strings.ContainsAny(s, ",\\")
}
func hostRefShape(r HostFileRef) bool {
	return hostPath(r.Path) && hostSHA.MatchString(r.SHA256) && r.Bytes >= 0 && r.Bytes <= 16<<20
}
func hostEnvironments(env []HostEnvironment) bool {
	if env == nil || len(env) > 128 {
		return false
	}
	seen := map[string]bool{}
	for _, v := range env {
		if !hostEnvKey.MatchString(v.Key) || seen[v.Key] || !hostSafe(v.Value) || len(v.Value) > 4096 || v.Class != "NONE" {
			return false
		}
		seen[v.Key] = true
	}
	return true
}
func hostExpectedEnvironment() []HostEnvironment {
	return []HostEnvironment{{"HOME", "/private/home", "NONE"}, {"TMPDIR", "/private/tmp", "NONE"}, {"PATH", "/usr/local/go/bin:/usr/bin:/bin", "NONE"}}
}
func hostExpectedLimits() HostLimits {
	return HostLimits{2000000000, 8 << 30, 8 << 30, 256, 1 << 20, 16 << 20, 30, 30, 60, 16 << 20}
}

// The native strict parser preserves integer, duplicate-key, depth and Unicode
// rules. The typed round trip also rejects omitted fields and case aliases.
func decodeHost(data []byte, dst any) error {
	if len(data) > HostWireLimit {
		return ErrHostInput
	}
	parsed, err := wire.Parse(data)
	if err != nil {
		return ErrHostInput
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(dst) != nil {
		return ErrHostInput
	}
	encoded, err := json.Marshal(dst)
	if err != nil {
		return ErrHostInput
	}
	round, err := wire.Parse(encoded)
	if err != nil || !bytes.Equal(wire.CanonicalValue(parsed), wire.CanonicalValue(round)) {
		return ErrHostInput
	}
	return nil
}
func DecodeHostProfile(data []byte) (HostProfile, error) {
	var p HostProfile
	if decodeHost(data, &p) != nil || p.Profile != "corvint-postmerge-author-host/0" || p.Mode != "candidate-qualification" {
		return p, ErrHostInput
	}
	i := p.Implementation
	a := p.Admission
	for _, oid := range []string{i.SourceCommit, i.SourceTree, a.ProductBase, a.ProductMerge, a.ProductTree} {
		if !hostOID.MatchString(oid) {
			return p, ErrHostInput
		}
	}
	for _, sha := range []string{i.LauncherSHA256, i.GuestShimSHA256, p.Engine.ClientSHA256, a.DeclarationSHA256, a.AdmittedInputSHA256, a.AuthorSHA256} {
		if !hostSHA.MatchString(sha) {
			return p, ErrHostInput
		}
	}
	if !hostName.MatchString(p.Engine.Context) || !hostName.MatchString(p.Engine.ServerVersion) || p.Engine.Platform != "linux/amd64" {
		return p, ErrHostInput
	}
	for _, d := range []string{p.Image.ManifestDigest, p.Image.ConfigDigest} {
		if !strings.HasPrefix(d, "sha256:") || !hostSHA.MatchString(strings.TrimPrefix(d, "sha256:")) {
			return p, ErrHostInput
		}
	}
	if !hostEnvironments(p.Image.Environment) || !hostEnvironments(p.Image.HoldEnvironment) || !reflect.DeepEqual(p.Command.Environment, hostExpectedEnvironment()) {
		return p, ErrHostInput
	}
	if p.Command.Executable != "/tools/author" || p.Command.WorkingDirectory != "/product" || !reflect.DeepEqual(p.Command.Argv, []string{"/admitted/author-input.json", "/product"}) || p.Limits != hostExpectedLimits() {
		return p, ErrHostInput
	}
	return p, nil
}
func DecodeHostRequest(data []byte, p HostProfile) (HostRequest, error) {
	var r HostRequest
	if decodeHost(data, &r) != nil || r.Profile != "corvint-postmerge-author-request/0" || !hostRun.MatchString(r.RunID) || !hostPath(r.Product.Root) || !hostPath(r.OutputRoot) || r.Product.Root == r.OutputRoot {
		return r, ErrHostInput
	}
	if r.Observer.ID != "observer_"+r.RunID || r.Product.Base != p.Admission.ProductBase || r.Product.Merge != p.Admission.ProductMerge || r.Product.Tree != p.Admission.ProductTree {
		return r, ErrHostInput
	}
	for _, ref := range []HostFileRef{r.OperatorManifest, r.Declaration, r.Observer.GitBinary, r.AdmittedInput, r.Author, r.ExecShim} {
		if !hostRefShape(ref) {
			return r, ErrHostInput
		}
	}
	if r.AdmittedInput.Bytes > 1<<20 || r.AdmittedInput.SHA256 != p.Admission.AdmittedInputSHA256 || r.Author.SHA256 != p.Admission.AuthorSHA256 || r.ExecShim.SHA256 != p.Implementation.GuestShimSHA256 {
		return r, ErrHostInput
	}
	return r, nil
}
func DecodeHostControl(data []byte, r HostRequest, requestSHA, profileSHA string) (HostControl, error) {
	var c HostControl
	if decodeHost(data, &c) != nil || c.Profile != "corvint-postmerge-host-control/0" || c.RunID != r.RunID || !hostSHA.MatchString(requestSHA) || !hostSHA.MatchString(profileSHA) || c.RequestSHA256 != requestSHA || c.ProfileSHA256 != profileSHA {
		return c, ErrHostInput
	}
	switch c.Operation {
	case "execute":
		if c.Reason != nil {
			return c, ErrHostInput
		}
		for _, ref := range []*HostFileRef{c.Host, c.Before, c.Preflight} {
			if ref == nil || !hostRefShape(*ref) || !hostWithin(r.OutputRoot, ref.Path) {
				return c, ErrHostInput
			}
		}
	case "cancel":
		if c.Host != nil || c.Before != nil || c.Preflight != nil || c.Reason == nil {
			return c, ErrHostInput
		}
		switch *c.Reason {
		case "supervisor-cancelled", "preflight-blocked", "supervisor-lost":
		default:
			return c, ErrHostInput
		}
	default:
		return c, ErrHostInput
	}
	return c, nil
}
func hostWithin(root, path string) bool {
	return path != root && strings.HasPrefix(path, root+string(filepath.Separator))
}

// Inputs are trusted snapshots but still checked against static aliases and
// changes during the read. This does not assert immunity to concurrent host
// mutation; native observation retains that separate qualification limit.
func readHostFile(ref HostFileRef, limit int64) ([]byte, error) {
	if !hostRefShape(ref) || ref.Bytes > limit {
		return nil, ErrHostInput
	}
	canonical, err := filepath.EvalSymlinks(ref.Path)
	if err != nil || canonical != ref.Path {
		return nil, ErrHostInput
	}
	before, err := os.Lstat(ref.Path)
	if err != nil || !before.Mode().IsRegular() || before.Size() != ref.Bytes || !hostSingleLink(before) {
		return nil, ErrHostInput
	}
	f, err := os.Open(ref.Path)
	if err != nil {
		return nil, ErrHostInput
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return nil, ErrHostInput
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || int64(len(data)) != ref.Bytes || hostDigest(data) != ref.SHA256 {
		return nil, ErrHostInput
	}
	after, err := os.Lstat(ref.Path)
	if err != nil || !os.SameFile(opened, after) || after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) || !hostSingleLink(after) {
		return nil, ErrHostInput
	}
	return data, nil
}
func hostSingleLink(info os.FileInfo) bool {
	// Darwin and Linux expose Nlink. Other platforms fail closed rather than
	// treating an unavailable alias observation as a single link.
	v := reflect.ValueOf(info.Sys())
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return false
	}
	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return false
	}
	n := v.FieldByName("Nlink")
	return n.IsValid() && n.CanUint() && n.Uint() == 1
}

// HostEnvironmentDigest retains names and hashes, never unapproved values.
type HostEnvironmentDigest struct {
	Key    string `json:"key"`
	SHA256 string `json:"sha256"`
}
type HostEnvelopeObservation struct {
	Profile         string                  `json:"profile"`
	AuthorSHA256    string                  `json:"author_sha256"`
	Environment     []HostEnvironmentDigest `json:"environment"`
	HoldEnvironment []HostEnvironmentDigest `json:"hold_environment"`
}

func hostObserveEnvironment(env []string) ([]HostEnvironmentDigest, error) {
	if env == nil || len(env) > 128 {
		return nil, ErrHostInput
	}
	out := make([]HostEnvironmentDigest, 0, len(env))
	seen := map[string]bool{}
	for _, pair := range env {
		key, value, ok := strings.Cut(pair, "=")
		if !ok || !hostEnvKey.MatchString(key) || seen[key] || len(value) > 4096 || !hostSafe(value) {
			return nil, ErrHostInput
		}
		seen[key] = true
		out = append(out, HostEnvironmentDigest{key, hostDigest([]byte(value))})
	}
	return out, nil
}
func hostEnvironmentMatches(actual []HostEnvironmentDigest, want []HostEnvironment) bool {
	if actual == nil || len(actual) != len(want) {
		return false
	}
	expected := map[string]string{}
	for _, v := range want {
		expected[v.Key] = hostDigest([]byte(v.Value))
	}
	seen := map[string]bool{}
	for _, v := range actual {
		if seen[v.Key] || expected[v.Key] != v.SHA256 {
			return false
		}
		seen[v.Key] = true
	}
	return true
}

// HostInputs is validated input material, not authorization, readiness or a
// qualified host. The trusted supervisor owns the original operator manifest's
// semantic interpretation; the launcher retains its exact byte identity.
type HostInputs struct {
	Profile       HostProfile
	Request       HostRequest
	Declaration   stepverify.Declaration
	Intake        intake.Record
	Manifest      []byte
	ProfileSHA256 string
	RequestSHA256 string
	Mounts        []HostMount
}
type HostMount struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	ReadOnly    bool   `json:"read_only"`
}

func hostPinnedFile(path, sha string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, ErrHostInput
	}
	return readHostFile(HostFileRef{path, sha, info.Size()}, limit)
}
func LoadHostInputs(profilePath, profileSHA, requestPath, requestSHA, output string) (*HostInputs, error) {
	data, err := hostPinnedFile(profilePath, profileSHA, HostWireLimit)
	if err != nil {
		return nil, err
	}
	p, err := DecodeHostProfile(data)
	if err != nil {
		return nil, err
	}
	data, err = hostPinnedFile(requestPath, requestSHA, HostWireLimit)
	if err != nil {
		return nil, err
	}
	r, err := DecodeHostRequest(data, p)
	if err != nil || output != r.OutputRoot {
		return nil, ErrHostInput
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(output))
	if err != nil || parent != filepath.Dir(output) {
		return nil, ErrHostInput
	}
	if _, err = os.Lstat(output); !os.IsNotExist(err) {
		return nil, ErrHostInput
	}
	canonical, err := filepath.EvalSymlinks(r.Product.Root)
	if err != nil || canonical != r.Product.Root || hostWithin(r.Product.Root, output) || hostWithin(output, r.Product.Root) {
		return nil, ErrHostInput
	}
	// Every operator/tool/native authority stays outside the author-visible root.
	for _, path := range []string{profilePath, requestPath, r.OperatorManifest.Path, r.Declaration.Path, r.Observer.GitBinary.Path, r.AdmittedInput.Path, r.Author.Path, r.ExecShim.Path} {
		if path == r.Product.Root || hostWithin(r.Product.Root, path) || path == output || hostWithin(output, path) {
			return nil, ErrHostInput
		}
	}
	data, err = readHostFile(r.Declaration, HostWireLimit)
	if err != nil {
		return nil, err
	}
	d, err := stepverify.DecodeDeclaration(data)
	if err != nil {
		return nil, ErrHostInput
	}
	if d.SessionID != "run_"+r.RunID || d.CapabilityID != "authoring" || d.Author.ID != "author" || d.Author.Root != r.Product.Root || stepverify.DeclarationDigest(d) != p.Admission.DeclarationSHA256 || len(d.ReadOnly) != 0 || !reflect.DeepEqual(d.WritePaths, []string{"docs/", "tests/"}) || !reflect.DeepEqual(d.GuardPaths, []string{"calc.go", "docs/.keep", "go.mod", "tests/.keep"}) {
		return nil, ErrHostInput
	}
	keys := []stepverify.EnvironmentKey{{Key: "HOME", Class: "NONE"}, {Key: "PATH", Class: "NONE"}, {Key: "TMPDIR", Class: "NONE"}}
	if !reflect.DeepEqual(d.Environment, keys) {
		return nil, ErrHostInput
	}
	data, err = readHostFile(r.AdmittedInput, 1<<20)
	if err != nil {
		return nil, err
	}
	record, err := intake.Decode(data)
	if err != nil || record.Base != r.Product.Base || record.Head != r.Product.Merge {
		return nil, ErrHostInput
	}
	manifest, err := readHostFile(r.OperatorManifest, HostWireLimit)
	if err != nil {
		return nil, err
	}
	v, err := wire.Parse(manifest)
	if err != nil || v.Kind != wire.KindObject {
		return nil, ErrHostInput
	}
	for _, ref := range []HostFileRef{r.Author, r.ExecShim, r.Observer.GitBinary} {
		if _, err := readHostFile(ref, 16<<20); err != nil {
			return nil, err
		}
		info, err := os.Stat(ref.Path)
		if err != nil || info.Mode()&0111 == 0 {
			return nil, ErrHostInput
		}
	}
	mounts, err := hostCandidateMounts(r, d)
	if err != nil {
		return nil, err
	}
	return &HostInputs{p, r, d, record, manifest, profileSHA, requestSHA, mounts}, nil
}
func hostCandidateMounts(r HostRequest, d stepverify.Declaration) ([]HostMount, error) {
	root := r.Product.Root
	// First candidate supports a normal, non-linked checkout. A .git indirection
	// is refused, not copied or normalized into another native identity.
	git, err := os.Lstat(filepath.Join(root, ".git"))
	if err != nil || !git.IsDir() || git.Mode()&os.ModeSymlink != 0 {
		return nil, ErrHostInput
	}
	for _, name := range []string{"docs", "tests"} {
		i, err := os.Lstat(filepath.Join(root, name))
		if err != nil || !i.IsDir() || i.Mode()&os.ModeSymlink != 0 {
			return nil, ErrHostInput
		}
	}
	count := 0
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return ErrHostInput
		}
		if path == filepath.Join(root, ".git") {
			return filepath.SkipDir
		}

		relative, relativeErr := filepath.Rel(root, path)
		if relativeErr != nil {
			return ErrHostInput
		}
		// This candidate exposes only the reviewed Add fixture. A broad RO root
		// does not make additional product files safe to disclose to its author.
		switch filepath.ToSlash(relative) {
		case ".", "docs", "tests", "calc.go", "go.mod", "docs/.keep", "tests/.keep":
		default:
			return ErrHostInput
		}
		count++
		if count > 20000 {
			return ErrHostInput
		}
		info, err := entry.Info()
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return ErrHostInput
		}
		if info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() || !hostSingleLink(info) {
			return ErrHostInput
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	mounts := []HostMount{{root, "/product", true}, {filepath.Join(root, "docs"), "/product/docs", false}, {filepath.Join(root, "tests"), "/product/tests", false}}
	for _, guard := range d.GuardPaths {
		path := filepath.Join(root, guard)
		i, err := os.Lstat(path)
		if err != nil || !i.Mode().IsRegular() || !hostSingleLink(i) {
			return nil, ErrHostInput
		}
		if guard == "docs/.keep" || guard == "tests/.keep" {
			mounts = append(mounts, HostMount{path, "/product/" + guard, true})
		}
	}
	mounts = append(mounts, HostMount{r.AdmittedInput.Path, "/admitted/author-input.json", true}, HostMount{r.Author.Path, "/tools/author", true}, HostMount{r.ExecShim.Path, "/tools/exec-shim", true})
	return mounts, nil
}
func verifyHostProduct(ctx context.Context, in *HostInputs) error {
	r := in.Request
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"rev-parse", "HEAD^{commit}"}, r.Product.Merge},
		{[]string{"rev-parse", "HEAD^{tree}"}, r.Product.Tree},
		{[]string{"rev-parse", "--absolute-git-dir"}, filepath.Join(r.Product.Root, ".git")},
		{[]string{"rev-parse", r.Product.Base + "^{commit}"}, r.Product.Base},
		{[]string{"status", "--porcelain=v1", "--untracked-files=all"}, ""},
	} {
		args := []string{r.Observer.GitBinary.Path, "--no-optional-locks", "-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false", "-C", r.Product.Root}
		args = append(args, test.args...)
		observation := procgroup.Run(ctx, procgroup.Spec{Argv: args, Dir: r.Product.Root, Env: []string{"PATH=/usr/bin:/bin", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null"}, Timeout: 30 * time.Second, OutputLimit: 4096, StderrLimit: 4096, ObserveDescendants: true})
		if observation.Err != nil || !observation.ExitObserved || observation.ExitStatus != 0 || !observation.WaitCompleted || !observation.OwnedProcessGroupCleanup || observation.OutputOverflow || strings.TrimSpace(string(observation.Stdout)) != test.want {
			return ErrHostInput
		}
	}
	return nil
}
func validateHostNativeControl(c HostControl, in *HostInputs) error {
	if c.Operation != "execute" {
		return ErrHostInput
	}
	data, err := readHostFile(*c.Host, HostWireLimit)
	if err != nil {
		return err
	}
	h, err := stepverify.DecodeHost(data, in.Declaration)
	if err != nil || h.Environment == nil || h.ObserverID != in.Request.Observer.ID || h.GitBinary != in.Request.Observer.GitBinary.Path || h.GitBinaryDigest != in.Request.Observer.GitBinary.SHA256 {
		return ErrHostInput
	}
	if !reflect.DeepEqual(*h.Environment, in.Declaration.Environment) {
		return ErrHostInput
	}
	data, err = readHostFile(*c.Before, 16<<20)
	if err != nil {
		return err
	}
	before, err := stepverify.DecodeState(data, in.Declaration, h)
	if err != nil || !before.Complete || len(before.Checkouts) != 1 || !before.Checkouts[0].Complete || !before.Checkouts[0].ContentComplete || before.Checkouts[0].Root != in.Request.Product.Root || before.Checkouts[0].GitDir != filepath.Join(in.Request.Product.Root, ".git") || before.Checkouts[0].CommonDir != filepath.Join(in.Request.Product.Root, ".git") || before.Checkouts[0].Commit != in.Request.Product.Merge || before.Checkouts[0].Tree != in.Request.Product.Tree {
		return ErrHostInput
	}
	expected, code, err := stepverify.Preflight(in.Declaration, h)
	if err != nil || code != 0 {
		return ErrHostInput
	}
	actual, err := readHostFile(*c.Preflight, HostWireLimit)
	if err != nil {
		return err
	}
	a, err := wire.Parse(actual)
	if err != nil {
		return ErrHostInput
	}
	b, err := wire.Parse(stepverify.Encode(expected))
	if err != nil || !bytes.Equal(wire.CanonicalValue(a), wire.CanonicalValue(b)) {
		return ErrHostInput
	}
	return nil
}

// The Engine API describes the launched shim instance. Its exit status can be
// attributed to the author only after a bounded, server-observed post-exec start
// join; neither readiness nor a successful client call supplies that join.
type hostExecConfig struct {
	Privileged bool     `json:"privileged"`
	User       string   `json:"user"`
	TTY        bool     `json:"tty"`
	Entrypoint string   `json:"entrypoint"`
	Arguments  []string `json:"arguments"`
}
type hostExecInspection struct {
	CanRemove     bool           `json:"CanRemove"`
	ContainerID   string         `json:"ContainerID"`
	DetachKeys    string         `json:"DetachKeys"`
	ExitCode      int            `json:"ExitCode"`
	ID            string         `json:"ID"`
	OpenStderr    bool           `json:"OpenStderr"`
	OpenStdin     bool           `json:"OpenStdin"`
	OpenStdout    bool           `json:"OpenStdout"`
	ProcessConfig hostExecConfig `json:"ProcessConfig"`
	Running       bool           `json:"Running"`
	PID           int            `json:"Pid"`
}
type hostTop struct {
	Titles    []string   `json:"Titles"`
	Processes [][]string `json:"Processes"`
}

func decodeHostExec(data []byte, execID, containerID, authorSHA string) (hostExecInspection, error) {
	var x hostExecInspection
	if decodeHost(data, &x) != nil || !hostSHA.MatchString(execID) || !hostSHA.MatchString(containerID) || !hostSHA.MatchString(authorSHA) || x.ID != execID || x.ContainerID != containerID || x.PID < 0 || x.ExitCode < 0 || x.ExitCode > 255 || x.DetachKeys != "" || !x.OpenStdin || !x.OpenStdout || !x.OpenStderr || x.ProcessConfig.Privileged || x.ProcessConfig.TTY || x.ProcessConfig.User != "65532:65532" || x.ProcessConfig.Entrypoint != "/tools/exec-shim" || !reflect.DeepEqual(x.ProcessConfig.Arguments, []string{"--internal-envelope", authorSHA}) {
		return x, ErrHostInput
	}
	if x.Running && (x.PID == 0 || x.ExitCode != 0) {
		return x, ErrHostInput
	}
	return x, nil
}
func hostObservedAuthorStart(before, after hostExecInspection, rawTop []byte) bool {
	if !before.Running || !after.Running || before.ID != after.ID || before.ContainerID != after.ContainerID || before.PID <= 0 || before.PID != after.PID {
		return false
	}
	var top hostTop
	if decodeHost(rawTop, &top) != nil || !reflect.DeepEqual(top.Titles, []string{"PID", "COMMAND"}) || top.Processes == nil || len(top.Processes) > 256 {
		return false
	}
	seen := map[int]bool{}
	matched := false
	for _, row := range top.Processes {
		if len(row) != 2 || len(row[0]) > 20 || len(row[1]) > 4096 || !hostSafe(row[1]) {
			return false
		}
		pid, err := strconv.Atoi(row[0])
		if err != nil || pid <= 0 || strconv.Itoa(pid) != row[0] || seen[pid] {
			return false
		}
		seen[pid] = true
		if pid == before.PID {
			if row[1] != "/tools/author /admitted/author-input.json /product" {
				return false
			}
			matched = true
		}
	}
	return matched
}
func hostAuthorExitFromInspection(x hostExecInspection, startObserved bool) *HostAuthorExit {
	if !startObserved || x.Running || x.ExitCode < 0 || x.ExitCode > 255 {
		return nil
	}
	code := x.ExitCode
	// Engine ExecInspect exposes no signal. Do not decode 128+N into one.
	return &HostAuthorExit{Code: &code, Signal: nil, TimedOut: false}
}

// hostEngine has no exported endpoint selector. Production construction resolves
// one independently pinned CLI context; its transport can reach only that Unix
// socket. Neither the environment proxy nor HTTP redirects are consulted.
type hostEngine struct {
	socket    string
	version   string
	client    *http.Client
	transport *http.Transport
}
type hostAPIRange struct {
	Minimum       string                `json:"minimum"`
	Maximum       string                `json:"maximum"`
	Selected      string                `json:"selected"`
	Server        string                `json:"server"`
	Socket        string                `json:"socket"`
	ContextClient procgroup.Observation `json:"context_client_observation"`
	RawVersion    json.RawMessage       `json:"raw_version"`
}

func hostAPINumber(s string) (int, bool) {
	parts := strings.Split(s, ".")
	if len(parts) != 2 || parts[0] != "1" {
		return 0, false
	}
	n, e := strconv.Atoi(parts[1])
	return n, e == nil && n >= 0 && strconv.Itoa(n) == parts[1]
}
func hostNegotiateAPI(min, max string) (string, error) {
	lo, ok := hostAPINumber(min)
	if !ok {
		return "", ErrHostInput
	}
	hi, ok := hostAPINumber(max)
	if !ok || lo > hi || hi < 44 || lo > 56 {
		return "", ErrHostInput
	}
	if hi > 56 {
		hi = 56
	}
	return "1." + strconv.Itoa(hi), nil
}
func newHostEngine(ctx context.Context, p HostProfile) (*hostEngine, hostAPIRange, error) {
	var facts hostAPIRange
	path, err := exec.LookPath("docker")
	if err != nil {
		return nil, facts, ErrHostInput
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil || !hostPath(path) {
		return nil, facts, ErrHostInput
	}
	if err = hostExecutableDigest(path, p.Engine.ClientSHA256); err != nil {
		return nil, facts, err
	}
	home, err := os.UserHomeDir()
	if err != nil || !hostPath(home) {
		return nil, facts, ErrHostInput
	}
	observed := procgroup.Run(ctx, procgroup.Spec{Argv: []string{path, "context", "inspect", p.Engine.Context}, Dir: "/", Env: []string{"HOME=" + home, "PATH=/usr/bin:/bin"}, Timeout: 30 * time.Second, OutputLimit: HostWireLimit, StderrLimit: 4096, ObserveDescendants: true})
	if observed.Err != nil || !observed.ExitObserved || observed.ExitStatus != 0 || !observed.WaitCompleted || !observed.OwnedProcessGroupCleanup || observed.OutputOverflow {
		return nil, facts, ErrHostInput
	}
	var contexts []struct {
		Name      string
		Endpoints map[string]struct {
			Host          string
			SkipTLSVerify bool
		}
		TLSMaterial map[string]json.RawMessage
	}
	if hostEngineJSON(observed.Stdout, &contexts) != nil || len(contexts) != 1 || contexts[0].Name != p.Engine.Context || len(contexts[0].Endpoints) != 1 || len(contexts[0].TLSMaterial) != 0 {
		return nil, facts, ErrHostInput
	}
	endpoint, ok := contexts[0].Endpoints["docker"]
	if !ok || endpoint.SkipTLSVerify || !strings.HasPrefix(endpoint.Host, "unix://") {
		return nil, facts, ErrHostInput
	}
	rawSocket := strings.TrimPrefix(endpoint.Host, "unix://")
	if !hostPath(rawSocket) {
		return nil, facts, ErrHostInput
	}
	socket, err := filepath.EvalSymlinks(rawSocket)
	if err != nil || !hostPath(socket) {
		return nil, facts, ErrHostInput
	}
	st, err := os.Stat(socket)
	if err != nil || st.Mode()&os.ModeSocket == 0 {
		return nil, facts, ErrHostInput
	}
	engine := hostUnixEngine(socket)
	raw, err := engine.call(ctx, "GET", "/version", nil, 200)
	if err != nil {
		engine.close()
		return nil, facts, err
	}
	var version struct {
		Version       string
		ApiVersion    string
		MinAPIVersion string
		Os            string
		Arch          string
	}
	if hostEngineJSON(raw, &version) != nil || version.Version != p.Engine.ServerVersion || version.Os+"/"+version.Arch != p.Engine.Platform {
		engine.close()
		return nil, facts, ErrHostInput
	}
	selected, err := hostNegotiateAPI(version.MinAPIVersion, version.ApiVersion)
	if err != nil {
		engine.close()
		return nil, facts, err
	}
	engine.version = "/v" + selected
	facts = hostAPIRange{Minimum: version.MinAPIVersion, Maximum: version.ApiVersion, Selected: selected, Server: version.Version, Socket: socket, ContextClient: observed, RawVersion: raw}
	return engine, facts, nil
}
func hostUnixEngine(socket string) *hostEngine {
	transport := &http.Transport{Proxy: nil, DisableKeepAlives: true, ResponseHeaderTimeout: 30 * time.Second, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 30 * time.Second}).DialContext(ctx, "unix", socket)
	}}
	return &hostEngine{socket: socket, transport: transport, client: &http.Client{Transport: transport, Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (e *hostEngine) close() { e.transport.CloseIdleConnections() }
func (e *hostEngine) call(ctx context.Context, method, path string, body any, status int) ([]byte, error) {
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") || strings.ContainsAny(path, "\r\n#") {
		return nil, ErrHostInput
	}
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil || len(b) > HostWireLimit {
			return nil, ErrHostInput
		}
		reader = bytes.NewReader(b)
	}
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(bounded, method, "http://engine"+e.version+path, reader)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := e.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, HostWireLimit+1))
	if err != nil || len(raw) > HostWireLimit {
		return nil, ErrHostInput
	}
	if response.StatusCode != status {
		return raw, fmt.Errorf("engine status %d (wanted %d)", response.StatusCode, status)
	}
	return raw, nil
}

// Engine schemas are extensible and can contain signed/default numbers. Reject
// duplicate keys and excess nesting without imposing the paired wire's unsigned
// numeric rules on original engine facts.
func hostEngineJSON(raw []byte, dst any) error {
	if len(raw) > HostWireLimit || !utf8.Valid(raw) {
		return ErrHostInput
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var walk func(int) error
	walk = func(depth int) error {
		if depth > 64 {
			return ErrHostInput
		}
		token, err := dec.Token()
		if err != nil {
			return err
		}
		d, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch d {
		case '{':
			seen := map[string]bool{}
			for dec.More() {
				key, e := dec.Token()
				if e != nil {
					return e
				}
				s, ok := key.(string)
				if !ok || seen[s] {
					return ErrHostInput
				}
				seen[s] = true
				if e = walk(depth + 1); e != nil {
					return e
				}
			}
		case '[':
			for dec.More() {
				if e := walk(depth + 1); e != nil {
					return e
				}
			}
		default:
			return ErrHostInput
		}
		end, err := dec.Token()
		if err != nil || (d == '{' && end != json.Delim('}')) || (d == '[' && end != json.Delim(']')) {
			return ErrHostInput
		}
		return nil
	}
	if err := walk(0); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return ErrHostInput
	}
	return json.Unmarshal(raw, dst)
}
func (e *hostEngine) inspectExec(ctx context.Context, execID, cid, sha string) (hostExecInspection, []byte, error) {
	if !hostSHA.MatchString(execID) || !hostSHA.MatchString(cid) {
		return hostExecInspection{}, nil, ErrHostInput
	}
	raw, err := e.call(ctx, "GET", "/exec/"+execID+"/json", nil, 200)
	if err != nil {
		return hostExecInspection{}, raw, err
	}
	x, err := decodeHostExec(raw, execID, cid, sha)
	return x, raw, err
}
func (e *hostEngine) top(ctx context.Context, cid string) ([]byte, error) {
	if !hostSHA.MatchString(cid) {
		return nil, ErrHostInput
	}
	return e.call(ctx, "GET", "/containers/"+cid+"/top?ps_args="+url.QueryEscape("-eo pid,args"), nil, 200)
}

// The hijacked stream is retained by one owner. A cancelled read/write closes the
// actual Unix connection, and close joins its cancellation watcher. HTTP client
// completion never implies guest completion.
type hostExecStream struct {
	conn      net.Conn
	reader    *bufio.Reader
	done      chan struct{}
	joined    chan struct{}
	once      sync.Once
	remaining int64
}

func (e *hostEngine) startExec(ctx context.Context, id string) (*hostExecStream, error) {
	if !hostSHA.MatchString(id) {
		return nil, ErrHostInput
	}
	conn, err := (&net.Dialer{Timeout: 30 * time.Second}).DialContext(ctx, "unix", e.socket)
	if err != nil {
		return nil, err
	}
	s := &hostExecStream{conn: conn, reader: bufio.NewReader(conn), done: make(chan struct{}), joined: make(chan struct{}), remaining: 16 << 20}
	go func() {
		defer close(s.joined)
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-s.done:
		}
	}()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	body := []byte(`{"Detach":false,"Tty":false}`)
	request, err := http.NewRequest("POST", "http://engine"+e.version+"/exec/"+id+"/start", bytes.NewReader(body))
	if err == nil {
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Connection", "Upgrade")
		request.Header.Set("Upgrade", "tcp")
		err = request.Write(conn)
	}
	if err != nil {
		s.close()
		return nil, err
	}
	response, err := http.ReadResponse(s.reader, request)
	// A successful upgrade is required; ordinary bodies cannot serve as the
	// bidirectional barrier. Reject redirects, fallback and partial handshakes.
	if err != nil || response.StatusCode != 101 || !strings.EqualFold(response.Header.Get("Upgrade"), "tcp") || !strings.EqualFold(response.Header.Get("Connection"), "upgrade") {
		s.close()
		return nil, ErrHostInput
	}
	return s, nil
}
func (s *hostExecStream) close() { s.once.Do(func() { close(s.done); _ = s.conn.Close() }); <-s.joined }
func (s *hostExecStream) frame() (byte, []byte, error) {
	var header [8]byte
	_, err := io.ReadFull(s.reader, header[:])
	if err != nil {
		return 0, nil, err
	}
	n := int64(binary.BigEndian.Uint32(header[4:]))
	if (header[0] != 1 && header[0] != 2) || header[1] != 0 || header[2] != 0 || header[3] != 0 || n == 0 || n > s.remaining {
		return 0, nil, ErrHostInput
	}
	s.remaining -= n
	payload := make([]byte, int(n))
	_, err = io.ReadFull(s.reader, payload)
	return header[0], payload, err
}
func (s *hostExecStream) envelope(p HostProfile) (HostEnvelopeObservation, []byte, error) {
	var raw []byte
	for len(raw) <= HostWireLimit {
		kind, part, err := s.frame()
		if err != nil || kind != 1 {
			return HostEnvelopeObservation{}, raw, ErrHostInput
		}
		raw = append(raw, part...)
		if i := bytes.IndexByte(raw, '\n'); i >= 0 {
			if i != len(raw)-1 {
				return HostEnvelopeObservation{}, raw, ErrHostInput
			}
			var observed HostEnvelopeObservation
			if decodeHost(raw, &observed) != nil || !hostEnvelopeMatches(observed, p) {
				return observed, raw, ErrHostInput
			}
			return observed, raw, nil
		}
	}
	return HostEnvelopeObservation{}, raw, ErrHostInput
}

func hostEnvelopeMatches(observed HostEnvelopeObservation, p HostProfile) bool {
	return observed.Profile == "corvint-postmerge-internal-envelope/0" && observed.AuthorSHA256 == p.Admission.AuthorSHA256 && hostEnvironmentMatches(observed.Environment, p.Command.Environment) && hostEnvironmentMatches(observed.HoldEnvironment, p.Image.HoldEnvironment)
}
func hostExecutableDigest(path, sha string) error {
	if !hostPath(path) || !hostSHA.MatchString(sha) {
		return ErrHostInput
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0111 == 0 || !hostSingleLink(info) || info.Size() > 256<<20 {
		return ErrHostInput
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return ErrHostInput
	}
	hash := sha256.New()
	n, err := io.Copy(hash, io.LimitReader(f, (256<<20)+1))
	if err != nil || n != info.Size() || hex.EncodeToString(hash.Sum(nil)) != sha {
		return ErrHostInput
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(info, after) || info.Size() != after.Size() || !info.ModTime().Equal(after.ModTime()) || !hostSingleLink(after) {
		return ErrHostInput
	}
	return nil
}

var hostMaskedPaths = []string{"/proc/acpi", "/proc/asound", "/proc/interrupts", "/proc/kcore", "/proc/keys", "/proc/latency_stats", "/proc/timer_list", "/proc/timer_stats", "/proc/sched_debug", "/proc/scsi", "/sys/firmware", "/sys/devices/virtual/powercap", "/product/.git"}
var hostReadonlyPaths = []string{"/proc/bus", "/proc/fs", "/proc/irq", "/proc/sys", "/proc/sysrq-trigger"}

func hostLabels(in *HostInputs) map[string]string {
	return map[string]string{"com.corvint.author-run": in.Request.RunID, "com.corvint.author-request": in.RequestSHA256, "com.corvint.author-profile": in.ProfileSHA256}
}
func hostContainerConfig(in *HostInputs) map[string]any {
	mounts := []any{}
	for _, m := range in.Mounts {
		mounts = append(mounts, map[string]any{"Type": "bind", "Source": m.Source, "Target": m.Destination, "ReadOnly": m.ReadOnly, "BindOptions": map[string]any{"Propagation": "rprivate", "CreateMountpoint": false}})
	}
	env := []string{}
	for _, v := range in.Profile.Image.Environment {
		env = append(env, v.Key+"="+v.Value)
	}
	return map[string]any{"Image": in.Profile.Image.ManifestDigest, "Hostname": "corvint-author", "User": "65532:65532", "Entrypoint": []string{"/bin/sleep"}, "Cmd": []string{"infinity"}, "WorkingDir": "/", "Env": env, "Labels": hostLabels(in), "Tty": false, "OpenStdin": false, "AttachStdin": false, "AttachStdout": false, "AttachStderr": false, "HostConfig": map[string]any{
		"NetworkMode": "none", "ReadonlyRootfs": true, "Privileged": false, "CapDrop": []string{"ALL"}, "CapAdd": []string{}, "SecurityOpt": []string{"no-new-privileges:true"}, "PidMode": "", "IpcMode": "private", "CgroupnsMode": "private", "RestartPolicy": map[string]any{"Name": "no", "MaximumRetryCount": 0}, "PublishAllPorts": false, "PortBindings": map[string]any{}, "Devices": []any{}, "DeviceRequests": []any{}, "Mounts": mounts, "Tmpfs": map[string]string{"/private": "rw,nosuid,nodev,noexec,size=16777216,mode=0700,uid=65532,gid=65532"}, "NanoCpus": in.Profile.Limits.CPUsNanos, "Memory": in.Profile.Limits.MemoryBytes, "MemorySwap": in.Profile.Limits.SwapBytes, "PidsLimit": in.Profile.Limits.PIDs, "ShmSize": in.Profile.Limits.ShmBytes, "MaskedPaths": hostMaskedPaths, "ReadonlyPaths": hostReadonlyPaths, "AutoRemove": false, "Init": false, "LogConfig": map[string]any{"Type": "none", "Config": map[string]any{}}}}
}
func hostJSONEqual(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return bytes.Equal(x, y)
}
func hostZero(v any) bool {
	if v != nil {
		r := reflect.ValueOf(v)
		switch r.Kind() {
		case reflect.Array, reflect.Slice, reflect.Map:
			return r.Len() == 0
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return r.Int() == 0
		}
	}
	switch x := v.(type) {
	case nil:
		return true
	case bool:
		return !x
	case float64:
		return x == 0
	case string:
		return x == ""
	case []any:
		return len(x) == 0
	case map[string]any:
		return len(x) == 0
	}
	return false
}
func hostImageMatches(raw []byte, p HostProfile) bool {
	var image struct {
		ID           string `json:"Id"`
		RepoDigests  []string
		Architecture string
		Os           string
		Config       struct {
			Env     []string
			Volumes map[string]json.RawMessage
		}
	}
	if hostEngineJSON(raw, &image) != nil || image.ID != p.Image.ConfigDigest || image.Os+"/"+image.Architecture != p.Engine.Platform || len(image.Config.Volumes) != 0 {
		return false
	}
	found := false
	for _, digest := range image.RepoDigests {
		if strings.HasSuffix(digest, "@"+p.Image.ManifestDigest) {
			found = true
		}
	}
	actual, err := hostObserveEnvironment(image.Config.Env)
	return found && err == nil && hostEnvironmentMatches(actual, p.Image.Environment)
}
func hostOwnedContainer(raw []byte, id string, in *HostInputs) bool {
	var x struct {
		ID     string `json:"Id"`
		Name   string
		Config struct{ Labels map[string]string }
	}
	return hostEngineJSON(raw, &x) == nil && hostSHA.MatchString(x.ID) && (id == "" || x.ID == id) && x.Name == "/corvint-author-"+in.Request.RunID && reflect.DeepEqual(x.Config.Labels, hostLabels(in))
}

// Engine v1.56 Mount documents these optional bind booleans as false by
// default. Normalize only those exact fields (and empty consistency), never
// arbitrary unknown fields, mount sources, propagation, or read-only intent.
func hostInspectMountsMatch(actual, expected any) bool {
	raw, err := json.Marshal(actual)
	if err != nil {
		return false
	}
	var got []map[string]any
	if json.Unmarshal(raw, &got) != nil {
		return false
	}
	raw, err = json.Marshal(expected)
	if err != nil {
		return false
	}
	var want []map[string]any
	if json.Unmarshal(raw, &want) != nil || len(got) != len(want) {
		return false
	}
	for i, mount := range got {
		if mount == nil || want[i] == nil {
			return false
		}
		if value, ok := mount["Consistency"]; ok {
			if value != "" {
				return false
			}
			delete(mount, "Consistency")
		}
		if _, ok := mount["ReadOnly"]; !ok && want[i]["ReadOnly"] == false {
			mount["ReadOnly"] = false
		}
		options, ok := mount["BindOptions"].(map[string]any)
		if !ok {
			return false
		}
		if _, ok := options["CreateMountpoint"]; !ok {
			options["CreateMountpoint"] = false
		}
		for _, key := range []string{"NonRecursive", "ReadOnlyNonRecursive", "ReadOnlyForceRecursive"} {
			if value, ok := options[key]; ok {
				if value != false {
					return false
				}
				delete(options, key)
			}
		}
	}
	return hostJSONEqual(got, want)
}

func hostContainerMatches(raw []byte, id string, in *HostInputs) bool {
	var present struct {
		State  map[string]json.RawMessage
		Mounts []map[string]json.RawMessage
	}
	if hostEngineJSON(raw, &present) != nil {
		return false
	}
	for _, key := range []string{"Running", "Paused", "Restarting", "Dead", "Pid"} {
		value, ok := present.State[key]
		if !ok || bytes.Equal(value, []byte("null")) {
			return false
		}
	}
	for _, mount := range present.Mounts {
		for _, key := range []string{"Type", "Source", "Destination", "RW", "Propagation"} {
			value, ok := mount[key]
			if !ok || bytes.Equal(value, []byte("null")) {
				return false
			}
		}
	}
	if !hostOwnedContainer(raw, id, in) {
		return false
	}
	var x struct {
		Image string
		State struct {
			Running    bool
			Paused     bool
			Restarting bool
			Dead       bool
			Pid        int
		}
		Config     map[string]any
		HostConfig map[string]any
		Mounts     []struct {
			Type        string
			Source      string
			Destination string
			RW          bool
			Propagation string
		}
		NetworkSettings struct {
			Ports    map[string]any
			Networks map[string]struct {
				IPAddress         string
				GlobalIPv6Address string
			}
		}
	}
	if hostEngineJSON(raw, &x) != nil || x.Image != in.Profile.Image.ConfigDigest || !x.State.Running || x.State.Paused || x.State.Restarting || x.State.Dead || x.State.Pid <= 0 || len(x.NetworkSettings.Ports) != 0 || len(x.NetworkSettings.Networks) != 1 {
		return false
	}
	network, ok := x.NetworkSettings.Networks["none"]
	if !ok || network.IPAddress != "" || network.GlobalIPv6Address != "" {
		return false
	}
	expected := hostContainerConfig(in)
	for _, key := range []string{"Image", "Hostname", "User", "Entrypoint", "Cmd", "WorkingDir", "Labels", "Tty", "OpenStdin", "AttachStdin", "AttachStdout", "AttachStderr"} {
		v, ok := x.Config[key]
		if !ok || !hostJSONEqual(v, expected[key]) {
			return false
		}
	}
	rawEnv, _ := json.Marshal(x.Config["Env"])
	var env []string
	if json.Unmarshal(rawEnv, &env) != nil {
		return false
	}
	observed, err := hostObserveEnvironment(env)
	if err != nil || !hostEnvironmentMatches(observed, in.Profile.Image.Environment) {
		return false
	}
	for _, key := range []string{"Volumes", "ExposedPorts", "Healthcheck", "OnBuild", "Shell"} {
		if !hostZero(x.Config[key]) {
			return false
		}
	}
	want := expected["HostConfig"].(map[string]any)
	for key, value := range want {
		got, ok := x.HostConfig[key]
		if !ok {
			return false
		}
		if key == "Mounts" {
			if !hostInspectMountsMatch(got, value) {
				return false
			}
			continue
		}
		if !hostJSONEqual(got, value) {
			if hostZero(got) && hostZero(value) {
				continue
			}
			return false
		}
	}
	// Unspecified effective host settings must be empty/default. Explicitly allow
	// only these harmless engine defaults; daemon-added security exceptions fail.
	for key, value := range x.HostConfig {
		if _, ok := want[key]; ok {
			continue
		}
		// A non-TTY console's documented height/width pair is harmless only
		// at exactly [0,0]; do not generalize collection zero-ness.
		if key == "ConsoleSize" {
			if hostJSONEqual(value, []int{0, 0}) {
				continue
			}
			return false
		}
		if hostZero(value) {
			continue
		}
		if key == "Runtime" && value == "runc" {
			continue
		}
		return false
	}
	if len(x.Mounts) != len(in.Mounts) {
		return false
	}
	seen := map[string]bool{}
	for _, mount := range x.Mounts {
		if mount.Type != "bind" || mount.Propagation != "rprivate" || seen[mount.Destination] {
			return false
		}
		seen[mount.Destination] = true
		found := false
		for _, want := range in.Mounts {
			if mount.Source == want.Source && mount.Destination == want.Destination && mount.RW != want.ReadOnly {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}

type hostStored struct {
	root  string
	bytes int64
	refs  int
	facts []HostFact
}

func (s *hostStored) write(name string, data []byte) (HostFileRef, error) {
	if strings.ContainsAny(name, "/\\") || !hostName.MatchString(name) || s.refs >= 32 || int64(len(data))+s.bytes > 16<<20 {
		return HostFileRef{}, ErrHostInput
	}
	path := filepath.Join(s.root, name)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return HostFileRef{}, err
	}
	n, writeErr := file.Write(data)
	syncErr := file.Sync()
	closeErr := file.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil || n != len(data) {
		return HostFileRef{}, ErrHostInput
	}
	s.bytes += int64(n)
	s.refs++
	return HostFileRef{path, hostDigest(data), int64(n)}, nil
}

// Native records are written by the trusted supervisor, but share the same
// per-run retention budget as launcher facts and raw author logs.
func (s *hostStored) reserveNative(c HostControl) error {
	total := int64(0)
	seen := map[string]bool{}
	for _, ref := range []*HostFileRef{c.Host, c.Before, c.Preflight} {
		if ref == nil || !hostRefShape(*ref) || seen[ref.Path] {
			return ErrHostInput
		}
		seen[ref.Path] = true
		total += ref.Bytes
	}
	if s.refs+3 > 32 || s.bytes+total > 16<<20 {
		return ErrHostInput
	}
	s.refs += 3
	s.bytes += total
	return nil
}
func (s *hostStored) fact(kind, name string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	ref, err := s.write(name, raw)
	if err != nil {
		return err
	}
	s.facts = append(s.facts, HostFact{kind, ref})
	return nil
}

type hostEngineFact struct {
	Operation string          `json:"operation"`
	Body      json.RawMessage `json:"body"`
	Failed    bool            `json:"failed"`
}
type hostTranscript struct {
	Entries []hostEngineFact `json:"entries"`
	bytes   int
}

func (t *hostTranscript) add(operation string, raw []byte, err error) error {
	if len(raw) == 0 {
		raw = []byte("null")
	}
	if !json.Valid(raw) {
		return ErrHostInput
	}
	if len(t.Entries) >= 512 || t.bytes+len(raw) > 2<<20 {
		return ErrHostInput
	}
	t.bytes += len(raw)
	t.Entries = append(t.Entries, hostEngineFact{operation, append([]byte{}, raw...), err != nil})
	return nil
}

// cleanup always gets a fresh bounded context. It never deletes by an unverified
// name or applies a broad prune, and ambiguous create completion stays held even
// if a current absence is observed.
type hostEngineOperations interface {
	call(context.Context, string, string, any, int) ([]byte, error)
	inspectExec(context.Context, string, string, string) (hostExecInspection, []byte, error)
	top(context.Context, string) ([]byte, error)
	startExec(context.Context, string) (*hostExecStream, error)
}

func hostCleanup(e hostEngineOperations, in *HostInputs, cid string, createCertain bool) (hostTranscript, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	t := hostTranscript{Entries: []hostEngineFact{}}
	name := cid
	if name == "" {
		name = "corvint-author-" + in.Request.RunID
	}
	raw, err := e.call(ctx, "GET", "/containers/"+name+"/json", nil, 200)
	if err != nil {
		_ = t.add("inspect-owned", raw, err)
		absent, absenceErr := e.call(ctx, "GET", "/containers/"+name+"/json", nil, 404)
		_ = t.add("absence", absent, absenceErr)
		return t, createCertain && absenceErr == nil
	}
	if !hostOwnedContainer(raw, cid, in) {
		_ = t.add("identity-conflict", raw, ErrHostInput)
		return t, false
	}
	if t.add("inspect-owned", raw, nil) != nil {
		return t, false
	}
	var found struct {
		ID string `json:"Id"`
	}
	if hostEngineJSON(raw, &found) != nil {
		return t, false
	}
	raw, err = e.call(ctx, "DELETE", "/containers/"+found.ID+"?force=true&v=true", nil, 204)
	if t.add("remove-owned", raw, err) != nil || err != nil {
		return t, false
	}
	raw, err = e.call(ctx, "GET", "/containers/"+found.ID+"/json", nil, 404)
	if t.add("absence", raw, err) != nil {
		return t, false
	}
	return t, createCertain && err == nil
}

type hostControlRead struct {
	raw []byte
	err error
}

func hostReadControl(input io.ReadCloser, stop <-chan struct{}) (<-chan hostControlRead, <-chan struct{}) {
	frames := make(chan hostControlRead, 2)
	joined := make(chan struct{})
	go func() {
		defer close(joined)
		reader := bufio.NewReader(io.LimitReader(input, HostWireLimit+1))
		raw, err := reader.ReadBytes('\n')
		if len(raw) > HostWireLimit {
			err = ErrHostInput
		}
		select {
		case frames <- hostControlRead{raw, err}:
		case <-stop:
			return
		}
		_, err = reader.ReadByte()
		if err == nil {
			err = ErrHostInput
		}
		select {
		case frames <- hostControlRead{nil, err}:
		case <-stop:
		}
	}()
	return frames, joined
}
func hostControllerStart(ctx context.Context) (string, error) {
	observed := procgroup.Run(ctx, procgroup.Spec{Argv: []string{"/bin/ps", "-p", strconv.Itoa(os.Getpid()), "-o", "lstart="}, Dir: "/", Env: []string{"PATH=/usr/bin:/bin", "LC_ALL=C"}, Timeout: 5 * time.Second, OutputLimit: 4096, StderrLimit: 4096, ObserveDescendants: true})
	value := strings.TrimSpace(string(observed.Stdout))
	if observed.Err != nil || !observed.ExitObserved || observed.ExitStatus != 0 || !observed.WaitCompleted || !observed.OwnedProcessGroupCleanup || value == "" || !hostSafe(value) {
		return "", ErrHostInput
	}
	return value, nil
}

// RunHostLauncher owns a single run until exact cleanup or a durable hold. It
// accepts no engine flags, alternate command, or post-author narrowed scope.
// A returned error always means blocked; retained events never assert native
// acceptance, and the supervisor must observe this controller's eventual exit.
func RunHostLauncher(ctx context.Context, in *HostInputs, input io.ReadCloser, output io.Writer) (result error) {
	if in == nil || input == nil || output == nil {
		return ErrHostInput
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	self, err = filepath.EvalSymlinks(self)
	if err != nil {
		return err
	}
	if err = hostExecutableDigest(self, in.Profile.Implementation.LauncherSHA256); err != nil {
		return err
	}
	if err = verifyHostProduct(ctx, in); err != nil {
		return err
	}
	e, api, err := newHostEngine(ctx, in.Profile)
	if err != nil {
		return err
	}
	defer e.close()
	start, err := hostControllerStart(ctx)
	if err != nil {
		return err
	}
	return runHostOwned(ctx, in, e, api, start, input, output)
}

// The private engine boundary permits deterministic lifecycle tests without
// starting a daemon. Production reaches it only through the pinned constructor.
func runHostOwned(ctx context.Context, in *HostInputs, e hostEngineOperations, api hostAPIRange, start string, input io.ReadCloser, output io.Writer) (result error) {
	var err error
	if err = os.Mkdir(in.Request.OutputRoot, 0700); err != nil {
		return err
	}
	stored := hostStored{root: in.Request.OutputRoot, facts: []HostFact{}}
	ownership := struct {
		Profile string            `json:"profile"`
		Run     string            `json:"run_id"`
		Name    string            `json:"name"`
		Labels  map[string]string `json:"labels"`
		PID     int               `json:"controller_pid"`
		Start   string            `json:"controller_start_observed"`
		API     hostAPIRange      `json:"engine"`
	}{"corvint-author-ownership/0", in.Request.RunID, "corvint-author-" + in.Request.RunID, hostLabels(in), os.Getpid(), start, api}
	if err = stored.fact("ownership", "ownership-pending.json", ownership); err != nil {
		return err
	}
	if err = os.Mkdir(filepath.Join(stored.root, "native"), 0700); err != nil {
		return err
	}
	event := HostEvent{Profile: "corvint-postmerge-host-event/0", RunID: in.Request.RunID, RequestSHA256: in.RequestSHA256, ProfileSHA256: in.ProfileSHA256, Facts: []HostFact{}, Cleanup: "NOT_RUN", Reasons: []string{}}
	emit := func() error {
		event.Facts = append([]HostFact{}, stored.facts...)
		return json.NewEncoder(output).Encode(event)
	}
	var cid string
	var stream *hostExecStream
	created := false
	createCertain := false
	reasons := []string{}
	hold := func(code string) { reasons = append(reasons, code); result = ErrHostInput }
	transcript := hostTranscript{Entries: []hostEngineFact{}}
	// Finalization is deliberately independent from ctx, including a cancelled
	// supervisor. A lost connection is never treated as successful retirement.
	defer func() {
		if stream != nil {
			stream.close()
		}
		if len(transcript.Entries) > 0 {
			finalKind := "exec"
			if event.AuthorExit != nil {
				finalKind = "inspect-exit"
			}
			if stored.fact(finalKind, "engine-final.json", transcript) != nil {
				hold("retention-incomplete")
			}
		}
		if created {
			cleanup, removed := hostCleanup(e, in, cid, createCertain)
			if stored.fact("cleanup", "cleanup.json", cleanup) != nil {
				removed = false
			}
			if removed {
				event.Cleanup = "REMOVED"
			} else {
				event.Cleanup = "HELD"
				hold("cleanup-unresolved")
			}
		}
		if result != nil && len(reasons) == 0 {
			reasons = append(reasons, "boundary-incomplete")
		}
		event.Reasons = reasons
		event.Event = "held"
		if result == nil && event.AuthorExit != nil && event.Cleanup == "REMOVED" {
			event.Event = "finished"
		} else {
			result = ErrHostInput
		}
		if emit() != nil {
			result = ErrHostInput
		}
	}()
	// A name conflict is never removed. No resource creation has begun yet.
	raw, err := e.call(ctx, "GET", "/containers/"+ownership.Name+"/json", nil, 404)
	if transcript.add("prior-absence", raw, err) != nil || err != nil {
		hold("ownership-conflict")
		return result
	}
	raw, err = e.call(ctx, "GET", "/images/"+in.Profile.Image.ManifestDigest+"/json", nil, 200)
	if transcript.add("image-inspect", raw, err) != nil || err != nil || !hostImageMatches(raw, in.Profile) {
		hold("image-unverified")
		return result
	}
	created = true
	raw, err = e.call(ctx, "POST", "/containers/create?name="+ownership.Name, hostContainerConfig(in), 201)
	if transcript.add("create", raw, err) != nil || err != nil {
		hold("create-ambiguous")
		return result
	}
	var create struct {
		ID       string `json:"Id"`
		Warnings []string
	}
	if hostEngineJSON(raw, &create) != nil || !hostSHA.MatchString(create.ID) || len(create.Warnings) != 0 {
		hold("create-ambiguous")
		return result
	}
	cid = create.ID
	createCertain = true
	event.ContainerID = &cid
	if err = stored.fact("create", "engine-create.json", transcript); err != nil {
		hold("retention-incomplete")
		return result
	}
	transcript = hostTranscript{Entries: []hostEngineFact{}}
	raw, err = e.call(ctx, "POST", "/containers/"+cid+"/start", nil, 204)
	if transcript.add("start-inert", raw, err) != nil || err != nil {
		hold("hold-start-failed")
		return result
	}
	raw, err = e.call(ctx, "GET", "/containers/"+cid+"/json", nil, 200)
	if transcript.add("inspect-ready", raw, err) != nil || err != nil || !hostContainerMatches(raw, cid, in) {
		hold("boundary-unverified")
		return result
	}
	if err = stored.fact("inspect-ready", "engine-ready.json", transcript); err != nil {
		hold("retention-incomplete")
		return result
	}
	transcript = hostTranscript{Entries: []hostEngineFact{}}
	execConfig := map[string]any{"AttachStdin": true, "AttachStdout": true, "AttachStderr": true, "Tty": false, "Privileged": false, "User": "65532:65532", "WorkingDir": "/product", "Cmd": []string{"/tools/exec-shim", "--internal-envelope", in.Request.Author.SHA256}, "Env": []string{}}
	raw, err = e.call(ctx, "POST", "/containers/"+cid+"/exec", execConfig, 201)
	if transcript.add("exec-create", raw, err) != nil || err != nil {
		hold("shim-create-failed")
		return result
	}
	var execution struct {
		ID string `json:"Id"`
	}
	if hostEngineJSON(raw, &execution) != nil || !hostSHA.MatchString(execution.ID) {
		hold("shim-identity-unobserved")
		return result
	}
	stream, err = e.startExec(ctx, execution.ID)
	if err != nil {
		hold("shim-stream-failed")
		return result
	}
	observed, _, err := stream.envelope(in.Profile)
	if err != nil {
		hold("environment-unobserved")
		return result
	}
	if stored.fact("hold-environment-ready", "hold-environment.json", observed.HoldEnvironment) != nil || stored.fact("environment-ready", "author-environment.json", observed) != nil {
		hold("retention-incomplete")
		return result
	}
	stop := make(chan struct{})
	frames, inputJoined := hostReadControl(input, stop)
	defer func() { close(stop); _ = input.Close(); <-inputJoined }()
	event.Event = "ready"
	if emit() != nil {
		hold("supervisor-lost")
		return result
	}
	var frame hostControlRead
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		hold("supervisor-lost")
		return result
	case <-timer.C:
		hold("control-timeout")
		return result
	case frame = <-frames:
	}
	control, err := DecodeHostControl(frame.raw, in.Request, in.RequestSHA256, in.ProfileSHA256)
	if frame.err != nil || err != nil {
		hold("control-invalid")
		return result
	}
	controlRef, controlErr := stored.write("control.json", frame.raw)
	if controlErr != nil {
		hold("retention-incomplete")
		return result
	}
	stored.facts = append(stored.facts, HostFact{"exec", controlRef})
	if control.Operation == "cancel" {
		hold(*control.Reason)
		return result
	}
	if validateHostNativeControl(control, in) != nil {
		hold("native-control-unverified")
		return result
	}
	if stored.reserveNative(control) != nil {
		hold("retention-overflow")
		return result
	}
	if _, err = stream.conn.Write([]byte("execute\n")); err != nil {
		hold("barrier-disconnected")
		return result
	}
	deadline := time.Now().Add(30 * time.Second)
	_ = stream.conn.SetDeadline(deadline)
	authorCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	type drained struct {
		stdout, stderr []byte
		err            error
	}
	logs := make(chan drained, 1)
	go func() {
		var d drained
		for {
			kind, part, err := stream.frame()
			if err != nil {
				if err != io.EOF {
					d.err = err
				}
				logs <- d
				return
			}
			if len(d.stdout)+len(d.stderr)+len(part) > 8<<20 {
				d.err = ErrHostInput
				logs <- d
				return
			}
			if kind == 1 {
				d.stdout = append(d.stdout, part...)
			} else {
				d.stderr = append(d.stderr, part...)
			}
		}
	}()
	// Always join the stream collector before final evidence or process cleanup.
	var logResult drained
	gotLogs := false
	defer func() {
		stream.close()
		if !gotLogs {
			logResult = <-logs
		}
		if logResult.err != nil {
			hold("stream-incomplete")
		}
		stdoutRef, e1 := stored.write("author-stdout.bin", logResult.stdout)
		stderrRef, e2 := stored.write("author-stderr.bin", logResult.stderr)
		if e1 != nil || e2 != nil || stored.fact("exec", "author-logs.json", struct {
			ProcessKind string      `json:"process_kind"`
			ExecID      string      `json:"exec_id"`
			Stdout      HostFileRef `json:"stdout"`
			Stderr      HostFileRef `json:"stderr"`
		}{"shim-or-author", execution.ID, stdoutRef, stderrRef}) != nil {
			hold("retention-incomplete")
		}
	}()
	startObserved := false
	for {
		before, raw, inspectErr := e.inspectExec(authorCtx, execution.ID, cid, in.Request.Author.SHA256)
		if transcript.add("exec-inspect", raw, inspectErr) != nil || inspectErr != nil {
			hold("exec-inspect-unresolved")
			return result
		}
		if !before.Running {
			event.AuthorExit = hostAuthorExitFromInspection(before, startObserved)
			if event.AuthorExit == nil {
				hold("author-start-unobserved")
				return result
			}
			break
		}
		if !startObserved {
			top, topErr := e.top(authorCtx, cid)
			if transcript.add("author-top", top, topErr) != nil || topErr != nil {
				hold("author-start-unobserved")
				return result
			}
			after, raw, inspectErr := e.inspectExec(authorCtx, execution.ID, cid, in.Request.Author.SHA256)
			if transcript.add("exec-inspect-after-top", raw, inspectErr) != nil || inspectErr != nil {
				hold("exec-inspect-unresolved")
				return result
			}
			startObserved = hostObservedAuthorStart(before, after, top)
		}
		tick := time.NewTimer(100 * time.Millisecond)
		select {
		case <-authorCtx.Done():
			tick.Stop()
			hold("author-timeout-or-supervisor-lost")
			return result
		case <-frames:
			tick.Stop()
			hold("control-repeated-or-supervisor-lost")
			return result
		case logResult = <-logs:
			gotLogs = true
			tick.Stop()
			if logResult.err != nil {
				hold("stream-incomplete")
				return result
			}
		case <-tick.C:
		}
		if gotLogs {
			last, raw, inspectErr := e.inspectExec(authorCtx, execution.ID, cid, in.Request.Author.SHA256)
			if transcript.add("exec-final", raw, inspectErr) != nil || inspectErr != nil || last.Running {
				hold("late-completion-unresolved")
				return result
			}
			event.AuthorExit = hostAuthorExitFromInspection(last, startObserved)
			if event.AuthorExit == nil {
				hold("author-start-unobserved")
				return result
			}
			break
		}
	}
	if !gotLogs {
		select {
		case logResult = <-logs:
			gotLogs = true
			if logResult.err != nil {
				hold("stream-incomplete")
				return result
			}
		case <-authorCtx.Done():
			hold("stream-incomplete")
			return result
		case <-frames:
			hold("control-repeated-or-supervisor-lost")
			return result
		}
	}
	return nil
}
