package postmergeconnector

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"slices"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

const RepublishProfile = "postmerge-republish-plan/0"
const RepublishPendingProfile = "postmerge-republish-pending/0"

// RepublishInput has no title, body, template or approval field. Admission is
// performed by the independently configured host before this local renderer.
type RepublishInput struct {
	Repository            string   `json:"repository"`
	Target                string   `json:"target"`
	DocumentationRevision string   `json:"documentation_revision"`
	SourceRevision        string   `json:"source_revision"`
	PolicySHA256          string   `json:"policy_sha256"`
	ResultSHA256          string   `json:"result_sha256"`
	CorpusSHA256          string   `json:"corpus_sha256"`
	IndexSHA256           string   `json:"index_sha256"`
	Added                 int      `json:"added"`
	Retired               int      `json:"retired"`
	Changed               int      `json:"changed"`
	EvidenceURL           string   `json:"evidence_url"`
	URLOrigins            []string `json:"url_origins"`
}
type RepublishPlan struct {
	Profile string         `json:"profile"`
	Input   RepublishInput `json:"input"`
	Request Request        `json:"request"`
}
type RepublishPending struct {
	Profile    string        `json:"profile"`
	Generation uint64        `json:"generation"`
	Plan       RepublishPlan `json:"plan"`
}

func RepublishKey(repository, target string) string {
	return "pm-" + digest([]string{repository, target, "corpus-republish"})
}
func BuildRepublishPlan(in RepublishInput) (RepublishPlan, error) {
	if !wire.IsGitOid(in.Repository) || !idPattern.MatchString(in.Target) || !wire.IsGitOid(in.DocumentationRevision) || !wire.IsGitOid(in.SourceRevision) {
		return RepublishPlan{}, fmt.Errorf("republish identity invalid")
	}
	for _, s := range []string{in.PolicySHA256, in.ResultSHA256, in.CorpusSHA256, in.IndexSHA256} {
		if !wire.IsSha256(s) {
			return RepublishPlan{}, fmt.Errorf("republish digest invalid")
		}
	}
	for _, n := range []int{in.Added, in.Retired, in.Changed} {
		if n < 0 || n > 1000000 {
			return RepublishPlan{}, fmt.Errorf("republish parity count invalid")
		}
	}
	if len(in.URLOrigins) == 0 || len(in.URLOrigins) > 16 || !slices.IsSorted(in.URLOrigins) || hasDuplicateOrigin(in.URLOrigins) || !validURL(in.EvidenceURL, Policy{URLOrigins: in.URLOrigins}) {
		return RepublishPlan{}, fmt.Errorf("republish evidence URL invalid")
	}
	key := RepublishKey(in.Repository, in.Target)
	r := Request{Profile: Profile, Key: key, Operation: "upsert-draft-change", Source: in.Repository, Parent: in.Target, Route: "ordinary", Branch: "corvint/" + key + "/corpus", Draft: true}
	r.Body = fmt.Sprintf("Corpus republish\nRepository: %s\nTarget: %s\nDocumentation: %s\nSource: %s\nPolicy: %s\nResult: %s\nCorpus: %s\nIndex: %s\nAdded: %d\nRetired: %d\nChanged: %d\nEvidence: %s\nDraft: true\n", in.Repository, in.Target, in.DocumentationRevision, in.SourceRevision, in.PolicySHA256, in.ResultSHA256, in.CorpusSHA256, in.IndexSHA256, in.Added, in.Retired, in.Changed, in.EvidenceURL)
	return RepublishPlan{RepublishProfile, in, r}, nil
}
func ValidateRepublishPlan(plan RepublishPlan) error {
	actual, err := BuildRepublishPlan(plan.Input)
	if err != nil {
		return err
	}
	a, _ := Encode(actual)
	b, _ := Encode(plan)
	if !bytes.Equal(a, b) {
		return fmt.Errorf("republish plan mismatch")
	}
	return nil
}

// ApplyRepublishLocal uses the same stable-key Writer consumer as ordinary
// connector plans. A second approved documentation revision replaces one key.
func ApplyRepublishLocal(ctx context.Context, plan RepublishPlan, state State) (State, error) {
	if err := ValidateRepublishPlan(plan); err != nil {
		return State{}, err
	}
	if state.Profile != Profile || state.Tracker == nil || state.Forge == nil {
		return State{}, fmt.Errorf("republish state invalid")
	}
	next := NewState()
	for k, v := range state.Tracker {
		if k != v.Key || v.Profile != Profile || v.Operation == "upsert-draft-change" {
			return State{}, fmt.Errorf("republish tracker state invalid")
		}
		next.Tracker[k] = v
	}
	for k, v := range state.Forge {
		if k != v.Key || v.Profile != Profile || v.Operation != "upsert-draft-change" || !v.Draft {
			return State{}, fmt.Errorf("republish forge state invalid")
		}
		next.Forge[k] = v
	}
	if err := ctx.Err(); err != nil {
		return State{}, err
	}
	if err := (memoryWriter{next.Forge}).Upsert(plan.Request); err != nil {
		return State{}, err
	}
	return next, nil
}
func readRepublishPrior(name string) ([]byte, bool, error) {
	if _, err := os.Lstat(name); os.IsNotExist(err) {
		return nil, false, nil
	} else if err != nil {
		return nil, false, err
	}
	raw, err := ReadFile(name)
	return raw, true, err
}

