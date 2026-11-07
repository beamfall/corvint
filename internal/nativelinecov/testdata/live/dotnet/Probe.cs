// SPDX-License-Identifier: Apache-2.0
using Xunit;
public class Probe {
    public static int Value(int x) {
        if (x == 1) {
            return 11;
        }
        if (x == 2) {
            return 22;
        }
        return 33;
    }
    [Fact] public void Alpha() { Assert.Equal(11, Value(1)); }
    [Fact] public void Beta() { Assert.Equal(22, Value(2)); }
}
