package example.solo;

import static org.junit.jupiter.api.Assertions.assertTrue;

import org.junit.jupiter.api.Test;

class SoloTest {
    @Test
    void works() {
        assertTrue(Solo.value() >= 0);
    }
}
