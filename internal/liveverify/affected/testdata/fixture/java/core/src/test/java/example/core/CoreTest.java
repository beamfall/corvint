package example.core;

import static org.junit.jupiter.api.Assertions.assertTrue;

import org.junit.jupiter.api.Test;

class CoreTest {
    @Test
    void works() {
        assertTrue(Core.value() >= 0);
    }
}
