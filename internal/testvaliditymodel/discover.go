package testvaliditymodel

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// EmptyPackageDigest is what the JavaScript provider binds when neither a
// package.json nor a lockfile was named: sha256("=\n=\n").
var EmptyPackageDigest = DigestHex([]byte("=\n=\n"))

// ReceiptAge provides a stable age origin rather than a clock-dependent duration.
// The local retained file's mtime is not an authenticated execution timestamp.
type ReceiptAge struct {
	Basis string `json:"basis"`
	Since string `json:"since,omitempty"`
}

// Discovery records which retained document was selected and whether its
// bound identity still matches the worktree. It is emitted only by discovery.
type Discovery struct {
	ReceiptAge ReceiptAge         `json:"receiptAge"`
	Location   string             `json:"location"`
	Evidence   string             `json:"evidence,omitempty"`
	Freshness  *testvalidity.Axis `json:"freshness,omitempty"`
	Skipped    int                `json:"skipped"`
	Abstention *Abstention        `json:"abstention,omitempty"`
}

// Abstention explains why discovery yields no evidence that could project a
// passing test (LPCV-V0-056). Reason and Requires are closed; an unattributable
// cause is UNKNOWN, never a guess. Rediscovery alone cannot change any reason:
// Requires names the change that can.
type Abstention struct {
	Reason               string           `json:"reason"`
	Requires             string           `json:"requires"`
	Observed             *ObservedRuntime `json:"observed,omitempty"`
	Producers            []ProducerHint   `json:"producers,omitempty"`
	ExpectedEvidencePath string           `json:"expectedEvidencePath,omitempty"`
	ReceiptRevision      string           `json:"receiptRevision,omitempty"`
}

// ProducerHint is an inert template, not an instruction to execute a provider.
// Qualification is unknown on this host; retained runtime tuples cannot prove it.
type ProducerHint struct {
	Command       string `json:"command"`
	Qualification string `json:"qualification"`
}

// ObservedRuntime is the retained receipt's own runtime tuple. Platform and
// architecture are not retained by an unqualified receipt, so they are absent.
type ObservedRuntime struct {
	RunnerVersion string `json:"runnerVersion"`
	NodeVersion   string `json:"nodeVersion"`
}

func DigestHex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
