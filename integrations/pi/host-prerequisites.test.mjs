// SPDX-License-Identifier: Apache-2.0
import test from 'node:test';
import assert from 'node:assert/strict';
import {hostAvailable,hostFile} from './host-prerequisites.mjs';

test('PWV-V0-8300 absent optional tools and artifacts skip with their named cause',t=>{
 const prior=process.env.CORVINT_REQUIRE_PI;process.env.CORVINT_REQUIRE_PI='0';
 t.after(()=>{if(prior===undefined)delete process.env.CORVINT_REQUIRE_PI;else process.env.CORVINT_REQUIRE_PI=prior});
 const reasons=[],optional={skip:reason=>reasons.push(reason)};
 assert.equal(hostAvailable(optional,'/corvint-nonexistent-pi-host'),false);
 assert.equal(hostFile(optional,'/corvint-nonexistent-pi-image'),false);
 assert.match(reasons[0],/corvint-nonexistent-pi-host is not installed/);
 assert.match(reasons[1],/host artifact is not installed/);
});
test('PWV-V0-8300 installed failures cannot be mistaken for an optional missing host',()=>{
 assert.throws(()=>hostAvailable({skip(){assert.fail('must fail')}},process.execPath,['-e','process.exit(7)']),/prerequisite failed/);
});

test('PWV-V0-8300 required host qualification refuses an absent host',t=>{
 const prior=process.env.CORVINT_REQUIRE_PI;process.env.CORVINT_REQUIRE_PI='1';
 t.after(()=>{if(prior===undefined)delete process.env.CORVINT_REQUIRE_PI;else process.env.CORVINT_REQUIRE_PI=prior});
 assert.throws(()=>hostAvailable({skip(){assert.fail('required prerequisite must fail')}},'/corvint-nonexistent-pi-host'),/required Pi qualification prerequisite/);
});
