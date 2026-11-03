'use strict';
// Test-only main-process HTTP bridge. No renderer scripting or UI substitution.
// The acceptance driver sends the real API requests through the packaged
// titan:// protocol and its production transport, never straight to a mock.
const {app}=require('electron');
const fs=require('node:fs'),os=require('node:os'),path=require('node:path'),readline=require('node:readline'),assert=require('node:assert/strict');
const args=process.argv.slice(process.argv.indexOf(__filename)+1),stage=args[0],endpoint=args[1];
assert.ok(path.isAbsolute(stage));assert.match(endpoint,/^http:\/\/127\.0\.0\.1:[1-9][0-9]*$/);
const root=fs.mkdtempSync(path.join(os.tmpdir(),'titan-pilot-native-'));app.setPath('userData',root);
fs.writeFileSync(path.join(root,'connection-v1.json'),JSON.stringify({version:1,mode:'local',operational_url:endpoint+'/local/v1',online_url:null}),{mode:0o600});
function reply(v){process.stdout.write('TITAN_PILOT_RPC:'+JSON.stringify(v)+'\n');}
process.on('uncaughtException',()=>{reply({type:'failure',code:'native_uncaught'});app.exit(1);});
(async()=>{
 const main=require(path.join(stage,'main.cjs')),win=await main.ready;
 assert.ok(win&&!win.isDestroyed());assert.equal(win.webContents.getURL(),'titan://app/local/login');
 const prefs=win.webContents.getLastWebPreferences();assert.equal(prefs.sandbox,true);assert.equal(prefs.contextIsolation,true);assert.equal(prefs.nodeIntegration,false);assert.equal(prefs.webSecurity,true);
 const ses=win.webContents.session;
 reply({type:'ready',origin:'titan://app',sandbox:true,contextIsolation:true,nodeIntegration:false,electron:process.versions.electron});
 const input=readline.createInterface({input:process.stdin,crlfDelay:Infinity});
 for await(const line of input){
  if(line.length>2*1024*1024)throw Error('input_limit');const command=JSON.parse(line);
  if(command.type==='stop')break;
  assert.equal(command.type,'request');assert.ok(['GET','POST'].includes(command.method));assert.match(command.path,/^\/local\/v1\//);assert.ok(!/[\r\n\\]/.test(command.path));
  const url=new URL('titan://app'+command.path);assert.equal(url.origin,'null');assert.equal(url.hostname,'app');assert.equal(url.protocol,'titan:');assert.ok(url.pathname.startsWith('/local/v1/'));
  const headers={};for(const key of ['Authorization','Content-Type'])if(command.headers?.[key])headers[key]=command.headers[key];
  const r=await ses.fetch(url.href,{method:command.method,headers,credentials:'omit',cache:'no-store',redirect:'manual',signal:AbortSignal.timeout(20000),...(command.method==='POST'?{body:Buffer.from(command.body||'','base64')}:{})});
  const body=Buffer.from(await r.arrayBuffer());assert.ok(body.length<=4*1024*1024);
  reply({type:'response',status:r.status,headers:{'Content-Type':r.headers.get('content-type')||''},body:body.toString('base64')});
 }
 reply({type:'stopped'});app.exit(0);
})().catch(()=>{reply({type:'failure',code:'native_pilot_failed'});app.exit(1);});
