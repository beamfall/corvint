#include "BeamfallTestWatchdog.h"

#include <errno.h>
#include <pthread.h>
#include <stdio.h>
#include <stdlib.h>
#include <time.h>
#include <unistd.h>

/*
 * Default wall-clock budget for one test-bundle process. The suite runs in ~112s
 * on an idle host, so this leaves ample headroom for a loaded gate host while
 * still bounding a hang to minutes instead of the 27h that occupied a gate lane.
 * Override with BEAMFALL_TEST_WATCHDOG_SECONDS; set it to 0 to disable.
 */
static const long kBeamfallWatchdogDefaultSeconds = 1800;

static long g_beamfall_watchdog_budget_seconds = 0;

long beamfall_test_watchdog_budget_seconds(void) {
    return g_beamfall_watchdog_budget_seconds;
}

static long beamfall_watchdog_configured_budget(void) {
    const char *override = getenv("BEAMFALL_TEST_WATCHDOG_SECONDS");
    if (override == NULL || *override == '\0') {
        return kBeamfallWatchdogDefaultSeconds;
    }

    errno = 0;
    char *end = NULL;
    long parsed = strtol(override, &end, 10);
    if (errno != 0 || end == override || *end != '\0' || parsed < 0) {
        return kBeamfallWatchdogDefaultSeconds;
    }
    return parsed;
}

static void *beamfall_watchdog_main(void *unused) {
    (void)unused;

    /* Deadline-driven, not sleep-accumulating: sleep() returns the seconds it did
     * NOT sleep, so subtracting its result never advances the clock. */
    const time_t deadline = time(NULL) + (time_t)g_beamfall_watchdog_budget_seconds;
    for (;;) {
        const time_t now = time(NULL);
        if (now >= deadline) {
            break;
        }
        const time_t remaining = deadline - now;
        sleep((unsigned int)(remaining > 5 ? 5 : remaining));
    }

    fprintf(
        stderr,
        "\nBeamfallTestWatchdog: test bundle exceeded its %ld second budget and is "
        "being aborted.\nThe process made no progress in that window, which means a "
        "deadlock, not slowness.\nThe crash report for this abort carries a backtrace "
        "for every thread — read it to name\nthe two parties. Raise or disable the "
        "budget with BEAMFALL_TEST_WATCHDOG_SECONDS.\n",
        g_beamfall_watchdog_budget_seconds);
    fflush(stderr);

    /* abort(), not exit(): it reds the run AND leaves a full all-thread backtrace,
     * which is exactly the diagnostic a silent 0%-CPU hang otherwise denies. */
    abort();
}

/*
 * Runs at bundle load, before XCTest builds any suite, so the bound covers every
 * test in the bundle regardless of which target or class runs first.
 */
__attribute__((constructor)) static void beamfall_test_watchdog_arm(void) {
    long budget = beamfall_watchdog_configured_budget();
    if (budget == 0) {
        return;
    }
    g_beamfall_watchdog_budget_seconds = budget;

    pthread_t thread;
    if (pthread_create(&thread, NULL, beamfall_watchdog_main, NULL) != 0) {
        g_beamfall_watchdog_budget_seconds = 0;
        return;
    }
    pthread_detach(thread);
}
