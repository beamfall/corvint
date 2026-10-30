// Phosphor Scope — Standard — vintage CRT oscilloscope in a dark room:
// bass-X vs mid-Y Lissajous beam with phosphor persistence, faint graticule,
// onset beam-kick overshoot, treble HF fuzz on the beam.
// param0 Persistence ; param1 Figure Complexity ; param2 Beam Fuzz

vec3 visual(vec2 uv, VisualUniforms u) {
    vec2 p = centered(uv, u.resolution);

    // CRT face: gentle barrel bulge
    float r2 = dot(p, p);
    vec2 pc = p * (1.0 + 0.055 * r2);

    // screen rectangle (rounded-ish via soft edges)
    vec2 scr = vec2(0.86, 0.62);
    vec2 q = abs(pc) - scr;
    float dScreen = max(q.x, q.y);
    float inScreen = 1.0 - smoothstep(-0.006, 0.004, dScreen);
    // corner-darkening shade inside the tube
    float shade = 1.0 - smoothstep(0.35, 1.05, max(abs(pc.x) / scr.x, abs(pc.y) / scr.y)) * 0.55;

    vec3 col = vec3(0.0);

    // ---- dark room + bezel rim ----
    float bezel = exp(-max(dScreen, 0.0) * 9.0);
    col += palRoleFog(0.72, u) * bezel * (1.0 - inScreen) * (0.035 + 0.05 * u.amplitude);

    // ---- faint phosphor idle glow + graticule ----
    vec3 fog = palRoleFog(0.40 + 0.06 * u.spectralCentroid, u);
    col += fog * inScreen * shade * 0.028;

    float gx = abs(fract(pc.x / 0.215 + 0.5) - 0.5) * 0.215;
    float gy = abs(fract(pc.y / 0.215 + 0.5) - 0.5) * 0.215;
    float grid = max(aaLine(gx, 0.0011), aaLine(gy, 0.0011));
    float axis = max(aaLine(pc.x, 0.0016), aaLine(pc.y, 0.0016));
    col += fog * inScreen * shade * (grid * 0.055 + axis * 0.10);

    // ---- Lissajous beam ----
    // figure morphing steered by spectral centroid; complexity from param1
    float comp = mix(1.0, 3.0, u.param1);
    float fx = 1.0 + comp + 0.85 * sin(u.time * 0.11 + u.spectralCentroid * 2.4);
    float fy = 2.0 + comp * 0.7 + 0.85 * sin(u.time * 0.073 + 1.9 + u.spectralCentroid * 1.6);
    float phX = u.time * 0.42 + u.spectralCentroid * 2.2;

    // onset kick: amplitude overshoot + fast decaying ring in the deflection
    float kick = u.onset;
    float ax = scr.x * (0.52 + 0.30 * u.bass) * (1.0 + 0.16 * kick);
    float ay = scr.y * (0.52 + 0.30 * u.mid) * (1.0 + 0.22 * kick);
    float wob = kick * 0.05 * sin(u.time * 34.0);

    float fuzz = u.param2 * (0.3 + 1.4 * u.treble);

    vec3 beamCol = palRoleTrace(0.34 + 0.10 * u.spectralCentroid, u);
    vec3 haloCol = palVisual(0.30 + 0.08 * u.spectralCentroid, u);
    float core = 0.0;
    float halo = 0.0;
    vec2 head = vec2(0.0);
    vec2 prev = vec2(ax * sin(phX) * (1.0 + wob), ay * sin(wob * 2.0));

    // continuous beam: distance to 96 chained segments of the Lissajous curve
    for (int i = 1; i <= 96; ++i) {
        float fi = float(i);
        float s = fi / 96.0;
        float a = s * 6.2831853;
        vec2 pos = vec2(ax * sin(fx * a + phX) * (1.0 + wob),
                        ay * sin(fy * a + wob * 2.0));
        // treble HF fuzz: smooth micro-ripple along the sweep, not per-dot hash
        pos += vec2(sin(a * 61.0 + u.time * 9.3), cos(a * 73.0 - u.time * 7.1))
             * 0.0048 * fuzz;
        if (i == 96) head = pos;

        vec2 ab = pos - prev;
        float h = clamp(dot(pc - prev, ab) / (dot(ab, ab) + 1e-6), 0.0, 1.0);
        vec2 d = pc - prev - ab * h;
        float d2 = dot(d, d);
        float w = 0.30 + 0.70 * s * s; // beam head hotter than the tail
        core += w * 1.5e-5 / (d2 + 9.0e-6);
        halo += w * 7.0e-5 / (d2 + 8.0e-4);
        prev = pos;
    }

    float energy = 0.9 + 1.9 * u.amplitude + 1.4 * u.bassImpact;
    col += beamCol * core * energy * 1.35 * inScreen * shade;
    col += haloCol * halo * energy * 0.16 * inScreen * shade;

    // white-hot beam spot at the write head
    vec2 dh = pc - head;
    float spot = 6.0e-5 / (dot(dh, dh) + 3.0e-5);
    col += peakWhite(beamCol, 0.85) * spot * (0.35 + 0.5 * u.level) * inScreen;

    // subtle scanline shimmer of the tube glass
    col *= 1.0 - 0.045 * inScreen * (0.5 + 0.5 * sin(pc.y * 320.0));

    // ---- phosphor persistence ----
    float persist = clamp(mix(0.74, 0.93, u.param0), 0.0, 0.93);
    col = feedbackTrail(col, uv, persist, 0.0006, 0.0);
    return col; // LINEAR HDR
}
