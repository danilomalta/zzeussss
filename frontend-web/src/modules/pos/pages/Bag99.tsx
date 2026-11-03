import {BagBoard109} from './CashDesk109';
import {useEffect,useState,useRef} from 'react';
import {Link} from 'react-router-dom';
import {useTranslation} from 'react-i18next';
import {words99} from '../../../core/local/words99';
import {bag99,bagAction99} from '../../../core/local/ledger99.mjs';
import {seal94,bagStep94,manager94,type Till94,type Actor94,type Bag94} from '../../../core/local/till94.mjs';
import {readClosing92} from '../../../core/local/closingStorage92';
import {cashDenominations,count92} from '../../../core/local/closing92.mjs';
import {print95} from '../../../core/local/print95';
import './cash99.css';
type Props={data:Till94;actor:Actor94;locked:boolean;canCreate:boolean;mutate:(fn:(data:Till94)=>void)=>Promise<boolean>;onDirty:(dirty:boolean)=>void};
export default function Bag99({data,actor,locked,canCreate,mutate,onDirty}:Props){
 const {i18n}=useTranslation(),locale=i18n.resolvedLanguage||i18n.language,w=words99(locale),money=(n:number)=>new Intl.NumberFormat(locale,{style:'currency',currency:'BRL'}).format(n/100),date=(s:string)=>new Intl.DateTimeFormat(locale,{dateStyle:'short',timeStyle:'short'}).format(new Date(s));
 const [destination,setDestination]=useState(''),[policy,setPolicy]=useState('dual'),[counts,setCounts]=useState(()=>cashDenominations.map(()=>'')),[reason,setReason]=useState(''),[dirty,setDirty]=useState(false),[notice,setNotice]=useState(''),[busy,setBusy]=useState(false),[selectedBag,setSelectedBag]=useState('');
 useEffect(()=>{setDestination('');setPolicy('dual');setCounts(cashDenominations.map(()=>''));setReason('');setDirty(false);setNotice('');setSelectedBag('');},[data.scope,data.cash,actor.id]);
 useEffect(()=>{onDirty(dirty);return()=>onDirty(false);},[dirty,onDirty]);
 const generation=useRef(0);useEffect(()=>{generation.current++;return()=>{generation.current++;};},[data.scope,data.cash,actor.id]);
 const actionLatch=useRef(false);
 const change=()=>{setDirty(true);setNotice('');};
 let counted:number|null=null;try{if(counts.some(s=>s!==''))counted=count92(counts);}catch{/* Display a translated validation message instead of the raw error. */}
 let valid=true;try{for(const b of data.bags)bag99(b,data.cash);}catch{valid=false;}
 async function run(fn:(data:Till94)=>void){if(locked||busy||actionLatch.current)return;actionLatch.current=true;setBusy(true);setNotice('');try{if(await mutate(fn)){setDirty(false);setNotice('saved99');}else setNotice('error99');}catch{setNotice('error99');}finally{actionLatch.current=false;setBusy(false);}}
 async function seal(){if(locked||busy||!canCreate)return;const version=generation.current;setBusy(true);setNotice('');try{const count=await readClosing92(data.scope,data.cash);if(generation.current!==version)return;if(!count.countConfirmed){setNotice('closeFirst99');return;}const amount=count92(count.counts);if(amount<=0){setNotice('emptyBag99');return;}if(!window.confirm(w.sealConfirm99+' '+money(amount)))return;const scope=data.scope,cash=data.cash;if(await mutate(d=>{if(d.scope!==scope||d.cash!==cash)throw Error('conflict');seal94(d,actor,amount,destination,policy,count.countConfirmed);})){setDirty(false);setNotice('saved99');}}catch{setNotice('error99');}finally{setBusy(false);}}
 function label(b:Bag94){try{print95(w.label99,[[w.bagCode,b.id],[w.destination,b.destination],[w.amount,money(b.amount)],[w.policy,w[b.policy]],[w.state,w[b.state]],...b.chain.map(e=>[w[e.stage],date(e.at)+' · '+(e.actor===actor.id?w.currentUser:w.otherUser)] as [string,string])],w.localOnly99,{paper:'80',lang:locale});}catch{setNotice('popupBlocked');}}
 async function copy(id:string){try{await navigator.clipboard.writeText(id);setNotice('copied99');}catch{setNotice('copyFailed99');}}
 const disabled=locked||busy||!valid;
 useEffect(()=>{if(selectedBag)document.getElementById('bag109-active')?.scrollIntoView({block:'nearest'});},[selectedBag]);
 return <section className="cash99" aria-label={w.bag} aria-busy={busy}>
  <header><div><h3>{w.bag}</h3><p>{w.bagHelp99}</p></div><span className="cash99-badge">{w.local99}</span></header><p>{w.localOnly99}</p>{notice&&<p role="status">{w[notice]}</p>}{!valid&&<p role="alert">{w.invalidBag99}</p>}
  <BagBoard109 data={data} actor={actor} locked={disabled} onSelect={id=>{if(id===selectedBag)return;if(dirty&&!window.confirm(w.unsaved))return;setSelectedBag(id);setCounts(cashDenominations.map(()=>''));setReason('');setDirty(false);setNotice('');}}/>
  {valid&&data.bags.length===0&&<form onSubmit={e=>{e.preventDefault();void seal();}}><Link to="/local/cash/register">{w.countFirst}</Link><div className="cash99-filters"><label>{w.destination}<input name="destination99" value={destination} maxLength={160} minLength={3} required disabled={disabled||!canCreate} onChange={e=>{change();setDestination(e.target.value);}}/></label><label>{w.policy}<select name="policy99" value={policy} disabled={disabled||!canCreate} onChange={e=>{change();setPolicy(e.target.value);}}><option value="dual">{w.dual}</option>{manager94(actor)&&<option value="self">{w.self}</option>}</select></label></div><button className="brand-primary" disabled={disabled||!canCreate||destination.trim().length<3}>{w.seal}</button></form>}
  {valid&&data.bags.filter(b=>b.id===selectedBag).map(b=>{const action=bagAction99(b,actor),nextLabel=action.next==='handed'?w.handover:action.next==='received'?w.receiveBag:w.verifyBag;return <article id="bag109-active" className="cash99-bag" key={b.id}>
   <header><h4>{w.bagCode}</h4><span className="cash99-badge">{w[b.state]}</span></header><code>{b.id}</code><strong className="cash99-amount">{money(b.amount)}</strong><p>{w.destination}: {b.destination} · {w[b.policy]}</p>
   <ol aria-label={w.stage99} className="cash99-stages">{['sealed','handed','received','verified'].map((stage,n)=>{const e=b.chain[n];return <li key={stage} aria-current={!e&&n===b.chain.length?'step':undefined}><strong>{e?'✓ ':''}{w[stage]}</strong>{e?<><span>{date(e.at)} · {e.actor===actor.id?w.currentUser:w.otherUser}</span>{e.count!==undefined&&<span>{w.counted}: {money(e.count)} · {w.difference}: {money(e.count-b.amount)}{e.reason?' · '+e.reason:''}</span>}</>:<span>{w.pending}</span>}</li>;})}</ol>
   {action.reason&&<p role="status">{w[action.reason]}</p>}
   {b.state==='received'&&action.next==='verified'&&<form onSubmit={e=>{e.preventDefault();if(counted===null){setNotice('countZero99');return;}if(!window.confirm(w.confirmBag))return;void run(d=>bagStep94(d,actor,b.id,'verified',counted!,reason));}}>
    <h4>{w.count99}</h4><p>{w.countHelp99}</p>{counted===null&&counts.some(v=>v!=='')&&<p role="alert">{w.invalid99}</p>}<div className="cash99-denoms">{cashDenominations.map((amount,n)=><label key={amount}>{money(amount)}<input aria-label={w.units99+' '+money(amount)} name={'count99-'+amount} type="text" inputMode="numeric" pattern="[0-9]*" maxLength={9} disabled={disabled} value={counts[n]} placeholder="0" onChange={e=>{change();setCounts(rows=>rows.map((v,i)=>i===n?e.target.value:v));}}/></label>)}</div><p>{w.counted}: <strong className="cash99-amount">{counted===null?'—':money(counted)}</strong></p>{counted!==null&&<p>{w.difference}: <strong>{money(counted-b.amount)}</strong></p>}
    <label>{w.differenceReason}<textarea name="differenceReason99" maxLength={400} minLength={counted!==null&&counted!==b.amount?3:0} required={counted!==null&&counted!==b.amount} disabled={disabled} value={reason} onChange={e=>{change();setReason(e.target.value);}}/></label><button className="brand-primary" disabled={disabled||counted===null||counted!==b.amount&&reason.trim().length<3}>{w.verifyBag}</button>
   </form>}
   <div className="cash99-actions">{action.next&&action.next!=='verified'&&<button className="brand-primary" disabled={disabled} onClick={()=>{if(window.confirm(w.confirmBag))void run(d=>bagStep94(d,actor,b.id,action.next));}}>{nextLabel}</button>}<button disabled={disabled} onClick={()=>label(b)}>{w.printLabel}</button><button disabled={disabled} onClick={()=>void copy(b.id)}>{w.copy99}</button></div>
  </article>;})}

 </section>;
}
