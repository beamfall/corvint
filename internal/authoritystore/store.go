package authoritystore

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/authorityevent"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/frontiernext"
	"github.com/Beamfall/corvint/internal/frontiernextrepo"
	"github.com/Beamfall/corvint/internal/localauthority"
)

var errUnavailable = errors.New("protected-authority-unavailable")

const MaxArtifact = 4 << 20

// protectedFiles is private: no public API may substitute a filesystem root.
// Each read validates every descriptor's owner, mode, links and ACL before and
// after bounded ingestion. Nothing below reads the private signing directory.
type protectedFiles interface {
	read(string, uint32, int) ([]byte, error)
}

type publication struct {
	enrollment          localauthority.Enrollment
	receipt             localauthority.Receipt
	cem, ocm, selection []byte
	target              Target
	terminal            Terminal
	evidence            EvidenceReference
}

// Resolve reads the current independently accepted root and completed public
// artifacts from the fixed operator store. Missing installation is unavailable.
// It neither creates policy nor enrolls, signs, runs tests, or writes a ledger.
func Resolve(ctx context.Context, handle string) (authorityevent.Resolution, error) {
	files, err := openProtectedFiles()
	if err != nil {
		return authorityevent.Resolution{}, errUnavailable
	}
	return resolve(ctx, handle, files, time.Now())
}

func resolve(ctx context.Context, handle string, files protectedFiles, now time.Time) (authorityevent.Resolution, error) {
	return resolveObserved(ctx, handle, files, now, nil)
}

// A lifecycle observer is internal read-only preparation, never a caller-supplied
// resolver or a substitute for any protected check below.
func resolveObserved(ctx context.Context, handle string, files protectedFiles, now time.Time, observe LifecycleObserver) (authorityevent.Resolution, error) {
	return resolveObservedProfile(ctx, handle, files, now, observe, RootProfile)
}

