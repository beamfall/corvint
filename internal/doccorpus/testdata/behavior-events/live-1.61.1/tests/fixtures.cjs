'use strict';
const { test: base } = require('playwright/test');
module.exports = { test: require('../../../../behavior-events.cjs').extend(base, { from: __dirname }) };
