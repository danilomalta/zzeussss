import {lazy,Suspense,useCallback,useEffect,useMemo,useRef,useState} from 'react';
import {Link,useBlocker,useNavigate,useSearchParams} from 'react-router-dom';
import {useTranslation} from 'react-i18next';
import {useLocalAccess} from '../../../core/local/LocalAccess';
import {useLocalSession} from '../../../core/local/useLocalSession';
import {contextOf,contextKey} from '../../../core/context74/model.mjs';
import {OperationsContext84,type Ops84} from '../../../core/operations84/context';
import {empty84,examples84} from '../../../core/operations84/data';
import {allowed84,areas84,csv84,manager84} from '../../../core/operations84/rules.mjs';
import {read84,write84} from '../../../core/operations84/storage';
import {dictionary84} from '../../../core/operations84/words';
import type {Workspace84,Area84} from '../../../core/operations84/types';
import {stageReport76} from '../../../core/loss76/report76.mjs';
import '../../../core/loss76/sessionReport76';
import './operations84/workflows84.css';
const Cash84=lazy(()=>import('./operations84/Cash84'));
const Journal84=lazy(()=>import('./operations84/Journal84'));
const Returns84=lazy(()=>import('./operations84/Returns84'));
const Inventory84=lazy(()=>import('./operations84/Inventory84'));
const Quality84=lazy(()=>import('./operations84/Quality84'));
const Supply84=lazy(()=>import('./operations84/Supply84'));
const Market84=lazy(()=>import('./operations84/Market84'));
const Performance84=lazy(()=>import('./operations84/Performance84'));
const Devices84=lazy(()=>import('./operations84/Devices84'));
const pages={closing:Cash84,journal:Journal84,returns:Returns84,inventory:Inventory84,quality:Quality84,supply:Supply84,opportunities:Market84,performance:Performance84,devices:Devices84};
export default function LocalOperations84({initialArea='closing'}:{initialArea?:Area84}){
 const {i18n}=useTranslation(),words=dictionary84(i18n.resolvedLanguage||i18n.language),t=useCallback((key:string)=>typeof words[key]==='string'?words[key] as string:key,[words]);
 const {capabilities}=useLocalAccess(),session=useLocalSession(s=>s.session),token=useLocalSession(s=>s.token),navigate=useNavigate(),[params,setParams]=useSearchParams();
 let scope='';try{if(token)scope=contextKey(contextOf(session,capabilities));}catch{/* Context gate denies missing context. */}
 const allowed=areas84.filter(a=>allowed84(capabilities,a)),requested=params.get('area') as Area84||initialArea,area=allowed.includes(requested)?requested:allowed[0];
 const [data,setData]=useState<Workspace84|null>(null),[dirty,setDirty]=useState(false),[saving,setSaving]=useState(false),[error,setError]=useState(''),[notice,setNotice]=useState(''),[readOnly,setReadOnly]=useState(false),[ask,setAsk]=useState<{title:string;run:()=>void}|null>(null);
 const current=useRef<Workspace84|null>(null),activeScope=useRef(scope),saveChain=useRef(Promise.resolve()),dialog=useRef<HTMLDialogElement>(null),report=useRef<(string|number)[][]>([]),[reportCount,setReportCount]=useState(0);activeScope.current=scope;
 const blocker=useBlocker(({currentLocation,nextLocation})=>dirty&&currentLocation.pathname!==nextLocation.pathname);
 const setReport=useCallback((rows:(string|number)[][])=>{report.current=rows;setReportCount(rows.length);},[]);
 const attempt=useCallback((run:()=>void)=>{try{setError('');run();}catch(e){const key=e instanceof Error?e.message:'invalid';setError(typeof words[key]==='string'?key:'invalid');}},[words]);
 const edit=useCallback((mutate:(d:Workspace84)=>void)=>{if(readOnly||!area||!allowed84(capabilities,area)||!current.current||current.current.scope!==scope||scope!==activeScope.current)throw Error('noPermission');const d=structuredClone(current.current);mutate(d);d.revision++;d.updated=new Date().toISOString();current.current=d;setData(d);setDirty(true);},[area,capabilities,scope,readOnly]);
 const commit=useCallback((action:string,reference:string,mutate:(d:Workspace84)=>void,approval=false)=>{if(approval&&!manager84(capabilities))throw Error('noPermission');edit(d=>{mutate(d);d.audit.push({id:crypto.randomUUID(),area,action,reference,at:new Date().toISOString(),actor:d.mode==='demo'?'DEMO':t('currentOperator'),detail:t('draft')});d.audit=d.audit.slice(-1000);});setNotice(t('simulated'));},[edit,area,capabilities,t]);
 const save=useCallback(async()=>{const snapshot=current.current;if(!snapshot||snapshot.scope!==scope)return false;setSaving(true);try{const task=saveChain.current.catch(()=>{}).then(()=>write84(structuredClone(snapshot)));saveChain.current=task;await task;if(activeScope.current===scope){if(current.current?.revision===snapshot.revision)setDirty(false);setSaving(false);setError('');}return true;}catch{if(activeScope.current===scope){setSaving(false);setError('saveFailed');}return false;}},[scope]);
 useEffect(()=>{let live=true;current.current=null;setData(null);setDirty(false);setSaving(false);setError('');setReadOnly(false);setAsk(null);if(scope)read84(scope).then(v=>{if(live){current.current=v||empty84(scope);setData(current.current);}}).catch(()=>{if(live){current.current=empty84(scope);setData(current.current);setError('saveFailed');}});return()=>{live=false;};},[scope]);
 useEffect(()=>{if(!dirty||saving||error)return;const timer=setTimeout(()=>void save(),600);return()=>clearTimeout(timer);},[data,dirty,saving,error,save]);
 useEffect(()=>{const handler=(e:BeforeUnloadEvent)=>{if(dirty){e.preventDefault();e.returnValue='';}};window.addEventListener('beforeunload',handler);return()=>window.removeEventListener('beforeunload',handler);},[dirty]);
 useEffect(()=>{if(ask||blocker.state==='blocked')dialog.current?.showModal();else dialog.current?.close();},[ask,blocker.state]);
 useEffect(()=>{report.current=[];setReportCount(0);setReadOnly(false);setNotice('');},[area]);
 const money=useCallback((n:number)=>new Intl.NumberFormat(i18n.language,{style:'currency',currency:'BRL'}).format(n/100),[i18n.language]);
 const qty=useCallback((n:number)=>new Intl.NumberFormat(i18n.language,{maximumFractionDigits:3}).format(n/1000),[i18n.language]);
 const date=useCallback((s:string)=>Number.isFinite(Date.parse(s))?new Intl.DateTimeFormat(i18n.language,{dateStyle:'short',timeStyle:'short'}).format(new Date(s)):'—',[i18n.language]);
 const askAction=useCallback((title:string,run:()=>void)=>setAsk({title,run}),[]);
 const context=useMemo<Ops84|null>(()=>data&&data.scope===scope&&capabilities&&area?{data,area,locale:i18n.language,cap:capabilities,manager:manager84(capabilities),readOnly,t,money,qty,date,edit,commit,attempt,ask:askAction,notice:setNotice,setReport}:null,[data,scope,capabilities,area,i18n.language,readOnly,t,money,qty,date,edit,commit,attempt,askAction,setReport]);
 const exportCSV=()=>{const url=URL.createObjectURL(new Blob([csv84([[t('draft'),t(area),data?.mode==='demo'?t('demoBadge'):''],...report.current])],{type:'text/csv;charset=utf-8'})),a=document.createElement('a');a.href=url;a.download='Titan-'+area+'-RASCUNHO.csv';a.click();setTimeout(()=>URL.revokeObjectURL(url),1000);};
 const openSheets=()=>{if(capabilities?.role!=='owner')throw Error('noPermission');if(report.current.length>500)throw Error('tooLarge');const id=crypto.randomUUID();stageReport76(scope,id,t('draft')+' · '+t(area),report.current);navigate('/local/sheets?report='+encodeURIComponent(id));};
 if(!scope||!capabilities||!area)return <main className="brand-app-content wf84"><h1>{t('title')}</h1><p>{t('forbidden')}</p></main>;
 const Page=pages[area];
 return <main className="brand-app-content wf84"><header className="wf84-heading"><div><p className="wf84-kicker">TITAN · {t('draft')}</p><h1>{t(area)}</h1><p>{t('intro')}</p></div><div className="wf84-actions"><button className="brand-secondary" onClick={()=>setAsk({title:t('replaceDemo'),run:()=>attempt(()=>{const previous=current.current?.audit||[];const next=examples84(scope);next.revision=(current.current?.revision||0)+1;next.audit=[...previous,{id:crypto.randomUUID(),area,action:'loadDemo',reference:'DEMO',at:new Date().toISOString(),actor:'DEMO',detail:t('demoBadge')}].slice(-1000);current.current=next;setData(next);setDirty(true);})})}>{t('loadDemo')}</button><button className="brand-primary" disabled={!dirty||saving} onClick={()=>void save()}>{saving?t('saving'):t('save')}</button></div></header>
 <div className="wf84-notice"><strong>{data?.mode==='demo'?t('demoBadge'):t('draft')}</strong><span>{t('localNotice')}</span><details><summary>{t('about')}</summary><p>{t('aboutText')}</p></details></div>
 <nav className="wf84-nav" aria-label={t('title')}>{allowed.map(a=><button key={a} aria-current={a===area?'page':undefined} onClick={()=>setParams({area:a})}>{t(a)}</button>)}</nav>
 {area==='closing'&&<Link className="brand-secondary wf84-live-link" to="/local/cash/register">↗ {t('realCash')}</Link>}
 <div className="wf84-subbar"><span role="status">{saving?t('saving'):dirty?t('unsaved'):t('saved')}</span><div className="wf84-actions">{area==='inventory'&&manager84(capabilities)&&<label className="wf84-check"><input type="checkbox" checked={readOnly} onChange={e=>setReadOnly(e.target.checked)}/>{t('readOnly')}</label>}<button className="brand-secondary" disabled={reportCount<2} onClick={()=>attempt(exportCSV)}>{t('export')}</button><button className="brand-secondary" disabled={reportCount<2||capabilities.role!=='owner'} onClick={()=>attempt(openSheets)}>{t('sheets')}</button></div></div>
 {error&&<p role="alert" className="wf84-error">! {t(error)}</p>}{notice&&<p role="status" className="wf84-feedback">✓ {notice}</p>}
 {!context?<p>{t('loading')}</p>:<OperationsContext84.Provider value={context}><Suspense fallback={<p>{t('loading')}</p>}><Page/></Suspense><details className="wf84-activity"><summary>{t('activity')} ({data?.audit.filter(e=>e.area===area).length||0})</summary><ol>{data?.audit.filter(e=>e.area===area).slice(-50).reverse().map(e=><li key={e.id}><strong>{t(e.action)}</strong><span>{e.reference}</span><time>{date(e.at)}</time><small>{e.actor} · {e.detail}</small></li>)}</ol></details></OperationsContext84.Provider>}
 <dialog ref={dialog} className="wf84-confirm" onCancel={e=>{e.preventDefault();setAsk(null);if(blocker.state==='blocked')blocker.reset();}}><h2>{blocker.state==='blocked'?t('unsaved'):ask?.title}</h2><p>{blocker.state==='blocked'?t('unsaved'):t('demoAction')}</p><div className="wf84-actions"><button className="brand-secondary" onClick={()=>{setAsk(null);if(blocker.state==='blocked')blocker.reset();}}>{t('cancel')}</button>{blocker.state==='blocked'&&<button className="brand-secondary" onClick={()=>blocker.proceed()}>{t('discard')}</button>}<button className="brand-primary" onClick={()=>{if(blocker.state==='blocked'){void save().then(ok=>{if(ok)blocker.proceed();});}else{const run=ask?.run;setAsk(null);if(run)attempt(run);}}}>{blocker.state==='blocked'?t('save'):t('confirm')}</button></div></dialog>
 </main>;
}
