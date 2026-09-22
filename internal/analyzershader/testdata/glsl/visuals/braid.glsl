// Braid — Flagship — three luminous threads (bass / mid / treble) braiding
// around a shared horizontal axis in pseudo-3D. Each thread's radial
// displacement is its own band trace; beatPhase drives the twist rate;
// crossings emit sparks timed to beats; depth-sorted occlusion with HDR
// thread cores and feedbackTrail silk afterglow.
// param0 Twist ; param1 Core Heat ; param2 Sparks

vec3 visual(vec2 uv, VisualUniforms u) {
    vec2 p = motionCentered(uv, u);
    float x01 = saturate(uv.x);

    float twist = mix(0.9, 2.6, u.param0);
    float heat = mix(1.4, 3.6, u.param1);
    float sparkAmt = mix(0.4, 2.4, u.param2);

    // continuous beat-time: twist advances with the beat grid
    float tt = u.beatCount + u.beatPhase;
    float theta = x01 * twist * 6.2831853 + tt * 1.5707963 + u.time * 0.12;

    // per-thread band traces (radial displacement is the thread's own band)
    float trace0 = bandAt(u, x01 * 0.22);                 // bass
    float trace1 = bandAt(u, 0.30 + x01 * 0.30);          // mid
    float trace2 = bandAt(u, 0.66 + x01 * 0.30);          // treble

    float baseR = 0.20 + 0.06 * u.amplitude;

    float ys[3];
    float zs[3];
    float rads[3];
    rads[0] = baseR * (0.55 + 0.90 * trace0) * (1.0 + 0.35 * u.bassImpact);
    rads[1] = baseR * (0.55 + 0.90 * trace1);
    rads[2] = baseR * (0.55 + 0.90 * trace2);

    for (int i = 0; i < 3; ++i) {
        float ph = theta + float(i) * 2.0943951;          // 2pi/3 apart
        ys[i] = sin(ph) * rads[i]
              + sin(x01 * 9.0 + u.time * 0.8 + float(i) * 2.1) * 0.012; // silk waver
        zs[i] = cos(ph);                                   // -1 far .. +1 near
    }

    // ---- per-thread color + coverage ----
    vec4 frag[3]; // rgb = premultiplied-ish color, w = coverage
    for (int i = 0; i < 3; ++i) {
        float persp = 1.0 / (1.75 - 0.62 * zs[i]);         // pseudo-3D scale
        float d = p.y - ys[i] * persp;
        float band = (i == 0) ? trace0 : ((i == 1) ? trace1 : trace2);
        float wgt = (i == 0) ? 1.45 : ((i == 1) ? 1.0 : 0.7);  // bass fat, treble fine
        float w = 0.0042 * wgt * persp * (1.0 + 0.9 * band);
        float core = aaLine(d, w * 0.55) * 1.8 + aaLine(d, w) * 0.6; // white-hot center
        float halo = aaLine(d, w * 4.0) * 0.12;
        float sheen = exp(-d * d / (0.0035 * persp + 1e-5)) * 0.08;

        vec3 c;
        if (i == 0) c = palRoleBass(x01 * 0.30, u) * (0.8 + 2.0 * trace0);
        else if (i == 1) c = palRoleTrace(0.35 + x01 * 0.30, u) * (0.8 + 2.0 * trace1);
        else c = palRoleTreble(x01 * 0.5, u) * (0.8 + 2.2 * trace2);
        c = accentize(c, u.accent, 0.08);

        float lum = persp * persp;                         // near = brighter
        frag[i] = vec4(c * (core * heat + halo + sheen) * lum,
                       saturate(aaLine(d, w) * 0.95 + halo * 0.4));
    }

    // ---- depth sort far -> near (3-element bubble) and composite ----
    for (int pass = 0; pass < 2; ++pass) {
        for (int i = 0; i < 2; ++i) {
            if (zs[i] > zs[i + 1]) {
                float tz = zs[i]; zs[i] = zs[i + 1]; zs[i + 1] = tz;
                vec4 tf = frag[i]; frag[i] = frag[i + 1]; frag[i + 1] = tf;
            }
        }
    }
    vec3 col = vec3(0.0);
    for (int i = 0; i < 3; ++i) {
        col = col * (1.0 - frag[i].w * 0.92) + frag[i].rgb;  // near occludes far
    }

    // ---- crossing sparks, timed to beats ----
    float gate = phasePulse(u, 0.0, 0.14) * (0.4 + 0.8 * u.beat) + u.onset * 0.4;
    for (int i = 0; i < 3; ++i) {
        int j = (i + 1) % 3;
        float pi_ = theta + float(i) * 2.0943951;
        float pj_ = theta + float(j) * 2.0943951;
        float yi = sin(pi_) * rads[i];
        float yj = sin(pj_) * rads[j];
        float cross_ = exp(-((yi - yj) * 22.0)*((yi - yj) * 22.0));    // threads meeting
        float dpix = p.y - (yi + yj) * 0.5;
        float hot = exp(-dpix * dpix / 0.0008);
        // spark scatter around the crossing point
        float n = hash21(vec2(floor(x01 * 48.0), floor(u.beatCount)));
        float sparkle = step(0.72, n) * exp(-dpix * dpix / 0.00015);
        vec3 sc = palRolePeak(0.5 + 0.2 * u.spectralCentroid, u);
        col += sc * (hot * 1.3 + sparkle * 4.2) * cross_ * gate * sparkAmt;
    }
    // axis ghost: the shared axis the braid winds around
    col += palRoleFog(0.5, u) * exp(-p.y * p.y / 0.002) * 0.05 * (0.4 + 0.6 * u.amplitude);

    return feedbackTrail(col, uv, min(u.trailDecay, 0.82),
                         0.0016 + 0.0032 * u.bassImpact, 0.0007);
}
