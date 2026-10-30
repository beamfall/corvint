// Spectral City — Standard / frequency + beat.
// Portable port of frag_spectral_city in beamfall-apple-ui
// Shaders/CuratedRuntime.metal (keep in sync).
// A pulse-array skyline over a glossy black floor.
// param0 = Density ; param1 = Speed ; param2 = Reflection

float boxFill(vec2 p, vec2 b) {
    vec2 d = abs(p) - b;
    return aaFill(length(max(d, vec2(0.0))) + min(max(d.x, d.y), 0.0));
}

vec3 visual(vec2 uv, VisualUniforms u) {
    float floorY = 0.64;
    vec3 col = vec3(0.002, 0.004, 0.008);

    float road = smoothstep(floorY, 1.0, uv.y);
    if (road > 0.0) {
        float z = 1.0 / max(0.045, uv.y - floorY + 0.05);
        float lane = (uv.x - 0.5) * z;
        float grid = aaLineCyclic(lane * 0.34, 0.018) + aaLineCyclic(z * 0.10 - u.time * 0.22, 0.018);
        col += palVisual(fract(lane * 0.04 + z * 0.02), u) * grid * road * 0.26 * smoothstep(13.0, 1.2, z);
    }

    for (int i = 0; i < 72; i++) {
        float fi = float(i);
        float x = (fi + 0.5) / 72.0;
        float perspective = mix(0.50, 1.0, hash11(fi * 2.3));
        float bandv = bandAt(u, x);
        float w = mix(0.004, 0.014, perspective);
        float h = 0.07 + bandv * mix(0.28, 0.62, perspective) + hash11(fi * 5.1) * 0.12;
        float base = floorY - 0.01 * perspective;
        vec2 bp = vec2(uv.x - x, uv.y - (base - h * 0.5));
        float tower = boxFill(bp, vec2(w, h * 0.5));
        float edge = aaLine(abs(bp.x) - w, 0.0014) * step(abs(bp.y), h * 0.52);
        float rows = aaLineCyclic((uv.y - base + h) * 42.0, 0.03) * tower;
        vec3 c = palVisual(x + u.time * 0.018, u);
        col += c * tower * (0.12 + bandv * 0.65);
        col += peakWhite(c, rows * u.treble * 0.08) * rows * (0.7 + bandv * 3.0);
        col += c * edge * (0.9 + u.onset * 2.0);

        float reflY = floorY + (base - h * 0.5 - uv.y) * 0.34;
        float refl = boxFill(vec2(uv.x - x, uv.y - reflY), vec2(w, h * 0.18))
                   * smoothstep(floorY, 1.0, uv.y) * (1.0 - smoothstep(floorY, 1.0, uv.y));
        col += c * refl * bandv * 0.45;
    }

    col += palVisual(0.55, u) * aaLine(uv.y - floorY, 0.0015) * 0.65;
    col = feedbackTrail(col, uv, min(u.trailDecay, 0.82), 0.002 + 0.004 * u.bassImpact, 0.0);
    return col; // LINEAR HDR — post chain tonemaps
}
