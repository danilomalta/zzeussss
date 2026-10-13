import {useEffect,useState} from 'react';
import {Link,Navigate} from 'react-router-dom';
import {useTranslation} from 'react-i18next';
import {useLocalAccess} from '../../../core/local/LocalAccess';
import {allowedAreas} from '../../../core/local/accessModel.mjs';
import {localClient,useLocalSession} from '../../../core/local/useLocalSession';
import type {CashSession,SaleSummary} from '../../../core/local/localClient.mjs';
import {summarizeOwnerSales,formatOwnerMoney} from '../../../core/local/ownerSummary.mjs';
import pt from '../../../i18n/owner/pt-BR.json';
import en from '../../../i18n/owner/en.json';
import es from '../../../i18n/owner/es.json';
import fr from '../../../i18n/owner/fr.json';
import zh from '../../../i18n/owner/zh-CN.json';
import './ownerDashboard.css';
const translations:Record<string,typeof pt>={'pt-BR':pt,pt,en,es,fr,'zh-CN':zh,zh};
export default function LocalOwnerDashboard(){
 const {t,i18n}=useTranslation();const words=translations[i18n.resolvedLanguage||i18n.language]||pt;
 const {capabilities}=useLocalAccess();const token=useLocalSession(s=>s.token);
 const areas=allowedAreas(capabilities),owner=capabilities?.role==='owner',canRead=owner&&areas.includes('pos');
 const [rows,setRows]=useState<SaleSummary[]|null>(null),[cash,setCash]=useState<CashSession|null>(null),[cashReady,setCashReady]=useState(false);
 const [failed,setFailed]=useState(false),[loading,setLoading]=useState(false),[revision,setRevision]=useState(0),[updated,setUpdated]=useState<Date|null>(null);
 useEffect(()=>{let live=true;setRows(null);setCash(null);setCashReady(false);setUpdated(null);setFailed(false);
  if(!canRead||!token){setLoading(false);return;}
  setLoading(true);
  const sales=localClient.history(token).then(data=>{summarizeOwnerSales(data);if(live){setRows(data);setUpdated(new Date());}}).catch(()=>{if(live)setFailed(true);});
  const current=localClient.currentCash(token).then(data=>{if(live){setCash(data);setCashReady(true);}}).catch(()=>{if(live)setCashReady(false);});
  void Promise.all([sales,current]).finally(()=>{if(live)setLoading(false);});return()=>{live=false;};
 },[token,canRead,revision]);
 if(!owner)return <Navigate to="/local/home" replace/>;
 const locale=i18n.resolvedLanguage||'pt-BR',money=(value:number)=>formatOwnerMoney(value,locale);
 const date=(value:string|Date)=>new Intl.DateTimeFormat(locale,{dateStyle:'short',timeZone:'America/Sao_Paulo'}).format(new Date(value));
 const summary=rows?summarizeOwnerSales(rows):null,total=(summary?.completed||0)+(summary?.cancelled||0),angle=total?360*(summary?.completed||0)/total:0;
 const peak=Math.max(1,...(summary?.days.map(day=>day.cents)||[]));
 const unavailable=loading?words.loading:failed?words.failed:words.unavailable;
 const links=[{area:'map',path:'/local/map',name:words.map,icon:'⌖'},{area:'pos',path:'/local/pos',name:words.pos,icon:'▦'},{area:'catalog',path:'/local/catalog',name:words.catalog,icon:'▤'},{area:'orders',path:'/local/orders',name:words.orders,icon:'⇄'},{area:'plans',path:'/local/subscription',name:words.plans,icon:'⚙'}].filter(link=>areas.includes(link.area));
 const signed=['active','expired','not_yet_valid'].includes(capabilities?.license.state||'');
 return <main className="brand-app-content owner-dashboard">
  <header className="owner-heading"><div><span className="owner-eyebrow">TITAN</span><h1>{words.title}</h1><p>{words.subtitle}</p></div><button className="brand-primary" disabled={loading||!canRead} onClick={()=>setRevision(value=>value+1)}>{loading?words.loading:words.refresh}</button></header>
  <div className="owner-scope"><strong>{words.scope}</strong>{updated&&<span>{words.last}: {new Intl.DateTimeFormat(locale,{dateStyle:'short',timeStyle:'short',timeZone:'America/Sao_Paulo'}).format(updated)}</span>}</div>
  {failed&&<p className="owner-error" role="alert">{words.failed}</p>}
  <section className="owner-kpis" aria-label={words.title}>
   {[{title:words.sales,value:summary?money(summary.total_cents):'—',icon:'↗'},{title:words.completed,value:summary?String(summary.completed):'—',icon:'✓'},{title:words.cancelled,value:summary?String(summary.cancelled):'—',icon:'↶'},{title:words.cash,value:cashReady?(cash?words.open:words.closed):'—',icon:'▣'}].map((card,index)=><article className="brand-panel owner-kpi" key={card.title}><div><span>{card.title}</span><i aria-hidden="true">{card.icon}</i></div><strong>{card.value}</strong><small>{index===3&&!cashReady?words.unavailable:!summary?unavailable:words.scope}</small></article>)}
  </section>
  <p className="owner-data-note">{words.dataNote}</p>
  <div className="owner-charts">
   <section className="brand-panel"><h2>{words.trend}</h2>{summary?.days.length?<><div className="owner-bar-chart" role="img" aria-label={words.trend}>{summary.days.map(day=><div key={day.date} className="owner-bar-column"><span>{money(day.cents)}</span><div className="owner-bar-track"><div className="owner-bar" style={{height:`${Math.max(1,day.cents/peak*100)}%`}}/></div><small>{date(day.date+'T12:00:00-03:00')}</small></div>)}</div><details><summary>{words.recent}</summary><ul className="owner-chart-data">{summary.days.map(day=><li key={day.date}>{date(day.date+'T12:00:00-03:00')}: {money(day.cents)}</li>)}</ul></details></>:<div className="owner-empty"><span aria-hidden="true">▥</span><p>{rows?words.empty:unavailable}</p></div>}</section>
   <section className="brand-panel"><h2>{words.status}</h2>{total>0?<><div className="owner-donut" role="img" aria-label={`${words.completed}: ${summary?.completed}; ${words.cancelled}: ${summary?.cancelled}`} style={{background:`conic-gradient(var(--accent) ${angle}deg,var(--border) 0)`}}><div><strong>{total}</strong><small>{words.recent}</small></div></div><div className="owner-legend"><span><i/>{words.completed}: {summary?.completed}</span><span><i/>{words.cancelled}: {summary?.cancelled}</span></div></>:<div className="owner-empty"><span aria-hidden="true">◯</span><p>{rows?words.empty:unavailable}</p></div>}</section>
  </div>
  <div className="owner-finance">{[words.expenses,words.profit,words.losses,words.reserve].map(title=><section className="brand-panel" key={title}><h2>{title}</h2><strong>—</strong><p>{words.pending}</p></section>)}</div>
  <div className="owner-bottom"><section className="brand-panel"><h2>{words.recent}</h2>{rows?.length?<div className="owner-table-scroll"><table><thead><tr><th>{words.date}</th><th>{words.saleStatus}</th><th>{words.amount}</th></tr></thead><tbody>{rows.map(row=><tr key={row.sale_id}><td>{date(row.committed_at)}</td><td><span className={`owner-sale-badge ${row.status}`}>{row.status==='committed'?words.committed:words.cancelled}</span></td><td>{money(row.total_cents)}</td></tr>)}</tbody></table></div>:<p className="owner-no-data">{rows?words.empty:unavailable}</p>}</section>
   <section className="brand-panel owner-access"><h2>{words.quick}</h2><nav>{links.map(link=><Link key={link.area} className="brand-secondary" to={link.path}><span aria-hidden="true">{link.icon}</span>{link.name}<span aria-hidden="true">→</span></Link>)}</nav><h2>{words.modules}</h2><p>{t('subscription.state.'+(capabilities?.license.state||'unavailable'))}</p><strong>{signed?capabilities?.license.modules.length:'—'}</strong>{!signed&&<p>{words.moduleEmpty}</p>}</section></div>
  <footer>{words.footer}</footer>
 </main>;
}
