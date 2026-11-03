'use strict';
const {contextBridge,ipcRenderer}=require('electron');
// Only the dedicated local settings page receives two bounded operations.
if(location.protocol==='titan:'&&location.hostname==='app'&&location.pathname==='/desktop-setup/index.html'){
 contextBridge.exposeInMainWorld('titanConnection',Object.freeze({read:()=>ipcRenderer.invoke('titan:connection:read'),save:config=>ipcRenderer.invoke('titan:connection:save',config)}));
}
