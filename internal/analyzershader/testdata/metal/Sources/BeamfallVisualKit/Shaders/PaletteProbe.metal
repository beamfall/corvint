// PaletteProbe — compute-kernel test fixture for
// VISKIT-PALETTE-8TO31-NOT-IN-SHIPPING-KIT-1. Not used by any shipped Visual;
// exists purely so Tests/BeamfallVisualKitTests/PaletteShippingConformanceTests.swift
// can execute palByIndex() ON THE GPU and read back real rendered colours,
// rather than trusting compilation success alone. See that test file for the
// assertions this feeds.
#include "VisualShared.h"
using namespace metal;

// One thread per palette index (0..31). Writes palByIndex(t, idx) verbatim.
kernel void palette_probe_by_index(device float4* out [[buffer(0)]],
                                    constant float& t [[buffer(1)]],
                                    uint idx [[thread_position_in_grid]]) {
    out[idx] = float4(palByIndex(t, int(idx)), 1.0);
}

// Independently-authored dispatch ladder mirroring the DOCUMENTED intended
// index->family mapping (beamfall-visual-shaders/PORTABLE-CONTRACT.md,
// BeamfallAppleLab/Shaders/VisualShared.h). Comparing this against
// palette_probe_by_index proves — by actual GPU execution, not by reading the
// source — that palByIndex(t, idx) really does render each index's INTENDED
// family, and gives future edits to palByIndex's ladder a second, independent
// ladder to drift against.
kernel void palette_probe_named(device float4* out [[buffer(0)]],
                                 constant float& t [[buffer(1)]],
                                 uint gid [[thread_position_in_grid]]) {
    int idx = int(gid);
    float3 c;
    switch (idx) {
        case 1:  c = palEmberIce(t); break;
        case 2:  c = palUltraviolet(t); break;
        case 3:  c = palDeepSea(t); break;
        case 4:  c = palSolarFlare(t); break;
        case 5:  c = palNeon(t); break;
        case 6:  c = palHeatIce(t); break;
        case 7:  c = palNight(t); break;
        case 8:  c = palMiami(t); break;
        case 9:  c = palAbyss(t); break;
        case 10: c = palSakura(t); break;
        case 11: c = palHorizon(t); break;
        case 12: c = palMoltenGold(t); break;
        case 13: c = palGlacier(t); break;
        case 14: c = palVelvet(t); break;
        case 15: c = palPatina(t); break;
        case 16: c = palJade(t); break;
        case 17: c = palAmber(t); break;
        case 18: c = palSmoke(t); break;
        case 19: c = palCandy(t); break;
        case 20: c = palInfrared(t); break;
        case 21: c = palLagoon(t); break;
        case 22: c = palDesert(t); break;
        case 23: c = palAbsinthe(t); break;
        case 24: c = palRoseGold(t); break;
        case 25: c = palDeepSpace(t); break;
        case 26: c = palTuscany(t); break;
        case 27: c = palNeonNoir(t); break;
        case 28: c = palPorcelain(t); break;
        case 29: c = palRust(t); break;
        case 30: c = palCherryCola(t); break;
        case 31: c = palPrismatic(t); break;
        default: c = palCity(t); break;
    }
    out[idx] = float4(c, 1.0);
}
