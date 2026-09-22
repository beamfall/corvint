// Package authoritystore is the fixed read-only protected publication consumer.
// Production never accepts a caller-selected root, directory, key, or host tuple.
package authoritystore

import "github.com/Beamfall/corvint/internal/localauthority"

const RootPath = "/Library/CorvintAuthority"
const RootProfile = "corvint-protected-root/1"
const TargetProfile = "corvint-protected-target/0"
const TerminalProfile = "corvint-protected-terminal/0"
const FloorProfile = "corvint-protected-generation-floor/0"

type Image struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// HostQualification is accepted independently only after every AHI requirement
// and native case passes for these exact images and OS. Runtime checks also
// require current Engine ancestry and live kernel code-directory hashes, plus
// App ancestry or a reciprocal shared-daemon socket relationship. AppInstance
// and BootSessionUUID bind the fresh resource-audited launch, not just its PID. Ordinary app ownership is not an
// execution authority boundary; these pins identify a qualified display host.
// The digest commits the separately reviewed qualification evidence; its mere
// presence in a candidate document never admits that document.
type ProcessInstance struct {
	PID         uint32 `json:"pid,string"`
	Started     uint64 `json:"started,string"`
	StartedUsec uint64 `json:"startedUsec,string"`
}
type SurfaceQualification struct {
	Surface        string `json:"surface"`
	EvidenceSHA256 string `json:"evidenceSHA256"`
}
type HostQualification struct {
	Topology          string                 `json:"topology"`
	BootSessionUUID   string                 `json:"bootSessionUUID"`
	AppInstance       ProcessInstance        `json:"appInstance"`
	ControlSocket     string                 `json:"controlSocket,omitempty"`
	QualifiedSurfaces []SurfaceQualification `json:"qualifiedSurfaces,omitempty"`
	Profile           string                 `json:"profile"`
	Surface           string                 `json:"surface"`
	EvidenceSHA256    string                 `json:"evidenceSHA256"`
	App               Image                  `json:"app"`
	Engine            Image                  `json:"engine"`
	AppCDHash         string                 `json:"appCDHash"`
	EngineCDHash      string                 `json:"engineCDHash"`
	OSBuild           string                 `json:"osBuild"`
	Architecture      string                 `json:"architecture"`
}

// RootDocument is read exclusively from root:wheel accepted-root.json, with
// safe ancestors and no ACL entries. Root admission is an operator action;
// installer/signer cannot create an OPERATOR_ACCEPTED document for themselves.
// authorityUID names the dedicated noninteractive publication owner.
type RootDocument struct {
	DirectQualification *DirectQualification   `json:"-"`
	Profile             string                 `json:"profile"`
	Admission           string                 `json:"admission"`
	KeyClass            string                 `json:"keyClass"`
	RootID              string                 `json:"rootId"`
	PublicKey           string                 `json:"publicKey"`
	Epoch               string                 `json:"epoch"`
	Generation          string                 `json:"generation"`
	RepositoryID        string                 `json:"repositoryId"`
	RepositoryRoot      string                 `json:"repositoryRoot"`
	PolicySHA256        string                 `json:"policySHA256"`
	Audience            string                 `json:"audience"`
	AuthorityUID        string                 `json:"authorityUid"`
	AuthorityGID        string                 `json:"authorityGid"`
	ReaderUID           string                 `json:"readerUid"`
	Revoked             bool                   `json:"revoked"`
	Checks              []localauthority.Check `json:"checks"`
	Consumer            Image                  `json:"consumer"`
	Git                 Image                  `json:"git"`
	Adapter             Image                  `json:"adapter"`
	HostQualification   *HostQualification     `json:"hostQualification"`
	RemediationAllowed  bool                   `json:"remediationAllowed"`
}

// The floor is a separately protected operator-owned file, never overwritten
// by the consumer or signer. Restoring an older accepted-root cannot lower it.
type GenerationFloor struct {
	Profile    string `json:"profile"`
	RootID     string `json:"rootId"`
	Epoch      string `json:"epoch"`
	Generation string `json:"generation"`
}

type Target struct {
	Profile        string `json:"profile"`
	RepositoryID   string `json:"repositoryId"`
	RepositoryRoot string `json:"repositoryRoot"`
	Base           string `json:"base"`
	Target         string `json:"target"`
	Tree           string `json:"tree"`
}

type Terminal struct {
	Profile       string `json:"profile"`
	Nonce         string `json:"nonce"`
	State         string `json:"state"`
	ReceiptSHA256 string `json:"receiptSHA256"`
}

// ActiveEnrollment is atomically published after the complete immutable bundle.
// It keeps the qualified adapter bytes stable across enrollment changes.
type ActiveEnrollment struct {
	Profile          string `json:"profile"`
	EnrollmentHandle string `json:"enrollmentHandle"`
}
