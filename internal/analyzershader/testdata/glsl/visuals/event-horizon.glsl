// Event Horizon — Flagship — gravitational vortex tunnel.
// Web-heritage "event-horizon" upgraded. Identity: a black hole seen down the
// throat — spiral accretion arms sheared by gravity, doppler-lit disk, HDR
// photon ring, infalling ember streams. Deliberately distinct from Tunnel
// (no concentric ring flythrough, no circular equalizer) and from any lens
// visual: this stays an opaque vortex tunnel.
// param0 = Pull ; param1 = Spin ; param2 = Disk Detail

vec3 visual(vec2 uv, VisualUniforms u) {
    vec2 p = motionCentered(uv, u);
    p = rot2(-0.12) * p;
    p.y /= 0.86; // tilt the disc slightly toward the camera

    float r = length(p);
    float rS = max(r, 1e-3);
    float a = atan2_(p.y, p.x);

    float pull   = mix(0.55, 2.1, u.param0);
    float spin   = mix(0.12, 0.75, u.param1);
    float detail = mix(2.5, 7.0, u.param2);

    float coreR = 0.165 + 0.012 * sin(u.time * 0.7) + 0.045 * u.bassImpact;

    // gravitational shear — inner material orbits faster
    float swirl = a + pull / (rS + 0.11) + u.time * spin;

    vec3 col = vec3(0.0);

    // ---- spiral accretion arms ----
    float lr = log(rS + 0.05);
    float arm1 = 0.5 + 0.5 * sin(swirl * 3.0 - lr * detail);
    float arm2 = 0.5 + 0.5 * sin(swirl * 5.0 + lr * detail * 1.7 + 2.1);
    // Angular noise embedded on a circle of radius 0.85: it traverses
    // 2*pi*0.85 ~= 5.34 domain units per revolution, exactly the rate the old
    // fbm(swirl*0.85) ramp had, so the feature scale is preserved but the
    // domain no longer tears where atan2 wraps on the -x axis (rule 13).
    float turb = fbm(vec2(cos(swirl), sin(swirl)) * 0.85
                     + vec2(rS * 3.2 - u.time * 0.35, 0.0));

    float disk = smoothstep(0.28, 0.92, arm1) * (0.45 + 0.85 * turb)
               + 0.55 * smoothstep(0.45, 0.97, arm2) * turb;

    // radial window: hot at the horizon, fading into the void
    float win = smoothstep(coreR * 0.85, coreR * 1.35, r) * (1.0 - smoothstep(0.50, 1.30, r));
    float heat = exp(-(rS - coreR) * 2.4);

    // doppler beaming — the approaching side burns brighter and bluer
    float dopp = 0.5 + 0.5 * sin(a + u.time * spin * 0.6);

    float bandE = bandAt(u, saturate((rS - coreR) * 1.1)); // bass feeds the horizon
    float hue = fract(0.04 + heat * 0.22 + dopp * 0.10 + 0.06 * u.spectralCentroid);
    vec3 hot = palRoleBass(hue, u);
    vec3 cold = palVisual(hue + 0.34, u);
    vec3 armCol = mix(cold, hot, saturate(heat * 1.2));

    float exposure = 0.65 + 1.3 * u.level + 1.6 * u.bassImpact * heat;
    col += armCol * disk * win * (0.55 + 2.6 * heat) * (0.5 + 0.9 * dopp) * exposure * (0.6 + 0.8 * bandE);

    // ---- infalling ember streams ----
    for (int i = 0; i < 36; ++i) {
        float fi = float(i);
        vec2 h = hash22(vec2(fi, 5.7));
        float life = fract(h.y + u.time * (0.10 + 0.22 * h.x) + u.beatCount * 0.05);
        float er = mix(1.25, coreR * 1.05, life); // falls inward
        float ea = h.x * 6.2831853 + pull / (er + 0.11) * 0.9 + u.time * spin;
        vec2 ep = vec2(cos(ea), sin(ea)) * er;
        float dd = length(p - ep);
        float ember = min(2.2, 3.5e-4 / (dd * dd + 3.5e-4));
        vec3 ec = palRoleBass(h.x * 0.3, u);
        ec = peakWhite(ec, life * u.onset * 0.25);
        col += ec * ember * life * (0.4 + 1.8 * u.mid) * smoothstep(coreR, coreR * 1.5, er);
    }

    // ---- HDR photon ring ----
    float ring = aaLine(r - coreR * 1.06, 0.006 + 0.004 * u.bassImpact);
    vec3 ringCol = peakWhite(palVisual(0.07 + 0.05 * u.spectralCentroid, u),
                             0.35 + 0.45 * u.bassImpact);
    col += ringCol * ring * (3.5 + 4.5 * u.bassImpact) * (0.55 + 0.45 * dopp);

    // faint lensed halo just outside the ring
    col += palRoleRim(0.12, u) * exp(-abs(r - coreR * 1.06) * 22.0) * (0.5 + 1.4 * u.bass);

    // onset flare: a shockwave ripple racing outward
    float shockR = coreR * 1.1 + fract(u.beatPhase) * 0.9;
    col += palRolePeak(0.2, u) * aaLine(r - shockR, 0.010) * u.onset * 2.4
         * (1.0 - smoothstep(0.6, 1.1, r));

    // ---- absolute black core ----
    col *= smoothstep(coreR * 0.70, coreR * 1.00, r);

    // swirl smear trails
    col = feedbackTrail(col, uv, min(u.trailDecay, 0.88),
                        0.006 + 0.011 * u.bassImpact,
                        0.004 + 0.004 * spin);
    return col; // LINEAR HDR
}
