import {lazy,Suspense,useCallback,useEffect,useMemo,useRef,useState} from 'react';
import {Link,useBlocker,useNavigate,useSearchParams} from 'react-router-dom';
import {useTranslation} from 'react-i18next';
import {useLocalAccess} from '../../../core/local/LocalAccess';
import {useLocalSession} from '../../../core/local/useLocalSession';
import {contextKey,contextOf} from '../../../core/context74/model.mjs';
import {BusinessContext85,type Context85} from '../../../core/business85/context';
import {empty85,examples85} from '../../../core/business85/data';
import {allowed85,areas85,csv85,manager85} from '../../../core/business85/rules.mjs';
import {read85,write85} from '../../../core/business85/storage';
import {dictionary85} from '../../../core/business85/words';
import type {Area85,Workspace85} from '../../../core/business85/types';
import {stageReport76} from '../../../core/loss76/report76.mjs';
import '../../../core/loss76/sessionReport76';
import './business85/business85.css';
const Fleet85=lazy(()=>import('./business85/Fleet85')),Food85=lazy(()=>import('./business85/Food85')),Documents85=lazy(()=>import('./business85/Documents85')),Purchasing85=lazy(()=>import('./business85/Purchasing85')),Commercial85=lazy(()=>import('./business85/Commercial85'));
const pages={fleet:Fleet85,food:Food85,documents:Documents85,purchasing:Purchasing85,commercial:Commercial85};
export default function LocalBusiness85({initialArea='fleet'}:{initialArea?:Area85}){
 const {i18n}=useTranslation(),words=dictionary85(i18n.resolvedLanguage||i18n.language),t=useCallback((k:string)=>typeof words[k]==='string'?words[k] as string:k,[words]);
 const {capabilities}=useLocalAccess(),session=useLocalSession(s=>s.session),token=useLocalSession(s=>s.token),navigate=useNavigate(),[params,setParams]=useSearchParams();
 let scope='';try{if(token)scope=contextKey(contextOf(session,capabilities));}catch{/* Missing trusted context remains blocked. */}
 const allowed=areas85.filter(a=>allowed85(capabilities,a)),requested=(params.get('area')||initialArea) as Area85,area=allowed.includes(requested)?requested:undefined;
 const [data,setData]=useState<Workspace85|null>(null),[dirty,setDirty]=useState(false),[saving,setSaving]=useState(false),[error,setError]=useState(''),[notice,setNotice]=useState(''),[confirmation,setConfirmation]=useState<{title:string;run:()=>void}|null>(null),[reportSize,setReportSize]=useState(0);
 const current=useRef<Workspace85|null>(null),scopeRef=useRef(scope),saveChain=useRef(Promise.resolve()),dialog=useRef<HTMLDialogElement>(null),report=useRef<(string|number)[][]>([]);scopeRef.current=scope;
 const blocker=useBlocker(({currentLocation,nextLocation})=>dirty&&currentLocation.pathname!==nextLocation.pathname);
 const attempt=useCallback((run:()=>void)=>{try{setError('');run();}catch(e){const key=e instanceof Error?e.message:'invalid';setError(typeof words[key]==='string'?key:'invalid');}},[words]);
 const edit=useCallback((run:(d:Workspace85)=>void)=>{if(!area||!allowed85(capabilities,area)||!current.current||current.current.scope!==scope||scopeRef.current!==scope)throw Error('noPermission');const d=structuredClone(current.current);run(d);d.revision++;d.updated=new Date().toISOString();current.current=d;setData(d);setDirty(true);},[scope,area,capabilities]);
 const commit=useCallback((action:string,reference:string,run:(d:Workspace85)=>void,approval=false)=>{if(approval&&!manager85(capabilities))throw Error('noPermission');edit(d=>{run(d);d.audit.push({id:crypto.randomUUID(),at:new Date().toISOString(),area:area!,action,reference,actor:d.mode==='demo'?'DEMO':t('currentOperator')});d.audit=d.audit.slice(-1000);});setNotice('savedDraftAction');},[area,edit,capabilities,t]);
 const save=useCallback(async()=>{const snapshot=current.current;if(!snapshot||snapshot.scope!==scope)return false;setSaving(true);try{const task=saveChain.current.catch(()=>{}).then(()=>write85(structuredClone(snapshot)));saveChain.current=task;await task;if(scopeRef.current===scope){if(current.current?.revision===snapshot.revision)setDirty(false);setSaving(false);setError('');}return true;}catch{if(scopeRef.current===scope){setSaving(false);setError('saveFailed');}return false;}},[scope]);
 useEffect(()=>{let live=true;current.current=null;setData(null);setDirty(false);setError('');setConfirmation(null);if(scope)read85(scope).then(value=>{if(live){current.current=value||empty85(scope);setData(current.current);}}).catch(()=>{if(live){current.current=empty85(scope);setData(current.current);setError('saveFailed');}});return()=>{live=false;};},[scope]);
 useEffect(()=>{if(!dirty||saving||error)return;const timer=setTimeout(()=>void save(),650);return()=>clearTimeout(timer);},[data,dirty,saving,error,save]);
 useEffect(()=>{const handler=(e:BeforeUnloadEvent)=>{if(dirty){e.preventDefault();e.returnValue='';}};window.addEventListener('beforeunload',handler);return()=>window.removeEventListener('beforeunload',handler);},[dirty]);
 useEffect(()=>{if(confirmation||blocker.state==='blocked')dialog.current?.showModal();else dialog.current?.close();},[confirmation,blocker.state]);
 const money=useCallback((n:number)=>new Intl.NumberFormat(i18n.language,{style:'currency',currency:'BRL'}).format(n/100),[i18n.language]),qty=useCallback((n:number)=>new Intl.NumberFormat(i18n.language,{maximumFractionDigits:3}).format(n/1000),[i18n.language]),date=useCallback((s:string)=>Number.isFinite(Date.parse(s))?new Intl.DateTimeFormat(i18n.language,{dateStyle:'short',timeStyle:s.length>10?'short':undefined}).format(new Date(s.length===10?s+'T12:00:00':s)):'—',[i18n.language]);
 const ask=useCallback((title:string,run:()=>void)=>setConfirmation({title,run}),[]),setReport=useCallback((rows:(string|number)[][])=>{report.current=rows;setReportSize(rows.length);},[]);
 const context=useMemo<Context85|null>(()=>data&&data.scope===scope&&area&&capabilities?{data,area,cap:capabilities,locale:i18n.language,manager:manager85(capabilities),t,money,qty,date,edit,commit,attempt,ask,notice:setNotice,setReport}:null,[data,scope,area,capabilities,i18n.language,t,money,qty,date,edit,commit,attempt,ask,setReport]);
 const exportCSV=()=>{const url=URL.createObjectURL(new Blob([csv85([[t('draft'),data?.mode==='demo'?t('demoBadge'):''],...report.current])],{type:'text/csv;charset=utf-8'})),a=document.createElement('a');a.href=url;a.download='Titan-'+area+'-RASCUNHO.csv';a.click();setTimeout(()=>URL.revokeObjectURL(url),1000);};
 const sheets=()=>{if(capabilities?.role!=='owner'||report.current.length>500)throw Error('noPermission');const id=crypto.randomUUID();stageReport76(scope,id,t(area!)+' · '+t('draft'),report.current);navigate('/local/sheets?report='+encodeURIComponent(id));};
 if(!scope||!capabilities||!area)return <main className="brand-app-content wf85"><h1>{t(initialArea)}</h1><p>{t('forbidden')}</p><Link to="/local/home">{t('backMenu')}</Link></main>;
 const Page=pages[area];
 return <main className="brand-app-content wf85"><header className="wf85-heading"><div><p className="wf85-kicker">TITAN · {t('draft')}</p><h1>{t(area)}</h1><p>{t(area+'Intro')}</p></div><div className="wf85-actions"><button className="brand-secondary" onClick={()=>ask(t('replaceDemo'),()=>{const next=examples85(scope);next.revision=(current.current?.revision||0)+1;next.audit=current.current?.audit||[];current.current=next;setData(next);setDirty(true);setNotice('demoBadge');})}>{t('loadDemo')}</button><button className="brand-primary" disabled={!dirty||saving} onClick={()=>void save()}>{saving?t('saving'):t('save')}</button></div></header>
 <div className="wf85-notice"><strong>{t(data?.mode==='demo'?'demoBadge':'draft')}</strong><span>{t('businessLocal')}</span><details><summary>{t('about')}</summary><p>{t('businessAbout')}</p></details></div>
 <nav className="wf85-nav" aria-label={t('businessTitle')}>{allowed.map(a=><button key={a} aria-current={a===area?'page':undefined} onClick={()=>setParams({area:a})}>{t(a)}</button>)}</nav>
 <div className="wf85-subbar"><span role="status">{t(saving?'saving':dirty?'unsaved':'saved')}</span><div className="wf85-actions"><button className="brand-secondary" disabled={reportSize<2} onClick={()=>attempt(exportCSV)}>{t('export')}</button><button className="brand-secondary" disabled={reportSize<2||capabilities.role!=='owner'} onClick={()=>attempt(sheets)}>{t('sheets')}</button></div></div>
 {error&&<p className="wf85-error" role="alert">! {t(error)}</p>}{notice&&<p className="wf85-feedback" role="status">✓ {t(notice)}</p>}
 {context?<BusinessContext85.Provider value={context}><Suspense fallback={<p>{t('loading')}</p>}><Page/></Suspense><details className="wf85-activity"><summary>{t('activity')}</summary><ol>{data?.audit.filter(e=>e.area===area).slice(-50).reverse().map(e=><li key={e.id}><strong>{t(e.action)}</strong><span>{e.reference}</span><time>{date(e.at)}</time><small>{e.actor}</small></li>)}</ol></details></BusinessContext85.Provider>:<p>{t('loading')}</p>}
 <dialog ref={dialog} className="wf85-confirm" onCancel={e=>{e.preventDefault();setConfirmation(null);if(blocker.state==='blocked')blocker.reset();}}><h2>{blocker.state==='blocked'?t('unsaved'):confirmation?.title}</h2><p>{t('demoAction')}</p><div className="wf85-actions"><button className="brand-secondary" onClick={()=>{setConfirmation(null);if(blocker.state==='blocked')blocker.reset();}}>{t('cancel')}</button>{blocker.state==='blocked'&&<button className="brand-secondary" onClick={()=>blocker.proceed()}>{t('discard')}</button>}<button className="brand-primary" onClick={()=>{if(blocker.state==='blocked'){void save().then(ok=>{if(ok)blocker.proceed();});}else{const run=confirmation?.run;setConfirmation(null);if(run)attempt(run);}}}>{t(blocker.state==='blocked'?'save':'confirm')}</button></div></dialog>
 </main>;
}
