// Lumen Field — Standard — a field of spectral light pillars on a mirror floor.
// Web-heritage "lumen-field" upgraded: bottom-anchored light stalks, one per
// spectrum slice, HDR tip orbs, rising energy pulses, rippled floor reflection.
// param0 = Pillars ; param1 = Bloom ; param2 = Sway

vec3 visual(vec2 uv, VisualUniforms u) {
    vec2 p = centered(uv, u.resolution);
    int nP = int(mix(7.0, 13.0, u.param0) + 0.5);
    float bloom = mix(0.6, 2.2, u.param1);
    float sway = mix(0.012, 0.10, u.param2);

    float floorY = 0.42; // +y is down (origin top-left)
    float below = p.y - floorY;
    float isRefl = step(0.0, below);
    // mirror coordinate + water ripple in the reflection
    float yA = mix(p.y, floorY - below, isRefl);
    float rippleX = isRefl * sin(p.y * 34.0 + u.time * 1.7) * 0.010 * (1.0 + 2.0 * u.bass);

    vec3 col = vec3(0.0);

    for (int i = 0; i < 13; ++i) {
        if (i >= nP) break;
        float fi = float(i);
        float fr = (fi + 0.5) / float(nP);
        float bandE = bandAt(u, fr * 0.85);

        float x0 = mix(-1.45, 1.45, fr)
                 + sin(u.time * (0.30 + 0.11 * fi) + fi * 1.9) * sway;
        float dx = p.x + rippleX - x0;

        float wCore = 0.011 + 0.010 * bandE;
        float wHalo = wCore * (3.2 + 1.5 * bloom);
        float core = exp(-dx * dx / max(wCore * wCore, 1e-6));
        float halo = exp(-dx * dx / max(wHalo * wHalo, 1e-6));

        float hgt = 0.22 + 1.00 * bandE + 0.14 * u.bassImpact;
        float tipY = floorY - hgt;
        float ywin = smoothstep(tipY - 0.06, tipY + 0.14, yA)
                   * (1.0 - smoothstep(floorY - 0.005, floorY + 0.03, yA));
        // brighter toward the tip
        float vshade = mix(0.35, 1.15, saturate((floorY - yA) / max(hgt, 1e-3)));

        // rising energy pulses inside the stalk
        float pulse = 0.6 + 0.4 * sin((floorY - yA) * 16.0 + u.time * (2.0 + 3.0 * bandE) + fi);

        float hue = fr * 0.7 + 0.08 * u.spectralCentroid + 0.02 * u.time;
        vec3 c = palVisual(hue, u);
        c = accentize(c, u.accent, 0.10);

        float refDim = mix(1.0, 0.34 * saturate(1.0 - below * 1.8), isRefl);
        float energy = (0.35 + 1.9 * bandE) * refDim;

        col += c * halo * ywin * vshade * pulse * energy * 0.45 * bloom;
        col += peakWhite(c, 0.25 * u.onset) * core * ywin * vshade * energy * 1.4; // HDR spine

        // HDR tip orb
        float tdx = p.x + rippleX - x0;
        float tdy = yA - tipY;
        float td2 = tdx * tdx + tdy * tdy;
        float orb = min(3.0, 2.2e-4 / (td2 + 2.2e-4));
        vec3 oc = palRoleTreble(hue + 0.2, u);
        col += oc * orb * (0.8 + 2.4 * bandE + 1.6 * u.bassImpact) * refDim * bloom;
    }

    // floor line + soft haze hugging it
    col += palRoleRim(0.5 + 0.1 * u.spectralCentroid, u)
         * aaLine(p.y - floorY, 0.0028) * (1.2 + 2.0 * u.bassImpact);
    col += palRoleFog(0.55, u) * exp(-abs(p.y - floorY) * 5.5) * (0.06 + 0.30 * u.bass);

    // dark ceiling / edge falloff keeps the room black
    col *= 1.0 - smoothstep(0.75, 1.45, length(p * vec2(0.75, 1.15))) * 0.85;

    col = feedbackTrail(col, uv, min(u.trailDecay, 0.70), 0.0018, 0.0004);
    return col; // LINEAR HDR
}
