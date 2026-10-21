package gitauth

import "testing"

// TestCEMV1335ChargeBlobsToSharesTheBudget proves an external repository read
// charges the primary repository's distinct-blob budget: it starts from the
// primary total, shares the charged OID set, and carries its total back.
func TestCEMV1335ChargeBlobsToSharesTheBudget(t *testing.T) {
	t.Parallel()
	primary := &Repository{blobBytes: MaxTotalBlobBytes - 1}
	external := &Repository{}
	release := external.ChargeBlobsTo(primary)
	if external.blobBytes != MaxTotalBlobBytes-1 {
		t.Fatalf("external starts at %d, want the primary total", external.blobBytes)
	}
	external.chargedOids["a"] = true
	external.blobBytes += 2
	release()
	if primary.blobBytes != MaxTotalBlobBytes+1 || !primary.chargedOids["a"] {
		t.Fatalf("primary = %d %v, want the external charge carried back", primary.blobBytes, primary.chargedOids)
	}
}
