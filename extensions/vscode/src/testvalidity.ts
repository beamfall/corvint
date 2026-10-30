/**
 * The TypeScript mirror of internal/testvalidity (Go), the shared native
 * test-validity projection for roadmap IPR-06. Both projectors must agree on
 * the vectors in conformance/test-validity-v0/vectors.json.
 *
 * Five axes stay independent: association (which source the test covers),
 * hygiene (empty/always-skipped/wrong target), freshness (stale execution vs
 * current source), execution (pass/fail/skipped/infrastructure/cancelled),
 * and measured strength (mutation witness: a killed mutation establishes
 * only the witnessed distinction, never general adequacy). No axis rewrites
 * another. Passing never implies adequacy, and an unsupported case never
 * collapses into a universal "valid" boolean: project() always returns every
 * axis, stated UNSUPPORTED with a reason when its producer supplied nothing,
 * rather than a derived pass/fail summary field.
 *
 * This module is pure and holds no VS Code, filesystem, or process
 * dependency. It does not read the extension's disabled observation-import
 * path (see observations.ts); that path stays disabled here as everywhere
 * else.
 */

export const STATE_UNSUPPORTED = "UNSUPPORTED";

export interface Axis {
  readonly state: string;
  readonly reason: string;
  readonly anchors: readonly string[] | null;
}

export interface Projection {
  readonly association: Axis;
  readonly hygiene: Axis;
  readonly freshness: Axis;
  readonly execution: Axis;
  readonly strength: Axis;
}

/** The subset of a TCQ claim result this projection draws from. */
export interface ClaimFacts {
  readonly associationState: string;
  readonly hygieneState: string;
  readonly reportState: string;
  readonly reasons: readonly string[];
  readonly anchors: readonly string[];
}

/** The caller-normalized shape of one LPCV/GLTP execution result; outcome is PASSED, FAILED, SKIPPED, INCOMPLETE, or "". */
export interface ExecutionFacts {
  readonly outcome: string;
  readonly cause: string;
  readonly currency: string;
  readonly anchors: readonly string[];
}

export interface MutationWitness {
  readonly operator: string;
  readonly killingTest: string;
}

/** The strength-axis input adapted from one mutation report. */
export interface MutationFacts {
  readonly verdict: string;
  readonly detail: string;
  readonly witness: MutationWitness | null;
}

export interface Input {
  readonly claim: ClaimFacts | null;
  readonly execution: ExecutionFacts | null;
  readonly mutation: MutationFacts | null;
}

const REPORT_PASSED = "PASSED";
const REPORT_FAILED = "FAILED";
const REPORT_SKIPPED = "SKIPPED";
const REPORT_ERROR = "ERROR";
const REPORT_NOT_MATCHED = "NOT_MATCHED";
const REPORT_AMBIGUOUS = "AMBIGUOUS";

/**
 * project() composes one Projection from whatever facts Input carries. It
 * never throws and never collapses to a boolean: a completely empty Input
 * yields every axis UNSUPPORTED with reason "no-input-supplied", the
 * unsupported-input case a caller must not read as a universal "valid".
 */
export function project(input: Input): Projection {
  if (input.claim === null && input.execution === null && input.mutation === null) {
    const unsupported: Axis = { state: STATE_UNSUPPORTED, reason: "no-input-supplied", anchors: null };
    return {
      association: unsupported,
      hygiene: unsupported,
      freshness: unsupported,
      execution: unsupported,
      strength: unsupported,
    };
  }
  return {
    association: associationAxis(input.claim),
    hygiene: hygieneAxis(input.claim),
    freshness: freshnessAxis(input.execution),
    execution: executionAxis(input.claim, input.execution),
    strength: strengthAxis(input.mutation),
  };
}

function pickReason(reasons: readonly string[], candidates: readonly string[]): string {
  const present = new Set(reasons);
  for (const candidate of candidates) {
    if (present.has(candidate)) {
      return candidate;
    }
  }
  return "";
}

