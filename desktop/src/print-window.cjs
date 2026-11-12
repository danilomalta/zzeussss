'use strict';
// Only blank, script-free print surfaces from the first-party ERP renderer.
const guardedSessions=new WeakMap();
function installPrintWindows(win,{ownURL,onExternal}){
 const children=new Set();
 win.webContents.setWindowOpenHandler(({url})=>{
  if(url!=='about:blank'||!ownURL(win.webContents.getURL())||!win.webContents.getURL().startsWith('titan://app/local/')||children.size>=3){if(url!=='about:blank')void onExternal(url);return {action:'deny'};}
  return {action:'allow',overrideBrowserWindowOptions:{parent:win,width:820,height:720,autoHideMenuBar:true,webPreferences:{nodeIntegration:false,nodeIntegrationInSubFrames:false,contextIsolation:true,sandbox:true,webSecurity:true,allowRunningInsecureContent:false,webviewTag:false,javascript:false}}};
 });
 win.webContents.on('did-create-window',child=>{
  const contents=child.webContents,ses=contents.session;
  let ids=guardedSessions.get(ses);
  if(!ids){ids=new Set();guardedSessions.set(ses,ids);ses.webRequest.onBeforeRequest({urls:['<all_urls>']},(request,callback)=>callback({cancel:ids.has(request.webContentsId)&&request.url!=='about:blank'&&!request.url.startsWith('data:image/')}));ses.on('will-download',(event,_item,source)=>{if(source&&ids.has(source.id))event.preventDefault();});}
  ids.add(contents.id);children.add(child);child.once('closed',()=>{children.delete(child);ids.delete(contents.id);});
  child.webContents.setWindowOpenHandler(()=>({action:'deny'}));
  for(const event of ['will-navigate','will-redirect','will-attach-webview'])child.webContents.on(event,e=>e.preventDefault());
 });
 win.once('closed',()=>{for(const child of children)if(!child.isDestroyed())child.destroy();children.clear();});
}
module.exports={installPrintWindows};
