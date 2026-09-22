// Volumetric Light — Lite — projector haze, band-driven beam fan, dust, leaks.
// Web-heritage "volumetric-light" upgraded: defined god-rays cut through a
// dark room from an offscreen projector; drifting smoke breaks the shafts
// into visible volume, beat slams the fan bright, dust motes spark where
// they cross a beam. Calm-capable: degrades to slow drifting haze when quiet.
// param0 = Haze ; param1 = Beam Count ; param2 = Dust

vec3 visual(vec2 uv, VisualUniforms u) {
    vec2 p = centered(uv, u.resolution);
    float haze = mix(0.45, 1.6, u.param0);
    int nBeams = int(mix(3.0, 7.0, u.param1) + 0.5);
    float dustAmt = mix(0.2, 1.4, u.param2);

    vec2 src = vec2(0.0, -1.30); // projector above the frame
    vec2 d = p - src;
    float dist = length(d);
    float ang = atan2_(d.x, d.y); // 0 = straight down

    vec3 col = vec3(0.0);

    // drifting smoke: what makes the shafts read as volume, not gradients
    float smoke = fbm(p * 2.3 + vec2(u.time * 0.07, -u.time * 0.045));
    smoke = 0.25 + 1.1 * smoke * smoke;

    // ---- beam fan: hard shafts through darkness ----
    float beamSum = 0.0;
    for (int i = 0; i < 7; ++i) {
        if (i >= nBeams) break;
        float fi = float(i);
        float fr = (fi + 0.5) / float(nBeams);
        float bandE = bandAt(u, 0.05 + fr * 0.55);

        float a0 = mix(-0.55, 0.55, fr)
                 + 0.06 * sin(u.time * (0.11 + 0.05 * fi) + fi * 2.3);
        float wCore = 0.013 + 0.009 * u.bassImpact;
        float wHalo = wCore + 0.030 + 0.030 * bandE;

        float da = ang - a0;
        float core = exp(-da * da / max(wCore * wCore, 1e-5));
        float halo = exp(-da * da / max(wHalo * wHalo, 1e-5));

        // shimmer traveling down the beam
        float sh = 0.55 + 0.45 * fbm(vec2(da * 14.0 + fi * 7.0, dist * 2.2 - u.time * 0.5));
        float fall = exp(-dist * 0.55);

        vec3 bc = palVisual(fr * 0.55 + 0.06 * u.spectralCentroid + 0.015 * u.time, u);
        bc = accentize(bc, u.accent, 0.15);

        // beat-driven: each shaft slams on its own phase offset
        float slam = phasePulse(u, fr * 0.35, 0.10) * u.bassImpact;
        float energy = 0.10 + 2.4 * bandE * bandE + 2.0 * slam;

        col += bc * halo * sh * smoke * fall * haze * energy * 0.40;
        col += peakWhite(bc, 0.25 + 0.4 * u.onset) * core * smoke * fall * energy * 0.85;
        beamSum += (halo * 0.5 + core) * fall * energy;
    }

    // ---- thin ambient haze + tight projector hot spot ----
    vec3 fog = palRoleFog(0.58 + 0.10 * u.spectralCentroid, u);
    col += fog * smoke * haze * 0.035 * exp(-dist * 0.9);
    col += peakWhite(palVisual(0.10, u), 0.5)
         * exp(-dist * dist * 7.0) * (0.5 + 1.1 * u.level);

    // ---- faint light leak at the top frame edge only ----
    float leakN = 0.5 + 0.5 * fbm(vec2(p.x * 1.6 + 9.0, u.time * 0.07));
    col += palVisual(0.82, u) * pow(saturate(1.0 - uv.y), 5.0) * leakN * haze * 0.10;

    // ---- dust motes: only visible where they cross the light ----
    for (int i = 0; i < 40; ++i) {
        float fi = float(i);
        vec2 h = hash22(vec2(fi, 11.3));
        vec2 mp = vec2(
            mix(-1.6, 1.6, fract(h.x + u.time * (0.008 + 0.02 * h.y))),
            mix(-1.0, 1.0, fract(h.y + u.time * (0.012 + 0.015 * h.x))));
        float dd = length(p - mp);
        float mote = 2.2e-5 / (dd * dd + 2.2e-5);
        mote = min(mote, 2.0);
        float flicker = 0.5 + 0.5 * hash11(fi + floor(u.time * 9.0));
        vec3 mc = peakWhite(palVisual(h.x * 0.4 + 0.5, u), u.treble * 0.3);
        col += mc * mote * min(beamSum, 2.5) * dustAmt * flicker
             * (0.10 + 0.55 * u.treble);
    }

    // dim floor pool where the beams land
    col += fog * smoothstep(0.70, 1.10, p.y) * exp(-abs(p.x) * 1.8)
         * haze * (0.04 + 0.16 * u.bass) * smoke;

    col = feedbackTrail(col, uv, min(u.trailDecay, 0.50), 0.0012, 0.0003);
    return col; // LINEAR HDR
}
