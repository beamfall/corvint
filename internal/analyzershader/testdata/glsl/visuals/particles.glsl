// Particles — Standard — frequency + beat.
// Portable port of beamfall-apple-ui Shaders/Particles.metal (keep in sync).
//
// Beat-driven ember swarm orbiting spectral attractor rings. Bass ejects orange
// embers outward; treble produces cyan/white pin-sparks. Each particle is analytic
// (deterministic per frame) — no state stored across frames except via feedbackTrail.
//
// param0 Density      → particle count (maps to 56..80)
// param1 Burst        → outward explosion radius on bassImpact (0.2..0.5 scale)
// param2 Trail Length → feedbackTrail zoom factor (trail persistence)

vec3 visual(vec2 uv, VisualUniforms u) {
    vec2 p = motionCentered(uv, u);

    // ---- param interpretation ----
    int   K          = int(mix(56.0, 80.0, clamp(u.param0, 0.0, 1.0)));   // Density
    float burstScale = mix(0.2, 0.5, clamp(u.param1, 0.0, 1.0));          // Burst
    float trailZoom  = mix(0.003, 0.018, clamp(u.param2, 0.0, 1.0));      // Trail Length

    // ---- attractor ring radii: 3 spectral bands (bass/mid/treble) ----
    float bassRing   = 0.28 + 0.12 * bandRange(u,  0, 10);
    float midRing    = 0.52 + 0.10 * bandRange(u, 16, 40);
    float trebleRing = 0.76 + 0.08 * bandRange(u, 44, 64);

    // ---- bassImpact burst: burst pushes all particles radially outward ----
    float burstAmt = u.bassImpact * burstScale;

    vec3 col = vec3(0.0);

    for (int i = 0; i < 96; ++i) {
        if (i >= K) break;   // cap at param-driven count (≤96 by spec)

        float fi = float(i);

        // ---- deterministic per-particle seeds ----
        float h0 = hash11(fi * 1.3713);             // radius tier selector [0,1]
        float h1 = hash11(fi * 2.7183);             // initial angle [0,1]
        float h2 = hash11(fi * 0.5731);             // angular speed weight [0,1]
        float h3 = hash11(fi * 4.6692);             // wobble phase
        float h4 = hash11(fi * 3.1416);             // size scatter
        float h5 = hash11(fi * 7.3891);             // hue fine offset

        // ---- assign to an attractor ring ----
        // tier: 0=bass ring(~35%), 1=mid ring(~40%), 2=treble ring(~25%)
        float rBase;
        int   bandLo, bandHi;
        float tierFrac;
        if (h0 < 0.35) {
            rBase   = bassRing;
            bandLo  = 0; bandHi = 10;
            tierFrac = 0.0;
        } else if (h0 < 0.75) {
            rBase   = midRing;
            bandLo  = 16; bandHi = 40;
            tierFrac = 0.5;
        } else {
            rBase   = trebleRing;
            bandLo  = 44; bandHi = 64;
            tierFrac = 1.0;
        }

        // ---- band energy for this particle (within its tier's range) ----
        int bandIdx = bandLo + int(h5 * float(bandHi - bandLo));
        bandIdx = clamp(bandIdx, 0, NBANDS - 1);
        float energy = band(u, bandIdx);

        // ---- angular speed: bass orbits slower, treble faster ----
        float angSpeed = mix(0.10, 0.55, h2 + 0.25 * tierFrac);

        // ---- orbit angle ----
        float theta0 = h1 * 6.2831853;
        float angle  = theta0 + u.time * angSpeed;
        float wobble = 0.06 * sin(u.time * (0.4 + h3) + theta0 * 2.0);
        float ang    = angle + wobble;

        // ---- position on the attractor ring ----
        vec2 dir = vec2(cos(ang), sin(ang));
        vec2 pos = dir * rBase;

        // ---- bassImpact burst: eject all particles radially (bass ring most) ----
        float tierBurstMult = (h0 < 0.35) ? 1.4 : ((h0 < 0.75) ? 0.8 : 0.4);
        pos += dir * burstAmt * rBase * tierBurstMult;

        // ---- treble onset: add angular scatter for pin-sparks ----
        float onsetJitter = (tierFrac > 0.9) ? u.onset * 0.12 * (h4 - 0.5) : 0.0;
        pos += vec2(-sin(ang), cos(ang)) * onsetJitter;

        // ---- velocity direction (tangential) for streak ----
        vec2  vel = vec2(-sin(ang), cos(ang)) * angSpeed;
        float velLen = length(vel) + 1e-6;

        // ---- per-particle size: energy + tier ----
        float sizeBase = (tierFrac < 0.1)
                          ? mix(0.015, 0.060, energy)           // bass: fat embers
                          : ((tierFrac < 0.6)
                             ? mix(0.008, 0.035, energy)        // mid: medium
                             : mix(0.003, 0.012, energy));      // treble: pin-sparks
        float pSize = sizeBase * (1.0 + h4 * 0.4);
        float visibilityFade = smoothstep(0.003, 0.010, pSize) * smoothstep(1.4, 0.9, length(pos));

        // ---- Gaussian glow at pixel p ----
        vec2  delta   = p - pos;
        float distSq  = dot(delta, delta);
        float pSizeSq = max(pSize * pSize, 1e-8);

        float glow = exp(-distSq / max(pSizeSq * 0.72, 1e-8));
        glow *= smoothstep(0.012, 0.045, glow);

        // ---- streak along velocity (motion trail per-particle, not feedback) ----
        vec2  vhat  = vel / velLen;
        float along = dot(delta, vhat);
        float perp  = dot(delta, vec2(-vhat.y, vhat.x));
        float streakLen = pSize * (2.5 + 3.0 * energy + 2.0 * u.treble * tierFrac);
        float streakGlow = 0.0;
        if (along < 0.0) {  // only behind particle
            float perpG  = exp(-(perp * perp) / pSizeSq);
            float alongG = exp(-(along * along) / max(streakLen * streakLen, 1e-8));
            streakGlow   = perpG * alongG * 0.42;
        }

        // ---- brightness ----
        float bassBeat    = (tierFrac < 0.1) ? (1.0 + u.beat * 5.0 + u.bassImpact * 4.0) : (1.0 + u.beat * 1.6);
        float trebleFlash = (tierFrac > 0.9) ? (1.0 + u.onset * 3.5 + u.treble * 2.5) : 1.0;
        float brightness  = (0.4 + 1.6 * energy) * bassBeat * trebleFlash * visibilityFade;
        float hdrCap = (tierFrac < 0.1) ? 6.0 : 3.5;
        brightness = min(brightness, hdrCap);

        // ---- per-particle hue ----
        float hueT = float(bandIdx) / float(NBANDS - 1)
                   + 0.10 * u.spectralCentroid
                   + 0.05 * h5          // per-particle scatter
                   + 0.02 * u.time;     // gentle drift
        vec3 pCol = palVisual(fract(hueT), u);

        // Bass embers: tint orange/red
        if (tierFrac < 0.1) {
            vec3 emberCol = palVisual(0.02 + 0.10 * h5, u);  // orange-red range
            pCol = mix(pCol, emberCol, 0.70 + 0.25 * u.bassImpact);
        }
        // Treble sparks: tint cyan-white
        if (tierFrac > 0.9) {
            vec3 sparkCol = peakWhite(vec3(0.0, 0.78, 1.0), u.treble * u.onset * 0.28);
            pCol = mix(pCol, sparkCol, 0.55 + 0.35 * u.onset);
        }

        pCol = accentize(pCol, u.accent, 0.10);

        col += pCol * (glow + streakGlow) * brightness;
    }

    // ---- feedback trails: small zoom gives motion persistence ----
    float trailRot = 0.002 * u.beat + 0.001 * u.bassImpact;
    col = feedbackTrail(col, uv, u.trailDecay, trailZoom, trailRot);

    return col; // LINEAR HDR — post chain tonemaps
}
