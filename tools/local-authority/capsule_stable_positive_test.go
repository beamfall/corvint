package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/authoritystore"
	"github.com/Beamfall/corvint/internal/cem/stablefixture"
	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/localauthority"
)

// stableCapsuleFixture is a real cem/1.0 change from the producer workflow,
// stabilized with six artifacts, and the minimal capsule for its base and
// target: both commits, their trees, the changed blobs, the evidence blob
// and the committed sidecar. History and unchanged blobs are not supplied.
type stableCapsuleFixture struct {
	change stablefixture.Change
	stable stablefixture.Stable
	ocm    []byte
}

func newStableCapsuleFixture(t *testing.T) stableCapsuleFixture {
	t.Helper()
	change := stablefixture.NewChangeWithUnchanged(t, map[string]string{stableCapsuleDriver: "#!/bin/sh\nexit 0\n"})
	return stableCapsuleFixture{
		change: change, stable: change.Stable(t),
		ocm: []byte(`{"intentScope":{"path":"docs/rule.txt"},"claims":[{"path":"src/app.txt"}]}` + "\n"),
	}
}

func gitObjectBytes(t *testing.T, root, oid string) capsuleObject {
	t.Helper()
	kind, err := exec.Command("git", "-C", root, "cat-file", "-t", oid).Output()
	if err != nil {
		t.Fatal(err)
	}
	body, err := exec.Command("git", "-C", root, "cat-file", strings.TrimSpace(string(kind)), oid).Output()
	if err != nil {
		t.Fatal(err)
	}
	return capsuleObject{OID: oid, Data: append([]byte(fmt.Sprintf("%s %d\x00", strings.TrimSpace(string(kind)), len(body))), body...)}
}

// enrolled builds the capsule and enrolled input for base..target with
// cemRaw as cem.json, leaving out the objects named in omit.
func (f stableCapsuleFixture) enrolled(t *testing.T, base, target string, cemRaw []byte, omit map[string]bool) (enrolledInput, objectSet, error) {
	t.Helper()
	root := f.change.Root
	demand := map[string]bool{base: true, target: true}
	for _, commit := range []string{base, target} {
		demand[f.change.Git("rev-parse", commit+"^{tree}")] = true
		for _, line := range strings.Split(f.change.Git("ls-tree", "-r", "-t", "--format=%(objecttype) %(objectname) %(path)", commit), "\n") {
			fields := strings.Fields(line)
			if len(fields) != 3 {
				continue
			}
			changed := f.change.Git("diff", "--name-only", base, target, "--", fields[2]) != ""
			if fields[0] == "tree" || changed || fields[2] == "docs/rule.txt" || fields[2] == stableCapsuleDriver || fields[2] == wire.ExcludedCEMPath {
				demand[fields[1]] = true
			}
		}
	}
	original := capsule{Profile: "corvint-source-capsule/0"}
	for oid := range demand {
		if !omit[oid] {
			original.Objects = append(original.Objects, gitObjectBytes(t, root, oid))
		}
	}
	tree := f.change.Git("rev-parse", target+"^{tree}")
	in := enrolledInput{
		root:       authoritystore.RootDocument{RootID: "unit-only", Epoch: "1", Generation: "1", AuthorityGID: "600"},
		enrollment: localauthority.Enrollment{Binding: localauthority.Binding{Base: base, Target: target, Tree: tree}},
		raw:        map[string][]byte{"cem.json": cemRaw, "ocm.json": f.ocm, "objects.json": mustJSONLine(original)},
	}
	c, objects, err := completeCapsule(original, base, target, tree)
	in.capsule = c
	return in, objects, err
}

