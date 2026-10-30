// Package localauthority verifies the experimental protected execution profile.
// A signature is not root admission. PolicySnapshot must come from a separately
// protected, current operator policy; no command accepts it from claimant JSON.
package localauthority

import "crypto/ed25519"

const Profile = "corvint-protected-execution/0"
const CheckProfile = "tcq/1-experimental"
const SelectionMode = "ALL_SELECTED"
const MaxReceiptBytes = 128 << 10

type Check struct {
	ClaimSelector string `json:"claimSelector"`
	ID            string `json:"id"`
	Profile       string `json:"profile"`
	DriverSHA256  string `json:"driverSHA256"`
	DriverPath    string `json:"driverPath"`
	DriverUnit    string `json:"driverUnit"`
	Invocation    string `json:"invocation"`
	Subject       string `json:"subject"`
}

type Binding struct {
	Base            string `json:"base"`
	Target          string `json:"target"`
	Tree            string `json:"tree"`
	CEMSHA256       string `json:"cemSHA256"`
	OCMSHA256       string `json:"ocmSHA256"`
	SelectionSHA256 string `json:"selectionSHA256"`
	SourceSHA256    string `json:"sourceSHA256"`
	RecipeSHA256    string `json:"recipeSHA256"`
	WasmSHA256      string `json:"wasmSHA256"`
	WorkerSHA256    string `json:"workerSHA256"`
}

type Enrollment struct {
	RepositoryID string  `json:"repositoryId"`
	PolicySHA256 string  `json:"policySHA256"`
	Profile      string  `json:"profile"`
	Nonce        string  `json:"nonce"`
	Audience     string  `json:"audience"`
	RootID       string  `json:"rootId"`
	Epoch        string  `json:"epoch"`
	Generation   string  `json:"generation"`
	IssuedAt     string  `json:"issuedAt"`
	ExpiresAt    string  `json:"expiresAt"`
	Selection    string  `json:"selection"`
	Binding      Binding `json:"binding"`
	Checks       []Check `json:"checks"`
}

type Row struct {
	Check  Check  `json:"check"`
	Status string `json:"status"`
}

type Payload struct {
	Enrollment  Enrollment `json:"enrollment"`
	CompletedAt string     `json:"completedAt"`
	Cleanup     string     `json:"cleanup"`
	ExitCode    string     `json:"exitCode"`
	Rows        []Row      `json:"rows"`
}

type Receipt struct {
	Payload   Payload `json:"payload"`
	Signature string  `json:"signature"`
}

// PolicySnapshot is intentionally not a wire document. The consumer obtains it
// from its protected current-policy store on every read, including idempotent
// receipt reads. Fixture policies are never admitted by Verify.
type PolicySnapshot struct {
	RepositoryID      string
	PolicySHA256      string
	Accepted          bool
	Current           bool
	Fixture           bool
	Revoked           bool
	RootID            string
	PublicKey         ed25519.PublicKey
	Epoch             string
	Generation        string
	MinimumGeneration string
	Audience          string
	Checks            []Check
	// Exact terminal digest is read from the protected nonce journal. A nonce
	// with no authenticated complete observation or another terminal digest cannot verify.
	TerminalSHA256 map[string]string
}
