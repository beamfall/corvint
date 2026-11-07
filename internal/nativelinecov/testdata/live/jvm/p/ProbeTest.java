// SPDX-License-Identifier: Apache-2.0
package p;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.BeforeAll;
import static org.junit.jupiter.api.Assertions.*;
import org.jacoco.agent.rt.RT;
public class ProbeTest {
    @BeforeAll static void session() { RT.getAgent().setSessionId(Long.toString(ProcessHandle.current().pid())); }
    @Test void alpha() throws Exception {
        assertEquals(11, Logic.value(1));
        Process child = new ProcessBuilder(System.getProperty("java.home") + "/bin/java", "-cp", System.getProperty("covlive.classes"), "p.Child").inheritIO().start();
        assertEquals(0, child.waitFor());
    }
    @Test void beta() { assertEquals(22, Logic.value(2)); }
}
