'use strict';
const {test}=require('node:test'),assert=require('node:assert/strict'),{spawn}=require('node:child_process');
const {stopProcess}=require('../scripts/verify-installed.cjs');
test('installed-app harness terminates its own unresponsive child and releases pipes',async()=>{
 const child=spawn(process.execPath,['-e',"process.on('SIGTERM',()=>{});process.stdout.write('ready');setInterval(()=>{},1000)"],{detached:process.platform!=='win32',stdio:['ignore','pipe','pipe']});
 const closed=new Promise(resolve=>child.once('close',resolve));
 await new Promise((resolve,reject)=>{child.once('error',reject);child.stdout.once('data',resolve)});
 try{await stopProcess(child,closed);await closed;assert.notEqual(child.exitCode,0);assert.equal(child.stderr.destroyed,true);}finally{if(child.exitCode===null&&!child.signalCode)child.kill('SIGKILL')}
});
