import test from 'node:test';
import assert from 'node:assert/strict';
import {printFiscalReceipt} from '../src/core/local/fiscalPrint.mjs';
import {createCheckoutFiscalClient} from '../src/core/local/checkoutFiscal.mjs';
const html='<!doctype html><html><head></head><body>HOMOLOGAÇÃO - SEM VALOR FISCAL<img src="data:image/png;base64,QQ=="></body></html>';
function context(){return {token:'private-test-token',session:{tenant_id:'t',store_id:'s',identity_id:'u',device_id:'d'}};}
function popup(){const calls=[];return {calls,closed:false,opener:{},document:{images:[{async decode(){calls.push('qr');}}],open(){calls.push('open');},write(value){calls.push(value);},close(){calls.push('document-close');}},focus(){calls.push('focus');},print(){calls.push('print');},close(){this.closed=true;calls.push('close');}};}
test('prints only server HTML after QR decode, with no scripts, opener or token',async()=>{
 const w=popup();await printFiscalReceipt({open:()=>w,load:async()=>html});
 assert.equal(w.opener,null);const rendered=w.calls.find(v=>v.startsWith('<!doctype'));
 assert.ok(rendered.includes("script-src 'none'"));assert.ok(rendered.includes("default-src 'none'"));assert.ok(rendered.includes('SEM VALOR FISCAL'));
 assert.ok(w.calls.indexOf('qr')<w.calls.indexOf('print'));assert.equal(w.calls.filter(v=>v==='print').length,1);
 assert.ok(!rendered.includes('private-test-token'));
});
test('popup blocked does not load; missing archive closes without non-fiscal fallback',async()=>{
 let calls=0;await assert.rejects(printFiscalReceipt({open:()=>null,load:async()=>{calls++;return html;}}),/popupBlocked/);assert.equal(calls,0);
 const w=popup();await assert.rejects(printFiscalReceipt({open:()=>w,load:async()=>{throw Error('unavailable');}}),/unavailable/);assert.ok(w.closed);assert.ok(!w.calls.includes('print'));
});
test('QR failure or invalid document never prints',async()=>{
 for(const bad of ['', '<img src=x>', 'x'.repeat(2000001)]){const w=popup();await assert.rejects(printFiscalReceipt({open:()=>w,load:async()=>bad}));assert.ok(w.closed);assert.ok(!w.calls.includes('print'));}
 const w=popup();w.document.images[0].decode=async()=>{throw Error('qr');};await assert.rejects(printFiscalReceipt({open:()=>w,load:async()=>html}),/qr/);assert.ok(!w.calls.includes('print'));
});
test('fiscal download uses authenticated GET, selected paper and no customer browser storage',async()=>{
 const c=context();let request;
 const client=createCheckoutFiscalClient({getCurrent:()=>c,fetcher:async(url,options)=>{request={url,...options};return new Response(html,{headers:{'Content-Type':'text/html; charset=utf-8','X-Titan-Fiscal-Scope':'homologation'}});}}).bind(c);
 assert.equal(await client.document('sale',{width:58,timezone:'America/Sao_Paulo'}),html);
 assert.equal(request.method,'GET');assert.equal(request.cache,'no-store');assert.equal(request.credentials,'omit');assert.equal(request.headers.Authorization,'Bearer private-test-token');
 assert.match(request.url,/width_mm=58/);assert.ok(!request.url.includes(c.token));assert.equal(request.body,undefined);
 await assert.rejects(client.document('sale',{width:210,timezone:'UTC'}),/invalidReply/);
});
test('session change during download cannot expose the old document',async()=>{
 let c=context(),release;const client=createCheckoutFiscalClient({getCurrent:()=>c,fetcher:()=>new Promise(r=>release=r)}).bind(c);
 const pending=client.document('sale',{width:80,timezone:'UTC'});c={...c,token:'another-login'};
 release(new Response(html,{headers:{'Content-Type':'text/html','X-Titan-Fiscal-Scope':'homologation'}}));await assert.rejects(pending,/context/);
});
test('production, non-HTML and unsuccessful fiscal responses never print',async()=>{
 for(const response of [new Response(html,{headers:{'Content-Type':'text/html','X-Titan-Fiscal-Scope':'production'}}),new Response('{}',{headers:{'Content-Type':'application/json','X-Titan-Fiscal-Scope':'homologation'}}),new Response('missing',{status:409})]){
 const c=context();const client=createCheckoutFiscalClient({getCurrent:()=>c,fetcher:async()=>response}).bind(c);
 await assert.rejects(client.document('sale',{width:80,timezone:'UTC'}),/unavailable/);
 }
});

test('logout while the QR decodes closes the document and never prints',async()=>{
 const w=popup(),abort=new AbortController();let release;
 w.document.images[0].decode=()=>new Promise(r=>release=r);
 const pending=printFiscalReceipt({open:()=>w,load:async()=>html,signal:abort.signal});await new Promise(r=>setImmediate(r));
 abort.abort();release();await assert.rejects(pending,/aborted/);assert.ok(w.closed);assert.ok(!w.calls.includes('print'));
});

test('fiscal print controls and errors exist in five languages only for authorized notes',async()=>{
 const {createRequire}=await import('node:module'),fs=await import('node:fs');const require=createRequire(import.meta.url),ts=require('typescript'),React=require('react'),{renderToStaticMarkup}=require('react-dom/server');
 const source=fs.readFileSync(new URL('../src/modules/pos/pages/CheckoutFiscalStatus.tsx',import.meta.url),'utf8');
 const js=ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.CommonJS,jsx:ts.JsxEmit.ReactJSX,target:ts.ScriptTarget.ES2022}}).outputText;
 const mod={exports:{}};Function('require','exports','module',js)(name=>name.startsWith('react')?require(name):{},mod.exports,mod);
 for(const locale of ['pt-BR','en','es','fr','zh-CN']){
  const w=JSON.parse(fs.readFileSync(new URL(`../src/i18n/checkout96/${locale}.json`,import.meta.url)));
  const render=(state,extra={})=>renderToStaticMarkup(React.createElement(mod.exports.CheckoutFiscalStatusView,{view:{state},busy:false,unavailable:false,demo:false,connected:true,w,onRetry(){},onRefresh(){},onPrint(){},...extra}));
  const receipt=render('authorized');for(const key of ['checkoutFiscalPrint','checkoutFiscalPaper','checkoutFiscalPrintHelp'])assert.ok(receipt.includes(w[key]));
  assert.match(receipt,/>58 mm<\/option>/);assert.match(receipt,/>80 mm<\/option>/);
  for(const state of ['failed','pending','cancelled','cancellationFailed','none'])assert.ok(!render(state).includes('>'+w.checkoutFiscalPrint+'</button>'));
  assert.ok(!render('authorized',{demo:true}).includes('>'+w.checkoutFiscalPrint+'</button>'));
  assert.ok(render('authorized',{printError:true}).includes(w.checkoutFiscalPrintUnavailable));
 }
});
