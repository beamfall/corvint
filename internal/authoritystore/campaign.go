package authoritystore

import (
	"bytes"
	"context"
	"encoding/hex"
	"path/filepath"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/localauthority"
)

const stopCampaignScope = "ALL_NATIVE_STOPS_IN_EXACT_REPOSITORY"
const lifecycleCampaignScope = "ALL_NATIVE_LIFECYCLE_EVENTS_IN_EXACT_REPOSITORY"

const campaignProfile = "corvint-native-qualification-campaign/0"
const campaignPath = "native-campaign.json"
const maxCampaign = 32 << 10

// A campaign is independently admitted, bounded native exercise permission. It
// is never a HostQualification and cannot be supplied by the event or CLI.
type qualificationCampaign struct {
	DirectRuntime           *DirectRuntime   `json:"-"`
	Profile                 string           `json:"profile"`
	Status                  string           `json:"status"`
	CampaignID              string           `json:"campaignId"`
	IssuedAt                string           `json:"issuedAt"`
	ExpiresAt               string           `json:"expiresAt"`
	RootID                  string           `json:"rootId"`
	Epoch                   string           `json:"epoch"`
	Generation              string           `json:"generation"`
	RepositoryID            string           `json:"repositoryId"`
	RepositoryRoot          string           `json:"repositoryRoot"`
	PolicySHA256            string           `json:"policySHA256"`
	MinimumGenerationSHA256 string           `json:"minimumGenerationSHA256"`
	EnrollmentHandle        string           `json:"enrollmentHandle"`
	Target                  string           `json:"target"`
	Consumer                Image            `json:"consumer"`
	Adapter                 Image            `json:"adapter"`
	Git                     Image            `json:"git"`
	AllowStopRemediation    bool             `json:"allowStopRemediation"`
	Scope                   string           `json:"scope"`
	Runtime                 candidateRuntime `json:"runtime"`
}

type candidateRuntime struct {
	Topology        string          `json:"topology"`
	BootSessionUUID string          `json:"bootSessionUUID"`
	AppInstance     ProcessInstance `json:"appInstance"`
	EngineInstance  ProcessInstance `json:"engineInstance"`
	App             Image           `json:"app"`
	Engine          Image           `json:"engine"`
	AppCDHash       string          `json:"appCDHash"`
	EngineCDHash    string          `json:"engineCDHash"`
	OSBuild         string          `json:"osBuild"`
	Architecture    string          `json:"architecture"`
	ControlSocket   string          `json:"controlSocket,omitempty"`
}

// runtimePins deliberately contains no evidence digest or qualified surfaces.
// EngineInstance is optional only on the existing completed-qualification path.
type runtimePins struct {
	Topology, BootSessionUUID                                     string
	AppInstance, EngineInstance                                   ProcessInstance
	App, Engine                                                   Image
	AppCDHash, EngineCDHash, OSBuild, Architecture, ControlSocket string
}

