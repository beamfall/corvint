// SPDX-License-Identifier: AGPL-3.0-or-later
import { VERSION, type ExtensionAPI } from '@earendil-works/pi-coding-agent';
import { createRunner } from './runtime.js';
import register from './extension.js';

export default function(pi: ExtensionAPI) {
 register(pi, {runner:createRunner(), version:VERSION});
}
