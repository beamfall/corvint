// Prism Room — Flagship — faceted caustic chamber.
// Web-heritage "prism-room" upgraded: kaleidoscopic wedge fold builds a room
// of glass panes; each facet is lit by its own spectrum band, caustic sweeps
// wash across rows on the beat, HDR edge filaments carve the geometry.
// param0 = Facets ; param1 = Sweep ; param2 = Edge Glow

vec3 visual(vec2 uv, VisualUniforms u) {
    vec2 p = motionCentered(uv, u);
    float facetsF = floor(mix(5.0, 10.0, u.param0) + 0.5);
    float sweepSpd = mix(0.10, 0.55, u.param1);
    float edgeGlow = mix(0.9, 3.4, u.param2);

    float r = length(p);
    float a = atan2_(p.y, p.x) + u.time * 0.045 + u.beatPhase * 0.08;

    // ---- kaleidoscopic wedge fold ----
    float sector = 6.2831853 / facetsF;
    float ai = floor((a + 3.14159265) / sector);
    float am = a + 3.14159265 - (ai + 0.5) * sector; // wedge-local angle, centered

    // facet rows marching outward in radius
    float rowW = 0.155;
    float rr = r + 0.04 * sin(am * 3.0 + u.time * 0.2); // subtle pane warp
    float row = floor(rr / rowW);
    float rl = fract(rr / rowW);

    // per-facet identity
    float aid = mod(ai, facetsF);
    float cid = hash21(vec2(row * 7.1, aid * 3.3));
    int bi = int(mod(cid * 63.0 + row * 5.0, 63.0));
    float bandE = band(u, bi);

    vec3 col = vec3(0.0);

    // ---- glass pane fill ----
    float hue = cid * 0.55 + row * 0.055 + 0.10 * u.spectralCentroid + 0.04 * u.beatPhase;
    vec3 pane = palVisual(hue, u);
    // push the glass toward pure spectral color: darker but far more saturated
    pane = clamp(mix(vec3(luma(pane)), pane, 1.55), vec3(0.0), vec3(4.0));
    pane = pane * pane * 1.6; // spectral glass: kill the pastel, keep the hue
    pane = accentize(pane, u.accent, 0.15);

    // internal gradient: light pools toward the pane's outer edge, the rest
    // of the pane stays smoked glass over darkness
    float grad = mix(0.10, 1.0, rl * rl);
    float roomWin = smoothstep(0.10, 0.30, r) * (1.0 - smoothstep(0.85, 1.35, r));
    // only facets whose band is hot light up; the rest stay smoked glass —
    // the light source is the room's heart, so outer rows fall darker
    float lit = smoothstep(0.28, 0.80, bandE);
    float heart = mix(1.0, 0.30, smoothstep(0.40, 1.10, r));
    col += pane * grad * roomWin * heart * (0.025 + 1.5 * lit * bandE + 0.35 * bandE + 0.06 * u.level + 0.7 * u.onset + 0.5 * u.beat);

    // beat flash on random facets
    float flashGate = step(0.80, hash21(vec2(row + floor(u.beatCount), aid)));
    col += peakWhite(pane, 0.35) * flashGate * u.onset * u.onset * roomWin * 1.6;

    // ---- caustic sweep washing outward through the rows ----
    float sweepR = fract(u.time * sweepSpd + u.beatPhase * 0.25) * 1.5;
    float sweep = exp(-((rr - sweepR) * 16.0)*((rr - sweepR) * 16.0));
    vec3 sweepCol = palVisual(hue + 0.25, u);
    sweepCol = sweepCol * sweepCol * 1.6;
    col += sweepCol * sweep * roomWin * heart * (0.12 + 0.5 * u.mid + 1.1 * u.bassImpact);

    // ---- HDR edge filaments ----
    // wedge seams (arc-length distance so lines stay thin)
    float edgeA = aaLine((abs(am) - sector * 0.5) * max(r, 0.05), 0.0032);
    // row seams
    float dRow = min(rl, 1.0 - rl) * rowW;
    float edgeR = aaLine(dRow, 0.0030);
    vec3 rim = palVisual(hue + 0.12, u);
    rim = clamp(mix(vec3(luma(rim)), rim, 1.4), vec3(0.0), vec3(4.0));
    float edgeE = edgeGlow * (0.4 + 1.2 * u.bassImpact + 1.4 * bandE);
    col += rim * (edgeA + edgeR) * roomWin * heart * edgeE * 0.30;

    // vertex glints where seams cross — sparks ride the caustic sweep
    float glint = edgeA * edgeR;
    col += peakWhite(rim, 0.4) * glint * roomWin * edgeE
         * (0.25 + 3.0 * sweep + 1.5 * u.onset);

    // ---- dark heart of the room + falloff ----
    col *= smoothstep(0.05, 0.16, r);
    col += palVisual(hue, u) * exp(-r * r * 2.6) * smoothstep(0.05, 0.22, r) * (0.05 + 0.35 * u.bass);
    // walls beyond the glass fall to black
    col *= 1.0 - smoothstep(1.05, 1.5, r) * 0.95;

    col = feedbackTrail(col, uv, min(u.trailDecay + 0.20 * u.bassImpact, 0.55), 0.002, 0.0006 + 0.0012 * u.beat);
    return col; // LINEAR HDR
}
