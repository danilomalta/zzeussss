'use strict';
// Explicit CI/operator check: real Chromium output, never physical printing.
const fs=require('node:fs'),path=require('node:path'),os=require('node:os'),assert=require('node:assert/strict'),{spawn}=require('node:child_process');
async function verify(out){
 assert.ok(path.isAbsolute(out)&&!fs.existsSync(out),'new absolute output directory required');
 const desktop=path.resolve(__dirname,'..'),stage=path.join(desktop,'.build',JSON.parse(fs.readFileSync(path.join(desktop,'.build/current.json'))).stage),binary=require('electron'),test=path.join(desktop,'tests/native-print.cjs'),env={...process.env};delete env.ELECTRON_RUN_AS_NODE;delete env.WAYLAND_DISPLAY;env.XDG_SESSION_TYPE='x11';
 const args=[...(process.platform==='linux'?['--ozone-platform=x11']:[]),test,stage,path.resolve(desktop,'../frontend-web'),out];
 const child=spawn(process.platform==='linux'?'xvfb-run':binary,process.platform==='linux'?['-a',binary,...args]:args,{env,stdio:['ignore','pipe','pipe'],detached:process.platform!=='win32'}),closed=new Promise((resolve,reject)=>{child.once('error',reject);child.once('close',resolve)});
 let stdout='',stderr='';child.stdout.on('data',b=>{stdout+=b.toString()});child.stderr.on('data',b=>{if(stderr.length<16000)stderr+=b.toString()});
 const timer=setTimeout(()=>{if(child.exitCode===null)child.kill('SIGTERM')},30000);
 try{const code=await closed;if(code!==0){const diagnostic=/SUID sandbox|No usable sandbox|sandbox_linux|zygote_host/.test(stderr)?'sandbox_startup_refused':/error while loading shared libraries/.test(stderr)?'missing_runtime_library':'native_process_failure';throw Error('native print process failed: exit='+code+' diagnostic='+diagnostic);} const report=JSON.parse(fs.readFileSync(path.join(out,'print.json')));assert.equal(report.networkProbeRequests,0);assert.equal(report.physicalPrinting,false);assert.equal(report.results.length,3);console.log(JSON.stringify(report));}finally{clearTimeout(timer);if(child.exitCode===null)await require('./verify-installed.cjs').stopProcess(child,closed);}
}
if(require.main===module)verify(process.argv[2]||path.join(os.tmpdir(),'titan-print-'+Date.now())).catch(error=>{console.error(error.message);process.exitCode=1});
module.exports={verify};