// SaveRepublishPending requires a trusted private single-writer directory.
// The plan must be rederived through corpusrepublish by the host before calling.
// Rename is the local commit point; concurrent CAS and power-loss are unqualified.
func SaveRepublishPending(ctx context.Context, plan RepublishPlan, name, authorRoot string, inputs, roots []string, expectedGeneration uint64, expectedSHA256 string) (RepublishPending, bool, error) {
	if err := ValidateRepublishPlan(plan); err != nil {
		return RepublishPending{}, false, err
	}
	if len(roots) == 0 {
		return RepublishPending{}, false, fmt.Errorf("republish protected roots missing")
	}
	protected := slices.Clone(inputs)
	for _, root := range append(slices.Clone(roots), authorRoot) {
		paths, err := ProtectedGitPaths(ctx, root)
		if err != nil {
			return RepublishPending{}, false, err
		}
		protected = append(protected, root)
		protected = append(protected, paths...)
	}
	if err := CheckDestination(name, authorRoot, protected); err != nil {
		return RepublishPending{}, false, err
	}
	if err := ctx.Err(); err != nil {
		return RepublishPending{}, false, err
	}
	prior, exists, err := readRepublishPrior(name)
	if err != nil {
		return RepublishPending{}, false, err
	}
	old := RepublishPending{}
	if exists {
		if err := Decode(prior, &old); err != nil {
			return old, false, err
		}
		raw, e := Encode(old)
		if e != nil || !bytes.Equal(raw, prior) || old.Profile != RepublishPendingProfile || old.Generation == 0 || ValidateRepublishPlan(old.Plan) != nil || old.Plan.Input.Repository != plan.Input.Repository || old.Plan.Input.Target != plan.Input.Target {
			return old, false, fmt.Errorf("republish pending invalid")
		}
		oldPlan, _ := Encode(old.Plan)
		newPlan, _ := Encode(plan)
		if bytes.Equal(oldPlan, newPlan) {
			return old, true, nil
		}
	}
	if !exists {
		if expectedGeneration != 0 || expectedSHA256 != "NONE" {
			return old, false, fmt.Errorf("republish expected prior missing")
		}
	} else if expectedGeneration != old.Generation || !wire.IsSha256(expectedSHA256) || digestBytes(prior) != expectedSHA256 {
		return old, false, fmt.Errorf("republish pending stale")
	}
	if old.Generation == math.MaxUint64 {
		return old, false, fmt.Errorf("republish generation overflow")
	}
	next := RepublishPending{RepublishPendingProfile, old.Generation + 1, plan}
	raw, err := Encode(next)
	if err != nil || len(raw) > MaxBytes {
		return old, false, fmt.Errorf("republish pending output invalid")
	}
	f, err := os.CreateTemp(filepath.Dir(name), ".republish-*")
	if err != nil {
		return old, false, err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	n, err := f.Write(raw)
	if err == nil && n != len(raw) {
		err = io.ErrShortWrite
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return old, false, err
	}
	if closeErr != nil {
		return old, false, closeErr
	}
	if err := ctx.Err(); err != nil {
		return old, false, err
	}
	if err := CheckDestination(name, authorRoot, protected); err != nil {
		return old, false, err
	}
	current, currentExists, err := readRepublishPrior(name)
	if err != nil || currentExists != exists || !bytes.Equal(current, prior) {
		return old, false, fmt.Errorf("republish pending changed before replace")
	}
	if err := ctx.Err(); err != nil {
		return old, false, err
	}
	if err := os.Rename(tmp, name); err != nil {
		return old, false, err
	}
	return next, false, nil
}
func digestBytes(b []byte) string {
	// Keep canonical-object digests separate from original-byte state pins.
	return fmt.Sprintf("%x", sha256.Sum256(b))
}

func hasDuplicateOrigin(origins []string) bool {
	for i := 1; i < len(origins); i++ {
		if origins[i] == origins[i-1] {
			return true
		}
	}
	return false
}
