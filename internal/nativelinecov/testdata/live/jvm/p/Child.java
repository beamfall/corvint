// SPDX-License-Identifier: Apache-2.0
package p;
import org.jacoco.agent.rt.RT;
public class Child {
    public static void main(String[] args) {
        RT.getAgent().setSessionId(Long.toString(ProcessHandle.current().pid()));
        if (childValue() != 42) throw new AssertionError();
    }
    public static int childValue() {
        return 42;
    }
}
