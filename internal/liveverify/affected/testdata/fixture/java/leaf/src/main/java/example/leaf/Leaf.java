package example.leaf;

import example.mid.Mid;

public final class Leaf {
    public static int value() {
        return Mid.value() + 1;
    }
}
