"use strict";
// Explicit disposable acceptance only. Credentials arrive via stdin, never argv/logs.
const fs=require('node:fs'),path=require('node:path'),http=require('node:http'),assert=require('node:assert/strict'),{spawn}=require('node:child_process');
const {stopProcess}=require('./verify-installed.cjs');
const sleep=ms=>new Promise(r=>setTimeout(r,ms));
let phase='start';const mark=value=>{phase=value;console.error(JSON.stringify({acceptancePhase:value}))};
(async()=>{
 let raw='';for await(const c of process.stdin)raw+=c;const b=JSON.parse(raw);raw='';
 assert.ok(path.isAbsolute(b.binary)&&path.isAbsolute(b.root));
 const profile=path.join(b.root,'desktop-profile');fs.mkdirSync(profile,{mode:0o700});
 const reserve=http.createServer();await new Promise(r=>reserve.listen(0,'127.0.0.1',r));const port=reserve.address().port;await new Promise(r=>reserve.close(r));
 const env={...process.env,XDG_CONFIG_HOME:path.join(profile,'config'),XDG_CACHE_HOME:path.join(profile,'cache')};delete env.ELECTRON_RUN_AS_NODE;
 mark('launch');const child=spawn(b.binary,['--user-data-dir='+profile,'--remote-debugging-address=127.0.0.1','--remote-debugging-port='+port,'--software-rendering'],{env,detached:true,stdio:['ignore','ignore','pipe']});
 let sandboxRefused=false;child.stderr.on('data',chunk=>{if(/sandbox|Operation not permitted/i.test(String(chunk)))sandboxRefused=true});
 const closed=new Promise(r=>{child.on('error',()=>r(-1));child.on('close',r)});let socket,seq=0;const pending=new Map();
 const connect=async predicate=>{
  for(let n=0;n<120;n++){
   let pages=[];try{pages=await(await fetch('http://127.0.0.1:'+port+'/json/list',{signal:AbortSignal.timeout(500)})).json()}catch{}
   const page=pages.find(p=>p.type==='page'&&predicate(p.url));
   if(page){socket?.close();socket=new WebSocket(page.webSocketDebuggerUrl);await new Promise((r,j)=>{socket.onopen=r;socket.onerror=()=>j(Error('debug connection failed'))});socket.onmessage=e=>{const v=JSON.parse(e.data),p=pending.get(v.id);if(p){pending.delete(v.id);v.error?p.reject(Error('inspection refused')):p.resolve(v.result)}};return;}
   await sleep(200);
  }throw Error(sandboxRefused?'sandbox startup refused':'application page unavailable');
 };
 const send=(method,params={})=>new Promise((resolve,reject)=>{const id=++seq,t=setTimeout(()=>{pending.delete(id);reject(Error('inspection timeout'))},10000);pending.set(id,{resolve:v=>{clearTimeout(t);resolve(v)},reject:e=>{clearTimeout(t);reject(e)}});socket.send(JSON.stringify({id,method,params}))});
 const evaluate=async expression=>{const r=await send('Runtime.evaluate',{expression,awaitPromise:true,returnByValue:true});if(r.exceptionDetails)throw Error('screen action failed');return r.result?.value};
 const wait=async(expression,label)=>{for(let n=0;n<120;n++){if(await evaluate(expression))return;await sleep(100)}throw Error('screen missing: '+label)};
 const input=async(selector,value)=>{await wait(`!!document.querySelector(${JSON.stringify(selector)})`,'input');await evaluate(`(()=>{const el=document.querySelector(${JSON.stringify(selector)});Object.getOwnPropertyDescriptor(el.tagName==='SELECT'?HTMLSelectElement.prototype:HTMLInputElement.prototype,'value').set.call(el,${JSON.stringify(value)});el.dispatchEvent(new Event('input',{bubbles:true}));el.dispatchEvent(new Event('change',{bubbles:true}));})()`);await sleep(60)};
 try{
  mark('connection');await connect(u=>u==='titan://app/desktop-setup/index.html');await wait('!document.querySelector("#fields").disabled','connection');
  await input('#operational',b.url+'/local/v1');await evaluate('document.querySelector("#connection").requestSubmit()');
  mark('login');await connect(u=>u.startsWith('titan://app/local/'));
  await wait('!!document.querySelector("#brand-credential")','login');await input('#brand-credential',b.owner);await input('#brand-password',b.password);await evaluate('document.querySelector("#brand-password").form.requestSubmit()');
  mark('authenticate');await wait('location.pathname==="/local/home"','authenticated home');
  mark('company form');await send('Page.navigate',{url:'titan://app/local/company'});await wait('!!document.querySelector("#company-registration-form")','company form');
  await input('#company-registration-form input:nth-of-type(1)', 'Synthetic clean company');
  await evaluate('document.querySelector("#company-registration-form").requestSubmit()');
  await wait('document.querySelector("#company-registration-form fieldset")&&!document.querySelector("#company-registration-form fieldset").disabled','pre-registration saved');
  // Observe authoritative persisted revision, not just a transient UI message.
  await wait(`(async()=>{const r=await fetch('/local/v1/company/registration',{headers:{Authorization:'Bearer '+${JSON.stringify(b.token)}}});return r.ok&&(await r.json()).revision===1})()`,'persisted pre-registration');
  await input('#company-registration-form select','company_profile');
  mark('complete profile');const values=['Synthetic clean company','Synthetic test owner','11222333000181','Synthetic clean legal','SP','123456789','3550308','1'];
  for(let n=0;n<values.length;n++)await input(`#company-registration-form label:has(input):nth-of-type(${n+2}) input`,values[n]);
  const selects=await evaluate('document.querySelectorAll("#company-registration-form select").length');assert.equal(selects,3);
  await input('#company-registration-form label:nth-last-of-type(2) select','external');await input('#company-registration-form label:last-of-type select','homologation');
  await evaluate('document.querySelector("#company-registration-form").requestSubmit()');
  await wait(`(async()=>{const r=await fetch('/local/v1/company/registration',{headers:{Authorization:'Bearer '+${JSON.stringify(b.token)}}});return r.ok&&(await r.json()).revision===2})()`,'persisted complete profile');
  await wait('!document.querySelector("#company-registration-form fieldset").disabled','complete profile saved');
  const saved=await evaluate(`(async()=>{const r=await fetch('/local/v1/company/registration',{headers:{Authorization:'Bearer '+${JSON.stringify(b.token)}}});const d=await r.json();return {revision:d.revision,stage:d.input.stage,environment:d.input.environment}})()`);
  assert.deepEqual(saved,{revision:2,stage:'company_profile',environment:'homologation'});
  // No screen capture of personal fields or credentials is retained.
  console.log(JSON.stringify({packagedApp:true,freshProfile:true,connectionSaved:true,ownerLogin:true,preRegistrationRevision:1,completeRegistrationRevision:2,uiSource:'frontend-web',realHTTP:true,commercialData:false,officialFiscalAcceptance:false}));
 }finally{socket?.close();b.password='';b.token='';await stopProcess(child,closed)}
})().catch(error=>{const detail=/^(screen missing: [a-z -]+|sandbox startup refused|application page unavailable|screen action failed|inspection timeout)$/.test(error.message)?error.message:'assertion or inspection failed';console.error(JSON.stringify({acceptanceFailure:phase,detail}));process.exitCode=1});
