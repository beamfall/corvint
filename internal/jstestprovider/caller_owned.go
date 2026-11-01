package jstestprovider

import (
	"context"
	"net/netip"
	"net/url"
	"os"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
)

// admitCallerOwned refuses a caller-owned server run combined with a server the provider
// would start, the external PWP mode or freshness, or without a ready URL (NEA-V0-012).
func admitCallerOwned(cfg E2EConfig) error {
	if cfg.ExternalServer || cfg.Freshness != nil || len(cfg.ServerArgv) != 0 || cfg.ServerReadyURL == "" || cfg.ApplicationAttestation != nil || cfg.SensitiveInputPolicy != nil || cfg.KeepReporters || cfg.RetainAttemptDetails {
		return refuseE2E("caller-owned-server-composition-unsupported")
	}
	if !callerOwnedLoopback(cfg.ServerReadyURL) {
		return refuseE2E("caller-owned-server-not-loopback")
	}
	return nil
}

// callerOwnedLoopback admits only an http URL whose host is a loopback IP literal.
func callerOwnedLoopback(address string) bool {
	u, err := url.Parse(address)
	if err != nil || u.Scheme != "http" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	ip, err := netip.ParseAddr(u.Hostname())
	return err == nil && ip.Zone() == "" && ip.Unmap().IsLoopback()
}

// runCallerOwned runs the Playwright test command against a server the caller started and
// owns (NEA-V0-012). The provider never starts, signals or stops that server: it waits for
// ServerReadyURL before the runner and requires one 2xx answer after the runner exits, so an
// app that stopped answering is the infrastructure cause server-unavailable-at-publish. Neither
// poll follows a redirect, so a loopback app cannot point the provider at another host.
// ServerDescendantsGone stays nil because the provider owns no server process.
func runCallerOwned(ctx context.Context, cfg E2EConfig) (Receipt, error) {
	identity, err := cfg.identity(ctx, cfg.TestArgv)
	if err != nil {
		return Receipt{}, err
	}
	receipt := Receipt{Kind: "e2e", Identity: identity}
	atStart, err := DigestAppBuildDir(cfg.AppBuildDir)
	if err != nil {
		return Receipt{}, err
	}
	receipt.AppBuildAtStart = atStart
	readyLimit := cfg.ServerReadyLimit
	if readyLimit <= 0 {
		readyLimit = 15 * time.Second
	}
	if err := externalReady(ctx, cfg.ServerReadyURL, readyLimit); err != nil {
		receipt.AppBuildAtPublish = AppBuildIdentity{Unknown: true, Reason: "no test run reached publish"}
		receipt.Infrastructure = &InfrastructureFailure{Reason: "server-not-ready", Detail: err.Error()}
		if ctx.Err() != nil {
			receipt.Cancelled = true
			receipt.Infrastructure = &InfrastructureFailure{Reason: "cancelled", Detail: "run was cancelled before completion"}
		}
		return receipt, nil
	}
	testTimeout := cfg.Timeout
	if testTimeout <= 0 {
		testTimeout = defaultTimeout
	}
	testObs := procgroup.Run(ctx, procgroup.Spec{
		Argv:        resolveArgv(cfg.TestArgv),
		Dir:         cfg.Dir,
		Env:         os.Environ(),
		Timeout:     testTimeout,
		OutputLimit: outputLimitOrDefault(cfg.OutputLimit),
	})
	atPublish, err := DigestAppBuildDir(cfg.AppBuildDir)
	if err != nil {
		return Receipt{}, err
	}
	receipt.AppBuildAtPublish = atPublish
	receipt.StaleAppBuild = isStaleAppBuild(atStart, atPublish)
	if testObs.Cancelled || ctx.Err() != nil {
		receipt.Cancelled = true
		receipt.Infrastructure = &InfrastructureFailure{Reason: "cancelled", Detail: "run was cancelled before completion"}
		return receipt, nil
	}
	if boundary := e2eBoundaryFailure(testObs, testTimeout); boundary != nil {
		receipt.Infrastructure = boundary
		return receipt, nil
	}
	if externalReady(ctx, cfg.ServerReadyURL, time.Second) != nil {
		receipt.Infrastructure = &InfrastructureFailure{Reason: "server-unavailable-at-publish", Detail: "caller-owned-server-not-answering"}
		return receipt, nil
	}
	tests, infra, err := ParsePlaywrightJSON(testObs.Stdout)
	if err != nil {
		receipt.Infrastructure = &InfrastructureFailure{Reason: "report-unparseable", Detail: err.Error()}
		return receipt, nil
	}
	if infra != nil {
		receipt.Infrastructure = infra
		return receipt, nil
	}
	receipt.Tests = tests
	return receipt, nil
}
