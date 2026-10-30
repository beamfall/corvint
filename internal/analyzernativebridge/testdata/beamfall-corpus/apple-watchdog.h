/*
 * Per-case and bundle-wide hang bounds for BeamfallAppleUI tests
 * (APPLEUI-TESTHANG-1).
 *
 * A deadlocked xctest process consumes no CPU and emits no output, so nothing
 * upstream can tell it apart from a slow one. Two such processes once sat alive
 * for 27h and 35h holding a serialized gate lane. This target arms a watchdog at
 * bundle load so the process fails fast instead. An XCTest observer also resets a
 * shorter deadline around every individual test case.
 *
 * Test-only: deliberately not a package product, so no shipping target links it.
 */

#ifndef BEAMFALL_TEST_WATCHDOG_H
#define BEAMFALL_TEST_WATCHDOG_H

/* Wall-clock budget the watchdog is currently enforcing, in seconds, or 0 when
 * disabled. Exposed so a test can assert the bound is actually armed. */
long beamfall_test_watchdog_budget_seconds(void);

/* Wall-clock budget enforced separately for each XCTestCase, in seconds, or 0
 * when disabled. */
long beamfall_test_case_watchdog_budget_seconds(void);

/* Nonzero after the per-case XCTest observer was registered at bundle load. */
int beamfall_test_case_watchdog_is_registered(void);

#endif /* BEAMFALL_TEST_WATCHDOG_H */
