package main

import (
	"context"
	"errors"

	"github.com/Beamfall/corvint/internal/dashboard/adapters"
	"github.com/Beamfall/corvint/internal/dashboard/authority"
	"github.com/Beamfall/corvint/internal/dashboard/repository"
	"github.com/Beamfall/corvint/internal/dashboard/source"
)

const maxWholeScanAttempts = 2

type bridgeDependencies struct {
	newRepositoryBudget func(context.Context) *repository.Budget
	newSourceBudget     func(uint64) *source.Budget
	newAttempt          func(string, *repository.Budget) (authority.Authority, error)
	scan                func(context.Context, adapters.ScanRequest) ([]byte, error)
}

func compileSnapshot(ctx context.Context, request compileRequest) ([]byte, error) {
	return compileSnapshotWith(ctx, request, bridgeDependencies{
		newRepositoryBudget: repository.NewBudget,
		newSourceBudget:     source.NewBudget,
		newAttempt: func(root string, budget *repository.Budget) (authority.Authority, error) {
			attempt, _, failure := repository.NewAttempt(root, budget)
			if failure != nil {
				return nil, failure
			}
			return attempt, nil
		},
		scan: adapters.Scan,
	})
}

func compileSnapshotWith(ctx context.Context, request compileRequest, dependencies bridgeDependencies) ([]byte, error) {
	if ctx == nil || dependencies.newRepositoryBudget == nil || dependencies.newSourceBudget == nil ||
		dependencies.newAttempt == nil || dependencies.scan == nil {
		return nil, &dashboardError{code: errorInternal}
	}
	repositoryBudget := dependencies.newRepositoryBudget(ctx)
	if repositoryBudget == nil {
		return nil, &dashboardError{code: errorInternal}
	}
	defer repositoryBudget.Close()
	sourceBudget := dependencies.newSourceBudget(source.MaxAggregateBytes)
	if sourceBudget == nil {
		return nil, &dashboardError{code: errorInternal}
	}

	baseRequest := adapterRequest(request, sourceBudget)
	for attemptIndex := 0; attemptIndex < maxWholeScanAttempts; attemptIndex++ {
		if err := ctx.Err(); err != nil {
			return nil, &dashboardError{code: errorInterrupted}
		}
		repositoryAuthority, err := dependencies.newAttempt(request.Root, repositoryBudget)
		if err != nil {
			if ctx.Err() != nil {
				return nil, &dashboardError{code: errorInterrupted}
			}
			if repositoryAttemptChanged(err) {
				if attemptIndex+1 < maxWholeScanAttempts {
					continue
				}
				return nil, &dashboardError{code: errorRepository}
			}
			return nil, bridgeError(ctx, err)
		}
		if repositoryAuthority == nil {
			return nil, &dashboardError{code: errorInternal}
		}
		attemptRequest := baseRequest
		attemptRequest.Authority = repositoryAuthority
		attemptRequest.SourceBudget = sourceBudget.ForWholeScanAttempt()
		if attemptRequest.SourceBudget == nil {
			return nil, &dashboardError{code: errorInternal}
		}
		encoded, scanErr, finish := scanAndFinish(ctx, repositoryAuthority, dependencies.scan, attemptRequest)
		if ctx.Err() != nil {
			return nil, &dashboardError{code: errorInterrupted}
		}
		switch finish.Code {
		case authority.FinishStable:
			if scanErr != nil {
				return nil, bridgeError(ctx, scanErr)
			}
			return encoded, nil
		case authority.FinishChanged:
			if attemptIndex+1 < maxWholeScanAttempts {
				continue
			}
			return nil, &dashboardError{code: errorRepository}
		case authority.FinishUnavailable:
			return nil, &dashboardError{code: errorRepository}
		default:
			return nil, &dashboardError{code: errorInternal}
		}
	}
	return nil, &dashboardError{code: errorInternal}
}

func repositoryAttemptChanged(err error) bool {
	var failure *repository.Failure
	return errors.As(err, &failure) && failure.Code == repository.FailureRepositoryChanged
}

func adapterRequest(request compileRequest, budget *source.Budget) adapters.ScanRequest {
	result := adapters.ScanRequest{Root: request.Root, ClockSource: "PROCESS", SourceBudget: budget}
	if request.GeneratedAt != "" {
		generatedAt := request.GeneratedAt
		result.GeneratedAt = &generatedAt
		result.ClockSource = "CALLER"
	}
	result.Sources = make([]adapters.ConfiguredSource, len(request.Sources))
	for index, configured := range request.Sources {
		result.Sources[index] = adapters.ConfiguredSource{
			AdapterID: adapters.AdapterID(configured.AdapterID), RelativePath: configured.RelativePath,
		}
	}
	if request.Bundle != nil {
		result.CEMOCM = &adapters.CEMOCMBinding{
			CEMPath: request.Bundle.CEM, OCMPath: request.Bundle.OCM, Profile: request.Bundle.Profile,
			ExpectedBase: request.Bundle.ExpectedBase, Target: request.Bundle.Target,
		}
	}
	return result
}

func scanAndFinish(ctx context.Context, repositoryAuthority authority.Authority, scan func(context.Context, adapters.ScanRequest) ([]byte, error), request adapters.ScanRequest) (encoded []byte, scanErr error, finish authority.FinishResult) {
	defer func() {
		finish = repositoryAuthority.Finish(ctx)
	}()
	encoded, scanErr = scan(ctx, request)
	return encoded, scanErr, finish
}

func bridgeError(ctx context.Context, err error) error {
	if ctx != nil && ctx.Err() != nil || errors.Is(err, context.Canceled) {
		return &dashboardError{code: errorInterrupted}
	}
	var repositoryFailure *repository.Failure
	if errors.As(err, &repositoryFailure) {
		return &dashboardError{code: errorRepository}
	}
	var scanFailure *adapters.ScanError
	if errors.As(err, &scanFailure) {
		switch scanFailure.Code {
		case errorInvalidArgument, errorRepository, errorResource, errorInterrupted, errorInternal:
			return &dashboardError{code: scanFailure.Code}
		}
	}
	return &dashboardError{code: errorInternal}
}
