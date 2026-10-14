import {useEffect,useState} from 'react';
import {useBlocker} from 'react-router-dom';
import {useTranslation} from 'react-i18next';
import {languages} from '../../../i18n';
import {useLocalAccess} from '../../../core/local/LocalAccess';
import {useAppearance80} from '../../../core/settings80/runtime';
import {defaults80,type Settings80} from '../../../core/settings80/model.mjs';
import {raster80} from '../../../core/settings80/storage';
import {words80} from '../../../core/settings80/words';
import {ThemeToggle} from '../../../shared/brand/BrandShell';
const states='AC AL AP AM BA CE DF ES GO MA MT MS MG PA PB PR PE PI RJ RN RS RO RR SC SP SE TO'.split(' ');
export default function LocalSettings80(){
 const context=useAppearance80(),{capabilities}=useLocalAccess(),{i18n}=useTranslation(),w=words80(i18n.resolvedLanguage||i18n.language);
 const [draft,setDraft]=useState<Settings80>(defaults80),[dirty,setDirty]=useState(false),[busy,setBusy]=useState(false),[notice,setNotice]=useState(''),[tab,setTab]=useState('identity');
 const owner=capabilities?.role==='owner';
 const tabs=owner?['identity','appearance','fiscal','equipment','preferences']:['identity','appearance','preferences'];
 const current=tabs.includes(tab)?tab:'appearance';
 const blocker=useBlocker(({currentLocation,nextLocation})=>dirty&&currentLocation.pathname!==nextLocation.pathname);
 useEffect(()=>{if(blocker.state==='blocked'){if(window.confirm(w.unsaved)){blocker.proceed();}else blocker.reset();}},[blocker,w.unsaved]);
 useEffect(()=>{if(context?.loaded&&!dirty)setDraft(context.stored);},[context?.stored,context?.loaded,dirty]);
 useEffect(()=>{if(!dirty)return;const before=(event:BeforeUnloadEvent)=>{event.preventDefault();event.returnValue='';};window.addEventListener('beforeunload',before);return()=>window.removeEventListener('beforeunload',before);},[dirty]);
 const clearPreview=context?.clearPreview;
 useEffect(()=>()=>clearPreview?.(),[clearPreview]);
 function visual(value:Settings80):Settings80{return {...(context?.stored||defaults80()),companyName:value.companyName,logoLight:value.logoLight,logoDark:value.logoDark,background:value.background,backgroundFit:value.backgroundFit,mode:value.mode,accent:value.accent,compact:value.compact,lowEffects:value.lowEffects};}
 function change(value:Partial<Settings80>){
  const next={...draft,...value};
  setDraft(next);setDirty(true);setNotice('');
  if(Object.keys(value).some(key=>['companyName','logoLight','logoDark','background','backgroundFit','mode','accent','compact','lowEffects'].includes(key))){try{context?.preview(visual(next));}catch{setNotice(w.error);}}
 }
 async function image(file:File|undefined,key:'logoLight'|'logoDark'|'background'){
  if(!file)return;setBusy(true);setNotice('');
  try{const value=await raster80(file,key==='background');change({[key]:value,...(key==='background'?{mode:'photo' as const}:{})});}catch{setNotice(w.error);}finally{setBusy(false);}
 }
 async function save(){
  if(!context)return;setBusy(true);setNotice('');
  try{await context.save(draft);setDirty(false);setNotice(w.saved);}catch{setNotice(w.error);}finally{setBusy(false);}
 }
 function preview(){
  if(!context)return;
  try{context.preview(visual(draft));}catch{setNotice(w.error);}
 }
 return <main className="brand-app-content settings80-page">
  <header><h1>{w.title}</h1><p>{w.subtitle}</p><p className="settings80-status">{w.localOnly}</p></header>
  <nav className="settings80-tabs" aria-label={w.title}>{tabs.map(item=><button className="brand-secondary" key={item} onClick={()=>setTab(item)} aria-pressed={current===item}>{w[item as 'identity'|'appearance'|'fiscal'|'equipment'|'preferences']}</button>)}</nav>
  <fieldset disabled={busy||!context?.loaded}>
   {current==='identity'&&<section className="brand-panel"><h2>{w.identity}</h2><label>{w.companyName}<input maxLength={100} value={draft.companyName} onChange={e=>change({companyName:e.target.value})}/></label><p>{w.logoHelp}</p><div className="settings80-grid">{(['logoLight','logoDark'] as const).map(key=><div key={key}><label>{w[key]}<input type="file" accept="image/png,image/jpeg,image/webp" onChange={e=>{void image(e.target.files?.[0],key);e.target.value='';}}/></label>{draft[key]&&<><img className="settings80-image" data-logo={key==='logoLight'?'light':'dark'} src={draft[key]} alt={w[key]}/><button className="brand-secondary" onClick={()=>change({[key]:''})}>{w.remove}</button></>}</div>)}</div><p>{w.filesHint}</p></section>}
   {current==='appearance'&&<section className="brand-panel"><h2>{w.appearance}</h2><label>{w.background}<input type="file" accept="image/png,image/jpeg,image/webp" onChange={e=>{void image(e.target.files?.[0],'background');e.target.value='';}}/></label><p>{w.backgroundHelp}</p>{draft.background&&<><img className="settings80-image" src={draft.background} alt={w.background}/><button className="brand-secondary" onClick={()=>change({background:'',mode:'theme'})}>{w.remove}</button></>}
    <div className="settings80-actions"><button className="brand-secondary" disabled={!draft.logoDark&&!draft.logoLight} onClick={()=>change({background:draft.logoDark||draft.logoLight,backgroundFit:'contain',mode:'photo'})}>{w.useLogo}</button><button className="brand-secondary" onClick={()=>change({mode:'theme'})}>{w.restoreTheme}</button></div>
    <div className="settings80-grid"><label>{w.appearance}<select value={draft.mode} onChange={e=>change({mode:e.target.value as 'theme'|'photo'})}><option value="theme">{w.themeMode}</option><option value="photo" disabled={!draft.background}>{w.photoMode}</option></select></label><label>{w.background}<select value={draft.backgroundFit} onChange={e=>change({backgroundFit:e.target.value as 'cover'|'contain'})}><option value="cover">{w.fill}</option><option value="contain">{w.centered}</option></select></label></div>
    {draft.mode==='photo'?<p className="settings80-warning">{w.photoWarning}</p>:<ThemeToggle/>}
    <label>{w.color}<input type="color" value={draft.accent||'#707070'} onChange={e=>change({accent:e.target.value})}/></label><button className="brand-secondary" onClick={()=>change({accent:''})}>{w.reset}</button><p>{w.colorHelp}</p><label className="settings80-check"><input type="checkbox" checked={draft.lowEffects} onChange={e=>change({lowEffects:e.target.checked})}/>{w.lowEffects}</label>
   </section>}
   {current==='fiscal'&&owner&&<section className="brand-panel"><h2>{w.fiscal}</h2><p className="settings80-warning">{w.fiscalHint}</p><div className="settings80-grid">
    {(['cnpj','ie','series','nextNumber'] as const).map(key=><label key={key}>{w[key]}<input value={draft.fiscal[key]} maxLength={key==='cnpj'?18:key==='ie'?24:key==='series'?3:9} inputMode={key==='ie'?'text':'numeric'} onChange={e=>change({fiscal:{...draft.fiscal,[key]:e.target.value}})}/></label>)}
    <label>{w.uf}<select value={draft.fiscal.uf} onChange={e=>change({fiscal:{...draft.fiscal,uf:e.target.value}})}><option value="">—</option>{states.map(state=><option key={state}>{state}</option>)}</select></label><label>{w.regime}<select value={draft.fiscal.regime} onChange={e=>change({fiscal:{...draft.fiscal,regime:e.target.value}})}><option value="">—</option>{['MEI','Simples Nacional','Lucro Presumido','Lucro Real'].map(regime=><option key={regime}>{regime}</option>)}</select></label></div>
    <div className="settings80-grid">{[w.certificate,w.credentials].map(title=><section key={title} className="settings80-warning"><strong>{title}</strong><p>{w.unavailable}</p><button className="brand-secondary" disabled>{w.connect}</button></section>)}</div>
   </section>}
   {current==='equipment'&&owner&&<section className="brand-panel"><h2>{w.equipment}</h2><p className="settings80-warning">{w.equipmentHint}</p><div className="settings80-grid"><label>{w.printer}<input maxLength={120} value={draft.equipment.printer} onChange={e=>change({equipment:{...draft.equipment,printer:e.target.value}})}/></label><label>{w.connection}<select value={draft.equipment.connection} onChange={e=>change({equipment:{...draft.equipment,connection:e.target.value}})}><option value="system">{w.system}</option><option value="USB">{w.usb}</option><option value="network">{w.network}</option></select></label><label>{w.paper}<select value={draft.equipment.paper} onChange={e=>change({equipment:{...draft.equipment,paper:e.target.value}})}>{['58','80','A4'].map(paper=><option key={paper} value={paper}>{paper==='A4'?paper:paper+' mm'}</option>)}</select></label><label>{w.copies}<select value={draft.equipment.copies} onChange={e=>change({equipment:{...draft.equipment,copies:e.target.value}})}>{['1','2','3'].map(copies=><option key={copies}>{copies}</option>)}</select></label></div><button className="brand-secondary" disabled>{w.printTest}</button></section>}
   {current==='preferences'&&<section className="brand-panel"><h2>{w.preferences}</h2><label>{w.language}<select value={i18n.resolvedLanguage||i18n.language} onChange={e=>void i18n.changeLanguage(e.target.value)}>{languages.map(language=><option key={language.id} value={language.id}>{language.name}</option>)}</select></label><label className="settings80-check"><input type="checkbox" checked={draft.compact} onChange={e=>change({compact:e.target.checked})}/>{w.compact}</label><label className="settings80-check"><input type="checkbox" checked={draft.lowEffects} onChange={e=>change({lowEffects:e.target.checked})}/>{w.lowEffects}</label>{!owner&&<p>{w.readOnly}</p>}</section>}
  </fieldset>
  <footer className="settings80-save"><span className="settings80-status" role="status">{notice||(dirty?w.unsaved:w.localOnly)}</span><div className="settings80-actions"><button className="brand-secondary" disabled={busy||!context?.loaded} onClick={()=>{if(window.confirm(w.resetConfirm)){change({...defaults80(),...(!owner?{fiscal:draft.fiscal,equipment:draft.equipment}:{})});}}}>{w.reset}</button><button className="brand-secondary" disabled={busy||!context?.loaded} onClick={preview}>{w.preview}</button><button className="brand-primary" disabled={busy||!dirty||!context?.loaded} onClick={()=>void save()}>{w.save}</button></div></footer>
 </main>;
}