func resolveObservedProfile(ctx context.Context, handle string, files protectedFiles, now time.Time, observe LifecycleObserver, profile lifecycleProfile) (authorityevent.Resolution, error) {
	var unavailable authorityevent.Resolution
	deriveActive := observe != nil && handle == ""
	if (!hexDigest(handle) && !deriveActive) || ctx.Err() != nil {
		return unavailable, errUnavailable
	}
	rootBytes, err := files.read("accepted-root.json", 0, localauthority.MaxReceiptBytes)
	if err != nil {
		return unavailable, errUnavailable
	}
	floorBytes, err := files.read("minimum-generation.json", 0, 4096)
	if err != nil {
		return unavailable, errUnavailable
	}
	root, floor, owner, err := decodeRoot(rootBytes, floorBytes)
	if err != nil || root.Profile != string(profile) || owner == uint32(os.Getuid()) || os.Getuid() == 0 {
		return unavailable, errUnavailable
	}
	policyBytes, err := files.read("execution-policy.json", 0, localauthority.MaxReceiptBytes)
	if err != nil || localauthority.BytesDigest(policyBytes) != root.PolicySHA256 {
		return unavailable, errUnavailable
	}
	var derivedActiveBytes []byte
	if deriveActive {
		derivedActiveBytes, handle, err = readActiveEnrollment(files, owner)
		if err != nil {
			return unavailable, errUnavailable
		}
	}
	admit := admitRuntime
	if deriveActive {
		admit = admitLifecycleRuntime
	}
	admission, err := admit(ctx, root, files, floorBytes, handle, now)
	if err != nil {
		return unavailable, errUnavailable
	}
	binding, err := bindCurrentRepository(ctx, root)
	if err != nil {
		return unavailable, errUnavailable
	}
	defer binding.close()
	activeBytes, err := files.read("public/active-enrollment.json", owner, 1024)
	var active ActiveEnrollment
	if err != nil || localauthority.Decode(activeBytes, &active) != nil || active.Profile != "corvint-protected-active-enrollment/0" || active.EnrollmentHandle != handle || (deriveActive && !bytes.Equal(derivedActiveBytes, activeBytes)) {
		return unavailable, errUnavailable
	}
	p, err := readPublication(files, handle, owner)
	if err != nil {
		return unavailable, errUnavailable
	}
	snapshot, err := validatePublication(root, floor, handle, p)
	if err != nil || !admission.targetMatches(p.enrollment.Binding.Target) {
		return unavailable, errUnavailable
	}
	if _, _, err = requireEvidenceDescriptorHeadroom(); err != nil {
		return unavailable, errUnavailable
	}
	view, err := openEvidenceView(ctx, root, handle, p)
	if err != nil {
		return unavailable, errUnavailable
	}
	immutable, err := gitauth.Open(view.root, gitrun.NewDefaultBudget())
	if err != nil {
		return unavailable, errUnavailable
	}
	repo := binding.repo
	if err = repo.UseObjectView(ctx, immutable); err != nil {
		return unavailable, errUnavailable
	}
	// Only this trusted callsite creates the request memo, after the complete
	// protected view audit and format match. Live currentness never receives it.
	memo, err := gitauth.NewRequestReadMemo(immutable)
	if err != nil {
		return unavailable, errUnavailable
	}
	defer memo.Release()
	if err = repo.RequireTargetState(ctx, p.enrollment.Binding.Target); err != nil {
		return unavailable, errUnavailable
	}
	request := frontiernext.Request{CEM: p.cem, OCM: p.ocm, Selection: p.selection, Enrollment: p.enrollment, Receipt: p.receipt}
	result, err := frontiernext.Compute(ctx, request, snapshot, now, frontiernextrepo.NewRequestAdapter(memo))
	if err != nil || result.Authority != "VERIFIED" {
		return unavailable, errUnavailable
	}
	if observe != nil {
		scope, scopeErr := admission.lifecycleScope(root)
		if scopeErr != nil || observe(ctx, scope) != nil {
			return unavailable, errUnavailable
		}
	}
	if err = repo.RequireCleanTarget(ctx, p.enrollment.Binding.Target); err != nil {
		return unavailable, errUnavailable
	}
	// Admission and revocation are current reads, not a memoized caller assertion.
	if err = unchanged(files, "accepted-root.json", 0, rootBytes); err != nil {
		return unavailable, errUnavailable
	}
	if err = unchanged(files, "minimum-generation.json", 0, floorBytes); err != nil {
		return unavailable, errUnavailable
	}
	if err = unchanged(files, "execution-policy.json", 0, policyBytes); err != nil {
		return unavailable, errUnavailable
	}
	if err = unchanged(files, "public/active-enrollment.json", owner, activeBytes); err != nil {
		return unavailable, errUnavailable
	}
	again, err := readPublication(files, handle, owner)
	if err != nil || !reflect.DeepEqual(p, again) || ctx.Err() != nil {
		return unavailable, errUnavailable
	}
	if err = admission.recheck(ctx, root, files, floorBytes, handle, p.enrollment.Binding.Target, time.Now()); err != nil {
		return unavailable, errUnavailable
	}
	if err = view.unchanged(ctx, p.evidence); err != nil {
		return unavailable, errUnavailable
	}
	if err = binding.unchanged(ctx, root); err != nil {
		return unavailable, errUnavailable
	}
	if admission.campaign != nil {
		if !campaignStillCurrentScope(files, admission.raw, root, floorBytes, handle, p.enrollment.Binding.Target, time.Now(), admission.scope) {
			return unavailable, errUnavailable
		}
		return authorityevent.Resolution{State: result.State, UniverseSHA256: result.UniverseSHA256, RootCurrent: true, Exercise: &authorityevent.QualificationExercise{CampaignID: admission.campaign.CampaignID, StopPermitted: root.RemediationAllowed && admission.campaign.AllowStopRemediation}}, nil
	}
	qualification, err := localauthority.Digest(root.qualification())
	if err != nil {
		return unavailable, errUnavailable
	}
	resolution := authorityevent.Resolution{State: result.State, UniverseSHA256: result.UniverseSHA256, RootCurrent: true, QualifiedHostSHA256: qualification, RemediationAllowed: root.RemediationAllowed}
	if root.Profile == DirectRootProfile {
		resolution.SupportScope = "qualified-direct-native-runtime"
		resolution.QualifiedSurfaces = []string{"codex-cli"}
	} else if root.HostQualification.Topology == "shared-daemon" {
		resolution.SupportScope = "qualified-shared-runtime"
		for _, surface := range root.HostQualification.QualifiedSurfaces {
			resolution.QualifiedSurfaces = append(resolution.QualifiedSurfaces, surface.Surface)
		}
	}
	return resolution, nil
}

