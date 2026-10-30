#import "BeamfallTestWatchdog.h"

#import <XCTest/XCTest.h>

#include <dispatch/dispatch.h>
#include <errno.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>

static const long kBeamfallTestCaseWatchdogDefaultSeconds = 300;

static long g_beamfall_test_case_watchdog_budget_seconds = 0;
static int g_beamfall_test_case_watchdog_is_registered = 0;

long beamfall_test_case_watchdog_budget_seconds(void) {
    return g_beamfall_test_case_watchdog_budget_seconds;
}

int beamfall_test_case_watchdog_is_registered(void) {
    return g_beamfall_test_case_watchdog_is_registered;
}

static long beamfall_test_case_watchdog_configured_budget(void) {
    const char *override = getenv("BEAMFALL_TEST_CASE_WATCHDOG_SECONDS");
    if (override == NULL || *override == '\0') {
        return kBeamfallTestCaseWatchdogDefaultSeconds;
    }

    errno = 0;
    char *end = NULL;
    long parsed = strtol(override, &end, 10);
    if (errno != 0 || end == override || *end != '\0' || parsed < 0) {
        return kBeamfallTestCaseWatchdogDefaultSeconds;
    }
    if (parsed > INT64_MAX / NSEC_PER_SEC) {
        return kBeamfallTestCaseWatchdogDefaultSeconds;
    }
    return parsed;
}

@interface BeamfallTestCaseTimer : NSObject
@property(nonatomic, strong) dispatch_source_t source;
@property(nonatomic, strong) NSObject *token;
@end

@implementation BeamfallTestCaseTimer
@end

@interface BeamfallTestCaseWatchdog : NSObject <XCTestObservation>
- (instancetype)initWithBudgetSeconds:(long)budgetSeconds;
@end

@implementation BeamfallTestCaseWatchdog {
    long _budgetSeconds;
    dispatch_queue_t _stateQueue;
    NSMutableDictionary<NSValue *, BeamfallTestCaseTimer *> *_activeTimers;
}

- (instancetype)initWithBudgetSeconds:(long)budgetSeconds {
    self = [super init];
    if (self == nil) {
        return nil;
    }

    _budgetSeconds = budgetSeconds;
    _stateQueue = dispatch_queue_create(
        "com.beamfall.tests.per-case-watchdog",
        DISPATCH_QUEUE_SERIAL);
    _activeTimers = [NSMutableDictionary dictionary];
    return self;
}

- (void)testCaseWillStart:(XCTestCase *)testCase {
    NSValue *key = [NSValue valueWithPointer:(__bridge const void *)testCase];
    NSString *testName = [testCase.name copy];

    dispatch_sync(_stateQueue, ^{
        BeamfallTestCaseTimer *previous = self->_activeTimers[key];
        if (previous != nil) {
            [self->_activeTimers removeObjectForKey:key];
            dispatch_source_cancel(previous.source);
        }

        NSObject *token = [NSObject new];
        dispatch_source_t source = dispatch_source_create(
            DISPATCH_SOURCE_TYPE_TIMER,
            0,
            0,
            self->_stateQueue);
        BeamfallTestCaseTimer *timer = [BeamfallTestCaseTimer new];
        timer.source = source;
        timer.token = token;
        self->_activeTimers[key] = timer;

        __weak BeamfallTestCaseWatchdog *weakSelf = self;
        dispatch_source_set_event_handler(source, ^{
            BeamfallTestCaseWatchdog *strongSelf = weakSelf;
            BeamfallTestCaseTimer *active = strongSelf->_activeTimers[key];
            if (active.token != token) {
                return;
            }

            fprintf(
                stderr,
                "\nBeamfallTestWatchdog: test case '%s' exceeded its %ld second "
                "budget and is being aborted.\n",
                testName.UTF8String,
                strongSelf->_budgetSeconds);
            fflush(stderr);
            abort();
        });

        int64_t deadlineNanoseconds = (int64_t)self->_budgetSeconds * NSEC_PER_SEC;
        dispatch_source_set_timer(
            source,
            dispatch_time(DISPATCH_TIME_NOW, deadlineNanoseconds),
            DISPATCH_TIME_FOREVER,
            100 * NSEC_PER_MSEC);
        dispatch_resume(source);
    });
}

- (void)testCaseDidFinish:(XCTestCase *)testCase {
    NSValue *key = [NSValue valueWithPointer:(__bridge const void *)testCase];

    dispatch_sync(_stateQueue, ^{
        BeamfallTestCaseTimer *timer = self->_activeTimers[key];
        if (timer == nil) {
            return;
        }

        [self->_activeTimers removeObjectForKey:key];
        dispatch_source_cancel(timer.source);
    });
}

@end

__attribute__((constructor)) static void beamfall_test_case_watchdog_register(void) {
    @autoreleasepool {
        long budget = beamfall_test_case_watchdog_configured_budget();
        if (budget == 0) {
            return;
        }

        g_beamfall_test_case_watchdog_budget_seconds = budget;
        BeamfallTestCaseWatchdog *observer =
            [[BeamfallTestCaseWatchdog alloc] initWithBudgetSeconds:budget];
        [XCTestObservationCenter.sharedTestObservationCenter addTestObserver:observer];
        g_beamfall_test_case_watchdog_is_registered = 1;
    }
}
