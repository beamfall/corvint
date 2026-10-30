module.exports = async () => require('node:fs').writeFileSync(process.env.CORVINT_FIXTURE_MARKER + '.teardown', 'observed');
