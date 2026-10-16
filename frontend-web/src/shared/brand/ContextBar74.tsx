import {useEffect,useRef,useState} from 'react';
import {useTranslation} from 'react-i18next';
import {useLocalAccess} from '../../core/local/LocalAccess';
import {useLocalSession} from '../../core/local/useLocalSession';
import {contextState} from '../../core/context75/model.mjs';
import {words75} from '../../core/context75/words';
import {useAppearance80} from '../../core/settings80/runtime';
import './context75.css';
import {words74} from '../../core/context74/words';
import './context74.css';
export default function ContextBar74(){
 const {i18n}=useTranslation(),w=words74(i18n.resolvedLanguage||i18n.language),{capabilities}=useLocalAccess();const session=useLocalSession(s=>s.session);
 const v=words75(i18n.resolvedLanguage||i18n.language),appearance=useAppearance80();
 const [panel,setPanel]=useState<'context'|'help'|null>(null),dialog=useRef<HTMLDialogElement>(null),trigger=useRef<HTMLButtonElement|null>(null);
 const state=contextState(session,capabilities);
 useEffect(()=>{const d=dialog.current;if(!d)return;if(panel&&!d.open)d.showModal();if(!panel&&d.open)d.close();},[panel]);
 function open(kind:'context'|'help',button:HTMLButtonElement){trigger.current=button;setPanel(kind);}
 const title=panel==='context'?v.context:w.helpTitle;
 return <aside className="ctx74-bar" aria-label={w.company+' / '+w.store}>
  {state==='confirmed'?<><div className="ctx75-readonly"><span>{v.company}</span><strong title={appearance?.prefs.companyName?v.localDisplayName:undefined}>{appearance?.prefs.companyName||v.companyName}</strong></div><div className="ctx75-readonly"><span>{v.store}</span><strong>{v.storeName}</strong></div></>:null}
  <button className="brand-secondary ctx75-state" data-state={state} onClick={e=>open('context',e.currentTarget)}>{v[state]}</button>
  <div className="ctx74-actions"><button className="brand-secondary" onClick={e=>open('help',e.currentTarget)}>ⓘ {v.help}</button></div>
  <dialog ref={dialog} className="ctx74-dialog" aria-labelledby="ctx74-title" onCancel={()=>setPanel(null)} onClose={()=>{setPanel(null);trigger.current?.focus();}}><header><h2 id="ctx74-title">{title}</h2><button className="brand-secondary" onClick={()=>setPanel(null)} aria-label={w.closed}>×</button></header>
   {panel==='context'?<><p>{state==='confirmed'?w.contextInfo:state==='selection'?v.selectionHelp:v.deniedHelp}</p><p>{v.missingNames}</p><p>{v.noSwitch}</p><p>{w.tokenInfo}</p></>:<><p>{w.helpBody}</p><p>{w.posHelp}</p><p>{w.filesHelp}</p><p>{w.helpPermissions}</p></>}
  </dialog>
 </aside>;
}
