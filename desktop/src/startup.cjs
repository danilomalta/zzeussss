'use strict';
// Status only: never log request URLs, connection files, credentials or renderer text.
function rendering(app,environment=process.env,log=console.info,argv=[]){if(!Array.isArray(argv))throw Error('Argumentos inválidos.');const mode=argv.includes('--software-rendering')?'software':environment.TITAN_RENDERING_MODE||'default';if(!['default','software'].includes(mode))throw Error('TITAN_RENDERING_MODE deve ser default ou software.');if(mode==='software'){app.disableHardwareAcceleration();log('TitanSystem: renderização por software solicitada; sandbox preservado.');}return mode;}
function existingInstance(log=console.info){log('TitanSystem: outra instância já está aberta neste perfil. A solicitação foi encaminhada à janela existente. Feche-a antes de abrir uma compilação diferente.');}
function watchWindow(win,{dialog,log=console.info,setTimer=setTimeout,clearTimer=clearTimeout,timeout=20000}={}){
 let disposed=false,visible=false,reported=false;
 const timer=setTimer(()=>{if(!disposed&&!visible)failure('A janela não ficou pronta no prazo esperado.','startup_timeout');},timeout);timer?.unref?.();
 function failure(message,code){if(disposed||reported||win.isDestroyed())return;reported=true;clearTimer(timer);log('TitanSystem: falha de inicialização ('+code+'). Perfis e operações pendentes preservados.');dialog.showErrorBox('TitanSystem — abertura',message+'\nFeche a janela e tente o modo de renderização por software. Os dados e as operações pendentes foram preservados.');}
 function show(){if(disposed||visible||win.isDestroyed())return;visible=true;clearTimer(timer);win.show();log('TitanSystem: janela disponibilizada pelo processo desktop. O aceite visual e o login continuam dependendo da interface.');}
 function failLoad(event,code,description,url,mainFrame){if(mainFrame!==true||code===-3)return;failure('Não foi possível carregar a interface compilada.','load_'+(Number.isSafeInteger(code)?code:'unknown'));}
 function gone(){failure('O processo da interface foi interrompido. Não reenvie operações sem consultar o recibo.','renderer_gone');}
 function dispose(){if(disposed)return;disposed=true;clearTimer(timer);win.removeListener('ready-to-show',show);win.webContents.removeListener('did-finish-load',show);win.webContents.removeListener('did-fail-load',failLoad);win.webContents.removeListener('render-process-gone',gone);win.removeListener('closed',dispose);}
 win.once('ready-to-show',show);win.webContents.once('did-finish-load',show);win.webContents.on('did-fail-load',failLoad);win.webContents.on('render-process-gone',gone);win.once('closed',dispose);
 return dispose;
}
module.exports={rendering,existingInstance,watchWindow};
