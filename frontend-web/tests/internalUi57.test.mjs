import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {allowedAreas} from '../src/core/local/accessModel.mjs';

const root=new URL('../src/i18n/locales/',import.meta.url);
const locales=['pt-BR','en','es','fr','zh-CN'].map(id=>JSON.parse(readFileSync(new URL(id+'.json',root),'utf8')));

test('a tela interna traduz estados reais, navegação e contexto de licença nas cinco línguas',()=>{
 const keys=['subscription.title','subscription.state.active','subscription.state.expired','subscription.state.not_installed',
  'subscription.state.unavailable','subscription.billingUnavailable','subscription.noModules','subscription.plans',
  'home.licenseNotice','home.needsLicense','workspace.expand','workspace.catalog','workspace.plans'];
 for(const dictionary of locales)for(const key of keys)assert.ok(dictionary[key]?.trim(),key);
});

test('a interface não libera PDV nem estoque de laboratório sem contrato assinado',()=>{
 const base={role:'owner',permissions:['view_catalog','manage_stock','sell'],license:{modules:['inventory','pos'],state:'unavailable'}};
 assert.deepEqual(allowedAreas(base),['home','plans','map']);
 for(const state of ['not_installed','invalid','clock_blocked'])
  assert.deepEqual(allowedAreas({...base,license:{...base.license,state}}),['home','plans','map']);
 const signed=allowedAreas({...base,license:{...base.license,state:'active'}});
 assert.ok(signed.includes('catalog')&&signed.includes('stock')&&signed.includes('pos'));
});
