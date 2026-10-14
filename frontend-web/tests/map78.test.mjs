import test from 'node:test';
import assert from 'node:assert/strict';
import {elapsedMinutes,deliveryTimes,locationState} from '../src/core/map78/times.mjs';
test('tempos ausentes ou eventos fora de ordem não viram zero',()=>{
 assert.equal(elapsedMinutes(null,'2026-10-14T15:00:00Z'),null);
 assert.equal(elapsedMinutes('inválido','2026-10-14T15:00:00Z'),null);
 assert.equal(elapsedMinutes('2026-10-14T15:01:00Z','2026-10-14T15:00:00Z'),null);
});
test('espera deslocamento destino e total seguem os eventos registrados',()=>{
 assert.deepEqual(deliveryTimes({queued_at:'2026-10-14T14:00:00Z',picked_up_at:'2026-10-14T14:12:00Z',arrived_at:'2026-10-14T14:32:00Z',completed_at:'2026-10-14T14:37:00Z'}),{waiting:12,travel:20,atDestination:5,total:37});
});
test('posição antiga é sinalizada sem fingir rastreamento ao vivo',()=>{
 const now=Date.parse('2026-10-14T15:00:00Z');
 assert.equal(locationState(null,now),'unknown');
 assert.equal(locationState({recorded_at:'2026-10-14T14:59:00Z'},now),'recent');
 assert.equal(locationState({recorded_at:'2026-10-14T14:50:00Z'},now),'stale');
 assert.equal(locationState({recorded_at:'2026-10-14T16:00:00Z'},now),'unknown');
});