function associationAxis(claim: ClaimFacts | null): Axis {
  if (claim === null) {
    return { state: STATE_UNSUPPORTED, reason: "no-association-input", anchors: null };
  }
  const reason = pickReason(claim.reasons, ["claim-association-missing", "claim-association-ambiguous", "unsupported-anchor-profile"]);
  return { state: claim.associationState, reason, anchors: claim.anchors };
}

function hygieneAxis(claim: ClaimFacts | null): Axis {
  if (claim === null) {
    return { state: STATE_UNSUPPORTED, reason: "no-association-input", anchors: null };
  }
  const reason = pickReason(claim.reasons, ["empty-body", "unconditional-skip", "wrong-target", "unsupported-anchor-profile"]);
  return { state: claim.hygieneState, reason, anchors: claim.anchors };
}

function freshnessAxis(execution: ExecutionFacts | null): Axis {
  if (execution === null || execution.currency === "") {
    return { state: "UNKNOWN", reason: "no-execution-report", anchors: null };
  }
  const reason = execution.currency === "STALE" ? "workspace-execution-identity-mismatch" : "";
  return { state: execution.currency, reason, anchors: execution.anchors };
}

function executionAxis(claim: ClaimFacts | null, execution: ExecutionFacts | null): Axis {
  if (execution !== null && execution.outcome !== "") {
    return executionFromReport(execution);
  }
  if (claim !== null && claim.reportState !== "") {
    return executionFromClaim(claim);
  }
  return { state: STATE_UNSUPPORTED, reason: "no-execution-input", anchors: null };
}

function executionFromReport(execution: ExecutionFacts): Axis {
  switch (execution.outcome) {
    case "PASSED":
      return { state: "PASSED", reason: "", anchors: execution.anchors };
    case "FAILED":
      return { state: "FAILED", reason: execution.cause, anchors: execution.anchors };
    case "SKIPPED":
      // A skip carries no cause; one that claims a cause abstains (LPCV-V0-052).
      if (execution.cause !== "") {
        return { state: STATE_UNSUPPORTED, reason: "unsupported-execution-outcome", anchors: execution.anchors };
      }
      return { state: "SKIPPED", reason: "test-skipped", anchors: execution.anchors };
    case "INCOMPLETE":
      if (execution.cause === "CANCELLATION") {
        return { state: "CANCELLED", reason: execution.cause, anchors: execution.anchors };
      }
      return { state: "INFRASTRUCTURE", reason: execution.cause, anchors: execution.anchors };
    default:
      return { state: STATE_UNSUPPORTED, reason: "unsupported-execution-outcome", anchors: execution.anchors };
  }
}

function executionFromClaim(claim: ClaimFacts): Axis {
  const reason = pickReason(claim.reasons, [
    "test-not-matched",
    "test-skipped",
    "test-failed",
    "test-error",
    "repeated-test-rows",
    "execution-identity-ambiguous",
  ]);
  switch (claim.reportState) {
    case REPORT_PASSED:
      return { state: "PASSED", reason, anchors: claim.anchors };
    case REPORT_FAILED:
      return { state: "FAILED", reason, anchors: claim.anchors };
    case REPORT_SKIPPED:
      return { state: "SKIPPED", reason, anchors: claim.anchors };
    case REPORT_ERROR:
      return { state: "ERROR", reason, anchors: claim.anchors };
    case REPORT_NOT_MATCHED:
    case REPORT_AMBIGUOUS:
      return { state: "NOT_MATCHED", reason, anchors: claim.anchors };
    default:
      return { state: STATE_UNSUPPORTED, reason: "unsupported-report-state", anchors: claim.anchors };
  }
}

function strengthAxis(mutation: MutationFacts | null): Axis {
  if (mutation === null) {
    return { state: "NOT_MEASURED", reason: "no-mutation-run", anchors: null };
  }
  switch (mutation.verdict) {
    case "KILLED": {
      const anchors = mutation.witness === null ? null : [mutation.witness.operator, mutation.witness.killingTest];
      return { state: "KILLED", reason: `witnessed-kill: ${mutation.detail}`, anchors };
    }
    case "SURVIVED":
      return { state: "SURVIVED", reason: mutation.detail, anchors: null };
    default:
      return { state: "NOT_MEASURED", reason: mutation.detail, anchors: null };
  }
}
