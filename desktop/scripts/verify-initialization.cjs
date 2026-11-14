"use strict";
// Explicit disposable acceptance only. Credentials arrive via stdin, never argv/logs.
const fs=require('node:fs'),path=require('node:path'),http=require('node:http'),assert=require('node:assert/strict'),{spawn}=require('node:child_process');
const {stopProcess}=require('./verify-installed.cjs');
const sleep=ms=>new Promise(r=>setTimeout(r,ms));
let phase='start',phaseFile;const mark=value=>{phase=value;if(phaseFile)fs.writeFileSync(phaseFile,JSON.stringify({phase}),{mode:0o600});console.error(JSON.stringify({acceptancePhase:value}))};
(async()=>{
 let raw='';for await(const c of process.stdin)raw+=c;const b=JSON.parse(raw);raw='';
 assert.ok(path.isAbsolute(b.binary)&&path.isAbsolute(b.root));
 phaseFile=path.join(b.root,'acceptance-phase.json');
 const profile=path.join(b.root,'desktop-profile');if(b.resume)assert.ok(fs.statSync(profile).isDirectory());else fs.mkdirSync(profile,{mode:0o700});
 const reserve=http.createServer();await new Promise(r=>reserve.listen(0,'127.0.0.1',r));const port=reserve.address().port;await new Promise(r=>reserve.close(r));
 const env={...process.env,XDG_CONFIG_HOME:path.join(profile,'config'),XDG_CACHE_HOME:path.join(profile,'cache'),APPDATA:path.join(profile,'appdata'),LOCALAPPDATA:path.join(profile,'localappdata')};delete env.ELECTRON_RUN_AS_NODE;delete env.WAYLAND_DISPLAY;env.XDG_SESSION_TYPE='x11';
 for(const name of ['config','cache','appdata','localappdata'])fs.mkdirSync(path.join(profile,name),{recursive:true,mode:0o700});
 mark('launch');const child=spawn(b.binary,['--user-data-dir='+profile,'--remote-debugging-address=127.0.0.1','--remote-debugging-port='+port,'--software-rendering','--ozone-platform=x11'],{env,detached:process.platform!=='win32',stdio:['ignore','pipe','pipe']});
 fs.writeFileSync(path.join(b.root,'acceptance-process.json'),JSON.stringify({pid:child.pid,binary:b.binary}),{mode:0o600});
 let finishing=false,taskFailed=false,appFailure='';const observe=chunk=>{const text=String(chunk);if(finishing)return;const code=text.match(/falha de inicialização \(([a-z_0-9-]+)\)/);if(code){appFailure='desktop '+code[1];fs.writeFileSync(phaseFile,JSON.stringify({phase,appFailure}),{mode:0o600})}};child.stdout.on('data',observe);child.stderr.on('data',observe);
 const closed=new Promise(r=>{child.on('error',()=>r(-1));child.on('close',r)});let socket,seq=0;const pending=new Map();
 const connect=async predicate=>{
  // Detach from the closing settings target before discovering its successor.
  // Do not retain a debugger attached to a target that is closing.
  socket?.close();socket=null;
  const deadline=Date.now()+24000;
  for(let n=0;n<120&&Date.now()<deadline;n++){
   if(appFailure)throw Error(appFailure);
   let pages=[],discovery='ok';try{pages=await(await fetch('http://127.0.0.1:'+port+'/json/list',{signal:AbortSignal.timeout(500)})).json()}catch(error){discovery=error.name}
   fs.writeFileSync(phaseFile,JSON.stringify({phase,discovery,exited:child.exitCode!==null,targets:pages.map(p=>({type:p.type,route:String(p.url||'').split('?')[0]}))}),{mode:0o600});
   const page=pages.find(p=>p.type==='page'&&predicate(p.url));
   if(page){socket?.close();socket=new WebSocket(page.webSocketDebuggerUrl);await new Promise((r,j)=>{const timer=setTimeout(()=>j(Error('debug connection failed')),3000);socket.onopen=()=>{clearTimeout(timer);r()};socket.onerror=()=>{clearTimeout(timer);j(Error('debug connection failed'))}});socket.onmessage=e=>{const v=JSON.parse(e.data);if(v.method==='Page.javascriptDialogOpening'&&finishing&&v.params?.type==='beforeunload'){void send('Page.handleJavaScriptDialog',{accept:true}).catch(()=>{});}const p=pending.get(v.id);if(p){pending.delete(v.id);v.error?p.reject(Error('inspection refused')):p.resolve(v.result)}};return;}
   await sleep(200);
  }throw Error('application page unavailable');
 };
 const send=(method,params={})=>new Promise((resolve,reject)=>{const id=++seq,t=setTimeout(()=>{pending.delete(id);reject(Error('inspection timeout'))},10000);pending.set(id,{resolve:v=>{clearTimeout(t);resolve(v)},reject:e=>{clearTimeout(t);reject(e)}});socket.send(JSON.stringify({id,method,params}))});
 const evaluate=async expression=>{const r=await send('Runtime.evaluate',{expression,awaitPromise:true,returnByValue:true});if(r.exceptionDetails)throw Error('screen action failed');return r.result?.value};
 const wait=async(expression,label)=>{for(let n=0;n<120;n++){if(await evaluate(expression))return;await sleep(100)}throw Error('screen missing: '+label)};
 const input=async(selector,value)=>{await wait(`!!document.querySelector(${JSON.stringify(selector)})`,'input');await evaluate(`(()=>{const el=document.querySelector(${JSON.stringify(selector)});Object.getOwnPropertyDescriptor(el.tagName==='SELECT'?HTMLSelectElement.prototype:el.tagName==='TEXTAREA'?HTMLTextAreaElement.prototype:HTMLInputElement.prototype,'value').set.call(el,${JSON.stringify(value)});el.dispatchEvent(new Event('input',{bubbles:true}));el.dispatchEvent(new Event('change',{bubbles:true}));})()`);await sleep(60)};
 try{
  if(!b.resume){mark('connection');await connect(u=>u==='titan://app/desktop-setup/index.html');await wait('!!document.querySelector("#fields")&&!document.querySelector("#fields").disabled','connection');
  await input('#operational',b.url+'/local/v1');await evaluate('setTimeout(()=>document.querySelector("#connection").requestSubmit(),0)');}
  mark('login');await connect(u=>u.startsWith('titan://app/local/'));
  await wait('!!document.querySelector("#brand-credential")','login');if(b.business&&await evaluate('document.documentElement.lang!=="pt-BR"')){await wait('!!document.querySelector(".brand-contact-trigger")','language menu');await evaluate('document.querySelector(".brand-contact-trigger").click()');const languageButton=JSON.stringify('.language-list button[lang="pt-BR"]');await wait(`!!document.querySelector(${languageButton})`,'language choice');await evaluate(`document.querySelector(${languageButton}).click()`);await evaluate('document.querySelector(".contact-heading button").click()');await wait('document.documentElement.lang==="pt-BR"','Portuguese interface');}if(b.transitionOnly){console.log(JSON.stringify({transitionOnly:true,loginVisible:true}));return;}await input('#brand-credential',b.owner);await input('#brand-password',b.password);await evaluate('document.querySelector("#brand-password").form.requestSubmit()');
  mark('authenticate');await wait('location.pathname==="/local/home"','authenticated home');
  if(b.business==='trade'){console.log(JSON.stringify(await require('./verify-company-trade.cjs')({b,evaluate,wait,input,mark})));return;}
  if(b.business){if(b.registration&&(b.business==='sale'||b.business==='restored'||b.business==='closed-restored'))await require('./verify-company-flow.cjs')({b,evaluate,wait,input,mark});const result=await require('./verify-business-flow.cjs')({b,send,evaluate,wait,input,mark,sleep});console.log(JSON.stringify(result));return;}
  console.log(JSON.stringify(await require('./verify-company-flow.cjs')({b,evaluate,wait,input,mark})));
 }catch(error){taskFailed=true;throw error;}finally{
  finishing=true;b.password='';b.token='';
  let graceful=false;
  if(b.business&&socket?.readyState===WebSocket.OPEN&&child.exitCode===null){
   try{await send('Page.enable');void send('Runtime.evaluate',{expression:'window.close()'}).catch(()=>{});await Promise.race([closed,sleep(3000)]);graceful=child.exitCode===0;}catch{}
  }
  socket?.close();if(child.exitCode===null)await stopProcess(child,closed);
  if(b.business&&!graceful&&!taskFailed)throw Error('disposable application did not close gracefully');
 }
})().catch(error=>{const moduleCodes={ERR_UNSUPPORTED_ESM_URL_SCHEME:'controller module URL refused',ERR_MODULE_NOT_FOUND:'controller module unavailable'};const detail=moduleCodes[error.code]||(/^(screen missing: [a-z -]+|sandbox startup refused|application page unavailable|screen action failed|inspection timeout|disposable application did not close gracefully|profile fields mismatch|desktop [a-z_0-9-]+)$/.test(error.message)?error.message:'assertion or inspection failed');console.error(JSON.stringify({acceptanceFailure:phase,detail}));process.exitCode=1});
