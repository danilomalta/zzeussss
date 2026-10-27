"use strict";
const test=require('node:test'),assert=require('node:assert/strict'),path=require('node:path');
const {within,portableRelative}=require('../scripts/build-paths.cjs');
test('manifest names are portable for real Windows drive paths',()=>{assert.equal(portableRelative('C:\\build\\app','C:\\build\\app\\ui\\assets\\main.js',path.win32),'ui/assets/main.js');assert.equal(within('C:\\build\\app','C:\\build\\app-other\\key',path.win32),false);assert.equal(within('C:\\build\\app','D:\\outside',path.win32),false);});
test('build containment rejects root, parent and sibling escapes on both platforms',()=>{for(const p of [path.posix,path.win32]){const base=p.resolve('test-root','app');for(const candidate of [base,p.dirname(base),base+'-other'+p.sep+'entry.js'])assert.throws(()=>portableRelative(base,candidate,p));assert.equal(portableRelative(base,p.join(base,'ui','entry.js'),p),'ui/entry.js');}});

test('extensionless imports cannot hide module casing collisions on Windows',()=>{const {assertNoCaseCollisions}=require('../scripts/build-paths.cjs');assert.throws(()=>assertNoCaseCollisions(['src/FormGuard.tsx','src/formGuard.mjs']),/Ambiguous/);assert.doesNotThrow(()=>assertNoCaseCollisions(['src/ProductionFormGuard.tsx','src/formGuard.mjs','src/formGuard.d.mts']));});
