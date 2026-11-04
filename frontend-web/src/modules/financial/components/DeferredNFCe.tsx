import {useEffect,useMemo,useRef,useState} from 'react';
import {useLocalSession} from '../../../core/local/useLocalSession';
import {retailClient} from '../../../core/retail/client.mjs';
import {deferredFiscalClient,type Result} from '../../../core/deferredFiscal/client.mjs';
import {deferredWords} from '../../../core/deferredFiscal/words.mjs';
import {words96} from '../../../core/local/words96';

export default function DeferredNFCe({saleId,locale}:{saleId:string;locale:string}){
 const token=useLocalSession(s=>s.token),session=useLocalSession(s=>s.session),w=deferredWords(locale),statusWords=words96(locale);
 const client=useMemo(()=>token&&session?deferredFiscalClient({token,session,getCurrent:()=>useLocalSession.getState()}):null,[token,session]);
 const accessClient=useMemo(()=>token&&session?retailClient({token,session,getCurrent:()=>useLocalSession.getState()}):null,[token,session]);
 const [owner,setOwner]=useState(false),[busy,setBusy]=useState(false),[attempted,setAttempted]=useState(false),[error,setError]=useState(false),[result,setResult]=useState<Result|null>(null);
 const [code,setCode]=useState(''),[nature,setNature]=useState(''),[presence,setPresence]=useState(''),[confirmed,setConfirmed]=useState(false);
 const live=useRef(true),latch=useRef(false);
 useEffect(()=>{live.current=true;setOwner(false);accessClient?.request('/access').then(v=>{if(live.current)setOwner(v.can_configure===true);}).catch(()=>{if(live.current)setOwner(false);});return()=>{live.current=false;client?.dispose();accessClient?.dispose();};},[client,accessClient]);
 async function act(emit:boolean){
  if(!client||!owner||latch.current)return;latch.current=true;setBusy(true);setError(false);if(emit)setAttempted(true);
  try{const v=await(emit?client.emit(saleId,{payment_code:code,nature,presence,confirmed}):client.query(saleId));if(live.current)setResult(v);}
  catch{if(live.current)setError(true);}finally{latch.current=false;if(live.current)setBusy(false);}
 }
 const stateKey=result?.state==='authorized_homologation'?'checkoutFiscalAuthorized':result?.state==='failed'?'checkoutFiscalFailed':result?.state==='cancelled_homologation'?'checkoutFiscalCancelled':result?.state==='cancellation_failed'?'checkoutFiscalCancellationFailed':'checkoutFiscalPending';
 return <section className="brand-panel" aria-busy={busy} aria-label={w.title}><h3>{w.title}</h3><strong role="note">{w.banner}</strong><p>{w.notice}</p><p>{w.review}</p>
 {!owner?<p>{w.denied}</p>:<>
 <label>{w.code}<select disabled={busy||attempted} value={code} onChange={e=>setCode(e.target.value)}><option value="">{w.choose}</option><option value="21">{w.store}</option><option value="91">{w.later}</option></select></label>
 <label>{w.nature}<input disabled={busy||attempted} maxLength={60} value={nature} onChange={e=>setNature(e.target.value)}/></label>
 <label>{w.presence}<select disabled={busy||attempted} value={presence} onChange={e=>setPresence(e.target.value)}><option value="">{w.choose}</option><option value="1">{w.person}</option><option value="4">{w.delivery}</option></select></label>
 <label><input type="checkbox" checked={confirmed} disabled={busy||attempted} onChange={e=>setConfirmed(e.target.checked)}/>{w.confirm}</label>
 <button type="button" className="brand-secondary" disabled={busy||attempted||!confirmed||!code||!nature.trim()||!presence} onClick={()=>void act(true)}>{w.emit}</button>
 <button type="button" className="brand-secondary" disabled={busy} onClick={()=>void act(false)}>{w.query}</button>
 </>}
 {error&&<p role="alert">{w.failed}</p>}{result&&<p role="status" aria-live="polite">{w.state}: {statusWords[stateKey]} {result.message} {result.key}</p>}
 </section>;
}
