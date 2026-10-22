import Archives103 from './workspace103/Archives103';
import Sales103 from './workspace103/Sales103';
import Queue103 from './workspace103/Queue103';
import './workspace103/workspace103.css';
import OfficialReport101 from './OfficialReport101';
import {useContext,useEffect,useMemo,useState} from 'react';
import {useTranslation} from 'react-i18next';
import {GatewayContext95} from '../../../core/local/service95';
import {ClosingSourceContext92} from '../../../core/local/closingSource92';
import {useLocalSession} from '../../../core/local/useLocalSession';
import {words95} from '../../../core/local/words95';
import {words97} from '../../../core/local/words97';
import {words98} from '../../../core/local/words98';
import {reports95} from '../../../core/local/reportStore95';
import {readClosing92} from '../../../core/local/closingStorage92';
import {read94} from '../../../core/local/tillStorage94';
import {count92} from '../../../core/local/closing92.mjs';
import {csv97} from '../../../core/local/closingReview97.mjs';
import {reconciliationRows98,serviceFor98} from '../../../core/local/report98.mjs';
import {print95} from '../../../core/local/print95';
import type {Reply95} from '../../../core/local/desk95.mjs';
import type {Summary95} from '../../../core/local/report95.mjs';
import type {ClosingDraft92,ClosingSale92} from '../../../core/local/closing92.mjs';
import type {Till94} from '../../../core/local/till94.mjs';
import './report98.css';
import './report102.css';
import {words102} from '../../../core/local/words102';
type Props={scope:string;cash:string;service?:Reply95['report'];official?:Summary95};
type Loaded={key:string;at:string;loading:boolean;failed:boolean;count?:ClosingDraft92;till?:Till94;sales?:{sales:ClosingSale92[];complete:boolean}};
type Section={title:string;note?:string;rows:[string,string][]};
export default function Report98({scope,cash,service,official}:Props){
 const {i18n}=useTranslation(),locale=i18n.resolvedLanguage||i18n.language;
 const w=useMemo(()=>({...words95(locale),...words97(locale),...words98(locale),...words102(locale)}),[locale]);
 const money=(n:number)=>new Intl.NumberFormat(locale,{style:'currency',currency:'BRL'}).format(n/100);
 const date=(s:string)=>Number.isFinite(Date.parse(s))?new Intl.DateTimeFormat(locale,{dateStyle:'short',timeStyle:'short'}).format(new Date(s)):'—';
 const gateway=useContext(GatewayContext95),source=useContext(ClosingSourceContext92),token=useLocalSession(s=>s.token);
 const [archive,setArchive]=useState<{key:string;rows:Summary95[]}>({key:'',rows:[]}),[selection,setSelected]=useState({key:'',value:''}),[refresh,setRefresh]=useState(0),[loaded,setLoaded]=useState<Loaded>(),[error,setError]=useState(''),[page,setPage]=useState(0);
 const archiveKey=JSON.stringify([scope,gateway.mode]),archives=archive.key===archiveKey?archive.rows:[];
 const selected=selection.key===archiveKey?selection.value:'';
 const summary=official||archives.find(s=>s.cash===(selected||cash)),id=summary?.cash||selected||cash,key=JSON.stringify([scope,id,gateway.mode,token]);
 const data=loaded?.key===key?loaded:undefined,busy=!!id&&(!data||data.loading),currentService=serviceFor98(id,cash,service);
 useEffect(()=>{let active=true;setSelected({key:archiveKey,value:''});setArchive({key:archiveKey,rows:[]});void reports95(scope,gateway.mode).then(rows=>{if(active)setArchive({key:archiveKey,rows});}).catch(()=>{if(active)setError('storageError');});return()=>{active=false;};},[scope,gateway.mode,official,archiveKey]);
 useEffect(()=>{const c=new AbortController();setError('');setPage(0);setLoaded({key,at:'',loading:true,failed:false});if(!id)return()=>c.abort();void Promise.allSettled([readClosing92(scope,id),read94(scope,id),source.read(token||'',id,c.signal)]).then(([count,till,sales])=>{if(c.signal.aborted)return;setLoaded({key,at:new Date().toISOString(),loading:false,failed:[count,till,sales].some(r=>r.status==='rejected'),count:count.status==='fulfilled'?count.value:undefined,till:till.status==='fulfilled'?till.value:undefined,sales:sales.status==='fulfilled'?sales.value:undefined});});return()=>c.abort();},[scope,id,source,token,key,refresh]);
 const paymentResult=useMemo(()=>{if(!data?.sales)return undefined;try{return reconciliationRows98(data.sales.sales,data.count?.papers||[],data.sales.complete,data.count?.mode||'papers',w,n=>new Intl.NumberFormat(locale,{style:'currency',currency:'BRL'}).format(n/100));}catch{return null;}},[data,w,locale]);
 const officialRows:[string,string][]=[...(['opening','cashSales','withdrawals','deposits','refunds','debit','credit','pix','pending'] as const).map((k):[string,string]=>[k==='credit'?w.cardCredit:w[k],currentService&&Number.isSafeInteger(currentService[k])?money(currentService[k]):'—']),[w.expected,summary?money(summary.expected):currentService?money(currentService.expected):'—'],[w.counted,summary?money(summary.counted):currentService?money(currentService.counted):'—'],[w.difference,summary?money(summary.difference):currentService?money(currentService.difference):'—'],[w.reviewer,currentService?.reviewer||'—']];
 const localRows:[string,string][]=[[w.counted,data?.count?.countConfirmed?money(count92(data.count.counts)):'—']];
 const bags=data?.till?.bags||[],bagRows:[string,string][]=[];
 for(const bag of bags){bagRows.push([w.bagCode,bag.id],[w.amount,money(bag.amount)],[w.state,w[bag.state]||'—']);for(const event of bag.chain)bagRows.push([w[event.stage]||w.state,date(event.at)+' · '+(event.actor===scopeActor(scope)?w.currentUser:w.otherUser)]);}
 const sections:Section[]=[{title:w.previous,note:summary?w.archived+' · '+date(summary.at):currentService?undefined:w.noOfficial98,rows:officialRows},{title:w.payments98,note:paymentResult?w.scope98:paymentResult===null?w.invalid98:w.partial98,rows:paymentResult?.rows||[]},{title:w.local98,note:w.local97,rows:[...localRows,...bagRows]}];
 const printable:[string,string][]=[[w.updated98,data?.at?date(data.at):'—'],...sections.flatMap(s=>[[s.title,s.note||''] as [string,string],...s.rows]),[w.warning97,w.overlap98],[w.warning97,w.noSettlement97],...(!data?.sales?.complete||data.failed||paymentResult===null?[[w.warning97,w.partial98] as [string,string]]:[])];
 function download(){try{const url=URL.createObjectURL(new Blob([csv97([[w.kind,w.amount],...printable])],{type:'text/csv;charset=utf-8'})),a=document.createElement('a');a.href=url;a.download='Titan_Fechamento.csv';a.click();setTimeout(()=>URL.revokeObjectURL(url),1000);}catch{setError('exportError97');}}
 return <section className="report98" aria-label={w.unifiedReport} aria-busy={busy}>
  <header><div><h3>{w.unifiedReport}</h3><p>{w.same98}</p></div><button disabled={!id||busy} onClick={()=>setRefresh(n=>n+1)}>{busy?w.loading98:w.refresh98}</button></header>
  {archives.length>0&&!official&&<label>{w.archives}<select value={selected} onChange={e=>setSelected({key:archiveKey,value:e.target.value})}><option value="">{w.currentCash}</option>{archives.map((s,n)=><option key={s.cash} value={s.cash}>{w.closing} {n+1} · {date(s.at)}</option>)}</select></label>}
  {!id&&<p>{w.choose98}</p>}{busy&&<p role="status">{w.loading98}</p>}{error&&<p role="alert">{w[error]}</p>}{data?.failed&&<p role="status">⚠ {w.partial98}</p>}{data?.at&&<p>{w.updated98} {date(data.at)}</p>}
  <Archives103 key={archiveKey} rows={archives} scope={scope} mode={gateway.mode} locale={locale} locked={!!official} onOpen={value=>setSelected({key:archiveKey,value})}/><OfficialReport101 scope={scope} cash={id} demo={gateway.mode==='demo'} locale={locale}/><section className="report102-local"><h4>{w.localTitle}</h4><p>{w.localNote}</p>{(summary||currentService)&&<details><summary>{w.previous}</summary><p>{w.previousNote}</p><p>{sections[0].note}</p><dl>{officialRows.map(([label,value],n)=><div key={n}><dt>{label}</dt><dd>{value}</dd></div>)}</dl></details>}<div className="report98-grid">
   <article><details><summary>{w.paymentDetails}</summary><p>{w.scope98}</p>{paymentResult?<><div className="report98-kpis">{[[w.total97,paymentResult.review.total],[w.pending97,paymentResult.review.pending],[w.unverified98,paymentResult.review.unverified]].map(([label,value],n)=><div key={n}><span>{label}</span><strong>{value===null?'—':money(value as number)}</strong></div>)}</div>{!paymentResult.review.complete&&<p role="status">⚠ {w.partial97}</p>}<p>{w.overlap98}</p><div className="report98-table"><table><caption>{w.payments98}</caption><thead><tr>{[w.method97,w.expected97,w.electronic97,w.paper97,w.checked97,w.pending97].map(t=><th key={t} scope="col">{t}</th>)}</tr></thead><tbody>{paymentResult.review.methods.map(m=><tr key={m.method}><th scope="row">{w[m.method]}</th>{[m.expected,m.confirmed,m.documented,m.verified,m.pending].map((v,n)=><td key={n}>{v===null?'—':money(v)}</td>)}</tr>)}</tbody></table></div><p>{w.noSettlement97}</p></>:<p role="status">{busy?w.loading98:paymentResult===null?w.invalid98:w.partial98}</p>}</details></article></div>
  <article><details><summary>{w.local98}</summary><p>{w.local97}</p><dl><div><dt>{w.counted}</dt><dd>{localRows[0][1]}</dd></div></dl>{!data?.count?.countConfirmed&&<p>{w.noCount98}</p>}{!bags.length?<p>{w.noBag98}</p>:<><div className="report98-bags">{bags.slice(page*10,page*10+10).map(b=><details key={b.id}><summary>{w.bagCode} · {money(b.amount)} · {w[b.state]}</summary><p><code>{b.id}</code></p><ol>{b.chain.map((e,n)=><li key={n}>{w[e.stage]} · {date(e.at)} · {e.actor===scopeActor(scope)?w.currentUser:w.otherUser}</li>)}</ol></details>)}</div><nav aria-label={w.pagination97}><button disabled={!page} onClick={()=>setPage(p=>p-1)}>{w.previous97}</button><span>{page+1} / {Math.ceil(bags.length/10)}</span><button disabled={(page+1)*10>=bags.length} onClick={()=>setPage(p=>p+1)}>{w.next97}</button></nav></>}</details></article>
  <Sales103 key={key} scope={scope} cash={id} sales={data?.sales?.sales} complete={data?.sales?.complete===true} locale={locale}/><Queue103 key={key+'queue'} rows={data?.till?.requests||[]} available={!!data?.till} locale={locale}/><footer><button disabled={!id||busy} onClick={()=>{try{print95(w.unifiedReport,printable,w.reportHelp,{lang:locale});}catch{setError('popupBlocked');}}}>{w.localPrint}</button><button disabled={!id||busy} onClick={download}>{w.localExport}</button></footer></section>
 </section>;
}
function scopeActor(scope:string){try{return JSON.parse(scope)[3];}catch{return '';}}
