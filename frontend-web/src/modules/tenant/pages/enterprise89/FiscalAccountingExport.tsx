import {useEffect,useRef,useState} from 'react';
type PickerWindow=Window&{showSaveFilePicker?:(options:unknown)=>Promise<{createWritable:()=>Promise<{write:(data:Blob)=>Promise<void>;close:()=>Promise<void>;abort:()=>Promise<void>}>}>};
export function FiscalAccountingExport({token,isCurrent,words:fw,locale}:{token:string;isCurrent:()=>boolean;words:Record<string,string>;locale:string}){
 const [from,setFrom]=useState(''),[until,setUntil]=useState(''),[busy,setBusy]=useState(false),[message,setMessage]=useState('');const generation=useRef(0),active=useRef<AbortController|null>(null);
 useEffect(()=>{generation.current++;active.current?.abort();setFrom('');setUntil('');setMessage('');setBusy(false);return()=>{generation.current++;active.current?.abort();};},[token,isCurrent]);
 async function download(){
  if(busy||!isCurrent()||!from||!until)return;const g=++generation.current;setBusy(true);setMessage('');const controller=new AbortController();active.current=controller;let timer:ReturnType<typeof setTimeout>|undefined;
  try{
   const picker=(window as PickerWindow).showSaveFilePicker;
   const handle=picker?await picker.call(window,{suggestedName:'fiscal-homologacao.zip',types:[{description:'ZIP',accept:{'application/zip':['.zip']}}]}):null;
   if(g!==generation.current||!isCurrent())return;
   const start=new Date(from+'T00:00:00Z'),end=new Date(until+'T00:00:00Z');end.setUTCDate(end.getUTCDate()+1);
   if(!Number.isFinite(start.getTime())||!Number.isFinite(end.getTime())||start>=end)throw Error();
   const lang=['pt-BR','en','es','fr','zh-CN'].includes(locale)?locale:'pt-BR';timer=setTimeout(()=>controller.abort(),60000);
   const query=new URLSearchParams({from:start.toISOString(),until:end.toISOString(),lang});
   const response=await fetch('/local/v1/fiscal/accounting-export?'+query,{headers:{Authorization:`Bearer ${token}`},signal:controller.signal,cache:'no-store',credentials:'omit'});
   if(!response.ok||!response.headers.get('content-type')?.includes('application/zip'))throw Error();
   const length=Number(response.headers.get('content-length')||0);if(length>60*1024*1024)throw Error();
   const blob=await response.blob();if(blob.size>60*1024*1024||g!==generation.current||!isCurrent())throw Error();
   if(handle){const writer=await handle.createWritable();try{if(g!==generation.current||!isCurrent())throw Error();await writer.write(blob);await writer.close();}catch(e){await writer.abort();throw e;}}
   else{const url=URL.createObjectURL(blob);try{const a=document.createElement('a');a.href=url;a.download='fiscal-homologacao.zip';a.click();}finally{URL.revokeObjectURL(url);}}
   if(g===generation.current&&isCurrent())setMessage('exportSaved');
  }catch{if(g===generation.current&&isCurrent())setMessage('exportFailed');}finally{if(timer)clearTimeout(timer);if(g===generation.current)setBusy(false);}
 }
 return <section aria-labelledby="fiscal-export-title"><h3 id="fiscal-export-title">{fw.exportTitle}</h3><p>{fw.exportScope}</p><p role="note">{fw.exportPrivacy}</p><form onSubmit={e=>{e.preventDefault();void download();}}><fieldset disabled={busy||!isCurrent()}><label>{fw.exportFrom}<input required type="date" value={from} onChange={e=>setFrom(e.target.value)}/></label><label>{fw.exportUntil}<input required type="date" value={until} onChange={e=>setUntil(e.target.value)}/></label><button>{fw.exportDownload}</button></fieldset></form>{message&&<p role="status">{fw[message]}</p>}</section>;
}
