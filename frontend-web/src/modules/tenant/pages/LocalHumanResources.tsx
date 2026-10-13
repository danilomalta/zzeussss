import {useEffect,useState} from 'react';
import type {FormEvent} from 'react';
import {Link,Navigate} from 'react-router-dom';
import {useTranslation} from 'react-i18next';
import {useLocalAccess} from '../../../core/local/LocalAccess';
import {allowedAreas} from '../../../core/local/accessModel.mjs';
import {localClient,useLocalSession} from '../../../core/local/useLocalSession';
import type {StaffMember} from '../../../core/local/localClient.mjs';
import {hrSections,hrEditableSections,createHRDraft,validateHRDraft} from '../../../core/local/hrDrafts.mjs';
import type {HRDraftInput,HRDraft} from '../../../core/local/hrDrafts.mjs';
import pt from '../../../i18n/hr/pt-BR.json';
import en from '../../../i18n/hr/en.json';
import es from '../../../i18n/hr/es.json';
import fr from '../../../i18n/hr/fr.json';
import zh from '../../../i18n/hr/zh-CN.json';
import './humanResources.css';
const translations:Record<string,typeof pt>={'pt-BR':pt,pt,en,es,fr,'zh-CN':zh,zh};
const empty=(section:string):HRDraftInput=>({section,subject:'',person:'',email:'',date:'',endDate:'',notes:'',stage:'planned',amount:'',startTime:'',endTime:''});
const icons=['◇','♧','✓','◷','▦','☀','▤','♡','▧','↗','▥','✉','⌑'];
export default function LocalHumanResources(){
 const {t,i18n}=useTranslation();const words=translations[i18n.resolvedLanguage||i18n.language]||pt;
 const titles:Record<string,string>=words.sections,descriptions:Record<string,string>=words.descriptions;
 const {capabilities}=useLocalAccess();const token=useLocalSession(s=>s.token),session=useLocalSession(s=>s.session);
 const permitted=allowedAreas(capabilities).includes('hr');
 const [section,setSection]=useState('overview'),[people,setPeople]=useState<StaffMember[]|null>(null),[failed,setFailed]=useState(false),[loading,setLoading]=useState(false),[revision,setRevision]=useState(0);
 const [drafts,setDrafts]=useState<HRDraft[]>([]),[form,setForm]=useState<HRDraftInput>(empty('recruitment')),[editing,setEditing]=useState<string|null>(null),[query,setQuery]=useState(''),[message,setMessage]=useState(''),[error,setError]=useState('');
 const scope=[session?.tenant_id,session?.store_id,session?.device_id,session?.identity_id,token].join(':');
 useEffect(()=>{setDrafts([]);setForm(empty('recruitment'));setEditing(null);setMessage('');setError('');},[scope]);
 useEffect(()=>{let live=true;setPeople(null);setFailed(false);if(!token||!permitted)return;setLoading(true);localClient.staff(token).then(rows=>{if(live)setPeople(rows);}).catch(()=>{if(live)setFailed(true);}).finally(()=>{if(live)setLoading(false);});return()=>{live=false;};},[token,permitted,revision]);
 if(!permitted)return <Navigate to="/local/home" replace/>;
 function change(id:string){if(id===section)return;if(Object.entries(form).some(([key,value])=>!['section','stage'].includes(key)&&value)&&!window.confirm(words.switchConfirm))return;setSection(id);setForm(empty(id));setEditing(null);setQuery('');setMessage('');setError('');}
 function update(key:keyof HRDraftInput,value:string){setForm(current=>({...current,[key]:value}));setError('');}
 function prepare(event:FormEvent){event.preventDefault();if(!validateHRDraft(form)||(!editing&&drafts.length>=50)){setError(words.invalid);return;}
  const draft=createHRDraft(form,editing||crypto.randomUUID());setDrafts(current=>editing?current.map(row=>row.id===editing?draft:row):[draft,...current]);setEditing(null);setForm(empty(section));setMessage(words.prepared);setError('');}
 function edit(row:HRDraft){setForm({...row});setEditing(row.id);setMessage('');document.getElementById('hr-draft-title')?.focus();}
 const stageLabel=(value:string)=>({planned:words.planned,review:words.review,ready:words.ready}[value]||words.planned);
 const visible=drafts.filter(row=>row.section===section&&`${row.subject} ${row.person}`.toLocaleLowerCase().includes(query.toLocaleLowerCase()));
 const moneySection=['payroll','benefits'].includes(section),timeSection=['attendance','schedules'].includes(section);
 return <main className="brand-app-content hr-page">
  <header className="hr-heading"><div><span className="hr-eyebrow">TITAN</span><h1>{words.title}</h1><p>{words.subtitle}</p></div><Link className="brand-primary" to="/local/staff">{words.access} →</Link></header>
  <div className="hr-mode"><strong>{words.mode}</strong><p>{words.draftHelp}</p></div>
  <nav className="hr-navigation" aria-label={words.title}>{hrSections.map((id,index)=><button key={id} type="button" className={id===section?'selected':''} aria-pressed={id===section} onClick={()=>change(id)}><span aria-hidden="true">{icons[index]}</span>{titles[id]}</button>)}</nav>
  <header className="hr-section-heading"><div><h2>{titles[section]}</h2><p>{descriptions[section]}</p></div>{section==='overview'&&<button className="brand-secondary" disabled={loading} onClick={()=>setRevision(value=>value+1)}>{loading?words.loading:words.refresh}</button>}</header>
  {section==='overview'?<>
   {failed&&<p role="alert" className="hr-message error">{words.failed}</p>}
   <section className="hr-kpis">{[{label:words.people,value:people?.length},{label:words.active,value:people?.filter(row=>row.status==='active').length},{label:words.inactive,value:people?.filter(row=>row.status!=='active').length},{label:words.drafts,value:drafts.length}].map(card=><article className="brand-panel" key={card.label}><span>{card.label}</span><strong>{card.value===undefined?'—':card.value}</strong></article>)}</section>
   <section className="hr-overview-grid">{hrSections.filter(id=>id!=='overview').map(id=><button className="brand-panel hr-area-card" key={id} onClick={()=>change(id)}><span className="hr-area-icon" aria-hidden="true">{icons[hrSections.indexOf(id)]}</span><strong>{titles[id]}</strong><p>{descriptions[id]}</p><span className="hr-area-next" aria-hidden="true">→</span></button>)}</section>
   <section className="brand-panel hr-people"><h2>{words.people}</h2>{loading?<p role="status">{words.loading}</p>:people?.length?<div className="hr-table-scroll"><table><thead><tr><th>{words.person}</th><th>{t('staffUi.profile')}</th><th>{t('staffUi.status')}</th></tr></thead><tbody>{people.map(row=><tr key={row.identity_id}><td>{row.name}</td><td>{t('staffUi.role.'+row.role)}</td><td><span className={`hr-badge ${row.status==='active'?'active':'inactive'}`}>{row.status==='active'?words.active:words.inactive}</span></td></tr>)}</tbody></table></div>:<div className="hr-empty"><span aria-hidden="true">♧</span><p>{failed?words.failed:people?words.noPeople:words.unavailable}</p><Link className="brand-secondary" to="/local/staff">{words.access}</Link></div>}</section>
  </>:hrEditableSections.includes(section)?<>
   <div className="hr-process-layout"><section className="brand-panel hr-form-panel"><h3>{editing?words.edit:words.newDraft}</h3><p className="hr-help">{words.draftHelp}</p>
    <form onSubmit={prepare} className="hr-form"><label className="hr-wide">{words.subject}<input id="hr-draft-title" required maxLength={120} value={form.subject} onChange={e=>update('subject',e.target.value)} placeholder={words.subjectPlaceholder}/></label>
     <label>{words.person}<input maxLength={120} value={form.person} list="hr-known-people" onChange={e=>update('person',e.target.value)} placeholder={words.personPlaceholder}/><datalist id="hr-known-people">{people?.map(row=><option key={row.identity_id} value={row.name}/>)}</datalist></label>
     <label>{words.stage}<select value={form.stage} onChange={e=>update('stage',e.target.value)}>{['planned','review','ready'].map(stage=><option key={stage} value={stage}>{stageLabel(stage)}</option>)}</select></label>
     {['recruitment','portal'].includes(section)&&<label className="hr-wide">{words.email}<input type="email" maxLength={254} value={form.email} onChange={e=>update('email',e.target.value)} placeholder={words.emailPlaceholder}/></label>}
     <label>{words.date}<input type="date" value={form.date} onChange={e=>update('date',e.target.value)}/></label><label>{words.endDate}<input type="date" min={form.date||undefined} value={form.endDate} onChange={e=>update('endDate',e.target.value)}/></label>
     {timeSection&&<><label>{words.startTime}<input type="time" value={form.startTime} onChange={e=>update('startTime',e.target.value)}/></label><label>{words.endTime}<input type="time" value={form.endTime} onChange={e=>update('endTime',e.target.value)}/></label></>}
     {moneySection&&<label className="hr-wide">{words.amount}<input inputMode="decimal" maxLength={12} value={form.amount} onChange={e=>update('amount',e.target.value)} placeholder="0,00"/><small>{words.integration}</small></label>}
     <label className="hr-wide">{words.notes}<textarea maxLength={2000} rows={4} value={form.notes} onChange={e=>update('notes',e.target.value)} placeholder={words.notesPlaceholder}/></label>
     {error&&<p className="hr-message error hr-wide" role="alert">{error}</p>}{message&&<p className="hr-message hr-wide" role="status">{message}</p>}
     <div className="hr-form-actions hr-wide"><button className="brand-primary" type="submit">{words.prepare}</button><button className="brand-secondary" type="button" onClick={()=>{setForm(empty(section));setEditing(null);setMessage('');setError('');}}>{words.clear}</button></div>
    </form>
   </section><aside className="brand-panel hr-workflow"><h3>{titles[section]}</h3><p>{descriptions[section]}</p><ol>{[words.planned,words.review,words.ready].map(stage=><li key={stage}>{stage}</li>)}</ol><div className="hr-integration"><span aria-hidden="true">⌑</span><strong>{words.integration}</strong><p>{words.empty}</p></div>{['lifecycle','compliance','payroll'].includes(section)&&<p className="hr-help">{words.securityHelp}</p>}</aside></div>
   <section className="brand-panel hr-drafts"><header><h3>{words.drafts}</h3><label>{words.search}<input type="search" maxLength={120} value={query} onChange={event=>setQuery(event.target.value)} placeholder={words.search}/></label></header>
    {visible.length?<div className="hr-draft-grid">{visible.map(row=><article key={row.id} className="hr-draft-card"><span className="hr-badge">{stageLabel(row.stage)}</span><h4>{row.subject}</h4>{row.person&&<p>{row.person}</p>}{row.date&&<p>{new Intl.DateTimeFormat(i18n.resolvedLanguage||'pt-BR',{dateStyle:'short'}).format(new Date(row.date+'T12:00:00'))}{row.endDate&&<> — {new Intl.DateTimeFormat(i18n.resolvedLanguage||'pt-BR',{dateStyle:'short'}).format(new Date(row.endDate+'T12:00:00'))}</>}</p>}{row.email&&<p>{words.email}: {row.email}</p>}{row.startTime&&<p>{words.startTime}: {row.startTime}{row.endTime&&<> — {words.endTime}: {row.endTime}</>}</p>}{row.amount&&<p>{words.amount}: {row.amount}</p>}{row.notes&&<p className="hr-draft-notes">{row.notes}</p>}<div><button className="brand-secondary" onClick={()=>edit(row)}>{words.edit}</button><button className="brand-secondary" onClick={()=>{if(window.confirm(words.confirmRemove)){setDrafts(current=>current.filter(value=>value.id!==row.id));if(editing===row.id){setEditing(null);setForm(empty(section));}}}}>{words.remove}</button></div></article>)}</div>:<div className="hr-empty"><span aria-hidden="true">▤</span><p>{query?words.noResults:words.draftHelp}</p></div>}
   </section>
  </>:section==='reports'?<section className="brand-panel hr-report-panel"><p>{words.reportHelp}</p><div className="hr-report-grid">{['overview','attendance','payroll','leave','performance','benefits'].map(id=><article key={id}><h3>{titles[id]}</h3><p>{descriptions[id]}</p><strong>—</strong><small>{words.unavailable}</small></article>)}</div></section>:<section className="brand-panel hr-security"><h3>{titles.security}</h3><p>{words.securityHelp}</p><div className="hr-security-details"><div><span>{t('staffUi.profile')}</span><strong>{t('staffUi.role.'+capabilities?.role)}</strong></div><div><span>{t('subscription.title')}</span><strong>{t('subscription.state.'+capabilities?.license.state)}</strong></div></div><Link className="brand-primary" to="/local/staff">{words.access}</Link><p className="hr-help">{words.integration}</p></section>}
  <footer className="hr-footer"><Link to="/local/home">← {words.back}</Link><span>{words.draftHelp}</span></footer>
 </main>;
}
