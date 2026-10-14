import {useEffect,useRef,useState} from 'react';
import {useTranslation} from 'react-i18next';
import {useLocalAccess} from '../../core/local/LocalAccess';
import {useLocalSession} from '../../core/local/useLocalSession';
import {contextOf} from '../../core/context74/model.mjs';
import {words74} from '../../core/context74/words';
import './context74.css';
export default function ContextBar74(){
 const {i18n}=useTranslation(),w=words74(i18n.resolvedLanguage||i18n.language),{capabilities}=useLocalAccess();const session=useLocalSession(s=>s.session);
 const [panel,setPanel]=useState<'context'|'help'|'notifications'|null>(null),dialog=useRef<HTMLDialogElement>(null),trigger=useRef<HTMLButtonElement|null>(null);
 let valid=false;try{contextOf(session,capabilities);valid=true;}catch{/* No fabricated context fallback. */}
 useEffect(()=>{const d=dialog.current;if(!d)return;if(panel&&!d.open)d.showModal();if(!panel&&d.open)d.close();},[panel]);
 function open(kind:'context'|'help'|'notifications',button:HTMLButtonElement){trigger.current=button;setPanel(kind);}
 const title=panel==='context'?w.confirmed:panel==='notifications'?w.notifications:w.helpTitle;
 return <aside className="ctx74-bar" aria-label={w.company+' / '+w.store}>
  {valid?<><label>{w.company}<select aria-label={w.company} title={w.single} value="current" onChange={()=>{}}><option value="current">{w.currentCompany}</option></select></label><label>{w.store}<select aria-label={w.store} title={w.single} value="current" onChange={()=>{}}><option value="current">{w.currentStore}</option></select></label><button className="brand-secondary" onClick={e=>open('context',e.currentTarget)}>✓ {w.confirmed}</button></>:<p role="alert">{w.invalid}</p>}
  <div className="ctx74-actions"><button className="brand-secondary" onClick={e=>open('help',e.currentTarget)}>ⓘ {w.help}</button><button className="brand-secondary" onClick={e=>open('notifications',e.currentTarget)}>♧ {w.notifications}</button></div>
  <dialog ref={dialog} className="ctx74-dialog" aria-labelledby="ctx74-title" onCancel={()=>setPanel(null)} onClose={()=>{setPanel(null);trigger.current?.focus();}}><header><h2 id="ctx74-title">{title}</h2><button className="brand-secondary" onClick={()=>setPanel(null)} aria-label={w.closed}>×</button></header>
   {panel==='context'?<><p>{w.contextInfo}</p><p>{w.single}</p><p>{w.tokenInfo}</p></>:panel==='notifications'?<><h3>{w.notificationsUnavailable}</h3><p>{w.notificationsInfo}</p></>:<><p>{w.helpBody}</p><p>{w.posHelp}</p><p>{w.filesHelp}</p><p>{w.helpPermissions}</p></>}
  </dialog>
 </aside>;
}
