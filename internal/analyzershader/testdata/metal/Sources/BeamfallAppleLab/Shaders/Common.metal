#include "VisualShared.h"

// Single fullscreen triangle. Defined once here so the symbol is unique in the
// linked metallib; every frag_<id> shares it.
vertex VOut fullscreen_vertex(uint vid [[vertex_id]]) {
    float2 p = float2(float((vid << 1) & 2), float(vid & 2)); // (0,0)(2,0)(0,2)
    VOut o;
    o.position = float4(p * 2.0 - 1.0, 0.0, 1.0);
    o.uv = float2(p.x, 1.0 - p.y); // origin top-left
    return o;
}