// publishedView materializes the capsule's published evidence files.
func publishedView(t *testing.T, in enrolledInput) string {
	t.Helper()
	_, files, err := evidenceFiles(in, digest([]byte("fixture")))
	if err != nil {
		t.Fatal(err)
	}
	// The stable envelope admits only ordinary directories, so resolve a
	// symlinked temporary directory (macOS /var) first.
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".git/refs"), 0o700); err != nil {
		t.Fatal(err)
	}
	for path, raw := range files {
		p := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// TestCEMV1295CapsuleAdmitsStableClosure (CEM-V1-295, CEM-V1-296): a real
// cem/1.0 enrollment derives its object demand from the exact raw map, the
// closure accounts all six artifact roles opaquely, and the stable verifier
// over the published minimal view accepts the same map with every result
// key, all six artifact checks and explicit unknown authority axes.
func TestCEMV1295CapsuleAdmitsStableClosure(t *testing.T) {
	f := newStableCapsuleFixture(t)
	in, objects, err := f.enrolled(t, f.change.Base, f.stable.Target, f.stable.Map, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := objects.requireEvidencePaths(in); err != nil {
		t.Fatalf("stable enrollment: %v", err)
	}
	// A check whose driver is at base and target and whose subject is at
	// the target is admitted by the same demand.
	in.enrollment.Checks = stableCapsuleChecks
	if err := objects.requireEvidencePaths(in); err != nil {
		t.Fatalf("stable enrollment with checks: %v", err)
	}
	_, closure, err := stableClosure(in)
	if err != nil || closure.MapSHA256 != stablefixture.Digest(f.stable.Map) || len(closure.Roles) != 6 {
		t.Fatalf("closure = %+v, %v", closure, err)
	}
	for index, role := range closure.Roles {
		if role.Kind != stableCapsuleRoles[index] || role.Path != "refs/"+role.Kind+".json" {
			t.Fatalf("role %d = %+v", index, role)
		}
	}
	view := publishedView(t, in)
	result, exit := verify.Stable(context.Background(), f.stable.Map, verify.StableOptions{
		Repository: view, ExpectedBase: f.change.Base, Target: f.stable.Target, ArtifactRoot: f.stable.ArtifactRoot,
	})
	if exit != 0 || result.Outcome != "ACCEPT" || result.Sidecar != "EXACT" || len(result.ArtifactChecks) != 6 ||
		result.MapSha256 == nil || *result.MapSha256 != closure.MapSHA256 {
		t.Fatalf("published-view verification: exit=%d code=%v issues=%v", exit, deref(result.Code), result.IssueCodes)
	}
	direct, _ := verify.Stable(context.Background(), f.stable.Map, verify.StableOptions{
		Repository: f.change.Root, ExpectedBase: f.change.Base, Target: f.stable.Target, ArtifactRoot: f.stable.ArtifactRoot,
	})
	viewKeys, directKeys := resultKeys(t, result), resultKeys(t, direct)
	if !reflect.DeepEqual(viewKeys, directKeys) || len(viewKeys) != 23 || !reflect.DeepEqual(result.Axes, direct.Axes) {
		t.Fatalf("view keys %v axes %v, direct keys %v axes %v", viewKeys, result.Axes, directKeys, direct.Axes)
	}
	for _, axis := range []string{"nativeAuthority", "authentication", "runnerExecution", "executionAtCommit", "criterionAdequacy"} {
		if result.Axes[axis] != "NOT_OBSERVED" {
			t.Fatalf("axis %s = %q, want explicit NOT_OBSERVED", axis, result.Axes[axis])
		}
	}
	// The artifacts stay opaque to the capsule: a tampered artifact still
	// admits the object closure, and the verifier over the published view
	// refuses with its whole result.
	tampered := filepath.Join(f.stable.ArtifactRoot, filepath.FromSlash(f.stable.ArtifactPaths[5]))
	if err := os.WriteFile(tampered, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := objects.requireEvidencePaths(in); err != nil {
		t.Fatalf("closure read an artifact: %v", err)
	}
	refused, exit := verify.Stable(context.Background(), f.stable.Map, verify.StableOptions{
		Repository: view, ExpectedBase: f.change.Base, Target: f.stable.Target, ArtifactRoot: f.stable.ArtifactRoot,
	})
	if exit != 1 || refused.Outcome != "REJECT" || refused.Stage != "artifacts" ||
		!reflect.DeepEqual(refused.IssueCodes, []string{"artifact-digest-mismatch"}) ||
		refused.Axes["referenceIntegrity"] != "FAILED" || len(resultKeys(t, refused)) != 23 {
		t.Fatalf("tampered artifact: exit=%d outcome=%s stage=%s issues=%v", exit, refused.Outcome, refused.Stage, refused.IssueCodes)
	}
}

// stableCapsuleDriver is unchanged and cited by nothing, so only a check
// demands its blob: the capsule's changed-path and evidence demands never do.
const stableCapsuleDriver = "tests/check.sh"

// stableCapsuleChecks drive the check demand: one check's driver and the
// other's subject is stableCapsuleDriver.
var stableCapsuleChecks = []localauthority.Check{
	{ID: "check-driver", DriverPath: stableCapsuleDriver, Subject: "src/app.txt"},
	{ID: "check-subject", DriverPath: "docs/rule.txt", Subject: stableCapsuleDriver},
}

func resultKeys(t *testing.T, result verify.StableResult) []string {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// TestCEMV1297CapsuleRefusesStaleOrIncompleteStableClosure (CEM-V1-297): a
// stale base, a map without the six roles, a sidecar that is not the
// enrolled bytes, a missing evidence blob, an evidence OID that names
// another carried blob, a missing check driver or subject blob and a missing
// sidecar all refuse before publication, and the enrolled bytes stay
// unchanged.
func TestCEMV1297CapsuleRefusesStableClosureControls(t *testing.T) {
	f := newStableCapsuleFixture(t)
	// A later commit re-commits the sidecar with the producer bytes, so its
	// committed sidecar is not the enrolled stable map.
	if err := os.WriteFile(filepath.Join(f.change.Root, filepath.FromSlash(wire.ExcludedCEMPath)), f.change.Producer, 0o644); err != nil {
		t.Fatal(err)
	}
	f.change.Git("add", "-f", wire.ExcludedCEMPath)
	f.change.Git("commit", "-qm", "other sidecar")
	otherTarget := f.change.Git("rev-parse", "HEAD")
	evidenceBlob := f.change.Git("rev-parse", f.change.Base+":docs/rule.txt")
	baseApp := f.change.Git("rev-parse", f.change.Base+":src/app.txt")
	driverBlob := f.change.Git("rev-parse", f.change.Base+":"+stableCapsuleDriver)
	// The evidence names another blob that the capsule does carry, so only
	// the OID binding can refuse it.
	rebound := []byte(strings.Replace(string(f.stable.Map), `"blobOid":"`+evidenceBlob+`"`, `"blobOid":"`+baseApp+`"`, 1))
	if string(rebound) == string(f.stable.Map) {
		t.Fatal("evidence blob OID not found in the stable map")
	}
	omitDriver := map[string]bool{driverBlob: true}
	absent := []localauthority.Check{{ID: "check-absent", DriverPath: "tests/absent.sh", Subject: "src/app.txt"}}
	cases := []struct {
		name, base, target, want string
		cem                      []byte
		omit                     map[string]bool
		checks                   []localauthority.Check
	}{
		{"stale-base", f.change.ProducerTarget, f.stable.Target, "capsule CEM base", f.stable.Map, nil, nil},
		{"missing-role", f.change.Base, f.stable.Target, "capsule stable role missing", f.change.Producer, nil, nil},
		{"sidecar-binding", f.change.Base, otherTarget, "capsule CEM sidecar binding", f.stable.Map, nil, nil},
		{"missing-sidecar", f.change.Base, f.change.ProducerTarget, "required capsule path missing", f.stable.Map, nil, nil},
		{"missing-evidence-blob", f.change.Base, f.stable.Target, "changed blob missing", f.stable.Map, map[string]bool{evidenceBlob: true}, nil},
		{"mismatched-evidence-oid", f.change.Base, f.stable.Target, "capsule CEM blob binding", rebound, nil, nil},
		{"missing-driver-blob", f.change.Base, f.stable.Target, "changed blob missing", f.stable.Map, omitDriver, stableCapsuleChecks[:1]},
		{"missing-subject-blob", f.change.Base, f.stable.Target, "changed blob missing", f.stable.Map, omitDriver, stableCapsuleChecks[1:]},
		{"missing-driver-path", f.change.Base, f.stable.Target, "required capsule path missing", f.stable.Map, nil, absent},
	}
	for _, test := range cases {
		cemBefore := append([]byte(nil), test.cem...)
		in, objects, err := f.enrolled(t, test.base, test.target, test.cem, test.omit)
		in.enrollment.Checks = test.checks
		if err == nil {
			err = objects.requireEvidencePaths(in)
		}
		if err == nil || err.Error() != test.want {
			t.Fatalf("%s: err=%v, want %q", test.name, err, test.want)
		}
		if string(in.raw["cem.json"]) != string(cemBefore) {
			t.Fatalf("%s: enrolled map changed", test.name)
		}
	}
}

func deref(value *string) string {
	if value == nil {
		return "<nil>"
	}
	return *value
}
