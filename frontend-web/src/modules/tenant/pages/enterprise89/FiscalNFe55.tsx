import {useEffect,useMemo,useRef,useState} from 'react';
import {createNFe55Client,NFe55Error} from '../../../../core/fiscalCustomer/nfe55.mjs';
import type {NFe55Preview,NFe55Status} from '../../../../core/fiscalCustomer/nfe55.mjs';

type Props={token:string;isCurrent:()=>boolean;words:Record<string,string>;locale:string};
const empty=()=>({nature:'',purpose:'',freight_mode:'',recipient:{cnpj:'',cpf:'',name:'',ie:'',address:{street:'',number:'',district:'',city_code:'',city:'',uf:'',postal_code:''}}});
export function FiscalNFe55({token,isCurrent,words:w,locale}:Props){
 const api=useMemo(()=>createNFe55Client({token,isCurrent}),[token,isCurrent]);
 const [sale,setSale]=useState(''),[preview,setPreview]=useState<NFe55Preview|null>(null),[input,setInput]=useState(empty),[status,setStatus]=useState<NFe55Status|null>(null);
 const [busy,setBusy]=useState(false),[error,setError]=useState(''),[attempted,setAttempted]=useState(false),[reason,setReason]=useState(''),[confirmed,setConfirmed]=useState(false);
 const sequence=useRef(0),cancelID=useRef('');
 useEffect(()=>{sequence.current++;setSale('');setPreview(null);setInput(empty());setStatus(null);setError('');setBusy(false);setAttempted(false);setReason('');setConfirmed(false);cancelID.current='';return()=>{sequence.current++;};},[api]);
 function fail(e:unknown){setError(e instanceof NFe55Error?e.code:'network');}
 async function load(){const n=++sequence.current;setBusy(true);setError('');setPreview(null);setStatus(null);setAttempted(false);cancelID.current='';try{const p=await api.preview(sale);if(n===sequence.current&&isCurrent()){setPreview(p);setStatus(p.status);setInput(empty());}}catch(e){if(n===sequence.current&&isCurrent())fail(e);}finally{if(n===sequence.current)setBusy(false);}}
 async function action(kind:'emit'|'query'|'cancel'|'danfe'){
  if(!preview||!isCurrent())return;const n=++sequence.current;setBusy(true);setError('');
  try{
   if(kind==='danfe'){const blob=await api.danfe(preview.sale_id);if(n!==sequence.current||!isCurrent())return;const url=URL.createObjectURL(blob);const a=document.createElement('a');a.href=url;a.download='nfe55-homologacao.pdf';a.click();URL.revokeObjectURL(url);return;}
   let result:NFe55Status;
   if(kind==='emit'){setAttempted(true);result=await api.emit(preview.sale_id,input);}
   else if(kind==='query')result=await api.query(preview.sale_id);
   else{cancelID.current||=crypto.randomUUID();result=await api.cancel(preview.sale_id,{operation_id:cancelID.current,reason,confirmed});}
   if(n===sequence.current&&isCurrent()){setStatus(result);if(result.error_code==='focus_request_unsupported')setAttempted(false);if(kind==='emit')setInput(empty());}
  }catch(e){if(n===sequence.current&&isCurrent()){fail(e);if(kind==='emit'&&e instanceof NFe55Error&&['input_missing','invalid_fiscal_request','tax_mapping_unsupported'].includes(e.code))setAttempted(false);}}
  finally{if(n===sequence.current)setBusy(false);}
 }
 const money=(c:number)=>new Intl.NumberFormat(locale,{style:'currency',currency:'BRL'}).format(c/100);
 const states:Record<string,string>={authorized_homologation:'nfeAuthorized',failed:'nfeRejected',cancelled_homologation:'nfeCancelled',cancellation_failed:'nfeCancelError',pending:'nfePending',recovery_required:'nfePending',not_requested:'nfeNotRequested'};
 const disabled=busy||!isCurrent();
 return <section aria-label={w.nfeTitle} className="fiscal-nfe55"><h3>{w.nfeTitle}</h3><p role="note" className="nfe55-banner"><strong>{w.nfeBanner}</strong></p><p>{w.nfeScope}</p><p>{w.nfeUnavailable}</p>
 <form onSubmit={e=>{e.preventDefault();void load();}}><label>{w.nfeSale}<input required autoComplete="off" value={sale} disabled={busy} maxLength={128} onChange={e=>{setSale(e.target.value);setPreview(null);setStatus(null);setInput(empty());setAttempted(false);}}/></label><button disabled={disabled} className="brand-secondary">{w.nfeLoad}</button></form>
 {error&&<p role="alert">{w['nfeError_'+error]||w.nfeFailure}</p>}
 {preview&&<><table><caption>{w.nfeItems}</caption><thead><tr><th>{w.nfeProduct}</th><th>{w.nfeQuantity}</th><th>{w.nfeValue}</th></tr></thead><tbody>{preview.items.map((item,i)=><tr key={i}><td>{item.product_id}</td><td>{(item.quantity_milli/1000).toFixed(3)}</td><td>{money(item.total_cents)}</td></tr>)}</tbody></table><p>{w.nfeTotal}: {money(preview.total_cents)}</p>
 {!preview.mapping_supported&&<p role="alert">{w.nfeError_tax_mapping_unsupported}</p>}
 <form onSubmit={e=>{e.preventDefault();void action('emit');}}><fieldset disabled={disabled||attempted||!preview.mapping_supported}>
 <legend>{w.nfeDetails}</legend><label>{w.nfeNature}<input required value={input.nature} maxLength={60} autoComplete="off" onChange={e=>setInput({...input,nature:e.target.value})}/></label>
 <label>{w.nfePurpose}<select required value={input.purpose} onChange={e=>setInput({...input,purpose:e.target.value})}><option value="">{w.choose}</option><option value="1">{w.nfeNormal}</option></select></label>
 <label>{w.nfeFreight}<select required value={input.freight_mode} onChange={e=>setInput({...input,freight_mode:e.target.value})}><option value="">{w.choose}</option><option value="9">{w.nfeNoFreight}</option></select></label>
 <div className="erp89-form">{(['name','cnpj','cpf','ie'] as const).map(key=><label key={key}>{w['nfe_'+key]}<input required={key==='name'} maxLength={key==='name'?60:14} autoComplete="off" value={input.recipient[key]} onChange={e=>setInput({...input,recipient:{...input.recipient,[key]:e.target.value}})}/></label>)}
 {(['street','number','district','city_code','city','uf','postal_code'] as const).map(key=><label key={key}>{w['nfe_'+key]}<input required={!['city_code','postal_code'].includes(key)} autoComplete="off" maxLength={key==='uf'?2:key==='city_code'?7:key==='postal_code'?8:60} value={input.recipient.address[key]} onChange={e=>setInput({...input,recipient:{...input.recipient,address:{...input.recipient.address,[key]:e.target.value}}})}/></label>)}</div>
 <p>{w.nfePrivacy}</p><button className="brand-primary">{w.nfeEmit}</button></fieldset></form>
 {status&&<section role="status"><h4>{w[states[status.state]]||w.nfePending}</h4>{status.message&&<p>{status.message}</p>}{status.error_code&&<p>{w['nfeError_'+status.error_code]||w.nfeFailure}</p>}</section>}
 <button type="button" disabled={disabled} onClick={()=>void action('query')}>{w.nfeQuery}</button>
 <button type="button" disabled={disabled||status?.state!=='authorized_homologation'} onClick={()=>void action('danfe')}>{w.nfeDanfe}</button>
 {['authorized_homologation','cancellation_failed'].includes(status?.state||'')&&<form onSubmit={e=>{e.preventDefault();void action('cancel');}}><fieldset disabled={disabled}><legend>{w.nfeCancel}</legend><p>{w.nfeCancelOnly}</p><label>{w.nfeReason}<textarea required minLength={15} maxLength={255} value={reason} onChange={e=>setReason(e.target.value)}/></label><label><input type="checkbox" required checked={confirmed} onChange={e=>setConfirmed(e.target.checked)}/>{w.nfeConfirm}</label><button disabled={!confirmed}>{w.nfeCancel}</button></fieldset></form>}
 </>}
 </section>;
}
