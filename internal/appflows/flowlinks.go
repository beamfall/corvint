// SPDX-License-Identifier: Apache-2.0

package appflows

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/testid"
)

// FlowTestLink is the FLP-V1-006 record. Variation IDs are unique across an
// IntentSet. Kind belongs to the test inventory, never to this record.
// Stored acceptance is not proof of freshness or reachability agreement.
type FlowTestLink struct {
	TestID            string   `json:"test_id"`
	VariationID       string   `json:"variation_id"`
	Evidence          []string `json:"evidence"`
	Confidence        float64  `json:"confidence"`
	ConfidenceFormula string   `json:"confidenceFormula"`
	AcceptedBy        string   `json:"accepted_by"`
	AcceptedAt        string   `json:"accepted_at"`
	PinnedCommit      string   `json:"pinned_commit"`
	TestContentHash   string   `json:"test_content_hash"`
	RoutesDigest      string   `json:"routes_digest"`
}

// HasAcceptance checks only the stored acceptance fields. A true result must
// never alone authorize narrowing; freshness, inventory and static agreement
// are separate requirements (FLP-V1-007, FLP-V1-009, FLP-V1-010).
func (l FlowTestLink) HasAcceptance() bool {
	when, err := time.Parse(time.RFC3339Nano, l.AcceptedAt)
	_, offset := when.Zone()
	return strings.TrimSpace(l.AcceptedBy) != "" && oidPattern.MatchString(l.PinnedCommit) && err == nil && offset == 0
}

// Version 0's struct and member list stay frozen (FSV-V1-002). The explicit
// version 1 struct also keeps the closed-schema member walk unambiguous.
type flowsDeclarationV1File struct {
	Profile    string                 `json:"profile"`
	Flows      string                 `json:"flows"`
	Kinds      []flowsKindRule        `json:"kinds,omitempty"`
	Playwright *flowsPlaywrightMember `json:"playwright,omitempty"`
	Links      []FlowTestLink         `json:"links,omitempty"`
}

func validateFlowsV1(file *flowsDeclarationV1File) (*FlowsDeclaration, error) {
	out, err := validateFlowsV0(&flowsDeclarationFile{
		Profile: file.Profile, Flows: file.Flows, Kinds: file.Kinds, Playwright: file.Playwright,
	})
	if err != nil {
		return nil, err
	}
	if len(file.Links) > maxFlowLinks {
		return nil, fmt.Errorf("at most %d test links", maxFlowLinks)
	}
	for i, link := range file.Links {
		id, idErr := testid.Parse(link.TestID)
		if idErr != nil || id.String() != link.TestID || !memberPattern.MatchString(link.VariationID) {
			return nil, fmt.Errorf("test link %d must name a test and variation", i)
		}
		if math.IsNaN(link.Confidence) || math.IsInf(link.Confidence, 0) || link.Confidence < 0 || link.Confidence > 1 {
			return nil, fmt.Errorf("test link %d confidence must be in [0, 1]", i)
		}
		if len(link.Evidence) > maxFlowList {
			return nil, fmt.Errorf("test link %d has more than %d evidence items", i, maxFlowList)
		}
		fields := []string{link.TestID, link.ConfidenceFormula, link.AcceptedBy, link.AcceptedAt,
			link.PinnedCommit, link.TestContentHash, link.RoutesDigest}
		for _, field := range append(fields, link.Evidence...) {
			if len(field) > maxFlowText {
				return nil, fmt.Errorf("test link %d text exceeds %d bytes", i, maxFlowText)
			}
		}
	}
	out.Links = file.Links
	return out, nil
}
