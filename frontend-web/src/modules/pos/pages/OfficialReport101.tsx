import {journalMoney} from '../../../core/cashJournal/model.mjs';
import {useEffect,useRef,useState} from 'react';
import {createOfficial101,type Official101} from '../../../core/local/official101.mjs';
import {useLocalSession} from '../../../core/local/useLocalSession';
import {words101} from '../../../core/local/words101';
import {csv97} from '../../../core/local/closingReview97.mjs';
import {print95} from '../../../core/local/print95';
import './official101.css';
import './report102.css';
import {words102} from '../../../core/local/words102';
const read=createOfficial101();
export default function OfficialReport101({scope,cash,demo,locale}:{scope:string;cash:string;demo:boolean;locale:string}){
 const token=useLocalSession(s=>s.token),w={...words101(locale),...words102(locale)},key=JSON.stringify([scope,cash,token,demo]);
 const [state,setState]=useState<{key:string;value?:Official101;error?:string;busy?:boolean}>({key:''});
 const request=useRef<AbortController>(),current=useRef(key);current.current=key;
 useEffect(()=>()=>{request.current?.abort();},[key]);
 const data=state.key===key?state:undefined;
 const money=(n:number)=>journalMoney(n,locale);
 const date=(s:string)=>new Intl.DateTimeFormat(locale,{dateStyle:'short',timeStyle:'short'}).format(new Date(s));
 const v=data?.value,rows:[string,string][]=v?[[w.closed,date(v.closed_at)],...(['opening','movement','expected','declared','difference','supply','withdrawal'] as const).map((k):[string,string]=>[w[k],money(v.cash[({opening:'opening_cents',movement:'movement_net_cents',expected:'expected_cents',declared:'declared_cents',difference:'difference_cents',supply:'supply_cents',withdrawal:'withdrawal_cents'} as const)[k]])]),[w.sales,new Intl.NumberFormat(locale).format(v.sale_count)],...(['gross','cancelled','net'] as const).map((k):[string,string]=>[w[k],money(v[({gross:'gross_sales_cents',cancelled:'cancelled_sales_cents',net:'net_sales_cents'} as const)[k]])]),[w.returned,v.returned_sales_cents===undefined?w.returnsUnavailable:money(v.returned_sales_cents)],[w.refunded,v.cash.customer_refund_cents===undefined?w.returnsUnavailable:money(v.cash.customer_refund_cents)],...v.payments.flatMap(p=>[[w[p.method]+' · '+w.parts,new Intl.NumberFormat(locale).format(p.parts)],[w[p.method]+' · '+w.confirmed,money(p.recorded_confirmed_cents)],[w[p.method]+' · '+w.pending,money(p.recorded_pending_cents)],[w[p.method]+' · '+w.reversed,money(p.recorded_reversed_cents)]] as [string,string][])]:[];
 async function load(){if(!token||demo||!cash||data?.busy)return;request.current?.abort();const c=new AbortController();request.current=c;setState({key,busy:true});try{const value=await read(token,cash,c.signal);if(!c.signal.aborted&&current.current===key)setState({key,value});}catch(e){if(c.signal.aborted||current.current!==key)return;const status=(e as {status?:number}).status;const error=status===401?'session':status===403?'permission':status===404?'unavailable':status===409?'conflict':status===422||(e as Error).message==='invalid101'?'invalid101':'failed';setState({key,error});if(status===401)useLocalSession.getState().invalidate();}}
 function download(){try{const url=URL.createObjectURL(new Blob([csv97([[w.title,w.note],...rows])],{type:'text/csv;charset=utf-8'})),a=document.createElement('a');a.href=url;a.download='Titan_Caixa_Oficial.csv';a.click();setTimeout(()=>URL.revokeObjectURL(url),1000);}catch{setState({key,error:'failed'});}}
 return <article className="official101" aria-label={w.title} aria-busy={!!data?.busy}><h4>{w.title}</h4><p>{w.note}</p>{demo?<p>{w.preview}</p>:<button disabled={!token||!cash||!!data?.busy} onClick={()=>void load()}>{data?.busy?w.loading:w.load}</button>}{data?.error&&<p role="alert">{w[data.error]}</p>}{v&&<><div className="report102-summary">{[['expected',v.cash.expected_cents],['declared',v.cash.declared_cents],['difference',v.cash.difference_cents]].map(([label,value])=><div key={label} className={label==='difference'?'report102-difference':''}><span>{w[label]}</span><strong>{money(value as number)}</strong></div>)}</div><p role="status" className={'report102-status '+(v.cash.difference_cents===0?'balanced':v.cash.difference_cents<0?'shortage':'surplus')}>{v.cash.difference_cents===0?'✓ ':v.cash.difference_cents<0?'− ':'+ '}{w[v.cash.difference_cents===0?'balanced':v.cash.difference_cents<0?'shortage':'surplus']}</p><details><summary>{w.details}</summary><dl>{rows.filter((_,i)=>![3,4,5].includes(i)).map(([label,value],i)=><div key={i}><dt>{label}</dt><dd>{value}</dd></div>)}</dl></details><footer><button onClick={download}>{w.export}</button><button onClick={()=>{try{print95(w.title,rows,w.note,{lang:locale});}catch{setState({key,error:'failed'});}}}>{w.print}</button></footer></>}</article>;
}
