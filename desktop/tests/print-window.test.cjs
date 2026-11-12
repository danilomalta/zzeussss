'use strict';
const test=require('node:test'),assert=require('node:assert/strict'),{EventEmitter}=require('node:events'),{installPrintWindows}=require('../src/print-window.cjs');
function fixture(url='titan://app/local/pos'){
 const win=new EventEmitter(),wc=new EventEmitter(),external=[];win.webContents=wc;wc.getURL=()=>url;wc.setWindowOpenHandler=fn=>wc.open=fn;installPrintWindows(win,{ownURL:u=>u.startsWith('titan://app/'),onExternal:async u=>external.push(u)});return {win,wc,external};
}
function child(id=5){const c=new EventEmitter();c.webContents=new EventEmitter();c.webContents.id=id;c.webContents.setWindowOpenHandler=fn=>c.open=fn;const s=new EventEmitter();s.webRequest={onBeforeRequest:(_filter,fn)=>s.request=fn};c.webContents.session=s;c.isDestroyed=()=>false;c.destroy=()=>c.destroyed=true;return c;}
test('only blank ERP print windows are allowed, sandboxed and script-free',()=>{
 const f=fixture(),a=f.wc.open({url:'about:blank'});assert.equal(a.action,'allow');const p=a.overrideBrowserWindowOptions.webPreferences;
 for(const k of ['nodeIntegration','nodeIntegrationInSubFrames','allowRunningInsecureContent','webviewTag','javascript'])assert.equal(p[k],false);for(const k of ['contextIsolation','sandbox','webSecurity'])assert.equal(p[k],true);
});
test('setup, external URL, file, javascript and encoded blank cannot create print windows',()=>{
 for(const page of ['titan://app/desktop-setup/index.html','https://example.invalid/'])assert.equal(fixture(page).wc.open({url:'about:blank'}).action,'deny');
 const f=fixture();for(const url of ['https://example.invalid/','file:///tmp/private','javascript:alert(1)','data:text/html,hello','about:blank#x'])assert.equal(f.wc.open({url}).action,'deny');
});
test('print child cannot network, download, navigate or open nested windows; parent traffic is unchanged',()=>{
 const f=fixture(),c=child();f.wc.emit('did-create-window',c);const s=c.webContents.session;let prevented=0;
 for(const url of ['https://example.invalid/','http://127.0.0.1:8181/local/v1/me','file:///tmp/private','titan://app/local/v1/me']){let v;s.request({webContentsId:5,url},x=>v=x);assert.equal(v.cancel,true);s.request({webContentsId:1,url},x=>v=x);assert.equal(v.cancel,false);}
 for(const url of ['about:blank','data:image/png;base64,AA==']){let v;s.request({webContentsId:5,url},x=>v=x);assert.equal(v.cancel,false);}
 s.emit('will-download',{preventDefault(){prevented++;}},null,c.webContents);assert.equal(prevented,1);
 for(const event of ['will-navigate','will-redirect','will-attach-webview'])c.webContents.emit(event,{preventDefault(){prevented++;}});assert.equal(c.open().action,'deny');assert.equal(prevented,4);
 f.win.emit('closed');assert.equal(c.destroyed,true);
});
test('print window limit and closing one window release only its own slot',()=>{const f=fixture(),children=[];for(let i=0;i<3;i++){const c=child(i);f.wc.emit('did-create-window',c);children.push(c);}assert.equal(f.wc.open({url:'about:blank'}).action,'deny');children[0].emit('closed');assert.equal(f.wc.open({url:'about:blank'}).action,'allow');});
