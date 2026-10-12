package example.mid;

import static org.junit.jupiter.api.Assertions.assertTrue;

import org.junit.jupiter.api.Test;

class MidTest {
    @Test
    void works() {
        assertTrue(Mid.value() >= 0);
    }
}
