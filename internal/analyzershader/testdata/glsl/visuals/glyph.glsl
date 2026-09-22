// Glyph — Standard — frequency + beat.
// Portable port of beamfall-apple-ui Shaders/Glyph.metal (keep in sync).
//
// Rotating spectral sigil with radial bars and broken rings.
// Polar linework, visually distinct from Kaleidoscope's folded mandala.
// param0 Rings, param1 Rotation, param2 Glyph Density

vec3 visual(vec2 uv, VisualUniforms u) {
    vec2 p = motionCentered(uv, u);
    p = rot2(u.time * mix(0.02, 0.16, u.param1) + u.beatPhase * 0.05) * p;
    float r = length(p);
    float a = atan2_(p.y, p.x);
    float ang = a / 6.2831853 + 0.5;

    float rings = mix(4.0, 11.0, u.param0);
    // integer dash count so the ring joins across the ang 1->0 wrap (rule 13)
    float density = floor(mix(12.0, 42.0, u.param2) + 0.5);
    vec3 glow = vec3(0.0);

    for (int k = 0; k < 11; k++) {
        if (float(k) >= rings) break;
        float fk = float(k);
        float rr = 0.12 + fk * 0.065 + 0.012 * sin(u.time * 0.4 + fk);
        float bandv = bandAtWrapped(u, fk / 11.0 + ang);
        float cell = fract(ang * (density + fk * 2.0) + fk * 0.23);
        float dash = smoothstep(0.02, 0.10, cell) * smoothstep(0.72, 0.52, cell);
        float line = aaLine(r - rr - bandv * 0.025, 0.0028) * dash;
        vec3 c = palRoleTrace(ang + fk * 0.08 + 0.12 * u.spectralCentroid, u);
        glow += c * line * (0.35 + bandv * 2.6 + u.bassImpact * 1.8 + u.onset * 1.2);
    }

    for (int i = 0; i < 32; i++) {
        float fi = float(i);
        float spokeA = fi / 32.0;
        float ad = abs(fract(ang - spokeA + 0.5) - 0.5);
        float bandv = bandAt(u, spokeA);
        float spoke = aaLine(ad, 0.0015 + bandv * 0.0012) *
                      smoothstep(0.10, 0.18, r) * (1.0 - smoothstep(0.62 + bandv * 0.22, 0.75 + bandv * 0.22, r));
        glow += palRoleTreble(spokeA + 0.1 * u.spectralCentroid, u) * spoke * (0.15 + bandv * 2.2 + u.beat * 0.9);
    }

    glow += palRoleRim(0.58, u) * aaLine(r - 0.085, 0.004) * (0.35 + u.level * 0.6 + u.onset * 1.6 + u.beat * 1.1);

    vec3 col = feedbackTrail(glow, uv,
                             min(u.trailDecay, 0.88),
                             0.002 + 0.005 * u.bassImpact,
                             0.0025 * u.beat);
    return col; // LINEAR HDR — post chain tonemaps
}
