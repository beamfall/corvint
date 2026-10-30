// Crystal — Flagship / frequency + beat + onset.
// Portable port of beamfall-apple-ui Shaders/Crystal.metal (keep in sync).
// A faceted prism refracting a procedural neon event-horizon environment.
// param0 = Facet Count ; param1 = Dispersion ; param2 = Fracture

// ---------------------------------------------------------------------------
// SDF: faceted crystal
// ---------------------------------------------------------------------------
float sdOctahedron(vec3 p, float s) {
    vec3 q = abs(p);
    return (q.x + q.y + q.z - s) * 0.57735027;   // 1/sqrt(3)
}

float sdPlane(vec3 p, vec3 n, float d) {
    return dot(p, n) - d;
}

float crystalSDF(vec3 p, float facetSharp, float time) {
    float d = sdOctahedron(p, 0.80);
    float sphere = length(p) - 0.90;
    d = max(d, sphere);

    int N = int(clamp(facetSharp, 2.0, 12.0));
    float angleStep = 6.28318530 / float(N);

    for (int i = 0; i < 12; ++i) {
        if (i >= N) break;
        float a = float(i) * angleStep + time * 0.07;
        vec3 n1 = normalize(vec3(cos(a), 0.55, sin(a)));
        d = max(d, sdPlane(p, n1, 0.62));
    }
    for (int i = 0; i < 12; ++i) {
        if (i >= N) break;
        float a = float(i) * angleStep + angleStep * 0.5 + time * 0.07;
        vec3 n2 = normalize(vec3(cos(a), -0.55, sin(a)));
        d = max(d, sdPlane(p, n2, 0.62));
    }
    return d;
}

// ---------------------------------------------------------------------------
// Normal — tetrahedron method
// ---------------------------------------------------------------------------
vec3 crystalNormal(vec3 p, float facetSharp, float time) {
    const vec2 e = vec2(0.0015, -0.0015);
    vec3 n =
        e.xyy * crystalSDF(p + e.xyy, facetSharp, time) +
        e.yyx * crystalSDF(p + e.yyx, facetSharp, time) +
        e.yxy * crystalSDF(p + e.yxy, facetSharp, time) +
        e.xxx * crystalSDF(p + e.xxx, facetSharp, time);
    float len = length(n);
    return len > 1e-6 ? n / len : vec3(0.0, 1.0, 0.0);
}

// ---------------------------------------------------------------------------
// Procedural neon environment — polar spectral rings.
// No uImage dependency: gem looks premium even with no artwork loaded.
// ---------------------------------------------------------------------------
vec3 procEnv(vec3 dir, float time, float spectralCentroid) {
    // Spherical coords of the direction
    float phi   = atan2_(dir.z, dir.x);            // -pi..pi azimuth
    float theta = asin(clamp(dir.y, -1.0, 1.0));   // -pi/2..pi/2 elevation
    float su = phi / 6.28318530 + 0.5;             // 0..1
    float sv = theta / 3.14159265 + 0.5;           // 0..1

    vec3 env = vec3(0.0);

    // Several neon polar bands (ring layers at discrete elevation bands)
    for (int b = 0; b < 6; ++b) {
        float fb = float(b);
        float bandCenter = 0.15 + fb * 0.12;
        float bandV      = abs(sv - bandCenter);
        float ringMask   = 1.0 - smoothstep(0.012, 0.055, bandV);

        // Hue rotates around azimuth; animated slowly
        float t  = su + fb * 0.17 + time * (0.02 + fb * 0.007);
        vec3 c = palCity(t + 0.12 * spectralCentroid);
        c = c * c * 2.6; // deepen saturation — prismatic, never milky

        // Brightness varies by azimuth band for a spectrum effect
        float azBand = fract(su * 8.0 + fb * 0.3);
        float bright = 0.6 + 0.6 * sin(azBand * 6.28318530);

        env += c * ringMask * bright * 1.6;
    }

    // Dark baseline — the gem floats in black
    return env;
}

