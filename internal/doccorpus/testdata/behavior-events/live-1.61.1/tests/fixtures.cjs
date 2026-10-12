'use strict';
const { test: base } = require('playwright/test');
module.exports = { test: require('/private/tmp/claude-501/w22-835/internal/doccorpus/behavior-events.cjs').extend(base, { from: __dirname }) };
