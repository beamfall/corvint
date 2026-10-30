#include "VisualShared.h"

// Bloom — Lite / aurora ink.
// Slow aurora ink clouds with bright audio-reactive light leaks and electric veins
// over BLACK negative space. Large overlapping color fields drift diagonally; bass opens
// bright portals; treble draws electric vein filaments.
//
// param0 = Flow         (0..1): cloud drift speed
// param1 = Color Spread (0..1): palette spread across clouds
// param2 = Veins        (0..1): treble vein intensity
//
// HDR out — NO tonemap(). Light leaks 3x, vein hits 4x.
// Feedback via feedbackTrail() only (low decay, slow advection).

fragment float4 frag_bloom(VOut in [[stage_in]],
                           constant VisualUniforms& u [[buffer(0)]],
                           texture2d<float> uImage   [[texture(0)]],
                           texture2d<float> uPrev    [[texture(1)]],
                           sampler          uSamp    [[sampler(0)]]) {

    // -------------------------------------------------------------------------
    // Params
    // -------------------------------------------------------------------------
    float flowSpeed   = mix(0.04, 0.28, u.param0);      // cloud advection speed
    float colorSpread = mix(0.10, 0.90, u.param1);      // palette t-range
    float veinAmt     = mix(0.00, 1.00, u.param2);      // treble vein strength

    // -------------------------------------------------------------------------
    // Aspect-correct centered coords
    // -------------------------------------------------------------------------
    float2 p = motionCentered(in.uv, u);           // ~[-ar,ar] x [-1,1]
    float2 uv = in.uv;                                  // 0..1, for feedback

    // -------------------------------------------------------------------------
    // Audio drives
    // -------------------------------------------------------------------------
    float exposure    = 0.6 + 1.4 * u.level;            // slow AGC loudness
    float warpAmp     = 0.25 + 1.20 * u.amplitude;      // domain warp strength
    float hueTemp     = u.spectralCentroid;              // 0=cool, 1=warm
    float veinPower   = u.treble * veinAmt;             // electric veins
    float leakPower   = u.bassImpact;                   // circular light-leak

    // -------------------------------------------------------------------------
    // FBM domain warp — two layers of warped coords
    // Layer A: large diagonal aurora clouds
    // -------------------------------------------------------------------------
    float t = u.time;

    // Primary warp seed — diagonal slow drift
    float2 seed0 = p * 0.55 + float2(t * flowSpeed * 0.31, t * flowSpeed * 0.19);
    // First warp: fbm offset to distort the cloud coords
    float2 warpOff0 = float2(
        fbm(seed0 + float2(0.00, 0.00)),
        fbm(seed0 + float2(5.23, 3.78))
    ) - 0.5;
    float2 warped0 = seed0 + warpOff0 * warpAmp * 0.9;

    // Second warp pass: warp on top of the first
    float2 warpOff1 = float2(
        fbm(warped0 + float2(1.77, 9.20) + t * 0.04),
        fbm(warped0 + float2(8.31, 2.55) + t * 0.03)
    ) - 0.5;
    float2 warped1 = warped0 + warpOff1 * warpAmp * 0.5;

    // Primary cloud field
    float cloudA = fbm(warped1);

    // -------------------------------------------------------------------------
    // Layer B: secondary cloud at a different scale/angle — gives depth
    // -------------------------------------------------------------------------
    float2 seed1 = rot2(0.523) * (p * 0.38) + float2(-t * flowSpeed * 0.17, t * flowSpeed * 0.22);
    float2 wB = float2(
        fbm(seed1 + float2(3.1, 1.4)),
        fbm(seed1 + float2(6.5, 7.9))
    ) - 0.5;
    float cloudB = fbm(seed1 + wB * warpAmp * 0.7);

    // Combine: use max so both contribute and blacks stay black
    float cloud = max(cloudA, cloudB);

    // -------------------------------------------------------------------------
    // CRITICAL: gate the cloud strictly. Only values well above mid-grey become
    // visible; everything else stays black. This prevents mid-grey haze.
    // -------------------------------------------------------------------------
    float gate = smoothstep(0.66, 0.88, cloud);   // strictly dark below 0.66
    if (gate < 0.005) {
        return float4(0.0, 0.0, 0.0, 1.0);
    }

    // -------------------------------------------------------------------------
    // Color — palNight with spectralCentroid hue temperature + param1 spread
    // -------------------------------------------------------------------------
    // t-param: drive from the cloud field itself + drift + hue temperature
    float palT = fract(cloud * colorSpread * 1.4
                       + 0.18 * hueTemp           // centroid shifts hue warm/cool
                       + 0.06 * t
                       + 0.04 * u.mid);
    float3 col = palVisual(palT, u);
    col = accentize(col, u.accent, 0.15);

    // -------------------------------------------------------------------------
    // Apply gate and exposure — keep base cloud luminance HDR-capped at ~1.5x
    // to preserve bloom bloom-feed without washing blacks
    // -------------------------------------------------------------------------
    col = col * gate * exposure * 0.82;

    // -------------------------------------------------------------------------
    // Circular light-leak portal — bassImpact snaps to 1, decays fast.
    // Radial flash from screen center, bright portal ring, 3x HDR.
    // -------------------------------------------------------------------------
    float r = length(p);
    if (leakPower > 0.01) {
        // Radial falloff: bright ring at r~0.3 with soft edges
        float ringR   = 0.28 + 0.12 * leakPower;
        float ringW   = 0.08 + 0.10 * leakPower;
        float ringMask = exp(-pow((r - ringR) / max(ringW, 0.001), 2.0) * 4.0);

        // Color of the leak: palNight near the "impact" hue temperature
        float3 leakCol = palVisual(0.10 + 0.45 * hueTemp, u);
        leakCol = accentize(leakCol, u.accent, 0.2);

        // Portal interior radial glow
        float interior = exp(-r * (2.5 - 1.5 * leakPower));

        col += leakCol * (ringMask * 1.15 + interior * 0.45) * leakPower;
    }

    // -------------------------------------------------------------------------
    // Electric veins — ridged fbm at higher frequency; treble-reactive
    // Keep veins thin and bright (4x HDR on hits) against the dark field
    // -------------------------------------------------------------------------
    if (veinPower > 0.005) {
        // Domain warp for veins: use the first warp layer at higher freq
        float2 veinCoord = warped0 * 2.2 + float2(t * 0.08, -t * 0.05);
        float vein = fbmRidged(veinCoord + float2(4.1, 2.7));

        // Hard threshold: only the crest of the ridged fbm becomes a vein
        float veinLine = smoothstep(0.65, 0.85, vein);

        // Vein color: cool cyan/white from treble end of palette
        float3 veinCol = palVisual(0.72 + 0.18 * hueTemp + 0.05 * sin(t * 1.3), u);
        veinCol = peakWhite(veinCol, veinLine * u.treble * 0.20);

        // treble drives brightness, veins are sharp and brief
        col += veinCol * veinLine * veinPower * (1.0 + 1.8 * u.treble) * 2.2;
    }

    // -------------------------------------------------------------------------
    // Onset flash: brief white-teal rim at screen edges on transient
    // -------------------------------------------------------------------------
    if (u.onset > 0.05) {
        float edgeDist = 1.0 - smoothstep(0.6, 1.4, r);
        float onsetRim = (1.0 - edgeDist) * u.onset;
        col += float3(0.4, 1.0, 1.2) * onsetRim * 1.2;
    }

    // -------------------------------------------------------------------------
    // Feedback trail — slow advection, subtle zoom, slight rotation
    // Low decay to keep black regions black; max-blend prevents additive blowout
    // -------------------------------------------------------------------------
    float trailZoom = 0.003 + 0.008 * u.amplitude;
    float trailRot  = 0.002 + 0.003 * u.beat;
    col = feedbackTrail(col, uPrev, uSamp, uv, u.trailDecay, trailZoom, trailRot);

    return float4(col, 1.0);   // LINEAR HDR — post chain tonemaps
}
