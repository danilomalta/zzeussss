import {useMemo,useRef,useState} from 'react';
import {createNFe55Client,NFe55Error} from '../../../../core/fiscalCustomer/nfe55.mjs';
import type {NFe55OriginBound,NFe55Status} from '../../../../core/fiscalCustomer/nfe55.mjs';

type Props={token:string;isCurrent:()=>boolean;words:Record<string,string>};
const emptyItem=()=>({product_id:'',quantity_milli:'',unit_price_cents:'',total_cents:'',quote_operation_id:''});
const empty=()=>({operation_id:'',kind:'',source_id:'',referenced_access_key:'',freight_mode:'9',confirmed:false,recipient:{cnpj:'',cpf:'',name:'',uf:''},items:[emptyItem()]});
const emptyEmit=()=>({nature:'',purpose:'',freight_mode:'9',confirmed:false,recipient:{cnpj:'',cpf:'',name:'',ie:'',address:{street:'',number:'',district:'',city_code:'',city:'',uf:'',postal_code:''}}});

export function FiscalNFeOrigin({token,isCurrent,words:fw}:Props){
 const api=useMemo(()=>createNFe55Client({token,isCurrent}),[token,isCurrent]);
 const [input,setInput]=useState(empty),[bound,setBound]=useState<NFe55OriginBound|null>(null),[emit,setEmit]=useState(emptyEmit);
 const [status,setStatus]=useState<NFe55Status|null>(null),[busy,setBusy]=useState(false),[error,setError]=useState(''),[attempted,setAttempted]=useState(false);
 const sequence=useRef(0);
 const disabled=busy||!isCurrent();
 function fail(e:unknown){setError(e instanceof NFe55Error?e.code:'network');}
 async function bind(){
  const n=++sequence.current;setBusy(true);setError('');setBound(null);setStatus(null);setAttempted(false);
  try{
   const items=input.items.map(row=>({product_id:row.product_id,quantity_milli:Number(row.quantity_milli),unit_price_cents:Number(row.unit_price_cents),total_cents:Number(row.total_cents),tax:{quote_operation_id:row.quote_operation_id}}));
   const out=await api.bindOrigin({...input,items});
   if(n===sequence.current&&isCurrent()){setBound(out);setEmit({...emptyEmit(),purpose:input.kind==='customer_return'?'4':'1',recipient:{...emptyEmit().recipient,cnpj:input.recipient.cnpj,cpf:input.recipient.cpf,name:input.recipient.name,uf:input.recipient.uf,address:{...emptyEmit().recipient.address,uf:input.recipient.uf}}});}
  }catch(e){if(n===sequence.current&&isCurrent())fail(e);}finally{if(n===sequence.current)setBusy(false);}
 }
 async function send(){
  if(!bound||!isCurrent())return;const n=++sequence.current;setBusy(true);setError('');setAttempted(true);
   try{const out=await api.emitOrigin(bound.origin_id,emit);if(n===sequence.current&&isCurrent())setStatus(out);}
  catch(e){if(n===sequence.current&&isCurrent()){fail(e);if(e instanceof NFe55Error&&['input_missing','invalid_fiscal_request','tax_mapping_unsupported'].includes(e.code))setAttempted(false);}}
  finally{if(n===sequence.current)setBusy(false);}
 }
 return <section aria-label={fw.nfeOriginTitle} className="fiscal-nfe55-origin"><h4>{fw.nfeOriginTitle}</h4><p>{fw.nfeOriginHelp}</p>
  <form onSubmit={e=>{e.preventDefault();void bind();}}><fieldset disabled={disabled}>
   <legend>{fw.nfeOriginBind}</legend>
   <label>{fw.nfeOriginOperation}<input required autoComplete="off" maxLength={128} value={input.operation_id} onChange={e=>setInput({...input,operation_id:e.target.value})}/></label>
   <label>{fw.nfeOriginKind}<select required value={input.kind} onChange={e=>setInput({...input,kind:e.target.value})}><option value="">{fw.choose}</option><option value="inbound_receipt">{fw.nfeOriginInbound}</option><option value="customer_return">{fw.nfeOriginReturn}</option></select></label>
   <label>{fw.nfeOriginSource}<input required autoComplete="off" maxLength={128} value={input.source_id} onChange={e=>setInput({...input,source_id:e.target.value})}/></label>
   <label>{fw.nfeOriginKey}<input required autoComplete="off" inputMode="numeric" maxLength={44} value={input.referenced_access_key} onChange={e=>setInput({...input,referenced_access_key:e.target.value})}/></label>
   <label>{fw.nfeFreight}<select required value={input.freight_mode} onChange={e=>setInput({...input,freight_mode:e.target.value})}>{['9','0','1','2'].map(code=><option key={code} value={code}>{fw[code==='9'?'nfeNoFreight':`nfeFreight_${code}`]}</option>)}</select></label>
   {(['name','cnpj','cpf','uf'] as const).map(key=><label key={key}>{fw['nfe_'+key]}<input required={key==='name'||key==='uf'} autoComplete="off" maxLength={key==='uf'?2:key==='name'?60:14} value={input.recipient[key]} onChange={e=>setInput({...input,recipient:{...input.recipient,[key]:e.target.value}})}/></label>)}
   {input.items.map((row,i)=><fieldset key={i}><legend>{fw.nfeItems} {i+1}</legend>
    <label>{fw.nfeProduct}<input required autoComplete="off" value={row.product_id} onChange={e=>{const items=[...input.items];items[i]={...row,product_id:e.target.value};setInput({...input,items});}}/></label>
    <label>{fw.nfeQuantity}<input required inputMode="numeric" value={row.quantity_milli} onChange={e=>{const items=[...input.items];items[i]={...row,quantity_milli:e.target.value};setInput({...input,items});}}/></label>
    <label>{fw.nfeValue}<input required inputMode="numeric" value={row.total_cents} onChange={e=>{const items=[...input.items];items[i]={...row,total_cents:e.target.value};setInput({...input,items});}}/></label>
    <label>{fw.nfeOriginUnit}<input required inputMode="numeric" value={row.unit_price_cents} onChange={e=>{const items=[...input.items];items[i]={...row,unit_price_cents:e.target.value};setInput({...input,items});}}/></label>
    <label>{fw.nfeOriginQuote}<input required autoComplete="off" value={row.quote_operation_id} onChange={e=>{const items=[...input.items];items[i]={...row,quote_operation_id:e.target.value};setInput({...input,items});}}/></label>
   </fieldset>)}
   <label><input type="checkbox" required checked={input.confirmed} onChange={e=>setInput({...input,confirmed:e.target.checked})}/>{fw.nfeOriginConfirm}</label>
   <button className="brand-secondary">{fw.nfeOriginBind}</button>
  </fieldset></form>
  {error&&<p role="alert">{fw['nfeError_'+error]||fw.nfeFailure}</p>}
  {bound&&<p role="status">{fw.nfeOriginFrozen}: {bound.origin_id}</p>}
  {bound&&<form onSubmit={e=>{e.preventDefault();void send();}}><fieldset disabled={disabled||attempted}>
   <legend>{fw.nfeOriginEmit}</legend>
   <label>{fw.nfeNature}<input required maxLength={60} autoComplete="off" value={emit.nature} onChange={e=>setEmit({...emit,nature:e.target.value})}/></label>
   <label>{fw.nfePurpose}<select required value={emit.purpose} onChange={e=>setEmit({...emit,purpose:e.target.value})}><option value="">{fw.choose}</option><option value="1">{fw.nfeNormal}</option><option value="4">{fw.nfeOriginReturn}</option></select></label>
   <label>{fw.nfeFreight}<select required value={emit.freight_mode} onChange={e=>setEmit({...emit,freight_mode:e.target.value})}>{['9','0','1','2'].map(code=><option key={code} value={code}>{fw[code==='9'?'nfeNoFreight':`nfeFreight_${code}`]}</option>)}</select></label>
   {(['name','cnpj','cpf','ie'] as const).map(key=><label key={key}>{fw['nfe_'+key]}<input required={key==='name'} autoComplete="off" maxLength={key==='name'?60:14} value={emit.recipient[key]} onChange={e=>setEmit({...emit,recipient:{...emit.recipient,[key]:e.target.value}})}/></label>)}
   {(['street','number','district','city_code','city','uf','postal_code'] as const).map(key=><label key={key}>{fw['nfe_'+key]}<input required={!['city_code','postal_code'].includes(key)} autoComplete="off" maxLength={key==='uf'?2:key==='city_code'?7:key==='postal_code'?8:60} value={emit.recipient.address[key]} onChange={e=>setEmit({...emit,recipient:{...emit.recipient,address:{...emit.recipient.address,[key]:e.target.value}}})}/></label>)}
   <label><input type="checkbox" required checked={emit.confirmed} onChange={e=>setEmit({...emit,confirmed:e.target.checked})}/>{fw.nfeOriginEmitConfirm}</label>
   <button className="brand-primary">{fw.nfeOriginEmit}</button>
  </fieldset></form>}
  {status&&<p role="status">{fw[status.state==='authorized_homologation'?'nfeAuthorized':'nfePending']}{status.error_code?` · ${fw['nfeError_'+status.error_code]||status.error_code}`:''}</p>}
 </section>;
}
