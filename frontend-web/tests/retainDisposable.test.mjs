import test from 'node:test';import assert from 'node:assert/strict';
import {retainDisposable} from '../src/core/local/retainDisposable.mjs';
test('StrictMode release/reacquire does not dispose a live adapter',async()=>{let calls=0;const api={dispose(){calls++;}},release=retainDisposable(api);release();const again=retainDisposable(api);await Promise.resolve();assert.equal(calls,0);again();await Promise.resolve();assert.equal(calls,1);});
test('actual unmount disposes before later network continuations',async()=>{const api={alive:true,dispose(){this.alive=false;}},release=retainDisposable(api);release();await Promise.resolve();assert.equal(api.alive,false);});
test('shared adapter is preserved until final consumer releases',async()=>{let calls=0;const api={dispose(){calls++;}},a=retainDisposable(api),b=retainDisposable(api);a();await Promise.resolve();assert.equal(calls,0);b();await Promise.resolve();assert.equal(calls,1);});
test('repeated cleanup is idempotent and cannot double-dispose',async()=>{let calls=0;const api={dispose(){calls++;}},release=retainDisposable(api);release();release();await Promise.resolve();assert.equal(calls,1);assert.throws(()=>retainDisposable(api),/disposedResource/);});
test('absent optional adapters need no disposal',()=>{retainDisposable(null)();retainDisposable(undefined)();});
