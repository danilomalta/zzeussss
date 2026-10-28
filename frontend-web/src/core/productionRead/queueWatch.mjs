// Read-only queue awareness. Never trusts event payloads, writes storage or calls an API.
export function watchProductionQueue({source,target,visibility,onChange,onError}) {
 if(!source||typeof source.scope!=='string'||!source.scope||typeof source.pending!=='function'||!target?.addEventListener||typeof onChange!=='function'||typeof onError!=='function')throw Error('productionInvalid');
 const key='titan-production-commands:v1:'+source.scope;
 let stopped=false,previous;
 function refresh(){
  if(stopped)return;
  let value,signature;
  try{value=source.pending();signature='value:'+JSON.stringify(value);}
  catch{if(previous!=='error'){previous='error';onError();}return;}
  if(signature!==previous){previous=signature;onChange(value);}
 }
 function storage(event){if(event.key===null||event.key===key)refresh();}
 function visible(){if(visibility?.visibilityState==='visible')refresh();}
 target.addEventListener('storage',storage);
 target.addEventListener('focus',refresh);
 target.addEventListener('pageshow',refresh);
 visibility?.addEventListener('visibilitychange',visible);
 refresh();
 return()=>{stopped=true;target.removeEventListener('storage',storage);target.removeEventListener('focus',refresh);target.removeEventListener('pageshow',refresh);visibility?.removeEventListener('visibilitychange',visible);};
}
