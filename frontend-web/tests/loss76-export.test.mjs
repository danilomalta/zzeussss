import test from 'node:test';import assert from 'node:assert/strict';import {readFileSync} from 'node:fs';
import {tableCSV} from '../src/core/loss76/export76.mjs';
import {analyze,defaultPolicy,examples} from '../src/core/loss76/engine.mjs';
const w=JSON.parse(readFileSync(new URL('../src/i18n/loss76/pt-BR.json',import.meta.url))),r=examples('2026-10-14')[0],a=analyze(r,defaultPolicy,'2026-10-14');
test('CSV contains table columns and values, not scenario JSON or context',()=>{const csv=tableCSV([{row:r,a}],w);assert.ok(csv.startsWith('\uFEFF"Produto / lote";'));assert.ok(csv.includes('"4";"7,5"'));assert.ok(csv.includes('"6,37"'));assert.ok(csv.includes(w.near));assert.ok(!csv.includes('titan-loss76-v1'));assert.ok(!csv.includes('tenant_id'));assert.equal(csv.trim().split('\r\n').length,2);});
test('CSV escapes quotes, delimiters, newlines and spreadsheet formulas',()=>{const csv=tableCSV([{row:{...r,name:'  =HYPERLINK("evil");\nname'},a}],w);assert.ok(csv.includes('"\x27  =HYPERLINK(""evil"");\nname"'));});
test('CSV localized numeric values retain no currency prefix or thousands separator',()=>{const csv=tableCSV([{row:{...r,cost_cents:123456},a}],w,'en');assert.ok(csv.includes('"1234.56"'));assert.ok(!csv.split('\r\n')[1].includes('R$'));});
