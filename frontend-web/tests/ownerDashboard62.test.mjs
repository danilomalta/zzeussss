import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {summarizeOwnerSales,formatOwnerMoney} from '../src/core/local/ownerSummary.mjs';
import {allowedAreas,areaForRoute} from '../src/core/local/accessModel.mjs';
const sale=(id,total,status='committed',date='2026-10-13T01:00:00Z')=>({sale_id:id,total_cents:total,status,committed_at:date});
test('painel agrega somente página carregada, exclui cancelamentos e usa dia de São Paulo',()=>{
 const summary=summarizeOwnerSales([sale('1',1001),sale('2',2002),sale('3',9999,'cancelled')]);
 assert.deepEqual(summary,{total_cents:3003,completed:2,cancelled:1,loaded:3,days:[{date:'2026-10-12',cents:3003}]});
 assert.deepEqual(summarizeOwnerSales([]),{total_cents:0,completed:0,cancelled:0,loaded:0,days:[]});
});
test('respostas duplicadas, datas inválidas, valores inseguros e páginas excessivas são rejeitados',()=>{
 for(const rows of [[sale('a',1),sale('a',2)],[sale('a',-1)],[sale('a',1.5)],[sale('a',1,'other')],[sale('a',1,'committed','invalid')],[sale('a',Number.MAX_SAFE_INTEGER),sale('b',1)],Array.from({length:21},(_,i)=>sale(String(i),1))])assert.throws(()=>summarizeOwnerSales(rows));
});
test('formatação preserva centavos inclusive no limite seguro nas cinco localidades',()=>{
 assert.equal(formatOwnerMoney(3003),'R$ 30,03');
 for(const lang of ['pt-BR','en','es','fr','zh-CN']){
  const expected=new Intl.NumberFormat(lang,{style:'currency',currency:'BRL'}).formatToParts(90071992547409n).map(part=>part.type==='fraction'?'91':part.value).join('');
  assert.equal(formatOwnerMoney(Number.MAX_SAFE_INTEGER,lang),expected);
 }
 assert.throws(()=>formatOwnerMoney(-1));
});
test('painel do dono não é liberado a gerente, caixa ou funcionário',()=>{
 assert.equal(areaForRoute('/local/owner'),'portfolio');assert.ok(allowedAreas({role:'owner'}).includes('portfolio'));
 for(const role of ['manager','cashier','employee','accountant'])assert.ok(!allowedAreas({role,permissions:['sell','manage_staff','manage_production'],license:{state:'active',modules:['pos','staff','production']}}).includes('portfolio'));
});
test('cinco idiomas cobrem escopo parcial e finanças ainda indisponíveis',()=>{
 const locales=['pt-BR','en','es','fr','zh-CN'].map(lang=>JSON.parse(readFileSync(new URL(`../src/i18n/owner/${lang}.json`,import.meta.url))));
 for(const locale of locales){assert.deepEqual(Object.keys(locale),Object.keys(locales[0]));for(const value of Object.values(locale))assert.ok(value.trim());assert.ok(locale.scope&&locale.dataNote&&locale.pending&&locale.footer);}
});
