import {useEffect,useMemo,useRef,useState} from 'react';
import {createNFe55Client,NFe55Error} from '../../../../core/fiscalCustomer/nfe55.mjs';
import type {NFe55Status} from '../../../../core/fiscalCustomer/nfe55.mjs';
export function FiscalCorrection({token,isCurrent,sale,words:fw}:{token:string;isCurrent:()=>boolean;sale:string;words:Record<string,string>}){
 const api=useMemo(()=>createNFe55Client({token,isCurrent}),[token,isCurrent]);
 const [text,setText]=useState(''),[confirmed,setConfirmed]=useState(false),[result,setResult]=useState<NFe55Status|null>(null),[busy,setBusy]=useState(false),[error,setError]=useState(''),[checked,setChecked]=useState(false),[attempted,setAttempted]=useState(false);
 const generation=useRef(0),inFlight=useRef(false);
 useEffect(()=>{generation.current++;inFlight.current=false;setText('');setConfirmed(false);setResult(null);setBusy(false);setError('');setChecked(false);setAttempted(false);return()=>{generation.current++;};},[api,sale]);
 const canSend=checked&&result?.state==='correction_not_requested'&&!attempted;
 async function run(query:boolean){
  if(!isCurrent()||inFlight.current||(!query&&(!canSend||!confirmed)))return;
  const g=++generation.current;inFlight.current=true;setBusy(true);setError('');
  if(!query)setAttempted(true);
  try{
   const out=query?await api.queryCorrection(sale):await api.correction(sale,{correction:text,confirmed_allowed_fields:confirmed});
   if(g===generation.current&&isCurrent()){
    setResult(out);
    if(query){setChecked(true);if(out.state==='correction_not_requested')setAttempted(false);}
    else{setText('');setConfirmed(false);}
   }
  }catch(e){if(g===generation.current&&isCurrent()){setError(e instanceof NFe55Error?e.code:'network');if(!query){setText('');setConfirmed(false);}}}
  finally{if(g===generation.current){inFlight.current=false;setBusy(false);}}
 }
 return <section aria-label={fw.cceTitle}><h4>{fw.cceTitle}</h4><p>{fw.cceLimits}</p><p>{fw.cceQueryFirst}</p>
 <button type="button" disabled={busy||!isCurrent()} onClick={()=>void run(true)}>{fw.cceQuery}</button>
 {error&&<p role="alert">{fw['nfeError_'+error]||fw.nfeFailure}</p>}
 {result&&<div role="status"><p>{fw[result.state]||fw.nfeFailure}</p>{result.number&&<p>{fw.cceSequence}: {result.number}</p>}{result.message&&<p>{result.message}</p>}{result.archive_state&&result.archive_state!=='archived'&&<p role="alert">{fw.nfeArchivePending}</p>}</div>}
 <form onSubmit={e=>{e.preventDefault();void run(false);}}><fieldset disabled={busy||!isCurrent()||!canSend}><legend>{fw.cceText}</legend><label>{fw.cceText}<textarea required minLength={15} maxLength={1000} autoComplete="off" value={text} onChange={e=>setText(e.target.value)}/></label><label><input type="checkbox" required checked={confirmed} onChange={e=>setConfirmed(e.target.checked)}/>{fw.cceConfirm}</label><button>{fw.cceSend}</button></fieldset></form></section>;
}
