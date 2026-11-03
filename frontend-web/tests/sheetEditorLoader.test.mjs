import test from 'node:test';
import assert from 'node:assert/strict';
import {currentSheetEditor} from '../src/core/sheets70/editorLoader.mjs';
import {sheetLanguageKey} from '../src/core/sheets70/language.mjs';
const defer=()=>{let resolve,reject;const promise=new Promise((a,b)=>{resolve=a;reject=b;});return {promise,resolve,reject};};
const pack=async language=>({locale:sheetLanguageKey(language)});
test('inactive context starts neither editor nor language import',async()=>{
 let calls=0;assert.equal(await currentSheetEditor(()=> 'pt-BR',()=>false,()=>{calls++;},()=>{calls++;}),null);assert.equal(calls,0);
});
test('editor and language start concurrently and return the editor unchanged',async()=>{
 const editor=defer(),language=defer(),calls=[];const module={mount(){}};
 const result=currentSheetEditor(()=> 'pt-BR',()=>true,()=>{calls.push('editor');return editor.promise;},()=>{calls.push('language');return language.promise;});
 assert.deepEqual(calls,['editor','language']);language.resolve({locale:'ptBR'});editor.resolve(module);
 assert.deepEqual(await result,{editor:module,pack:{locale:'ptBR'}});
});
test('language changes after locale completes but before heavy editor completes',async()=>{
 const editor=defer();let language='pt-BR';const calls=[];
 const result=currentSheetEditor(()=>language,()=>true,()=>editor.promise,async l=>{calls.push(l);return pack(l);});
 await new Promise(resolve=>setImmediate(resolve));language='fr';editor.resolve({});
 assert.equal((await result).pack.locale,'frFR');assert.deepEqual(calls,['pt-BR','fr']);
});
test('navigation or logout while editor waits cannot mount',async()=>{
 const editor=defer();let current=true;const result=currentSheetEditor(()=> 'en',()=>current,()=>editor.promise,pack);
 current=false;editor.resolve({});assert.equal(await result,null);
});
test('context change while replacement locale waits cannot mount',async()=>{
 const editor=defer(),locale=defer();let language='pt-BR',current=true;
 const result=currentSheetEditor(()=>language,()=>current,()=>editor.promise,l=>l==='pt-BR'?pack(l):locale.promise);
 await new Promise(resolve=>setImmediate(resolve));language='es';editor.resolve({});
 await new Promise(resolve=>setImmediate(resolve));current=false;locale.resolve({locale:'esES'});assert.equal(await result,null);
});
test('second locale change during refresh uses final language',async()=>{
 const editor=defer(),locale=defer();let language='pt-BR';const calls=[];
 const result=currentSheetEditor(()=>language,()=>true,()=>editor.promise,l=>{calls.push(l);return l==='fr'?locale.promise:pack(l);});
 await new Promise(resolve=>setImmediate(resolve));language='fr';editor.resolve({});
 await new Promise(resolve=>setImmediate(resolve));language='zh-CN';locale.resolve({locale:'frFR'});
 assert.equal((await result).pack.locale,'zhCN');assert.deepEqual(calls,['pt-BR','fr','zh-CN']);
});
for(const failing of ['editor','locale'])test(`${failing} failure is reported without a usable editor`,async()=>{
 const error=new Error('isolated import failure');await assert.rejects(currentSheetEditor(()=> 'en',()=>true,()=>failing==='editor'?Promise.reject(error):Promise.resolve({}),l=>failing==='locale'?Promise.reject(error):pack(l)),e=>e===error);
});
for(const language of ['pt-BR','en','es','fr','zh-CN'])test(`editor matches ${language}`,async()=>{
 assert.equal((await currentSheetEditor(()=>language,()=>true,async()=>({}),pack)).pack.locale,sheetLanguageKey(language));
});
