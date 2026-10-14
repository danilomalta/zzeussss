import {useEffect,useState} from 'react';
import {useNavigate} from 'react-router-dom';
import {useTranslation} from 'react-i18next';
import {useLocalAccess} from '../../../core/local/LocalAccess';
import {useLocalSession} from '../../../core/local/useLocalSession';
import {contextKey,contextOf} from '../../../core/context74/model.mjs';
import {readTracking,type TrackingResult} from '../../../core/map78/tracking';
import {deliveryTimes,locationState} from '../../../core/map78/times.mjs';
import {words78} from '../../../core/map78/words';
import {stageReport76} from '../../../core/loss76/report76.mjs';
import '../../../core/loss76/sessionReport76';

export default function DeliveryTracking78({close,selectState,period}:{close:()=>void;selectState:(uf:string)=>void;period:string}){
 const {capabilities}=useLocalAccess(),session=useLocalSession(s=>s.session),token=useLocalSession(s=>s.token),{i18n}=useTranslation(),navigate=useNavigate();
 const w=words78(i18n.resolvedLanguage||i18n.language);
 const allowed=capabilities?.role==='owner'||(capabilities?.license?.state==='active'&&capabilities.license.modules.includes('logistics')&&capabilities.permissions.includes('view_orders'));
 const [result,setResult]=useState<TrackingResult>({state:'unavailable'}),[loading,setLoading]=useState(false),[error,setError]=useState(false),[courier,setCourier]=useState(''),[revision,setRevision]=useState(0);
 useEffect(()=>{
  setResult({state:'unavailable'});setError(false);
  if(!allowed||!session||!token)return;
  const controller=new AbortController();
  let live=true;
  setLoading(true);
  try{
   const ctx=contextOf(session,capabilities);
   readTracking(ctx,controller.signal).then(value=>{
    if(!live)return;
    if(value.state==='ready'&&(value.snapshot.context.tenant_id!==ctx.tenant_id||value.snapshot.context.store_id!==ctx.store_id))throw Error('context');
    setResult(value);
   }).catch(()=>{if(live)setError(true);}).finally(()=>{if(live)setLoading(false);});
  }catch{setError(true);setLoading(false);}
  return ()=>{live=false;controller.abort();};
 },[allowed,session,token,capabilities,revision]);
 const people=allowed&&result.state==='ready'?result.snapshot.couriers:[];
 const cutoff=period==='today'?new Date().setHours(0,0,0,0):Date.now()-Number(period)*86400000;
 const deliveries=allowed&&result.state==='ready'?result.snapshot.deliveries.filter(row=>(!courier||row.courier_id===courier)&&row.queued_at&&Date.parse(row.queued_at)>=cutoff).slice(0,500):[];
 const headers=[w.deliveries,w.couriers,w.waitingTime,w.travelTime,w.destinationTime,w.totalTime];
 const matrix:(string|number)[][]=[headers,...deliveries.map(row=>{
  const times=deliveryTimes(row);
  return [row.reference,people.find(p=>p.id===row.courier_id)?.name||'—',times.waiting??'—',times.travel??'—',times.atDestination??'—',times.total??'—'];
 })];
 const canSheets=capabilities?.role==='owner';
 function csv(){
  const cell=(value:string|number)=>'"'+(typeof value==='string'&&/^\s*[=+@-]/.test(value)?"'"+value:String(value)).replace(/"/g,'""')+'"';
  const url=URL.createObjectURL(new Blob(['\uFEFF'+matrix.map(row=>row.map(cell).join(';')).join('\r\n')],{type:'text/csv;charset=utf-8'}));
  const link=document.createElement('a');link.href=url;link.download='Titan_Entregas.csv';document.body.append(link);link.click();link.remove();setTimeout(()=>URL.revokeObjectURL(url),1000);
 }
 function sheets(){if(!session||!canSheets)return;const ctx=contextOf(session,capabilities),id=crypto.randomUUID();stageReport76(contextKey(ctx),id,w.deliveries,matrix);navigate('/local/sheets?report='+id);}
 return <aside className="distribution-tracking" aria-label={w.deliveries}>
  <header><h2>{w.deliveries}</h2><button className="brand-secondary" onClick={close} aria-label={w.close}>×</button></header>
  {!allowed?<p role="status">{w.noPermission}</p>:<>
   <label>{w.couriers}<select value={courier} onChange={e=>setCourier(e.target.value)} disabled={!people.length}><option value="">{w.allCouriers}</option>{people.map(person=><option key={person.id} value={person.id}>{person.name}</option>)}</select></label>
   {loading?<p role="status">{w.loading}</p>:error?<p role="alert">{w.error}</p>:result.state==='unavailable'?<div className="distribution-tracking-empty"><h3>{w.awaiting}</h3><p>{w.connectionHint}</p></div>:<>
    <ul>{people.filter(p=>!courier||p.id===courier).map(person=><li key={person.id}><strong>{person.name}</strong><span>{w.lastPosition}: {person.position?new Intl.DateTimeFormat(i18n.language,{dateStyle:'short',timeStyle:'short'}).format(new Date(person.position.recorded_at)):w.unknown}</span><span>{locationState(person.position)==='stale'?w.stale:person.state||'—'}</span>{person.state&&<button className="brand-secondary" onClick={()=>selectState(person.state!)}>{person.state}</button>}</li>)}</ul>
    {!deliveries.length?<p>{w.noDeliveries}</p>:deliveries.map(row=>{const time=deliveryTimes(row);return <section className="distribution-delivery" key={row.id}><strong>{row.reference}</strong><span>{w[row.status]}</span><dl>{(['waiting','travel','atDestination','total'] as const).map((field,index)=><div key={field}><dt>{headers[index+2]}</dt><dd>{time[field]===null?'—':new Intl.NumberFormat(i18n.language).format(time[field]!)} {time[field]===null?'':w.minutes}</dd></div>)}</dl></section>;})}
   </>}
   <div className="distribution-tracking-actions"><button className="brand-secondary" disabled={loading} onClick={()=>setRevision(v=>v+1)}>{w.refresh}</button><button className="brand-secondary" disabled={!deliveries.length} onClick={csv}>{w.exportCSV}</button><button className="brand-secondary" disabled={!deliveries.length||!canSheets} onClick={sheets}>{w.openSheets}</button></div>
  </>}
 </aside>;
}
