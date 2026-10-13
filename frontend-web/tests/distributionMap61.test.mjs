import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {allowedAreas,areaForRoute} from '../src/core/local/accessModel.mjs';
const source=new URL('../src/',import.meta.url);
test('mapa limita acesso a dono, produção e logística sem liberar outras operações',()=>{
 assert.equal(areaForRoute('/local/map'),'map');
 assert.ok(!allowedAreas(null).includes('map'));
 assert.ok(!allowedAreas({role:'cashier',permissions:['sell'],license:{state:'active',modules:['pos']}}).includes('map'));
 for(const [module,permission] of [['production','manage_production'],['logistics','view_orders']]){
  const base={role:'manager',permissions:[permission],license:{state:'active',modules:[module]}};
  assert.ok(allowedAreas(base).includes('map'));
  assert.ok(!allowedAreas({...base,permissions:[]}).includes('map'));
  assert.ok(!allowedAreas({...base,license:{...base.license,state:'invalid'}}).includes('map'));
 }
 assert.ok(allowedAreas({role:'owner'}).includes('map'));
});
test('malha contém exatamente as 27 UFs, sem dados comerciais ou posições simuladas',()=>{
 const states=JSON.parse(readFileSync(new URL('modules/tenant/pages/brazilStates.json',source)));
 assert.equal(states.length,27);assert.equal(new Set(states.map(s=>s.uf)).size,27);
 for(const state of states){assert.ok(state.name);assert.match(state.path,/^M/);assert.ok(state.bounds.every(Number.isFinite));assert.ok(state.bounds[2]>0&&state.bounds[3]>0);assert.deepEqual(Object.keys(state).sort(),['bounds','name','path','uf']);}
});
test('cinco idiomas distinguem indicadores indisponíveis de zero e GPS ativo',()=>{
 const locales=['pt-BR','en','es','fr','zh-CN'].map(lang=>JSON.parse(readFileSync(new URL(`i18n/map/${lang}.json`,source))));
 for(const locale of locales){assert.deepEqual(Object.keys(locale),Object.keys(locales[0]));for(const value of Object.values(locale))assert.ok(typeof value==='string'&&value.trim());assert.ok(locale.unavailable&&locale.pending&&locale.noLocations);}
});
