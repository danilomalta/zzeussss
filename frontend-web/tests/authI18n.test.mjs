import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {createInstance} from 'i18next';
import {formatMoney} from '../src/modules/auth/pages/registrationModel.mjs';
const locales=['pt-BR','en','es','fr','zh-CN'];
const dictionaries=Object.fromEntries(locales.map(locale=>[locale,JSON.parse(readFileSync(new URL('../src/i18n/locales/'+locale+'.json',import.meta.url),'utf8'))]));

test('cinco idiomas têm todas as mesmas chaves e revisão nativa registrada',()=>{
 const keys=Object.keys(dictionaries['pt-BR']).sort();
 for(const locale of locales){
  assert.deepEqual(Object.keys(dictionaries[locale]).sort(),keys);
  assert.equal(dictionaries[locale]._meta.nativeReviewRequired,locale!=='pt-BR');
  for(const key of keys.filter(key=>key!=='_meta')) assert.ok(typeof dictionaries[locale][key]==='string'&&dictionaries[locale][key].length>0,key);
 }
});
test('i18next troca os cinco idiomas sem perder interpolação e mantém fallback português',async()=>{
 const i18n=createInstance();
 await i18n.init({resources:Object.fromEntries(locales.map(locale=>[locale,{translation:dictionaries[locale]}])),lng:'pt-BR',fallbackLng:'pt-BR',keySeparator:false});
 for(const locale of locales){await i18n.changeLanguage(locale);assert.equal(i18n.t('login.title'),dictionaries[locale]['login.title']);assert.match(i18n.t('register.userCount',{count:7}),/7/);assert.ok(!i18n.t('modules.savings',{price:'R$ 1.000,00'}).includes('{{'));}
 await i18n.changeLanguage('de');assert.equal(i18n.t('login.title'),dictionaries['pt-BR']['login.title']);
});
test('dinheiro mantém BRL e centavos nas cinco localidades',()=>{
 for(const locale of locales){const parts=new Intl.NumberFormat(locale,{style:'currency',currency:'BRL'}).formatToParts(350.01);assert.equal(parts.find(part=>part.type==='fraction')?.value,'01');assert.equal(formatMoney(35001,locale),parts.map(part=>part.value).join(''));}
});
test('estilos das telas solicitadas não contêm pigmentos ou classes azuis',()=>{
 const files=['../src/shared/brand/brand.css','../src/modules/auth/pages/registration.css','../src/shared/brand/BrandShell.tsx','../src/shared/brand/SignInCard.tsx','../src/shared/brand/ContactDrawer.tsx','../src/modules/auth/pages/Login.tsx','../src/modules/auth/pages/LocalLogin.tsx','../src/modules/auth/pages/Registration.tsx'];
 for(const file of files){const source=readFileSync(new URL(file,import.meta.url),'utf8');assert.ok(!/(?:blue|indigo|sky)-\d|#(?:2563eb|1d4ed8|1767f5|69a4ff|2880ff|1053c7|0c46ab|2f91ff|65adff)|rgb[a]?\(\s*(?:37[ ,]+99[ ,]+235|23[ ,]+103[ ,]+245|40[ ,]+116[ ,]+244)/i.test(source),file);}
});
