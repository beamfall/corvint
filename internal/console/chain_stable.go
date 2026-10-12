package console

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// The chain pane reads a sealed cem/1.0 map as display only (CEM-V1-077 to
// CEM-V1-079). It admits the exact stable profile only when the strict stable
// wire reader accepts the whole document, runs no stable verifier, opens no
// artifact named by the map, and executes nothing the map carries. Every
// reference row states that structural validity is not task completion, and
// every axis the pane cannot observe is drawn as a gap.

// stableUnverifiedReason is the envelope the pane shows for every stable map:
// the stable verifier owns acceptance and this pane never runs it.
const stableUnverifiedReason = "the console runs no stable verifier, so stable verification is NOT_RUN here; " +
	"`corvint cem verify-stable` with the expected base, target and artifact root owns acceptance"

const stableReferenceDetail = "structurally valid reference; its digest is not checked by this pane; " +
	"a valid reference is not task completion and not criterion satisfaction"

// stableAxes are the verifier axes the pane names. The integrity axes are
// NOT_CHECKED because the pane does not verify; the others are NOT_OBSERVED
// even under an accepted verification.
var stableAxes = []struct{ name, state string }{
	{"changeIntegrity", "NOT_CHECKED"},
	{"referenceIntegrity", "NOT_CHECKED"},
	{"nativeAuthority", "NOT_OBSERVED"},
	{"criterionAdequacy", "NOT_OBSERVED"},
	{"criterionDiscrimination", "NOT_OBSERVED"},
	{"runnerExecution", "NOT_OBSERVED"},
	{"authentication", "NOT_OBSERVED"},
}

// stableAuthenticationReason says why the pane never reports a receipt
// signature (CEM-V1-084): it trusts no key and checks no envelope.
const stableAuthenticationReason = ": the pane trusts no key and checks no receipt-attestation signature; " +
	"only `corvint cem verify-stable --trusted-keys FILE` with a consumer key file can report VERIFIED, " +
	"and a signature never raises executionAtCommit"

// stableReceiptSignatureDetail qualifies a dsse-ed25519 receipt member: the
// named keyid and envelope are references, not a checked signature.
func stableReceiptSignatureDetail(a wire.StableAuthentication) string {
	if !a.DSSE() {
		return ""
	}
	return " (signature not checked and keyid not trusted in this pane)"
}

// mentionsStable reports whether any top-level member whose name matches
// "spec" case-insensitively (the members json.Unmarshal may read as Spec)
// carries the string cem/1.0, or whether the document declares it exactly.
// Such a document is admitted only through admitStable, so a duplicate or
// case-variant member can never route a stable declaration to the 0.x reading.
func mentionsStable(text string) bool {
	if wire.DeclaresStable([]byte(text)) {
		return true
	}
	decoder := json.NewDecoder(strings.NewReader(text))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return false
	}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return false
		}
		name, _ := token.(string)
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return false
		}
		var spec string
		if strings.EqualFold(name, "spec") && json.Unmarshal(value, &spec) == nil && spec == wire.StableSpec {
			return true
		}
	}
	return false
}

// admitStable refuses a sealed map that declares cem/1.0 unless the strict
// stable reader accepts the exact bytes; ParseStable may return a document
// alongside a graph error, which is still a refusal.
func admitStable(text string) (*wire.StableMap, error) {
	raw := []byte(text)
	if !wire.DeclaresStable(raw) {
		return nil, errors.New(GapUnsupported + ": the sealed map does not declare exactly " + wire.StableSpec)
	}
	document, err := wire.ParseStable(raw)
	if err != nil || document == nil {
		message := "the stable wire reader refused it"
		if err != nil {
			message += ": " + err.Error()
		}
		return nil, errors.New(GapUnsupported + ": the sealed map declares " + wire.StableSpec + " but " + message + ", so no chain is drawn from it")
	}
	return document, nil
}

// stableEdges draws the stable envelope, axes, sidecar state and reference
// rows for an admitted stable map. It reads only the sealed bytes.
func stableEdges(chain *Chain) []ChainEdge {
	document, err := admitStable(chain.Sealed.Text)
	if err != nil {
		return []ChainEdge{{Gap: GapUnsupported, Reason: err.Error(), Target: chain.Change, Artifact: chain.SealedPath, Field: "spec"}}
	}
	axes := chain.Sealed.Source.Axes
	edge := func(target, field string) ChainEdge {
		return ChainEdge{Target: target, Artifact: chain.SealedPath, Field: field, Axes: axes}
	}
	envelope := edge(wire.StableSpec+" verification", "spec")
	envelope.Gap, envelope.Reason = GapUnverified, stableUnverifiedReason
	digest := edge("map digest", "sha256 of the sealed bytes")
	digest.Pin, digest.Detail = "sha256:"+sha256Hex(chain.Sealed.Text), "the full sealed map is shown unreduced; no stable map is re-encoded"
	sidecar := edge("target sidecar", boundMapPath)
	sidecar.Gap, sidecar.Reason = GapUnverified, "sidecar state NOT_CHECKED: only the stable verifier compares the target's committed sidecar"
	edges := []ChainEdge{envelope, digest, sidecar}
	for _, axis := range stableAxes {
		row := edge("axis "+axis.name, "axes."+axis.name)
		row.Gap, row.Reason = GapUnverified, axis.name+" "+axis.state+" in this pane"
		if axis.name == "authentication" {
			row.Reason += stableAuthenticationReason
		}
		edges = append(edges, row)
	}
	reference := func(target, field, detail string) {
		row := edge(target, field)
		row.Detail = stableReferenceDetail
		if detail != "" {
			row.Detail = detail + "; " + stableReferenceDetail
		}
		edges = append(edges, row)
	}
	for index, criterion := range document.Criteria {
		detail := "ticket " + criterion.TicketID
		if criterion.Plain() {
			detail = "span " + criterion.EvidenceID
		}
		reference(criterion.ID, "criterionBindings["+strconv.Itoa(index)+"]", detail)
	}
	for index, receipt := range document.Receipts {
		reference("sha256:"+receipt.Sha256, "runnerReceipts["+strconv.Itoa(index)+"]", "executionAuthority "+receipt.ExecutionAuthority+
			", sourceGitBinding "+receipt.SourceGitBinding+", dependencyClosure "+receipt.DependencyClosure+", authentication "+receipt.Authentication.String()+stableReceiptSignatureDetail(receipt.Authentication))
	}
	for index, link := range document.Links {
		reference(link.CriterionID, "criterionLinks["+strconv.Itoa(index)+"]", strconv.Itoa(len(link.HunkIDs))+" hunk(s), "+
			strconv.Itoa(len(link.EvidenceIDs))+" evidence item(s), "+strconv.Itoa(len(link.RunnerReceiptSha256s))+" runner receipt(s)")
	}
	for index, artifact := range document.Artifacts {
		reference(artifact.Path, "artifacts["+strconv.Itoa(index)+"]", artifact.Kind+" artifact sha256:"+artifact.Sha256+" (content not opened)")
	}
	completion := edge("task completion", "criterionLinks")
	completion.Gap, completion.Reason = GapUnverified, "task completion NOT_OBSERVED: a sealed stable map records references, never completion or criterion satisfaction"
	return append(edges, completion)
}
