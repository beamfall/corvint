package localcompletion

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/dogfoodflow"
	"github.com/Beamfall/corvint/internal/tracerecordrepo"
)

// TransportAdaptedRecoveryProfile is the closed request profile of the
// recovery-only aggregate finish that substitutes explicitly admitted,
// transport-adapted historical BASE and TREE verifiers (ALO-V0-023..026).
const TransportAdaptedRecoveryProfile = "corvint-transport-adapted-held-recovery/0"

// TransportAdaptedQualification names the changed verifier identity claim. It
// never satisfies a pristine historical-binary requirement.
const TransportAdaptedQualification = "TRANSPORT_ADAPTED_HISTORICAL"

const maxTransportRecoveryBytes = 16 << 10

// TransportRecoveryVerifier is one role's executable identity and the
// provenance it claims: an exact historical revision and tree plus the digest
// of the only adapter patch applied to that source.
type TransportRecoveryVerifier struct {
	AdapterPatchSHA256 string `json:"adapterPatchSha256"`
	HistoricalRevision string `json:"historicalRevision"`
	HistoricalTree     string `json:"historicalTree"`
	Path               string `json:"path"`
	SHA256             string `json:"sha256"`
}

// TransportRecoveryRequest is the closed canonical request. It names no store,
// root or key other than the enrollment the caller already selected.
type TransportRecoveryRequest struct {
	Base          TransportRecoveryVerifier `json:"base"`
	PlanDigest    string                    `json:"planDigest"`
	Profile       string                    `json:"profile"`
	Qualification string                    `json:"qualification"`
	Session       string                    `json:"session"`
	Tree          TransportRecoveryVerifier `json:"tree"`
}

// TransportRecoveryProvenance is returned with the recovery evaluation so the
// caller can retain the changed qualification beside the published report.
type TransportRecoveryProvenance struct {
	Qualification string `json:"qualification"`
	RequestSHA256 string `json:"requestSha256"`
	BaseSHA256    string `json:"baseVerifierSha256"`
	TreeSHA256    string `json:"treeVerifierSha256"`
}

// transportAdaptation is one owner-admitted recovery: the exact enrollment and
// the exact historical sources and adapter patches it may name.
type transportAdaptation struct {
	session, planDigest               string
	baseRevision, baseTree, basePatch string
	heldRevision, heldTree, heldPatch string
}

// admittedTransportAdaptations is closed. Only the #443 held recovery admitted
// by the owner's 2026-10-04 transport-adapter decision is listed. Its patch
// digests pin the owner-widened four-file adapter, which also guards
// internal/cem/gitrun/session.go (owner approval, 2026-10-04).
var admittedTransportAdaptations = []transportAdaptation{{
	session:      "b36b7ba8858646e93b1b86ac88933736cb66ad8974542e34ebeead7693f9f989",
	planDigest:   "3c6bab794327c66c014b7523ff64f278008389f373a288141dbe3b862f3d666c",
	baseRevision: "406f9dc3cb80d6af527de5f160370e132e324e9a",
	baseTree:     "e0dfeaa577e32c3793f686662f4a40952ff04109",
	basePatch:    "sha256:c610b712846b044b2c2f2050a08dfe81803a0c90ff909ca1193629316f0dd9b5",
	heldRevision: "a79439afd1a8dd0650e598b7a1ec51d79c5c6075",
	heldTree:     "d6d6b89020cf8459e2e019bd4e443e9450926147",
	heldPatch:    "sha256:6f58ea184aa98edbc7919f1f0ec23259d869517ac992184c9650d82d9f6e0c0c",
}}

// transportRecoveryTestAdmissions is empty in every product build. Only a test
// build links extra admissions with -ldflags -X, as ";"-separated entries of
// eight ","-separated hex fields in transportAdaptation order. No runtime
// input, environment variable or request field can reach it.
var transportRecoveryTestAdmissions string

