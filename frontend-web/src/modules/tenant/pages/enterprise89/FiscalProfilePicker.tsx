import {useEffect,useRef,useState} from 'react';
import {FiscalCustomerError,createFiscalCustomer} from '../../../../core/fiscalCustomer/client.mjs';
import type {ProfilePage} from '../../../../core/fiscalCustomer/client.mjs';
type Props={api:ReturnType<typeof createFiscalCustomer>;isCurrent:()=>boolean;disabled:boolean;words:Record<string,string>;onChoose:(id:string)=>void};
export function FiscalProfilePicker({api,isCurrent,disabled,words:fw,onChoose}:Props){
 const [page,setPage]=useState<ProfilePage|null>(null),[busy,setBusy]=useState(false),[error,setError]=useState('');
 const sequence=useRef(0),alive=useRef(true);
 useEffect(()=>{alive.current=true;return()=>{alive.current=false;sequence.current++;};},[]);
 useEffect(()=>{sequence.current++;setPage(null);setBusy(false);setError('');},[api]);
 async function read(after=''){
  if(disabled||busy||!isCurrent())return;
  const n=++sequence.current;setBusy(true);setError('');
  try{const value=await api.list(after,20);if(alive.current&&n===sequence.current&&isCurrent())setPage(value);}
  catch(e){if(alive.current&&n===sequence.current&&isCurrent())setError(e instanceof FiscalCustomerError?e.code:'network');}
  finally{if(alive.current&&n===sequence.current)setBusy(false);}
 }
 return <section aria-label={fw.profileList}><h3>{fw.profileList}</h3><p>{fw.profileListHint}</p>
  <button type="button" className="brand-secondary" disabled={disabled||busy||!isCurrent()} onClick={()=>void read()}>{fw.profileList}</button>
  {busy&&<p role="status">{fw.loading}</p>}{error&&<p role="alert">{fw[error]||fw.request}</p>}
  {page&&isCurrent()&&<><ul>{page.rows.map(p=><li key={p.input.profile_id}><button type="button" className="brand-secondary" disabled={disabled||busy} onClick={()=>{if(isCurrent())onChoose(p.input.profile_id);}}>{p.input.legal_name} · {p.input.model} · {fw[p.input.environment]} · {fw.revision} {p.revision} · {p.input.profile_id}</button></li>)}</ul>
   {!page.rows.length&&<p role="status">{fw.profileListEmpty}</p>}
   {page.has_more&&<button type="button" className="brand-secondary" disabled={disabled||busy} onClick={()=>void read(page.next_after)}>{fw.profileListNext}</button>}
  </>}
 </section>;
}