// ---------------------------------------------------------------------------
// Chromatic dispersion refraction (3 separate IOR channels → prism effect)
// ---------------------------------------------------------------------------
vec3 envRefractChromatic(vec3 rd, vec3 n, float iorBase, float dispersion,
                         float time, float spectralCentroid) {
    float dsp = dispersion * 0.30;
    float iorR = iorBase - dsp;
    float iorG = iorBase;
    float iorB = iorBase + dsp;

    vec3 rr = refract(rd, n, 1.0 / iorR);
    vec3 rg = refract(rd, n, 1.0 / iorG);
    vec3 rb = refract(rd, n, 1.0 / iorB);

    // Total internal reflection fallback
    if (length(rr) < 0.01) rr = reflect(rd, n);
    if (length(rg) < 0.01) rg = reflect(rd, n);
    if (length(rb) < 0.01) rb = reflect(rd, n);

    return vec3(
        procEnv(rr, time, spectralCentroid).r,
        procEnv(rg, time, spectralCentroid).g,
        procEnv(rb, time, spectralCentroid).b
    );
}

// ---------------------------------------------------------------------------
// Crack-flash pattern: high-freq sinusoidal thresholded to thin lines.
// Shard count driven by treble / onset — more treble = more fracture lines.
// ---------------------------------------------------------------------------
float crackPattern(vec3 p, float shardFreq) {
    float v = 0.0;
    v += abs(sin(p.x * 14.3 + p.y * 9.7));
    v += abs(sin(p.y * 11.1 + p.z * 17.3));
    v += abs(sin(p.z * 13.7 + p.x * 8.1));
    // Second harmonic for denser shards at high treble
    v += 0.4 * abs(sin(p.x * shardFreq + p.z * 21.1));
    v /= 3.4;
    return 1.0 - smoothstep(0.0, 0.20, v);
}

