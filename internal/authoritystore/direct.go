package authoritystore

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"regexp"

	"github.com/Beamfall/corvint/internal/localauthority"
)

const DirectRootProfile = "corvint-protected-root/2"
const directCampaignProfile = "corvint-native-qualification-campaign/1"
const directQualificationProfile = "corvint-native-qualified-direct-host/0"

// DirectRuntime pins a process lifetime and current mapped native image. It is
// not an exec epoch, an authenticated event, or proof of a completed code audit.
type DirectRuntime struct {
	Topology                       string          `json:"topology"`
	Host                           string          `json:"host"`
	Surface                        string          `json:"surface"`
	BootSessionUUID                string          `json:"bootSessionUUID"`
	HostInstance                   ProcessInstance `json:"hostInstance"`
	HostImage                      Image           `json:"hostImage"`
	HostCDHash                     string          `json:"hostCDHash"`
	RuntimeAdmissionEvidenceSHA256 string          `json:"runtimeAdmissionEvidenceSHA256"`
	ParentPolicy                   string          `json:"parentPolicy"`
	OSBuild                        string          `json:"osBuild"`
	Architecture                   string          `json:"architecture"`
}
type DirectQualification struct {
	Profile        string        `json:"profile"`
	EvidenceSHA256 string        `json:"evidenceSHA256"`
	Runtime        DirectRuntime `json:"runtime"`
}

var bootUUID = regexp.MustCompile(`^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$`)

func (r DirectRuntime) valid() bool {
	return r.Topology == "direct-native-cli" && r.Host == "codex" && r.Surface == "codex-cli" && validNativePin(r)
}
func validNativePin(r DirectRuntime) bool {
	return r.ParentPolicy == "immediate-host" && r.Architecture == "arm64" && r.OSBuild != "" && bootUUID.MatchString(r.BootSessionUUID) && processPin(r.HostInstance) && filepath.IsAbs(r.HostImage.Path) && filepath.Clean(r.HostImage.Path) == r.HostImage.Path && hexDigest(r.HostImage.SHA256) && len(r.HostCDHash) == 40 && objectID(r.HostCDHash) && hexDigest(r.RuntimeAdmissionEvidenceSHA256)
}
func (q *DirectQualification) valid() bool {
	return q != nil && q.Profile == directQualificationProfile && hexDigest(q.EvidenceSHA256) && q.Runtime.valid()
}

// Wire variants shadow exactly one member, preserving every old member and its
// canonical encoding. Decode's round trip enforces completeness and duplicates.
type rootWire RootDocument

func (r RootDocument) MarshalJSON() ([]byte, error) {
	var q any = r.HostQualification
	if r.Profile == DirectRootProfile {
		q = r.DirectQualification
	} else if r.Profile == PiRootProfile {
		q = r.PiQualification
	}
	return json.Marshal(struct {
		*rootWire
		Qualification any `json:"hostQualification"`
	}{(*rootWire)(&r), q})
}
func (r *RootDocument) UnmarshalJSON(raw []byte) error {
	var value RootDocument
	wire := struct {
		*rootWire
		Qualification json.RawMessage `json:"hostQualification"`
	}{rootWire: (*rootWire)(&value)}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&wire) != nil || len(wire.Qualification) == 0 {
		return errUnavailable
	}
	if !bytes.Equal(wire.Qualification, []byte("null")) {
		switch value.Profile {
		case RootProfile:
			var q HostQualification
			if localauthority.Decode(wire.Qualification, &q) != nil {
				return errUnavailable
			}
			value.HostQualification = &q
		case DirectRootProfile:
			var q DirectQualification
			if localauthority.Decode(wire.Qualification, &q) != nil {
				return errUnavailable
			}
			value.DirectQualification = &q
		case PiRootProfile:
			var q PiQualification
			if localauthority.Decode(wire.Qualification, &q) != nil || !q.valid() {
				return errUnavailable
			}
			value.PiQualification = &q
		default:
			return errUnavailable
		}
	}
	*r = value
	return nil
}

type campaignWire qualificationCampaign

func (c qualificationCampaign) MarshalJSON() ([]byte, error) {
	var runtime any = c.Runtime
	if c.Profile == directCampaignProfile {
		runtime = c.DirectRuntime
	} else if c.Profile == piCampaignProfile {
		runtime = c.PiRuntime
	}
	return json.Marshal(struct {
		*campaignWire
		Runtime any `json:"runtime"`
	}{(*campaignWire)(&c), runtime})
}
func (c *qualificationCampaign) UnmarshalJSON(raw []byte) error {
	var value qualificationCampaign
	wire := struct {
		*campaignWire
		Runtime json.RawMessage `json:"runtime"`
	}{campaignWire: (*campaignWire)(&value)}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&wire) != nil {
		return errUnavailable
	}
	switch value.Profile {
	case campaignProfile:
		if localauthority.Decode(wire.Runtime, &value.Runtime) != nil {
			return errUnavailable
		}
	case directCampaignProfile:
		var r DirectRuntime
		if localauthority.Decode(wire.Runtime, &r) != nil {
			return errUnavailable
		}
		value.DirectRuntime = &r
	case piCampaignProfile:
		var r PiRuntime
		if localauthority.Decode(wire.Runtime, &r) != nil || !r.valid() {
			return errUnavailable
		}
		value.PiRuntime = &r
	default:
		return errUnavailable
	}
	*c = value
	return nil
}
func (r RootDocument) hasQualification() bool {
	return r.HostQualification != nil || r.DirectQualification != nil || r.PiQualification != nil
}
func (r RootDocument) qualification() any {
	if r.Profile == PiRootProfile {
		return r.PiQualification
	}
	if r.Profile == DirectRootProfile {
		return r.DirectQualification
	}
	return r.HostQualification
}
