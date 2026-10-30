import {useEffect,useMemo,useRef,useState} from 'react';
import {Link,useBlocker} from 'react-router-dom';
import {useTranslation} from 'react-i18next';
import {useLocalSession} from '../../../core/local/useLocalSession';
import {createRegistrationClient,RegistrationError} from '../../../core/companyRegistration/client.mjs';
import type {RegistrationInput,RegistrationRecord} from '../../../core/companyRegistration/client.mjs';
import {companyWords} from '../../../core/companyRegistration/words';
export default function LocalCompanyRegistration(){
 const {i18n}=useTranslation(),w=companyWords(i18n.resolvedLanguage||i18n.language),token=useLocalSession(s=>s.token),session=useLocalSession(s=>s.session);
 const current=useMemo(()=>()=>{const now=useLocalSession.getState();return !!token&&now.token===token&&now.session===session&&!!session&&session.expires_unix>Date.now()/1000;},[token,session]);
 const api=useMemo(()=>createRegistrationClient({token:token||'',isCurrent:current}),[token,current]);
 const [saved,setSaved]=useState<RegistrationRecord|null>(null),[input,setInput]=useState<RegistrationInput|null>(null),[pending,setPending]=useState<RegistrationInput|null>(null),[busy,setBusy]=useState(false),[message,setMessage]=useState('');
 const generation=useRef(0),writing=useRef(false),alive=useRef(true);
 const dirty=!!input&&!!saved&&JSON.stringify({...input,operation_id:'',expected_revision:0})!==JSON.stringify({...saved.input,operation_id:'',expected_revision:0});
 const blocker=useBlocker(({currentLocation,nextLocation})=>(dirty||!!pending)&&currentLocation.pathname!==nextLocation.pathname);
 useEffect(()=>{if(blocker.state==='blocked'){if(window.confirm(w.unsaved))blocker.proceed();else blocker.reset();}},[blocker,w.unsaved]);
 useEffect(()=>{if(!dirty&&!pending)return;const warn=(e:BeforeUnloadEvent)=>{e.preventDefault();e.returnValue='';};window.addEventListener('beforeunload',warn);return()=>window.removeEventListener('beforeunload',warn);},[dirty,pending]);
 useEffect(()=>{alive.current=true;return()=>{alive.current=false;generation.current++;};},[]);
 useEffect(()=>{setSaved(null);setInput(null);setPending(null);void read();},[api]);
 async function read(){if(writing.current||!current())return;const n=++generation.current;setBusy(true);setMessage('');try{const value=await api.read(pending?.operation_id);if(alive.current&&n===generation.current&&current()){setSaved(value);setInput({...value.input,expected_revision:value.revision});setPending(null);}}catch(e){if(alive.current&&n===generation.current)setMessage(e instanceof RegistrationError?e.code:'error');}finally{if(alive.current&&n===generation.current)setBusy(false);}}
 async function save(){if(!input||!saved||pending||writing.current||!current())return;writing.current=true;generation.current++;setBusy(true);setMessage('');const attempt={...input,operation_id:crypto.randomUUID(),expected_revision:saved.revision};setPending(attempt);
 try{const value=await api.save(attempt);if(alive.current&&current()){setSaved(value);setInput({...value.input,expected_revision:value.revision});setPending(null);setMessage('saved');}}
 catch(e){if(alive.current&&current()){if(e instanceof RegistrationError&&e.status>=400&&e.status<500){setPending(null);setMessage(e.code);}else setMessage('uncertain');}}finally{writing.current=false;if(alive.current)setBusy(false);}}
 function stage(value:'pre_registration'|'company_profile'){if(!input)return;if(value==='pre_registration'&&saved?.input.stage==='company_profile')return;setInput(value==='pre_registration'?{...input,stage:value,cnpj:'',legal_name:'',uf:'',state_registration:'',municipality_ibge:'',crt:'',provider:'',environment:''}:{...input,stage:value});}
 const full=input?.stage==='company_profile';
 return <main className="brand-app-content"><h1>{w.title}</h1><p>{w.intro}</p><p>{w.manual}</p>
 <nav><Link to="/local/enterprise?area=fiscal">{w.fiscal}</Link> · <Link to="/local/enterprise?area=access">{w.staff}</Link> · <Link to="/local/settings">{w.settings}</Link></nav>
 <button className="brand-secondary" disabled={busy||writing.current||!current()} onClick={()=>void read()}>{pending?w.recover:w.load}</button>
 {message&&<p role="status">{w[message]||w.error}</p>}{pending&&<p>{w.uncertain} · {pending.operation_id}</p>}
 {input&&<form onSubmit={e=>{e.preventDefault();void save();}}><fieldset disabled={busy||!!pending||!current()}>
 <label>{w.stage}<select value={input.stage} onChange={e=>stage(e.target.value as RegistrationInput['stage'])}><option value="pre_registration" disabled={saved?.input.stage==='company_profile'}>{w.pre_registration}</option><option value="company_profile">{w.company_profile}</option></select></label>
 {(['company_name','responsible'] as const).map(k=><label key={k}>{w[k]}<input required maxLength={180} value={input[k]} onChange={e=>setInput({...input,[k]:e.target.value})}/></label>)}
 {full&&<><p>{w.fiscalNote}</p>{(['cnpj','legal_name','uf','state_registration','municipality_ibge','crt'] as const).map(k=><label key={k}>{w[k]}<input required maxLength={k==='legal_name'?180:k==='cnpj'?14:k==='uf'?2:k==='municipality_ibge'?7:k==='crt'?1:20} value={input[k]} onChange={e=>setInput({...input,[k]:e.target.value})}/></label>)}
 <label>{w.provider}<select required value={input.provider} onChange={e=>setInput({...input,provider:e.target.value})}><option value="">—</option><option value="own">{w.own}</option><option value="external">Focus</option></select></label>
 <label>{w.environment}<select required value={input.environment} onChange={e=>setInput({...input,environment:e.target.value})}><option value="">—</option><option value="homologation">{w.homologation}</option><option value="production">{w.production}</option></select></label></>}
 <button className="brand-primary">{w.save}</button></fieldset></form>}</main>;
}
