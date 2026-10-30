const setupDependency = require('./setup-dependency.cjs');

module.exports = async () => {
  setupDependency(process.env.CORVINT_FIXTURE_MARKER);
  require('node:fs').writeFileSync(process.env.CORVINT_FIXTURE_MARKER + '.setup', 'observed');
};
