module.exports = marker => require('node:fs').writeFileSync(marker + '.setup-dependency', 'observed');