func (r candidateRuntime) pins() runtimePins {
	return runtimePins{r.Topology, r.BootSessionUUID, r.AppInstance, r.EngineInstance, r.App, r.Engine, r.AppCDHash, r.EngineCDHash, r.OSBuild, r.Architecture, r.ControlSocket}
}
func qualifiedPins(q *HostQualification) runtimePins {
	return runtimePins{q.Topology, q.BootSessionUUID, q.AppInstance, ProcessInstance{}, q.App, q.Engine, q.AppCDHash, q.EngineCDHash, q.OSBuild, q.Architecture, q.ControlSocket}
}
func processPin(p ProcessInstance) bool { return p.PID > 1 && p.Started > 0 && p.StartedUsec < 1000000 }
func objectID(s string) bool {
	if (len(s) != 40 && len(s) != 64) || strings.ToLower(s) != s {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}
func campaignTime(s string) (time.Time, error) {
	value, err := time.Parse(time.RFC3339, s)
	if err != nil || value.UTC().Format(time.RFC3339) != s || value.Nanosecond() != 0 {
		return time.Time{}, errUnavailable
	}
	return value, nil
}
func (c qualificationCampaign) valid(root RootDocument, floorBytes []byte, handle string, now time.Time) bool {
	return c.validScope(root, floorBytes, handle, now, stopCampaignScope)
}

func (c qualificationCampaign) validScope(root RootDocument, floorBytes []byte, handle string, now time.Time, expectedScope string) bool {
	if (expectedScope != stopCampaignScope && expectedScope != lifecycleCampaignScope) || !((root.Profile == RootProfile && c.Profile == campaignProfile && c.DirectRuntime == nil) || (root.Profile == DirectRootProfile && c.Profile == directCampaignProfile && c.DirectRuntime != nil && expectedScope == lifecycleCampaignScope)) || c.Status != "NATIVE_QUALIFICATION_ONLY" || c.Scope != expectedScope || !hexDigest(c.CampaignID) {
		return false
	}
	issued, err := campaignTime(c.IssuedAt)
	if err != nil {
		return false
	}
	expires, err := campaignTime(c.ExpiresAt)
	if err != nil || !issued.Before(expires) || expires.Sub(issued) > 900*time.Second || now.Before(issued) || !now.Before(expires) {
		return false
	}
	if c.RootID != root.RootID || c.Epoch != root.Epoch || c.Generation != root.Generation || c.RepositoryID != root.RepositoryID || c.RepositoryRoot != root.RepositoryRoot || c.PolicySHA256 != root.PolicySHA256 || c.MinimumGenerationSHA256 != localauthority.BytesDigest(floorBytes) || c.EnrollmentHandle != handle || !hexDigest(handle) || !objectID(c.Target) {
		return false
	}
	if c.Consumer != root.Consumer || c.Adapter != root.Adapter || c.Git != root.Git {
		return false
	}
	if c.Profile == directCampaignProfile {
		return c.DirectRuntime.valid()
	}
	r := c.Runtime
	if !processPin(r.AppInstance) || !processPin(r.EngineInstance) || r.AppInstance.PID == r.EngineInstance.PID || r.BootSessionUUID == "" || r.OSBuild == "" || (r.Architecture != "arm64" && r.Architecture != "amd64") || !hexDigest(r.App.SHA256) || !hexDigest(r.Engine.SHA256) || len(r.AppCDHash) != 40 || !objectID(r.AppCDHash) || len(r.EngineCDHash) != 40 || !objectID(r.EngineCDHash) {
		return false
	}
	if !filepath.IsAbs(r.App.Path) || filepath.Clean(r.App.Path) != r.App.Path || !filepath.IsAbs(r.Engine.Path) || filepath.Clean(r.Engine.Path) != r.Engine.Path {
		return false
	}
	if r.Topology == "app-owned-stdio" {
		return r.ControlSocket == ""
	}
	return r.Topology == "shared-daemon" && validControlSocket(r.ControlSocket)
}

type runtimeAdmission struct {
	campaign *qualificationCampaign
	raw      []byte
	scope    string
}

func admitRuntime(ctx context.Context, root RootDocument, files protectedFiles, floorBytes []byte, handle string, now time.Time) (runtimeAdmission, error) {
	return admitRuntimeScope(ctx, root, files, floorBytes, handle, now, stopCampaignScope)
}

func admitLifecycleRuntime(ctx context.Context, root RootDocument, files protectedFiles, floorBytes []byte, handle string, now time.Time) (runtimeAdmission, error) {
	return admitRuntimeScope(ctx, root, files, floorBytes, handle, now, lifecycleCampaignScope)
}

func admitRuntimeScope(ctx context.Context, root RootDocument, files protectedFiles, floorBytes []byte, handle string, now time.Time, expectedScope string) (runtimeAdmission, error) {
	if root.hasQualification() {
		// A present completed document always selects the existing normal path; an
		// invalid one cannot fall back to a candidate admission.
		return runtimeAdmission{}, verifyRuntime(ctx, root)
	}
	raw, err := files.read(campaignPath, 0, maxCampaign)
	var candidate qualificationCampaign
	if err != nil || len(raw) > maxCampaign || localauthority.Decode(raw, &candidate) != nil || !candidate.validScope(root, floorBytes, handle, now, expectedScope) {
		return runtimeAdmission{}, errUnavailable
	}
	if err = verifyCampaignRuntime(ctx, root, candidate); err != nil {
		return runtimeAdmission{}, errUnavailable
	}
	return runtimeAdmission{campaign: &candidate, raw: raw, scope: expectedScope}, nil
}
func (a runtimeAdmission) targetMatches(target string) bool {
	return a.campaign == nil || a.campaign.Target == target
}
func (a runtimeAdmission) recheck(ctx context.Context, root RootDocument, files protectedFiles, floorBytes []byte, handle, target string, now time.Time) error {
	if a.campaign == nil {
		return verifyRuntime(ctx, root)
	}
	if root.hasQualification() || !campaignStillCurrentScope(files, a.raw, root, floorBytes, handle, target, now, a.scope) {
		return errUnavailable
	}
	return verifyCampaignRuntime(ctx, root, *a.campaign)
}

// Same bytes are mandatory at both observations. This helper is kept separate
// from live kernel verification so mutation/expiry tests require no forged host.
func campaignStillCurrent(files protectedFiles, before []byte, root RootDocument, floor []byte, handle, target string, now time.Time) bool {
	return campaignStillCurrentScope(files, before, root, floor, handle, target, now, stopCampaignScope)
}
func campaignStillCurrentScope(files protectedFiles, before []byte, root RootDocument, floor []byte, handle, target string, now time.Time, expectedScope string) bool {
	var c qualificationCampaign
	if localauthority.Decode(before, &c) != nil || !c.validScope(root, floor, handle, now, expectedScope) || c.Target != target {
		return false
	}
	after, err := files.read(campaignPath, 0, maxCampaign)
	return err == nil && bytes.Equal(before, after)
}

func validControlSocket(path string) bool {
	return filepath.IsAbs(path) && filepath.Clean(path) == path && len(path) <= 103
}

func (r runtimePins) acceptsEngine(p ProcessInstance) bool {
	return r.EngineInstance == (ProcessInstance{}) || r.EngineInstance == p
}

func verifyCampaignRuntime(ctx context.Context, root RootDocument, candidate qualificationCampaign) error {
	if root.Profile == DirectRootProfile && candidate.Profile == directCampaignProfile && candidate.DirectRuntime != nil {
		return verifyDirectRuntime(ctx, root, *candidate.DirectRuntime)
	}
	if root.Profile != RootProfile || candidate.Profile != campaignProfile || candidate.DirectRuntime != nil {
		return errUnavailable
	}
	return verifyRuntimePins(ctx, root, candidate.Runtime.pins())
}
