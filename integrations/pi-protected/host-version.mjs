// SPDX-License-Identifier: Apache-2.0
// The Pi release this image embeds; build.mjs refuses any other SDK. The ordinary
// fallback declares it, and the Pi runtime refuses it unless Core admits it (PWV-V0-016).
export const PROTECTED_PI_VERSION='0.85.1';
