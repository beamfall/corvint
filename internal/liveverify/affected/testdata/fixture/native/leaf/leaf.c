#include "leaf.h"
#include "../mid/mid.h"
int leaf_value(void) { return mid_value() + 1; }
