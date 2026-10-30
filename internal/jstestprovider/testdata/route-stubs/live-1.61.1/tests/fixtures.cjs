'use strict';
const { test: base, expect } = require('playwright/test');
module.exports = { test: require('../../../../route-stubs.cjs').extend(base), expect };
