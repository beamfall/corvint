// SPDX-License-Identifier: Apache-2.0
static int child_value(void) {
    return 42;
}
int main(void) {
    return child_value() == 42 ? 0 : 1;
}
