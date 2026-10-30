//go:build !darwin

package authoritystore

import (
	"context"
	"os"
)

type evidenceView struct{ root string }

func AuditEvidenceParent(uint32, uint32) error { return errUnavailable }
func openEvidenceView(context.Context, RootDocument, string, publication) (*evidenceView, error) {
	return nil, errUnavailable
}
func (*evidenceView) unchanged(context.Context, EvidenceReference) error { return errUnavailable }
func AuditStagedEvidence(context.Context, string, EvidenceManifest, uint32, uint32) error {
	return errUnavailable
}

func requireEvidenceDescriptorHeadroom() (uint64, uint64, error) { return 0, 0, errUnavailable }

func OpenEvidencePublicationParent(uint32, uint32) (*os.File, error) { return nil, errUnavailable }
func AuditEvidenceStageHandle(context.Context, *os.File, EvidenceManifest, uint32, uint32) error {
	return errUnavailable
}