func unchanged(files protectedFiles, path string, owner uint32, before []byte) error {
	after, err := files.read(path, owner, len(before))
	if err != nil || !bytes.Equal(before, after) {
		return errUnavailable
	}
	return nil
}

func hexDigest(value string) bool {
	if len(value) != 64 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func decimal(value string) (uint64, bool) {
	number, err := strconv.ParseUint(value, 10, 64)
	return number, err == nil && strconv.FormatUint(number, 10) == value
}

func decodeRoot(raw, floorRaw []byte) (RootDocument, GenerationFloor, uint32, error) {
	var root RootDocument
	var floor GenerationFloor
	if localauthority.Decode(raw, &root) != nil || localauthority.Decode(floorRaw, &floor) != nil {
		return root, floor, 0, errUnavailable
	}
	if (root.Profile != RootProfile && root.Profile != DirectRootProfile) || root.Admission != "OPERATOR_ACCEPTED" || root.KeyClass != "PRODUCTION" || root.Revoked {
		return root, floor, 0, errUnavailable
	}
	if root.RootID == "" || strings.Contains(strings.ToLower(root.RootID), "fixture") {
		return root, floor, 0, errUnavailable
	}
	if floor.Profile != FloorProfile || floor.RootID != root.RootID || floor.Epoch != root.Epoch {
		return root, floor, 0, errUnavailable
	}
	generation, ok := decimal(root.Generation)
	if !ok {
		return root, floor, 0, errUnavailable
	}
	minimum, ok := decimal(floor.Generation)
	if !ok || generation < minimum {
		return root, floor, 0, errUnavailable
	}
	if _, ok = decimal(root.Epoch); !ok {
		return root, floor, 0, errUnavailable
	}
	owner, ok := decimal(root.AuthorityUID)
	if !ok || owner == 0 || owner > 65535 {
		return root, floor, 0, errUnavailable
	}
	gid, gok := decimal(root.AuthorityGID)
	reader, rok := decimal(root.ReaderUID)
	if !gok || !rok || gid == 0 || gid > 65535 || reader == 0 || reader > 65535 || reader == owner {
		return root, floor, 0, errUnavailable
	}
	if !hexDigest(root.RepositoryID) || !hexDigest(root.PolicySHA256) || !hexDigest(root.PublicKey) {
		return root, floor, 0, errUnavailable
	}
	if !filepath.IsAbs(root.RepositoryRoot) || filepath.Clean(root.RepositoryRoot) != root.RepositoryRoot {
		return root, floor, 0, errUnavailable
	}
	if len(root.Checks) == 0 || len(root.Checks) > 256 || root.Audience == "" {
		return root, floor, 0, errUnavailable
	}
	return root, floor, uint32(owner), nil
}

func readPublication(files protectedFiles, handle string, owner uint32) (publication, error) {
	var p publication
	prefix := "public/" + handle + "/"
	documents := []struct {
		name  string
		value any
	}{{"enrollment.json", &p.enrollment}, {"receipt.json", &p.receipt}, {"target.json", &p.target}, {"terminal.json", &p.terminal}, {"evidence.json", &p.evidence}}
	for _, document := range documents {
		raw, err := files.read(prefix+document.name, owner, localauthority.MaxReceiptBytes)
		if err != nil || localauthority.Decode(raw, document.value) != nil {
			return p, errUnavailable
		}
	}
	var err error
	if p.cem, err = files.read(prefix+"cem.json", owner, MaxArtifact); err != nil {
		return p, errUnavailable
	}
	if p.ocm, err = files.read(prefix+"ocm.json", owner, MaxArtifact); err != nil {
		return p, errUnavailable
	}
	if p.selection, err = files.read(prefix+"selection.json", owner, localauthority.MaxReceiptBytes); err != nil {
		return p, errUnavailable
	}
	return p, nil
}

func validatePublication(root RootDocument, floor GenerationFloor, handle string, p publication) (localauthority.PolicySnapshot, error) {
	var snapshot localauthority.PolicySnapshot
	identity, err := localauthority.Digest(p.enrollment)
	if err != nil || identity != handle {
		return snapshot, errUnavailable
	}
	if p.target.Profile != TargetProfile || p.target.RepositoryRoot != root.RepositoryRoot || p.target.RepositoryID != root.RepositoryID {
		return snapshot, errUnavailable
	}
	binding := p.enrollment.Binding
	if p.target.Base != binding.Base || p.target.Target != binding.Target || p.target.Tree != binding.Tree {
		return snapshot, errUnavailable
	}
	receiptDigest, err := localauthority.Digest(p.receipt)
	if err != nil || p.terminal.Profile != TerminalProfile || p.terminal.State != "COMPLETE" || p.terminal.Nonce != p.enrollment.Nonce || p.terminal.ReceiptSHA256 != receiptDigest {
		return snapshot, errUnavailable
	}
	key, err := hex.DecodeString(root.PublicKey)
	if err != nil || len(key) != ed25519.PublicKeySize {
		return snapshot, errUnavailable
	}
	snapshot = localauthority.PolicySnapshot{RepositoryID: root.RepositoryID, PolicySHA256: root.PolicySHA256, Accepted: true, Current: true, RootID: root.RootID, PublicKey: key, Epoch: root.Epoch, Generation: root.Generation, MinimumGeneration: floor.Generation, Audience: root.Audience, Checks: root.Checks, TerminalSHA256: map[string]string{p.enrollment.Nonce: receiptDigest}}
	return snapshot, nil
}

// LoadExecutionRoot is shared with the protected authority's one-shot phase.
// It validates only operator execution admission, not native host qualification.
func LoadExecutionRoot() (RootDocument, GenerationFloor, error) {
	var root RootDocument
	var floor GenerationFloor
	files, err := openProtectedFiles()
	if err != nil {
		return root, floor, errUnavailable
	}
	raw, err := files.read("accepted-root.json", 0, localauthority.MaxReceiptBytes)
	if err != nil {
		return root, floor, errUnavailable
	}
	floorRaw, err := files.read("minimum-generation.json", 0, 4096)
	if err != nil {
		return root, floor, errUnavailable
	}
	root, floor, _, err = decodeRoot(raw, floorRaw)
	if err != nil {
		return root, floor, errUnavailable
	}
	policy, err := files.read("execution-policy.json", 0, localauthority.MaxReceiptBytes)
	if err != nil || localauthority.BytesDigest(policy) != root.PolicySHA256 {
		return root, floor, errUnavailable
	}
	return root, floor, nil
}

// ReadExecutionPolicy reads only the fixed root-owned execution-policy file.
// Its caller checks the exact bytes against the independently loaded root pin.
func ReadExecutionPolicy() ([]byte, error) {
	files, err := openProtectedFiles()
	if err != nil {
		return nil, errUnavailable
	}
	return files.read("execution-policy.json", 0, localauthority.MaxReceiptBytes)
}

// ReadExecutionFile is the signer's bounded reader for the fixed private
// enrollment/journal/key and immutable version tree. It confers no OS access;
// the hook never invokes it and never reads the private signing key.
func ReadExecutionFile(relative string, owner uint32, limit int) ([]byte, error) {
	if !executionPath(relative) || limit <= 0 || limit > 256<<20 {
		return nil, errUnavailable
	}
	files, err := openProtectedFiles()
	if err != nil {
		return nil, errUnavailable
	}
	return files.read(relative, owner, limit)
}

func executionPath(path string) bool {
	if filepath.IsAbs(path) || filepath.Clean(path) != path {
		return false
	}
	if path == "private/signing-key" {
		return true
	}
	parts := strings.Split(path, "/")
	if len(parts) == 4 && parts[0] == "private" && parts[1] == "enrollments" && hexDigest(parts[2]) {
		switch parts[3] {
		case "enrollment.json", "objects.json", "cem.json", "ocm.json", "selection.json", "target.json":
			return true
		}
	}
	if len(parts) == 3 && parts[0] == "private" && parts[1] == "journal" {
		nonce, suffix, ok := strings.Cut(parts[2], ".")
		return ok && hexDigest(nonce) && (suffix == "pending" || suffix == "built" || suffix == "terminal")
	}
	if len(parts) >= 3 && parts[0] == "versions" && hexDigest(parts[1]) {
		if len(parts) == 3 && parts[2] == "local-authority" {
			return true
		}
		return len(parts) > 3 && parts[2] == "go"
	}
	return false
}
