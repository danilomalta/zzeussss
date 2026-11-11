import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {onlineLogoutRequest,interpretOnlineLogout} from '../src/core/auth/onlineLogout.mjs';

test('online logout posts empty body to /auth/logout and keeps local logout separate',async()=>{
 const req=onlineLogoutRequest();
 assert.equal(req.method,'POST');
 assert.equal(req.url,'/auth/logout');
 assert.equal(req.data,undefined);
 assert.equal(req.transformRequest[0](),undefined);
 assert.equal(interpretOnlineLogout(204),'closed');
 assert.equal(interpretOnlineLogout(401),'closed');
 assert.equal(interpretOnlineLogout(503),'uncertain');
 assert.equal(interpretOnlineLogout(500),'uncertain');
 const store=await readFile(new URL('../src/core/auth/useAuthStore.ts',import.meta.url),'utf8');
 assert.match(store,/onlineLogoutRequest/);
 assert.doesNotMatch(store,/\/local\/v1\/logout/);
 const local=await readFile(new URL('../src/core/local/useLocalSession.ts',import.meta.url),'utf8');
 assert.match(local,/localClient\.logout/);
 const api=await readFile(new URL('../src/core/api/api.ts',import.meta.url),'utf8');
 assert.match(api,/forgetSession/);
 assert.doesNotMatch(api,/getState\(\)\.logout\(\)/);
 const dash=await readFile(new URL('../src/modules/tenant/pages/AdminDashboard.tsx',import.meta.url),'utf8');
 assert.match(dash,/account\.logoutUncertain/);
 assert.match(dash,/workspace\.logout/);
});

test('cinco idiomas têm texto de saída online incerta',async()=>{
 const keys=['workspace.logout','account.logoutUncertain'];
 for(const lang of ['pt-BR','en','es','fr','zh-CN']){
  const words=JSON.parse(await readFile(new URL(`../src/i18n/locales/${lang}.json`,import.meta.url)));
  for(const key of keys)assert.equal(typeof words[key],'string');
  assert.ok(words['account.logoutUncertain'].length>10);
 }
});
