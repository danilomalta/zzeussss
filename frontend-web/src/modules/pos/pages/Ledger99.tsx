import {LedgerSummary109,LedgerDetail109} from './CashDesk109';
import {entryStamp109} from '../../../core/local/cashDesk109.mjs';
import {useContext,useEffect,useMemo,useState} from 'react';
import {useTranslation} from 'react-i18next';
import {ClosingSourceContext92} from '../../../core/local/closingSource92';
import type {ClosingSale92} from '../../../core/local/closing92.mjs';
import type {Till94,Actor94} from '../../../core/local/till94.mjs';
import {ledger99,filterLedger99,range99,type LedgerEntry99} from '../../../core/local/ledger99.mjs';
import {validateSales97,csv97} from '../../../core/local/closingReview97.mjs';
import {words99} from '../../../core/local/words99';
import {print95} from '../../../core/local/print95';
import './cash99.css';
type Load99={key:string;sales:ClosingSale92[];complete:boolean;busy:boolean;error:string;at:string};
export default function Ledger99({data,token,actor,ready}:{data:Till94;token:string;actor:Actor94;ready:boolean}){
 const {i18n}=useTranslation(),locale=i18n.resolvedLanguage||i18n.language,w=words99(locale),source=useContext(ClosingSourceContext92),key=JSON.stringify([data.scope,data.cash,token]);
 const [load,setLoad]=useState<Load99>(),[refresh,setRefresh]=useState(0),[origin,setOrigin]=useState('sale'),[method,setMethod]=useState('all'),[state,setState]=useState('all'),[query,setQuery]=useState(''),[from,setFrom]=useState(''),[to,setTo]=useState(''),[order,setOrder]=useState('desc'),[page,setPage]=useState(0),[notice,setNotice]=useState(''),[selected,setSelected]=useState<{key:string;stamp:string}|null>(null);
 const current=load?.key===key?load:undefined,money=(n:number)=>new Intl.NumberFormat(locale,{style:'currency',currency:'BRL'}).format(n/100),date=(s:string)=>new Intl.DateTimeFormat(locale,{dateStyle:'short',timeStyle:'short'}).format(new Date(s));
 useEffect(()=>{setPage(0);setOrigin('sale');setMethod('all');setState('all');setQuery('');setFrom('');setTo('');setNotice('');setSelected(null);},[key]);
 useEffect(()=>{
  if(!ready||!data.cash)return;let disposed=false;let controller:AbortController|undefined;let reading=false;
  async function read(){if(reading||document.hidden)return;reading=true;controller=new AbortController();setLoad(previous=>({...previous?.key===key?previous:{key,sales:[],complete:false,at:'',error:''},busy:true}));
   try{const result=await source.read(token,data.cash,controller.signal);validateSales97(result.sales);if(typeof result.complete!=='boolean'||result.sales.reduce((n,s)=>n+s.payments.length,0)>20000)throw Error('invalid99');if(!disposed)setLoad({key,sales:result.sales,complete:result.complete,busy:false,error:'',at:new Date().toISOString()});}
   catch{if(!disposed)setLoad(previous=>({...previous?.key===key?previous:{key,sales:[],at:''},complete:false,busy:false,error:'partial99'}));}finally{reading=false;}}
  void read();const timer=window.setInterval(()=>void read(),60000),visible=()=>{if(!document.hidden)void read();};document.addEventListener('visibilitychange',visible);return()=>{disposed=true;controller?.abort();clearInterval(timer);document.removeEventListener('visibilitychange',visible);};
 },[ready,key,source,refresh,token,data.cash]);
 const result=useMemo(()=>{if(!ready)return undefined;try{return ledger99(current?.sales||[],data,current?.complete===true&&!current.busy&&!current.error);}catch{return null;}},[data,ready,current]);
 const filtered=useMemo(()=>{if(!result)return {rows:[] as LedgerEntry99[],error:''};try{return {rows:filterLedger99(result.entries,{origin,method,state,query,...range99(from,to),order}),error:''};}catch{return {rows:[],error:'range99'};}},[result,origin,method,state,query,from,to,order]);
 useEffect(()=>setPage(0),[origin,method,state,query,from,to,order]);const totalPages=Math.max(1,Math.ceil(filtered.rows.length/25)),activePage=Math.min(page,totalPages-1);
 const detail=selected?filtered.rows.find(r=>r.key===selected.key):undefined;
 function choose(r:LedgerEntry99){setSelected({key:r.key,stamp:entryStamp109(r)});setTimeout(()=>document.querySelector<HTMLElement>('.desk109-detail h3')?.focus(),0);}
 const label=(r:LedgerEntry99)=>`${w[r.kind]||w.unknown99} ${new Intl.NumberFormat(locale).format(r.label)}${r.part?' · '+w.part+' '+new Intl.NumberFormat(locale).format(r.part):''}`;
 const sourceLabel=(r:LedgerEntry99)=>w[r.origin==='sale'?'saleOrigin99':r.origin==='request'?'requestOrigin99':'auditOrigin99'];
 const csvRows=(rows:LedgerEntry99[])=>[[w.date,w.origin99,w.kind,w.method,w.amount,w.state,w.reason],...rows.map(r=>[date(r.at),sourceLabel(r),label(r),r.method?w[r.method]:'—',money(r.amount),r.origin==='sale'?w.committed:w[r.state]||w.local99,r.reason])];
 function download(rows:LedgerEntry99[],name:string){try{const url=URL.createObjectURL(new Blob([csv97([...csvRows(rows),[w.warning97,'','','','','',w.ledgerHelp99+' '+w.notBalance99+' '+(!result?.complete?w.partial99:w.source99)],[w.last99,current?.at?date(current.at):'—']])],{type:'text/csv;charset=utf-8'})),a=document.createElement('a');a.href=url;a.download=name;a.click();setTimeout(()=>URL.revokeObjectURL(url),1000);}catch{setNotice('exportError97');}}
 function reset(){setOrigin('sale');setMethod('all');setState('all');setQuery('');setFrom('');setTo('');setOrder('desc');setPage(0);}
 return <section className="cash99" aria-label={w.title99} aria-busy={!ready||current?.busy}>
  <header><div><h3>{w.title99}</h3><p>{w.ledgerHelp99}</p></div><button disabled={!ready||!data.cash||current?.busy} onClick={()=>setRefresh(n=>n+1)}>{current?.busy?w.loading99:w.refresh99}</button></header>
  <p>{w.auto99}{current?.at&&<> · {w.last99}: {date(current.at)}</>}</p>{current?.error&&<p role="alert">⚠ {w[current.error]}</p>}{result===null&&<p role="alert">{w.invalid99}</p>}{notice&&<p role="alert">{w[notice]}</p>}
  {result&&!result.complete&&<p role="status">{w.partial99}</p>}
  <div className="cash99-kpis">{[[w.cash99,result?.cash],[w.total99,result?.total],[w.requests99,result?.requests]].map(([name,value],n)=><article key={n}><span>{name}</span><strong>{typeof value==='number'?n===2?new Intl.NumberFormat(locale).format(value):money(value):'—'}</strong></article>)}</div><p>{w.notBalance99}</p>
  <div className="cash99-filters">
   <label>{w.search99}<input type="search" value={query} maxLength={400} placeholder={w.searchHelp99} onChange={e=>setQuery(e.target.value)}/></label>
   <label>{w.origin99}<select value={origin} onChange={e=>{setOrigin(e.target.value);setMethod('all');setState('all');}}>{[['all','all99'],['sale','saleOrigin99'],['request','requestOrigin99'],['audit','auditOrigin99']].map(([v,k])=><option key={v} value={v}>{w[k]}</option>)}</select></label>
   <label>{w.method}<select value={method} disabled={['request','audit'].includes(origin)} onChange={e=>setMethod(e.target.value)}>{['all','cash','debit','credit','pix','creditAccount'].map(v=><option key={v} value={v}>{w[v==='all'?'all99':v]}</option>)}</select></label>
   <label>{w.state}<select value={state} onChange={e=>setState(e.target.value)}>{['all','committed','requested','approved','denied','withdrawn','local99'].map(v=><option key={v} value={v}>{w[v==='all'?'all99':v]}</option>)}</select></label>
   <label>{w.from99}<input type="date" value={from} onChange={e=>setFrom(e.target.value)}/></label><label>{w.to99}<input type="date" value={to} onChange={e=>setTo(e.target.value)}/></label>
   <label>{w.order99}<select value={order} onChange={e=>setOrder(e.target.value)}>{[['desc','newest99'],['asc','oldest99']].map(([v,k])=><option key={v} value={v}>{w[k]}</option>)}</select></label><button onClick={reset}>{w.reset99}</button>
  </div>{filtered.error&&<p role="alert">{w[filtered.error]}</p>}
  {result&&!filtered.error&&<LedgerSummary109 entries={filtered.rows} complete={result.complete}/>}
  <div className="cash99-actions"><button disabled={!ready||!result||!!filtered.error} onClick={()=>download(filtered.rows,'Titan_Livro_Caixa.csv')}>{w.export99}</button><button disabled={!ready||!result||!!filtered.error} onClick={()=>{try{const rows=csvRows(filtered.rows);print95(w.title99,rows.map(row=>[row.slice(0,4).join(' · '),row.slice(4).join(' · ')]),w.ledgerHelp99+' '+w.notBalance99+' '+(!result?.complete?w.partial99:''),{lang:locale});}catch{setNotice('popupBlocked');}}}>{w.print99}</button></div>
  <p role="status">{new Intl.NumberFormat(locale).format(filtered.rows.length)} {w.rows99}</p>
  {!filtered.rows.length?<article><h4>{w.empty99}</h4><p>{w.emptyHelp99}</p></article>:<div className="cash99-table" tabIndex={0} role="region" aria-label={w.title99}><table><caption>{w.title99}</caption><thead><tr>{[w.date,w.origin99,w.kind,w.method,w.amount,w.state,w.reason].map(t=><th key={t} scope="col">{t}</th>)}</tr></thead><tbody>{filtered.rows.slice(activePage*25,activePage*25+25).map(r=><tr key={r.key}><td>{date(r.at)}</td><td><span className="cash99-badge">{sourceLabel(r)}</span></td><th scope="row"><button type="button" className="cash99-row109" onClick={()=>choose(r)}>{label(r)}</button></th><td>{r.method?w[r.method]:'—'}</td><td>{money(r.amount)}</td><td>{r.origin==='sale'?w.committed:<>{w[r.state]||w.local99}<small>{w.notApplied}</small></>}</td><td>{r.reason||'—'}{r.actor&&<small>{r.actor===actor.id?w.currentUser:w.otherUser}</small>}</td></tr>)}</tbody></table></div>}
  <nav aria-label={w.pagination97}><button disabled={activePage===0} onClick={()=>setPage(activePage-1)}>{w.previous97}</button><span>{new Intl.NumberFormat(locale).format(activePage+1)} / {new Intl.NumberFormat(locale).format(totalPages)}</span><button disabled={activePage+1>=totalPages} onClick={()=>setPage(activePage+1)}>{w.next97}</button></nav>
  {detail&&selected&&<LedgerDetail109 entry={detail} stamp={selected.stamp} actor={actor} onReload={()=>choose(detail)} onClose={()=>setSelected(null)}/>}
  <details><summary>{w.activity99}</summary><p>{w.localOnly99}</p><button disabled={!ready||!result} onClick={()=>download(result?.entries.filter(r=>r.origin==='audit')||[],'Titan_Atividade_Caixa.csv')}>{w.activity99}</button></details>
 </section>;
}