func transportAdmissions() []transportAdaptation {
	entries := slices.Clone(admittedTransportAdaptations)
	if transportRecoveryTestAdmissions == "" {
		return entries
	}
	for _, row := range strings.Split(transportRecoveryTestAdmissions, ";") {
		f := strings.Split(row, ",")
		if len(f) != 8 {
			return nil
		}
		entries = append(entries, transportAdaptation{f[0], f[1], f[2], f[3], "sha256:" + f[4], f[5], f[6], "sha256:" + f[7]})
	}
	return entries
}

// ReadTransportRecoveryRequest reads one bounded regular request file.
func ReadTransportRecoveryRequest(name string) ([]byte, error) {
	parent, err := filepath.EvalSymlinks(filepath.Dir(name))
	if err != nil {
		return nil, errors.New("transport-recovery-request-unavailable")
	}
	raw, err := readFile(filepath.Join(parent, filepath.Base(name)), maxTransportRecoveryBytes)
	if err != nil {
		return nil, errors.New("transport-recovery-request-unavailable")
	}
	return raw, nil
}

// FinishAggregateRecovery is the recovery-only aggregate finish. It is the
// ordinary aggregate finish transaction except that its strict check runs the
// request's admitted, independent BASE and TREE verifiers instead of the
// running binary. Finish and FinishWithAggregateProfile are unchanged.
func FinishAggregateRecovery(ctx context.Context, root, key string, request []byte, command PublicCommand) (Evaluation, *TransportRecoveryProvenance, error) {
	recovery, err := parseTransportRecovery(request)
	if err != nil {
		return Evaluation{}, nil, err
	}
	evaluation, err := finishWithProfile(ctx, root, key, tracerecordrepo.AggregateOutcomeProfile, command, recovery)
	return evaluation, recovery.provenance(), err
}

type transportRecovery struct {
	request TransportRecoveryRequest
	digest  string
}

func parseTransportRecovery(raw []byte) (*transportRecovery, error) {
	invalid := errors.New("transport-recovery-request-invalid")
	if len(raw) == 0 || len(raw) > maxTransportRecoveryBytes {
		return nil, invalid
	}
	var value TransportRecoveryRequest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&value) != nil {
		return nil, invalid
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, invalid
	}
	canonical, err := json.Marshal(value)
	if err != nil || !bytes.Equal(append(canonical, '\n'), raw) {
		return nil, invalid
	}
	if value.Profile != TransportAdaptedRecoveryProfile || value.Qualification != TransportAdaptedQualification || !keyPattern.MatchString(value.Session) || !keyPattern.MatchString(value.PlanDigest) {
		return nil, invalid
	}
	for _, role := range []TransportRecoveryVerifier{value.Base, value.Tree} {
		if !filepath.IsAbs(role.Path) || filepath.Clean(role.Path) != role.Path || !validAggregateDigest(role.SHA256) || !validAggregateDigest(role.AdapterPatchSHA256) || !gitObjectPattern(role.HistoricalRevision) || !gitObjectPattern(role.HistoricalTree) {
			return nil, invalid
		}
	}
	if value.Base.Path == value.Tree.Path || value.Base.SHA256 == value.Tree.SHA256 {
		return nil, errors.New("transport-recovery-fixed-verifier")
	}
	return &transportRecovery{request: value, digest: prefixedDigest(raw)}, nil
}

func gitObjectPattern(value string) bool {
	if len(value) != 40 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && strings.ToLower(value) == value
}

func (r *transportRecovery) provenance() *TransportRecoveryProvenance {
	if r == nil {
		return nil
	}
	return &TransportRecoveryProvenance{Qualification: TransportAdaptedQualification, RequestSHA256: r.digest, BaseSHA256: r.request.Base.SHA256, TreeSHA256: r.request.Tree.SHA256}
}

