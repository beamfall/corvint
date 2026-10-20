package verify

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// External criterion verification (CEM-V1-333..335). A consumer supplies each
// external repository by name; an unsupplied, unadmitted or unresolvable
// external document is never observed, so the verification abstains with
// artifact-unavailable, and only observed bytes can reject.
const (
	stableExternalUnavailable = "artifact-unavailable"
	stableExternalMismatch    = "artifact-digest-mismatch"
)

// checkStableExternal reads every external criterion span from the supplied
// repository, in map order, under the shared Stable budget. It returns nil
// when every span matches its criterionSha256. Every read charges primary's
// distinct-blob budget as well as the shared Git budget.
func checkStableExternal(ctx context.Context, primary *gitauth.Repository, criteria []wire.StableCriterion, roots map[string]string, budget *gitrun.Budget) error {
	for _, c := range criteria {
		if !c.External() {
			continue
		}
		root, ok := roots[c.ExternalRepository]
		if !ok {
			return cemcode.New(stableExternalUnavailable, "external repository %q was not supplied", c.ExternalRepository)
		}
		data, err := stableExternalBlob(ctx, primary, root, c.ExternalBlobOid, budget)
		if err != nil {
			return err
		}
		if c.ExternalSpan.End > int64(len(data)) {
			return cemcode.New(stableExternalMismatch, "external criterion span exceeds blob %s", c.ExternalBlobOid)
		}
		sum := sha256.Sum256(data[c.ExternalSpan.Start:c.ExternalSpan.End])
		if hex.EncodeToString(sum[:]) != c.CriterionSha256 {
			return cemcode.New(stableExternalMismatch, "external criterion span digest differs from criterionSha256")
		}
	}
	return nil
}

// stableExternalBlob admits root exactly as the verified repository is
// admitted, reads one blob in a checked object session and revalidates the
// admitted boundary. Containment and cancellation keep their own codes; every
// other failure is artifact-unavailable.
func stableExternalBlob(ctx context.Context, primary *gitauth.Repository, root, oid string, budget *gitrun.Budget) ([]byte, error) {
	repo, boundary, err := gitauth.OpenStable(root, budget, nil)
	if err != nil {
		return nil, stableExternalCause(err, budget)
	}
	defer boundary.Close()
	defer repo.ChargeBlobsTo(primary)()
	release := repo.BeginCheckedObjectSession(ctx)
	data, err := repo.BlobBytes(ctx, oid)
	if closeErr := release(); closeErr != nil || budget.Held() {
		return nil, cemcode.New(gitrun.ProcessContainment, "external repository session cleanup was not observed")
	}
	if err != nil {
		return nil, stableExternalCause(err, budget)
	}
	if err = boundary.Validate(); err != nil {
		return nil, cemcode.New(stableExternalUnavailable, "external repository boundary changed")
	}
	return data, nil
}

func stableExternalCause(err error, budget *gitrun.Budget) error {
	code := cemcode.CodeOf(err)
	switch {
	case code == gitrun.ProcessContainment || budget.Held():
		return cemcode.New(gitrun.ProcessContainment, "external repository process containment")
	case code == "git-cancelled" || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded):
		return cemcode.New("verification-timeout", "external repository read cancelled")
	}
	return cemcode.New(stableExternalUnavailable, "external repository object unavailable: %s", code)
}

// StableExternalFix names the recovery for an artifact-unavailable abstention
// whose cause is an unsupplied external repository (CEM-V1-336): every
// external repository name in raw that roots lacks, sorted. It returns ""
// when raw does not parse or every name is supplied. It goes to stderr, never
// into the closed result.
func StableExternalFix(raw []byte, roots map[string]string) string {
	d, err := wire.ParseStable(raw)
	if err != nil {
		return ""
	}
	missing := map[string]bool{}
	for _, c := range d.Criteria {
		if _, ok := roots[c.ExternalRepository]; c.External() && !ok {
			missing[c.ExternalRepository] = true
		}
	}
	if len(missing) == 0 {
		return ""
	}
	names := make([]string, 0, len(missing))
	for name := range missing {
		names = append(names, name)
	}
	sort.Strings(names)
	return "artifact-unavailable: external repository " + strings.Join(names, ", ") + " not supplied; external criteria stay NOT_OBSERVED; fix: pass --external-repository NAME=/absolute/path for each"
}

// ParseExternalRepository parses one --external-repository value, NAME=PATH,
// with a valid external repository name and an absolute clean PATH.
func ParseExternalRepository(value string) (name, root string, ok bool) {
	name, root, ok = strings.Cut(value, "=")
	if !ok || !wire.IsStableExternalRepository(name) || root == "" {
		return "", "", false
	}
	return name, root, true
}
