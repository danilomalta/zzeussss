"use strict";
// Runs only when explicitly invoked by the CI/operator against an installed app.
// Uses a fresh profile; never configures or starts a backend or disables sandbox.
const fs=require('node:fs'),os=require('node:os'),path=require('node:path'),http=require('node:http'),{spawn}=require('node:child_process'),assert=require('node:assert/strict');
const sleep=ms=>new Promise(r=>setTimeout(r,ms));
async function stopProcess(child,closed){
 if(!Number.isSafeInteger(child.pid)||child.pid<=1||child.pid===process.pid)return;
 if(process.platform==='win32'){
  if(child.exitCode===null){const killer=spawn('taskkill',['/PID',String(child.pid),'/T','/F'],{stdio:'ignore'});await new Promise((resolve,reject)=>{killer.once('error',reject);killer.once('close',resolve)});}
 }else{
  // detached creates a group containing only this disposable application tree.
  const signal=name=>{try{process.kill(-child.pid,name)}catch(error){if(error.code!=='ESRCH')throw error}};
  signal('SIGTERM');await Promise.race([closed,sleep(1000)]);signal('SIGKILL');
 }
 await Promise.race([closed,sleep(3000)]);child.stderr?.destroy();child.unref();
}

async function verify(binary,out){
 assert.ok(path.isAbsolute(binary)&&fs.statSync(binary).isFile(),'installed executable required');assert.ok(!fs.existsSync(out),'new evidence directory required');fs.mkdirSync(out,{recursive:true,mode:0o700});const profile=fs.mkdtempSync(path.join(os.tmpdir(),'titan-installed-profile-'));
 const reserve=http.createServer();await new Promise(r=>reserve.listen(0,'127.0.0.1',r));const port=reserve.address().port;await new Promise(r=>reserve.close(r));
 const env={...process.env,XDG_CONFIG_HOME:path.join(profile,'config'),XDG_CACHE_HOME:path.join(profile,'cache'),APPDATA:path.join(profile,'appdata'),LOCALAPPDATA:path.join(profile,'localappdata')};delete env.ELECTRON_RUN_AS_NODE;
 for(const name of ['config','cache','appdata','localappdata'])fs.mkdirSync(path.join(profile,name),{mode:0o700});
 const started=performance.now();let stderr='',socket,exited=false;
 const child=spawn(binary,['--user-data-dir='+profile,'--remote-debugging-address=127.0.0.1','--remote-debugging-port='+port,'--software-rendering'],{env,detached:process.platform!=='win32',stdio:['ignore','ignore','pipe']});child.stderr.on('data',b=>{if(stderr.length<64000)stderr+=b.toString()});const closed=new Promise(resolve=>{child.on('error',()=>{exited=true;resolve(-1)});child.on('close',code=>{exited=true;resolve(code)})});
 try{
  let page;const deadline=Date.now()+30000;
  while(Date.now()<deadline&&!exited){try{const r=await fetch('http://127.0.0.1:'+port+'/json/list',{signal:AbortSignal.timeout(1000)});page=(await r.json()).find(p=>p.type==='page'&&p.url==='titan://app/desktop-setup/index.html');if(page)break}catch{}await sleep(200)}
  assert.ok(page,'installed application did not load its connection window within 30s');
  socket=new WebSocket(page.webSocketDebuggerUrl);await new Promise((resolve,reject)=>{socket.addEventListener('open',resolve,{once:true});socket.addEventListener('error',reject,{once:true});setTimeout(()=>reject(Error('debug connection timeout')),3000).unref()});let id=0;const pending=new Map();socket.addEventListener('message',event=>{const r=JSON.parse(String(event.data));if(!r.id)return;const p=pending.get(r.id);if(!p)return;pending.delete(r.id);r.error?p.reject(Error(r.error.message)):p.resolve(r.result)});
  const send=(method,params={})=>new Promise((resolve,reject)=>{const n=++id;const timer=setTimeout(()=>{pending.delete(n);reject(Error('inspection timeout'))},3000);pending.set(n,{resolve:r=>{clearTimeout(timer);resolve(r)},reject:e=>{clearTimeout(timer);reject(e)}});socket.send(JSON.stringify({id:n,method,params}))});
  let visible;for(let n=0;n<30;n++){const r=await send('Runtime.evaluate',{expression:"({title:document.title,url:location.href,textLength:document.body.innerText.trim().length,inputs:document.querySelectorAll('input,select,button').length})",returnByValue:true});visible=r.result.value;if(visible?.textLength>50&&visible.inputs>0)break;await sleep(200)}
  assert.ok(visible?.textLength>50&&visible.inputs>0,'installed window is blank or lacks connection controls');assert.equal(visible.url,'titan://app/desktop-setup/index.html');
  const shot=await send('Page.captureScreenshot',{format:'png'});fs.writeFileSync(path.join(out,'installed-window.png'),Buffer.from(shot.data,'base64'),{flag:'wx',mode:0o600});
  const report={platform:process.platform,installedExecutable:true,freshProfile:true,window:visible.url,textLength:visible.textLength,controls:visible.inputs,startupMilliseconds:Math.round(performance.now()-started),backendStarted:false,commercialData:false,businessFlowProven:false};fs.writeFileSync(path.join(out,'installed-window.json'),JSON.stringify(report,null,2),{flag:'wx',mode:0o600});console.log(JSON.stringify(report));
 }finally{socket?.close();await stopProcess(child,closed);fs.writeFileSync(path.join(out,'process.log'),stderr,{flag:'wx',mode:0o600})}
}
if(require.main===module){const [binary,out,...extra]=process.argv.slice(2);if(!binary||!out||extra.length)throw Error('Usage: node verify-installed.cjs ABSOLUTE_BINARY NEW_EVIDENCE_DIRECTORY');verify(binary,out).catch(error=>{console.error(error.message);process.exitCode=1})}
module.exports={verify,stopProcess};
