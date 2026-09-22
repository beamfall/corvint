// Shattered Lens Kaleidoscope — the photo folded through a mirrored lens with
// chromatic dispersion at the fold lines; a dim unfolded photo sits behind the
// glass so the subject never disappears. Glints spark on the shard seams.
// Portable port of beamfall-apple-ui Shaders/PhotoVisuals.metal frag_photolens (keep in sync).
// param0 = Segments ; param1 = Spin ; param2 = Glints

float ringLine(float r, float target, float w) {
    return aaLine(r - target, w);
}

vec3 visual(vec2 uv, VisualUniforms u) {
    vec2 p = motionCentered(uv, u);
    float r = length(p);
    float a = atan2_(p.y, p.x) + u.time * (0.05 + u.param1 * 0.12)
            + u.bassImpact * 0.08;
    float segs = 8.0 + floor(u.param0 * 8.0);
    float glints = mix(0.4, 1.8, u.param2);

    float sector = 6.2831853 / segs;
    float ma = abs(fract(a / sector + 0.5) - 0.5) * sector;
    vec2 q = vec2(cos(ma), sin(ma)) * r;

    // kaleidoscope sample with chromatic dispersion along the fold direction
    float zoomB = 0.62 + 0.14 * sin(u.time * 0.10) + 0.05 * u.bassImpact;
    vec2 base = q * zoomB + 0.5;
    vec2 cdir = q / max(r, 1e-3) * (0.005 + 0.014 * u.treble);
    float cr = IMG(fract(base + cdir)).r;
    float cg = IMG(fract(base)).g;
    float cb = IMG(fract(base - cdir)).b;
    vec3 photo = max(vec3(cr, cg, cb), vec3(0.0));

    float lens = smoothstep(0.98, 0.25, r);
    // dim untouched photo behind the glass keeps the subject present
    vec3 back = max(IMG(clamp(uv, 0.0, 1.0)).rgb, vec3(0.0));
    vec3 col = back * 0.085 * (1.0 - lens);
    col += photo * lens * (0.42 + 0.22 * u.level + 0.10 * u.bass);

    // shard seams + rings, glinting with treble
    float shard = aaLineCyclic(a * segs / 6.2831853, 0.025)
                + aaLineCyclic(r * 7.0 + u.beatPhase, 0.035);
    col += palVisual(fract(a / 6.2831853 + 0.5), u) * shard * lens
         * glints * (0.30 + u.treble * 1.6);

    // glint sparks where seams cross the outer ring
    float ringD = ringLine(r, 0.58 + 0.04 * u.bassImpact, 0.006);
    col += peakWhite(palVisual(0.6, u), u.onset * 0.25) * ringD
         * (1.2 + 1.4 * u.onset) * glints;
    col += vec3(1.8, 1.75, 1.65) * shard * ringD * glints * (0.4 + 1.2 * u.treble);

    col *= 1.0 - smoothstep(1.02, 1.45, r) * 0.75;
    col = feedbackTrail(col, uv, min(u.trailDecay, 0.72), 0.004, 0.006);
    return col; // LINEAR HDR — post chain tonemaps
}
