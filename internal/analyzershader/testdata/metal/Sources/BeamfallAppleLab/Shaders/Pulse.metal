#include "VisualShared.h"

// Pulse — Lite — "Black-room Solar Core"
// A white-cyan solar disk breathes slowly; bass punches the radius outward; 3-5
// animated prismatic corona rings precess around it with fbm-turbulenced edges;
// treble ignites fine spark speckles; onset fires a white rim flash.
// Outputs LINEAR HDR — no tonemap here; the shared post chain handles it.
//
// param0 = Glow         (corona brightness, 0..1, default 0.70)
// param1 = Impact       (bass-impact sensitivity, 0..1, default 0.65)
// param2 = Corona Count (3-5 rings, 0..1, default 0.45)
fragment float4 frag_pulse(VOut in [[stage_in]],
                           constant VisualUniforms& u [[buffer(0)]],
                           texture2d<float> uImage  [[texture(0)]],
                           texture2d<float> uPrev   [[texture(1)]],
                           sampler uSamp [[sampler(0)]]) {

    float2 p = motionCentered(in.uv, u);
    float  r = length(p);
    float  a = atan2(p.y, p.x);           // -pi .. pi

    // ---- param decode ----
    float glowAmt    = mix(0.25, 1.40, u.param0);   // corona HDR brightness scale
    float impactAmt  = mix(0.06, 0.26, u.param1);   // max radius kick from bassImpact
    int   numCorona  = int(mix(3.0, 5.0, u.param2) + 0.5); // 3-5 rings

    // ---- core radius: breathes + bass punch ----
    float breathe = 0.007 * sin(u.time * 1.1) + 0.005 * sin(u.time * 2.3 + 0.8);
    float coreR   = 0.12 + 0.06 * u.amplitude + breathe
                  + impactAmt * u.bassImpact;        // snaps on kick, decays fast

    // ---- solar core disk — white/cyan, HDR 3-5x ----
    float3 col = float3(0.0);
    {
        // tight hot disk: smoothstep inside-out, clamp guard on r
        float rSafe   = max(r, 1e-4);
        float inner   = smoothstep(coreR, coreR * 0.40, rSafe);          // inside fill
        float rimW    = 0.012 + 0.006 * u.amplitude;
        float rim     = aaLine(rSafe - coreR, rimW);                     // bright edge

        // core color: hot orange rim with magenta/cyan bloom, white only at peak.
        float3 coreCol = mix(float3(1.00, 0.20, 0.02), float3(1.00, 0.05, 0.68),
                             smoothstep(0.2, 1.0, inner));
        coreCol = mix(coreCol, float3(0.00, 0.78, 1.00), rim * 0.35);
        coreCol = peakWhite(coreCol, u.bassImpact * inner * 0.08);
        float  exposure = 0.85 + 1.25 * u.level + 0.80 * u.bassImpact;
        col += coreCol * (inner * 2.1 + rim * 3.4) * exposure;

        // onset: white rim flash
        float onsetFlash = u.onset * 5.0 * aaLine(rSafe - (coreR + 0.018), 0.008);
        col += float3(1.8, 0.48, 0.08) * onsetFlash;
    }

    // ---- prismatic corona rings ----
    // Each ring is at a different radius; fbm disturbs the boundary; palNight rotates
    // through magenta/gold/teal as beatPhase advances.
    {
        // base radii: first ring just outside core, spaced outward
        float spacing = 0.065 + 0.020 * u.amplitude;

        for (int i = 0; i < numCorona; ++i) {
            float fi      = float(i);
            // ring centre radius (outer rings grow with bass push too, less so)
            float ringR   = coreR + (fi + 1.0) * spacing
                          + impactAmt * u.bassImpact * (1.0 - fi * 0.18);

            // fbm-warp the effective radius to give turbulent corona edge
            float noiseT  = a * (0.8 + fi * 0.15);           // angular noise coordinate
            float noiseR  = r + fi * 0.07;                    // radial noise layer
            float turb    = fbm(float2(noiseT, noiseR + u.time * (0.08 + fi * 0.03)));
            float turbAmt = 0.018 + 0.010 * u.mid;
            float d       = r - ringR - turb * turbAmt;       // signed distance to ring

            // ring width: thinner farther out, glows with amplitude
            float halfW   = 0.006 + 0.004 * u.amplitude - fi * 0.0008;
            halfW         = max(halfW, 0.002);
            float mask    = aaLine(d, halfW);

            // color: palNight keyed by ring index + precessing beatPhase + angle hue tilt
            float hue     = fi * 0.20 + u.beatPhase * 0.35
                          + a / 6.2831853 * 0.12 + 0.08 * u.spectralCentroid;
            float3 ringCol = palVisual(hue, u);
            ringCol        = accentize(ringCol, u.accent, 0.12);

            // HDR brightness: outer rings dimmer; bassImpact pumps all of them
            float bright = glowAmt * (1.5 - fi * 0.22) * (0.9 + u.amplitude * 0.5)
                         + 0.8 * u.bassImpact;
            bright = max(bright, 0.0);

            col += ringCol * mask * bright;
        }
    }

    // ---- treble spark speckles ----
    // Deterministic analytic sparks scattered in the corona zone
    {
        float trebleE = u.treble + u.flux * 0.5;
        for (int i = 0; i < 32; ++i) {
            float fi  = float(i);
            float2 h  = hash22(float2(fi, 3.7));
            // spark sits in corona band: [coreR + 0.03, coreR + numCorona*spacing + 0.06]
            float bandOuter = coreR + float(numCorona) * (0.065 + 0.020 * u.amplitude) + 0.06;
            float sr  = mix(coreR + 0.03, bandOuter, h.x);
            float sa  = h.y * 6.2831853 + u.time * (0.4 + 0.6 * h.x) + u.beatPhase * 1.8;
            float2 sp = float2(cos(sa), sin(sa)) * sr;
            float  dd = length(p - sp);
            // tiny punctual spark
            float spark = 5e-5 / (dd * dd + 5e-5);
            spark = min(spark, 3.0);
            // flicker with treble
            float flicker = hash11(fi + floor(u.time * 18.0)) * 0.5 + 0.5;
            float3 scol = palVisual(h.x + 0.05 * u.spectralCentroid, u);
            scol = peakWhite(scol, u.onset * trebleE * 0.18);
            col += scol * spark * trebleE * flicker * 4.5;
        }
    }

    // ---- black center guard — guarantee the core interior stays bright, surrounding
    //      frame stays near-zero. Kill anything beyond outer corona radius ----
    float outerFade = coreR + float(numCorona) * 0.09 + 0.12;
    col *= 1.0 - smoothstep(outerFade, outerFade + 0.18, r);

    // ---- stable feedback trails: small zoom so core afterimage glows outward ----
    float trailZoom = 0.008 + 0.014 * u.bassImpact;
    float trailRot  = 0.004 * u.beat + 0.001 * u.time * 0.01; // very gentle swirl
    col = feedbackTrail(col, uPrev, uSamp, in.uv,
                        u.trailDecay, trailZoom, trailRot);

    return float4(col, 1.0);   // LINEAR HDR — DO NOT tonemap here
}