vec3 visual(vec2 uv, VisualUniforms u) {
    // ------------------------------------------------------------------
    // Params
    // ------------------------------------------------------------------
    float facetSharp  = mix(2.0, 12.0, u.param0);          // cutting planes
    float dispersion  = mix(0.05, 1.0, u.param1);          // IOR spread
    float fracturePow = u.param2;                          // crack intensity

    // Bass drives gem scale and camera push
    float gemScale    = 1.0 + 0.18 * u.bassImpact;
    float camPush     = 0.22 * u.bassImpact;

    // Mid drives internal wobble on the SDF cutplanes (via time offset)
    float wobbleTime  = u.time + u.mid * 0.8;

    // Treble + onset -> fracture brightness and shard count
    float shardFreq   = mix(14.0, 34.0, u.treble);         // more treble = denser cracks
    float transient   = saturate(u.onset * 3.0 + u.beat * 0.5);
    float fractureAmt = fracturePow * transient;

    // beatPhase -> slow base rotation on top of the normal yaw
    float beatRot     = u.beatPhase * 0.4;

    // spectralCentroid -> tint hue bias
    float tintT       = u.spectralCentroid;

    // ------------------------------------------------------------------
    // Ray setup — orbiting camera
    // ------------------------------------------------------------------
    vec2 sp = motionCentered(uv, u);
    vec3 ro = vec3(0.0, 0.0, 2.75 - camPush);
    vec3 rd = normalize(vec3(sp, -1.6));

    float yaw   = u.time * 0.18 + u.bass * 0.4 + beatRot;
    float pitch = u.time * 0.09 + u.mid  * 0.2;
    mat2 Ry = rot2(yaw);
    mat2 Rx = rot2(pitch);
    ro.xz = Ry * ro.xz;
    ro.yz = Rx * ro.yz;
    rd.xz = Ry * rd.xz;
    rd.yz = Rx * rd.yz;

    // ------------------------------------------------------------------
    // Raymarch (scale gem with bassImpact)
    // ------------------------------------------------------------------
    float t   = 0.0;
    float hit = 0.0;
    const float TMAX = 12.0;
    const float EPS  = 0.001;

    for (int i = 0; i < 72; ++i) {
        vec3 p = ro + rd * t;
        // Scale the SDF query point inversely -> gem grows on bassImpact
        float d = crystalSDF(p / gemScale, facetSharp, wobbleTime) * gemScale;
        if (d < EPS) { hit = 1.0; break; }
        t += d * 0.85;
        if (t > TMAX) break;
    }

    // ------------------------------------------------------------------
    // Background — deep black with faint accent vignette
    // ------------------------------------------------------------------
    float vig = 1.0 - smoothstep(0.5, 1.8, length(sp));
    vec3 col = vec3(0.0) + u.accent.rgb * 0.012 * vig;

    // ------------------------------------------------------------------
    // Gem shading
    // ------------------------------------------------------------------
    if (hit > 0.5) {
        vec3 p = ro + rd * t;
        vec3 pLocal = p / gemScale;
        vec3 n = crystalNormal(pLocal, facetSharp, wobbleTime);

        // Fresnel (Schlick)
        float cosI = max(-dot(rd, n), 0.0);
        float F0   = 0.05;   // crystal base reflectance
        float fres = F0 + (1.0 - F0) * pow(1.0 - cosI, 5.0);

        // --- Reflection: procedural env ---
        vec3 refl    = reflect(rd, n);
        vec3 reflCol = procEnv(refl, u.time, tintT);

        // --- Refraction: chromatic dispersion through procedural env ---
        float ior    = 1.52;   // crystal-ish
        vec3 refrCol = envRefractChromatic(rd, n, ior, dispersion, u.time, tintT);

        // --- Per-facet tint: each facet carries its own gem color, so the
        // interior refraction is prismatic rather than washed white ---
        vec3 fq = floor(n * 3.5 + 0.5);
        float facetHue = fract(dot(fq, vec3(0.157, 0.311, 0.427)));
        vec3 facetTint = palVisual(facetHue * 0.6 + tintT * 0.3 + 0.05, u);
        facetTint = facetTint * facetTint * 2.2;
        vec3 refrTinted = refrCol * mix(vec3(1.0), facetTint, 0.90) * 1.30;

        // --- Mix refract/reflect by Fresnel (grazing -> more reflect) ---
        vec3 gemCol = mix(refrTinted, reflCol * 0.85, fres);

        // spectralCentroid tint: bias hue toward neon palette
        vec3 tintCol = palVisual(tintT + 0.3, u);
        gemCol = mix(gemCol, gemCol * (0.7 + 0.5 * tintCol), 0.22);

        // Internal bass glow — colored ember in the heart of the gem, not a wash
        float heart = exp(-dot(pLocal, pLocal) * 2.2);
        float glow = (u.bassImpact * 0.55 + u.amplitude * 0.12) * heart;
        glow *= (1.0 - fres);   // strongest on the interior
        vec3 glowCol = palVisual(0.1 + tintT * 0.3, u);
        glowCol = glowCol * glowCol * 2.2;
        gemCol += glowCol * glow * 1.1;

        // Specular glitter: flat facets make dot(refl,L) constant across a
        // facet, so gate the flash with a positional sparkle mask — glitter,
        // never a facet-wide white sheet.
        float spec = pow(max(dot(refl, normalize(vec3(0.4, 0.8, 0.5))), 0.0), 160.0);
        spec += 0.6 * pow(max(dot(refl, normalize(vec3(-0.6, 0.5, 0.3))), 0.0), 120.0);
        float glitter = smoothstep(0.55, 0.92,
            vnoise(pLocal.xy * 26.0 + pLocal.z * 15.0 + u.time * 0.8));
        vec3 specCol = peakWhite(palVisual(facetHue + 0.2, u), 0.45);
        gemCol += specCol * spec * glitter * (2.2 + 2.5 * u.treble);

        // Rim/edge brightening from Fresnel: facet outlines pop
        float rim = pow(1.0 - cosI, 3.5);
        vec3 rimCol = palVisual(tintT + 0.5 + u.beatPhase * 0.2, u);
        gemCol += rimCol * rim * (0.45 + u.bassImpact * 1.1);

        // Beat brightening (overall pulse, kept moderate — bloom does the heavy lift)
        gemCol += u.accent.rgb * u.beat * 0.18;

        // --- Crack-flash on transients ---
        if (fractureAmt > 0.001) {
            float crack = crackPattern(pLocal * (4.0 + u.param0 * 6.0), shardFreq);
            // Crack lines: spectrum-colored shards, 5-8x HDR
            vec3 crackCol = palVisual(tintT + crack * 0.4 + u.onset * 0.2, u);
            crackCol = peakWhite(crackCol, 0.22 * u.onset);
            gemCol += crackCol * crack * fractureAmt * 4.5;
        }

        col = gemCol;
    }

    // ------------------------------------------------------------------
    // Feedback trails — light decay 0.72ish per spec
    // ------------------------------------------------------------------
    // Cap the trail: the gem rotates, and long max-blend trails accumulate
    // ghost facets into a pastel wash that kills the prismatic saturation.
    col = feedbackTrail(col, uv,
                        min(u.trailDecay, 0.55),
                        0.002 + 0.006 * u.bassImpact,     // subtle zoom
                        0.001 * sin(u.time * 0.5));       // very slow swirl

    return col; // LINEAR HDR — post chain tonemaps
}
