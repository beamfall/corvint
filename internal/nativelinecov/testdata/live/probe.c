// SPDX-License-Identifier: Apache-2.0
#include <stdarg.h>
#include <stddef.h>
#include <setjmp.h>
#include <cmocka.h>
#include <spawn.h>
#include <sys/wait.h>
#include <stdlib.h>
extern char **environ;
static int value(int x) {
    if (x == 1) {
        return 11;
    }
    if (x == 2) {
        return 22;
    }
    return 33;
}
static void alpha(void **state) {
    (void)state;
    assert_int_equal(value(1), 11);
    pid_t pid;
    char *args[] = {getenv("COVLIVE_CHILD"), NULL};
    assert_int_equal(posix_spawn(&pid, args[0], NULL, NULL, args, environ), 0);
    int status;
    assert_int_equal(waitpid(pid, &status, 0), pid);
    assert_true(WIFEXITED(status));
    assert_int_equal(WEXITSTATUS(status), 0);
}
static void beta(void **state) {
    (void)state;
    assert_int_equal(value(2), 22);
}
int main(void) {
    const struct CMUnitTest tests[] = {cmocka_unit_test(alpha), cmocka_unit_test(beta)};
    return _cmocka_run_group_tests("probe", tests, 2, NULL, NULL);
}
