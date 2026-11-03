import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import ts from 'typescript';
const text=await readFile(new URL('../src/core/fleetMaintenance/words.ts',import.meta.url),'utf8');
const code=ts.transpileModule(text+'\nexport const keysForTest=Object.keys(rows);',{compilerOptions:{module:ts.ModuleKind.ESNext,target:ts.ScriptTarget.ES2022}}).outputText;
const {maintenanceWords,keysForTest}=await import('data:text/javascript;base64,'+Buffer.from(code).toString('base64'));
test('all five maintenance dictionaries cover every key with nonempty copy',()=>{assert.ok(keysForTest.length>80);for(const locale of ['pt-BR','en','es','fr','zh-CN'])for(const key of keysForTest){const value=maintenanceWords(locale)(key);assert.equal(typeof value,'string');assert.ok(value.trim());assert.notEqual(value,key,locale+': '+key);}});
test('remote dispatch and depreciation are explicitly qualified in all five languages',()=>{for(const [locale,local,market]of [['pt-BR',/Nenhum chamado/,/Não é cotação/],['en',/No request/,/Not a resale/],['es',/No se envían/,/No es cotización/],['fr',/Aucune demande/,/Ni cotation/],['zh-CN',/不会向其他设备/,/不是转售报价/]]){assert.match(maintenanceWords(locale)('localOnly'),local);assert.match(maintenanceWords(locale)('depreciationHelp'),market);}});
