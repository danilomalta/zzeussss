import {build} from 'esbuild';
import {createServer} from 'node:http';
import {spawn} from 'node:child_process';
import {mkdtemp,readFile,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join,dirname} from 'node:path';
import {fileURLToPath} from 'node:url';

// Every credential here is synthetic, passed over stdin to an isolated profile.
let raw='';for await(const c of process.stdin)raw+=c;
const b=JSON.parse(raw);raw='';
const dir=await mkdtemp(join(tmpdir(),'titan-p31c-screen-')),pending=new Map();let chrome,server,socket,seq=0;
const delay=ms=>new Promise(r=>setTimeout(r,ms));
const safeFetch=async path=>{const r=await fetch(new URL(path,b.url),{headers:{Authorization:'Bearer '+b.token}});if(!r.ok)throw Error('bootstrap API refused');return r.json();};
try{
 const session=await safeFetch('/local/v1/me'),catalog=await safeFetch('/local/v1/products?limit=50&offset=0'),product=catalog.items.find(p=>p.id===b.product);
 if(!product)throw Error('disposable product missing');
 const here=dirname(fileURLToPath(import.meta.url));
 await build({entryPoints:[join(here,'checkoutFocus.screen.tsx')],outfile:join(dir,'screen.js'),bundle:true,platform:'browser',format:'iife',jsx:'automatic',define:{'process.env.NODE_ENV':'"production"'},logLevel:'silent'});
 const bundle=await readFile(join(dir,'screen.js'));
 const requests=[];
 server=createServer(async(req,res)=>{
  try{
   if(req.url.startsWith('/local/v1/')){
    requests.push([req.method,req.url]);const body=[];for await(const c of req)body.push(c);
    const r=await fetch(new URL(req.url,b.url),{method:req.method,headers:{Authorization:req.headers.authorization||'','Content-Type':'application/json'},...(body.length?{body:Buffer.concat(body)}:{})});
    res.writeHead(r.status,{'Content-Type':r.headers.get('Content-Type')||'application/json','Cache-Control':'no-store',...(r.headers.get('X-Titan-Fiscal-Scope')?{'X-Titan-Fiscal-Scope':r.headers.get('X-Titan-Fiscal-Scope')}:{})});res.end(Buffer.from(await r.arrayBuffer()));return;
   }
   if(req.url==='/screen.js'){res.writeHead(200,{'Content-Type':'application/javascript','Cache-Control':'no-store'});res.end(bundle);return;}
   res.writeHead(200,{'Content-Type':'text/html','Cache-Control':'no-store'});res.end('<!doctype html><html lang="pt-BR"><meta charset="utf-8"><title>Disposable checkout test</title><div id="root"></div><script src="/screen.js"></script></html>');
  }catch{res.writeHead(502);res.end('disposable proxy unavailable');}
 });
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
 const origin='http://127.0.0.1:'+server.address().port;
 chrome=spawn(process.env.TITAN_TEST_CHROME||'/usr/bin/google-chrome',['--headless=new','--remote-debugging-port=0','--remote-debugging-address=127.0.0.1','--user-data-dir='+join(dir,'profile'),'--no-first-run','--no-default-browser-check','--disable-background-networking','--disable-component-update','--disable-sync','--no-proxy-server','--disable-dev-shm-usage','--host-resolver-rules=MAP * ~NOTFOUND, EXCLUDE 127.0.0.1, EXCLUDE localhost','about:blank'],{stdio:'ignore'});
 let port;for(let i=0;i<200;i++){try{port=Number((await readFile(join(dir,'profile','DevToolsActivePort'),'utf8')).split('\n')[0]);break;}catch{await delay(50);}}
 if(!port)throw Error('Chromium did not open its disposable debugging port');
 const tabs=await (await fetch('http://127.0.0.1:'+port+'/json/list')).json(),tab=tabs.find(t=>t.type==='page');
 socket=new WebSocket(tab.webSocketDebuggerUrl);await new Promise((resolve,reject)=>{socket.onopen=resolve;socket.onerror=reject;});
 socket.onmessage=e=>{const m=JSON.parse(e.data),p=pending.get(m.id);if(p){pending.delete(m.id);m.error?p.reject(Error('CDP command failed')):p.resolve(m.result);}};
 const command=(method,params={})=>new Promise((resolve,reject)=>{const id=++seq;pending.set(id,{resolve,reject});socket.send(JSON.stringify({id,method,params}));});
 const evaluate=async expression=>{const r=await command('Runtime.evaluate',{expression,awaitPromise:true,returnByValue:true});if(r.exceptionDetails)throw Error('browser evaluation failed: '+String(r.exceptionDetails.exception?.description||r.exceptionDetails.text).split('\n')[0]);return r.result?.value;};
 const wait=async(expr,label)=>{for(let i=0;i<160;i++){if(await evaluate(expr))return;await delay(50);}throw Error('screen did not reach '+label);};
 await command('Page.enable');await command('Page.navigate',{url:origin});await wait('typeof window.start==="function"','mounted harness');
 const button=label=>`[...document.querySelectorAll('button')].find(b=>b.textContent===${JSON.stringify(label)})`;
 const click=async label=>{const expr=button(label);await wait(`!!(${expr})&&!(${expr}).disabled`,label);await evaluate(`(${expr}).click()`);};
 const input=async(selector,value)=>{await wait(`!!document.querySelector(${JSON.stringify(selector)})`,'input control');await evaluate(`(()=>{const el=document.querySelector(${JSON.stringify(selector)});if(!el)throw Error('missing control');Object.getOwnPropertyDescriptor(el.tagName==='SELECT'?HTMLSelectElement.prototype:HTMLInputElement.prototype,'value').set.call(el,${JSON.stringify(value)});el.dispatchEvent(new Event('input',{bubbles:true}));el.dispatchEvent(new Event('change',{bubbles:true}));})()`);await delay(30);};
 const sales=[];
 for(let n=0;n<2;n++){
  await evaluate(`window.start(${JSON.stringify({token:b.token,session,product,location:b.location,source:'screen-case-'+n})})`);
  await wait('!!document.querySelector("[name=fiscal96]")&&!document.querySelector("[name=fiscal96]").disabled','ready checkout');
  await evaluate('document.querySelector("details").open=true');await input('[name=fiscal96]','nfce');
  await wait('!!document.querySelector("fieldset")','NFC-e form');
  await input('fieldset input','VENDA');await input('fieldset select','1');await input('fieldset label:nth-of-type(3) input',b.policy);
  await input('[name=amount96]','15,99');await click('Adicionar parcela');await input('[name=cashReceived96]','20,00');
  await click('Conferir valores e autorizações');await wait(`${button('Conferir valores e autorizações')}.disabled`,'immutable quote');
  await click('Finalizar compra');await wait('!!window.__receipt','committed sale');
  const receipt=await evaluate('window.__receipt');if(receipt.status!=='committed'||receipt.total_cents!==1599)throw Error('sale result mismatch');sales.push(receipt.sale_id);
  if(n===1){
   if(receipt.fiscal_authorized!==false||receipt.fiscal_state!=='pending')throw Error('dropped Focus reply was hidden');
   const postCount=requests.filter(([m,p])=>m==='POST'&&p.endsWith('/finalize-cash')).length;
   await click('Recuperar tentativa original');
   await wait(`[...document.querySelectorAll('[role=status]')].some(e=>e.textContent==="Autorizada")`,'explicit fiscal recovery');
   if(requests.filter(([m,p])=>m==='POST'&&p.endsWith('/finalize-cash')).length!==postCount)throw Error('recovery resent checkout');
  }else if(!receipt.fiscal_authorized)throw Error('first Focus simulation did not authorize');
  const artifact=await evaluate(`(async()=>{const r=await fetch('/local/v1/checkout/plans/'+${JSON.stringify(receipt.sale_id)}+'/fiscal/danfe?width_mm=80&timezone=America%2FSao_Paulo',{headers:{Authorization:'Bearer '+${JSON.stringify(b.token)}}});const text=await r.text();return {status:r.status,banner:text.includes('SEM VALOR FISCAL'),qr:text.includes('data:image/png;base64,')};})()`);
  if(artifact.status!==200||!artifact.banner||!artifact.qr)throw Error('archived 80mm coupon unavailable');
 }
 console.log(JSON.stringify({sales,rendered:'Checkout96 + CheckoutFiscalStatus',focus:'simulated only',duplicate_checkout_posts:0,physical_printing:false}));
}catch(e){console.error(e instanceof Error?e.message:'screen test failed');process.exitCode=1;}
finally{
 socket?.close();if(chrome){chrome.kill('SIGTERM');await Promise.race([new Promise(r=>chrome.once('exit',r)),delay(2000)]);if(chrome.exitCode===null)chrome.kill('SIGKILL');}
 if(server){server.closeAllConnections();await new Promise(r=>server.close(r));}await rm(dir,{recursive:true,force:true});
}
