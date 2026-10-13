import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {summarizeTodaySales,averageOwnerTicket} from '../src/core/local/ownerSummary.mjs';
import {areaForRoute} from '../src/core/local/accessModel.mjs';
test('recorte de hoje usa São Paulo, exclui cancelamento e arredonda ticket em centavos',()=>{
 const rows=[{sale_id:'a',status:'committed',total_cents:1000,committed_at:'2026-10-13T02:00:00Z'},{sale_id:'b',status:'committed',total_cents:1001,committed_at:'2026-10-13T03:00:00Z'},{sale_id:'c',status:'cancelled',total_cents:9000,committed_at:'2026-10-13T04:00:00Z'}];
 const summary=summarizeTodaySales(rows,new Date('2026-10-13T15:00:00Z'));
 assert.equal(summary.total_cents,1001);assert.equal(summary.completed,1);assert.equal(summary.cancelled,1);assert.equal(averageOwnerTicket(summary),1001);
 assert.equal(averageOwnerTicket({...summary,total_cents:2001,completed:2}),1001);
 assert.equal(averageOwnerTicket(summarizeTodaySales([],new Date('2026-10-13T15:00:00Z'))),null);
 assert.throws(()=>summarizeTodaySales([],new Date('invalid')));
 assert.throws(()=>averageOwnerTicket({...summary,completed:-1}));
});
test('rota executiva e alias mantêm permissão exclusiva de portfolio',()=>{
 assert.equal(areaForRoute('/local/overview'),'portfolio');assert.equal(areaForRoute('/local/owner'),'portfolio');
});
test('seis seções e indisponibilidade estão traduzidas sem chamar a tela de dono',()=>{
 for(const lang of ['pt-BR','en','es','fr','zh-CN']){
  const copy=JSON.parse(readFileSync(new URL(`../src/i18n/owner/${lang}.json`,import.meta.url)));
  for(const key of ['today','money','result','attention','approvals','operation','incomplete','approvalNote','alertNote','financeNote'])assert.ok(copy[key]?.trim());
  assert.equal(copy.title,copy.portfolio);
 }
 const copy=JSON.parse(readFileSync(new URL('../src/i18n/owner/pt-BR.json',import.meta.url)));assert.equal(copy.title,'Visão executiva');
});