// admit binds the request to the selected enrollment, its unchanged BASE and
// the current HELD target before any producer, preservation or publication.
func (r *transportRecovery) admit(ctx context.Context, repo *repository, saved *state, snap snapshot) error {
	request := r.request
	if a := saved.AggregateOutcome; a != nil && (a.Phase == "COMMITTED" || len(a.Issued) >= 3) {
		return errors.New("transport-recovery-not-pending")
	}
	if request.Session != saved.Session || request.PlanDigest != saved.PlanDigest {
		return errors.New("transport-recovery-enrollment-mismatch")
	}
	admitted := false
	for _, entry := range transportAdmissions() {
		if entry.session == request.Session && entry.planDigest == request.PlanDigest &&
			entry.baseRevision == request.Base.HistoricalRevision && entry.baseTree == request.Base.HistoricalTree && entry.basePatch == request.Base.AdapterPatchSHA256 &&
			entry.heldRevision == request.Tree.HistoricalRevision && entry.heldTree == request.Tree.HistoricalTree && entry.heldPatch == request.Tree.AdapterPatchSHA256 {
			admitted = true
		}
	}
	if !admitted {
		return errors.New("transport-recovery-not-admitted")
	}
	if request.Base.HistoricalRevision != saved.Plan.Base {
		return errors.New("transport-recovery-base-mismatch")
	}
	baseTree, err := repo.git(ctx, "rev-parse", saved.Plan.Base+"^{tree}")
	if err != nil || strings.TrimSpace(string(baseTree)) != request.Base.HistoricalTree {
		return errors.New("transport-recovery-base-mismatch")
	}
	if request.Tree.HistoricalRevision != snap.target || request.Tree.HistoricalTree != snap.tree {
		return errors.New("transport-recovery-held-mismatch")
	}
	return r.verifyIdentities()
}

// verifyIdentities requires both executables to be the requested regular files
// and neither to be the running binary, so no fixed role is substituted.
func (r *transportRecovery) verifyIdentities() error {
	self, err := os.Executable()
	if err != nil {
		return errors.New("transport-recovery-identity-unavailable")
	}
	selfSHA, err := regularFileSHA256(self)
	if err != nil {
		return errors.New("transport-recovery-identity-unavailable")
	}
	for _, role := range []TransportRecoveryVerifier{r.request.Base, r.request.Tree} {
		actual, err := regularFileSHA256(role.Path)
		if err != nil {
			return errors.New("transport-recovery-identity-unavailable")
		}
		if actual != role.SHA256 {
			return errors.New("transport-recovery-identity-drift")
		}
		if actual == selfSHA {
			return errors.New("transport-recovery-fixed-verifier")
		}
	}
	return nil
}

func regularFileSHA256(name string) (string, error) {
	info, err := os.Lstat(name)
	if err != nil || !info.Mode().IsRegular() {
		return "", errors.New("not-regular")
	}
	file, err := os.Open(name)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err = io.Copy(hash, file); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

// configure substitutes the admitted verifiers into the pending strict check
// and refuses publication unless the proposed report records exactly them.
func (r *transportRecovery) configure(options dogfoodflow.CheckOptions) dogfoodflow.CheckOptions {
	options.BaseVerifier = transportRecoveryRunner(r.request.Base.Path)
	options.TreeVerifier = transportRecoveryRunner(r.request.Tree.Path)
	options.Override = nil
	publish := options.AggregatePublish
	options.AggregatePublish = func(ctx context.Context, root string, capture dogfoodflow.AggregateCheckCapture) error {
		if capture.Exit == 0 && !capture.Failed {
			if err := r.verifyIdentities(); err != nil {
				return err
			}
			next, err := dogfoodflow.ParseAggregateReport(capture.ProposedReport)
			if err != nil {
				return err
			}
			if next.DogfoodCheck == nil || next.DogfoodCheck.BaseVerifierSHA256 != r.request.Base.SHA256 || next.DogfoodCheck.TreeVerifierSHA256 != r.request.Tree.SHA256 || next.DogfoodCheck.OverrideVerifierSHA256 != nil {
				return errors.New("transport-recovery-identity-drift")
			}
		}
		return publish(ctx, root, capture)
	}
	return options
}

// transportRecoveryRunner is only reached through the aggregate strict check,
// which execs the dogfood-verifier-worker startup route. Its public-route Run
// refuses, so a non-aggregate report cannot silently use a historical binary.
func transportRecoveryRunner(path string) dogfoodflow.Runner {
	return dogfoodflow.Runner{Path: path, Run: func(_ context.Context, _ string, _ []string, _, stderr io.Writer) int {
		_, _ = io.WriteString(stderr, "transport-recovery-requires-aggregate-check\n")
		return 2
	}}
}
