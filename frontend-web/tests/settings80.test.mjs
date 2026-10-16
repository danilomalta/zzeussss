import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {defaults80,validate80,settingsKey80,image80,palette80,contrast80} from '../src/core/settings80/model.mjs';
test('preferências usam quatro identidades de contexto e mudam ao trocar usuário ou loja',()=>{
 const context={tenant_id:'a',store_id:'b',identity_id:'c',device_id:'d'};
 assert.notEqual(settingsKey80(context),settingsKey80({...context,store_id:'x'}));
 assert.notEqual(settingsKey80(context),settingsKey80({...context,identity_id:'x'}));
 assert.throws(()=>settingsKey80({...context,tenant_id:''}));
});
test('credenciais e campos fiscais secretos não entram no armazenamento de preferências',()=>{
 const prefs=defaults80();
 assert.deepEqual(validate80(prefs),prefs);
 assert.throws(()=>validate80({...prefs,password:'secret'}));
 assert.throws(()=>validate80({...prefs,fiscal:{...prefs.fiscal,csc:'secret'}}));
 assert.throws(()=>validate80({...prefs,equipment:{...prefs.equipment,api_key:'secret'}}));
});
test('modo de foto exige imagem raster e não aceita SVG nem URL remota',()=>{
 assert.throws(()=>validate80({...defaults80(),mode:'photo'}));
 assert.equal(image80('data:image/svg+xml;base64,PHN2Zz4=',10000),false);
 assert.equal(image80('https://example.com/logo.png',10000),false);
 assert.equal(image80('data:image/png;base64,AA==',10000),true);
 assert.equal(image80('data:image/png;base64,AA==',4),false);
});
test('preferências antigas mantêm foto nítida e opção de desfoque é restrita',()=>{
 const old=defaults80();delete old.backgroundBlur;
 assert.equal(validate80(old).backgroundBlur,0);
 assert.equal(validate80({...defaults80(),backgroundBlur:3}).backgroundBlur,3);
 assert.throws(()=>validate80({...defaults80(),backgroundBlur:25}));
});
test('cor escolhida mantém contraste no foco e texto legível em ambos os temas',()=>{
 for(const color of ['#ffffff','#000000','#aabbcc','#ff0000','#ffff00','#111113']){
  for(const dark of [true,false]){
   const palette=palette80(color,dark);
   assert.ok(contrast80(palette.accent,dark?'#141417':'#ffffff')>=3);
   assert.ok(contrast80(palette.accent,palette.buttonText)>=4.5);
  }
 }
 assert.equal(palette80('var(--color)',false),null);
});
test('cinco idiomas têm todas as chaves e explicam a limitação do modo de foto',()=>{
 const locales=['pt-BR','en','es','fr','zh-CN'].map(locale=>JSON.parse(readFileSync(new URL('../src/i18n/settings80/'+locale+'.json',import.meta.url))));
 for(const locale of locales){assert.deepEqual(Object.keys(locale),Object.keys(locales[0]));assert.ok(locale.photoWarning&&locale.localOnly&&locale.fiscalHint);for(const value of Object.values(locale))assert.ok(typeof value==='string'&&value.trim());}
});
