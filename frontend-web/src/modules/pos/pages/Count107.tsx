import {useEffect,useId,useRef,useState} from 'react';
import {useTranslation} from 'react-i18next';
import {countSummary107,updateCount107,stepCount107,confirmCount107} from '../../../core/local/count107.mjs';
import type {CountSummary107} from '../../../core/local/count107.mjs';
import {words107} from '../../../core/local/words107';
import './count107.css';

export default function Count107({counts,confirmed,locked,onChange,onConfirm}:{counts:string[];confirmed:boolean;locked:boolean;onChange:(counts:string[])=>void;onConfirm:()=>Promise<boolean>}){
 const {i18n}=useTranslation(),locale=i18n.resolvedLanguage||i18n.language,w=words107(locale),id=useId();
 const [stamp,setStamp]=useState<string|null>(null),[ack,setAck]=useState(false),[error,setError]=useState(''),[saving,setSaving]=useState(false);
 const inputs=useRef<(HTMLInputElement|null)[]>([]),heading=useRef<HTMLHeadingElement>(null),reviewButton=useRef<HTMLButtonElement>(null),latch=useRef(false),alive=useRef(true);
 useEffect(()=>{alive.current=true;return()=>{alive.current=false;};},[]);
 useEffect(()=>{if(stamp)heading.current?.focus();},[stamp]);
 let summary:CountSummary107|null=null;try{summary=countSummary107(counts);}catch{/* Corrupt input is not an empty drawer. */}
 const money=(n:number)=>new Intl.NumberFormat(locale,{style:'currency',currency:'BRL'}).format(n/100),number=(n:number)=>new Intl.NumberFormat(locale).format(n),disabled=locked||saving;
 if(!summary)return <p role="alert">{w.invalidCount107}</p>;
 const changed=stamp!==null&&stamp!==summary.signature;
 function change(index:number,value:string){try{onChange(updateCount107(counts,index,value));setAck(false);setError('');}catch{setError('invalidCount107');}}
 function step(index:number,delta:number){try{onChange(stepCount107(counts,index,delta));setAck(false);setError('');}catch{setError('invalidCount107');}}
 async function submit(){if(latch.current||disabled||!stamp||!ack)return;latch.current=true;setSaving(true);setError('');try{confirmCount107(counts,stamp,ack);const ok=await onConfirm();if(alive.current&&!ok)setError('saveFailed107');}catch{if(alive.current)setError('saveFailed107');}finally{latch.current=false;if(alive.current)setSaving(false);}}
 return <section className="count107" aria-label={w.title107} aria-busy={saving}>
 <header><div><h3>{w.title107}</h3><p>{w.blind107}</p></div><span role="status">{confirmed?w.confirmed107:w.unconfirmed107}</span></header>
 {error&&<p role="alert">{w[error]}</p>}
 {stamp===null?<>
 <p>{w.keyboard107}</p>{(['notes','coins'] as const).map(kind=><fieldset key={kind} disabled={disabled}><legend>{w[kind+'107']}</legend><div className="count107-grid">{summary!.rows.filter(r=>r.kind===kind).map(row=><div className="count107-denom" key={row.value}>
 <strong>{money(row.value)}</strong><label htmlFor={id+'q'+row.index}>{w.quantity107}</label><div className="count107-counter"><button type="button" aria-label={w.remove107+' '+money(row.value)} disabled={disabled||row.quantity===0} onClick={()=>step(row.index,-1)}>−</button><input id={id+'q'+row.index} ref={el=>{inputs.current[row.index]=el;}} inputMode="numeric" pattern="[0-9]*" maxLength={9} autoComplete="off" value={counts[row.index]} disabled={disabled} aria-label={w.quantity107+' '+money(row.value)} onFocus={e=>e.currentTarget.select()} onChange={e=>change(row.index,e.target.value)} onKeyDown={e=>{if(e.key==='Enter'){e.preventDefault();const next=inputs.current[row.index+1];if(next)next.focus();else reviewButton.current?.focus();}else if(e.key==='ArrowUp'||e.key==='ArrowDown'){e.preventDefault();step(row.index,e.key==='ArrowUp'?1:-1);}}}/><button type="button" aria-label={w.add107+' '+money(row.value)} disabled={disabled||row.quantity>=999999999} onClick={()=>step(row.index,1)}>+</button></div><span>{w.subtotal107}: {money(row.subtotal)}</span></div>)}</div></fieldset>)}
 <button type="button" disabled={disabled||summary.pieces===0} onClick={()=>{if(!window.confirm(w.clearConfirm107))return;onChange(counts.map(()=>''));setAck(false);setError('');inputs.current[0]?.focus();}}>{w.clear107}</button>
 </>:<section className="count107-review"><h4 tabIndex={-1} ref={heading}>{w.review107}</h4>{changed&&<p role="alert">{w.changed107}</p>}{summary.total===0?<p>{w.zero107}</p>:<div className="count107-table"><table><caption>{w.breakdown107}</caption><thead><tr><th>{w.denom107}</th><th>{w.quantity107}</th><th>{w.subtotal107}</th></tr></thead><tbody>{summary.rows.filter(r=>r.quantity>0).map(r=><tr key={r.value}><td>{money(r.value)}</td><td>{number(r.quantity)}</td><td>{money(r.subtotal)}</td></tr>)}</tbody></table></div>}
 <label className="count107-check"><input type="checkbox" checked={ack&&!changed} disabled={disabled||changed} onChange={e=>setAck(e.target.checked)}/>{summary.total===0?w.zeroAck107:w.ack107}</label><button type="button" disabled={disabled} onClick={()=>{setStamp(null);setAck(false);setError('');}}>{w.edit107}</button></section>}
 <div className="count107-summary"><div><span>{w.notes107}</span><strong>{money(summary.notes)}</strong></div><div><span>{w.coins107}</span><strong>{money(summary.coins)}</strong></div><div><span>{w.pieces107}</span><strong>{number(summary.pieces)}</strong></div><div className="count107-total"><span>{w.total107}</span><strong>{money(summary.total)}</strong></div>
 {stamp===null?<button ref={reviewButton} type="button" className="brand-primary" disabled={disabled} onClick={()=>{setStamp(summary!.signature);setAck(false);setError('');}}>{w.next107}</button>:<button type="button" className="brand-primary" disabled={disabled||changed||!ack} onClick={()=>void submit()}>{saving?w.saving107:w.confirm107}</button>}</div>
 <p>{w.local107}</p>
 </section>;
}
