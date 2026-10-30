package example.leaf;

import static org.junit.jupiter.api.Assertions.assertTrue;

import org.junit.jupiter.api.Test;

class LeafTest {
    @Test
    void works() {
        assertTrue(Leaf.value() >= 0);
    }
}
