import {createRegistrationClient,prefillFiscalProfile} from '../../../../core/companyRegistration/client.mjs';
import {FiscalProviderCredentials} from './FiscalProviderCredentials';
import {useEffect,useMemo,useRef,useState} from 'react';
import {useEnterprise89} from '../../../../core/enterprise89/context';
import {useLocalSession} from '../../../../core/local/useLocalSession';
import {createFiscalCustomer,FiscalCustomerError,profileMatches,states} from '../../../../core/fiscalCustomer/client.mjs';
import type {OnboardingDiagnostic,Capabilities,Profile,ProfileInput,Requirements} from '../../../../core/fiscalCustomer/client.mjs';
import {fiscalWords} from '../../../../core/fiscalCustomer/words';
import './fiscalCustomer.css';

export function FiscalCustomer(){
 const {token,cap,locale,dirty}=useEnterprise89(), session=useLocalSession(s=>s.session), fw=fiscalWords(locale);
 const [diagnostic,setDiagnostic]=useState<OnboardingDiagnostic|null>(null);
 const [lookup,setLookup]=useState(''),[input,setInput]=useState<ProfileInput|null>(null),[profile,setProfile]=useState<Profile|null>(null);
 const [capabilities,setCapabilities]=useState<Capabilities|null>(null),[guide,setGuide]=useState<Requirements|null>(null);
 const [message,setMessage]=useState(''),[busy,setBusy]=useState(false),[edited,setEdited]=useState(false);
 const [pending,setPending]=useState<{input:ProfileInput;scope:string}|null>(null);
 const sequence=useRef(0),mounted=useRef(true),writing=useRef(false);
 const owner=cap.role==='owner', scope=[cap.tenant_id,cap.store_id,cap.device_id,cap.identity_id].join(' / ');
 const activeContext=useRef({token,scope,role:cap.role});activeContext.current={token,scope,role:cap.role};
 const current=useMemo(()=>()=>{
  const live=useLocalSession.getState();
  return activeContext.current.token===token && activeContext.current.scope===scope && activeContext.current.role===cap.role &&
   live.token===token && live.session===session && !!session && session.expires_unix>Date.now()/1000 &&
   session.tenant_id===cap.tenant_id && session.store_id===cap.store_id && session.identity_id===cap.identity_id && session.device_id===cap.device_id;
 },[token,session,scope,cap.role,cap.tenant_id,cap.store_id,cap.identity_id,cap.device_id]);
 const api=useMemo(()=>createFiscalCustomer({token,isCurrent:current}),[token,current]);
 const companyApi=useMemo(()=>createRegistrationClient({token,isCurrent:current}),[token,current]);
 useEffect(()=>{mounted.current=true;return()=>{mounted.current=false;sequence.current++;};},[]);
 useEffect(()=>{sequence.current++;setProfile(null);setInput(null);setGuide(null);setDiagnostic(null);setCapabilities(null);setEdited(false);setBusy(false);setMessage('');},[api]);
 useEffect(()=>{dirty('fiscal-customer',edited||!!pending);return()=>dirty('fiscal-customer',false);},[dirty,edited,pending]);
 function failure(error:unknown){setMessage(error instanceof FiscalCustomerError?error.code:'network');}
 async function diagnose(){
  if(!profile||edited)return;
  const n=++sequence.current;setBusy(true);setMessage('');setDiagnostic(null);
  try{const value=await api.onboarding(profile.input.profile_id);if(mounted.current&&n===sequence.current&&current()){
   if(value.profile_revision!==profile.revision){setMessage('conflict');return;}setDiagnostic(value);
  }}catch(error){if(mounted.current&&n===sequence.current)failure(error);}
  finally{if(mounted.current&&n===sequence.current)setBusy(false);}
 }
 async function refresh(){
  const n=++sequence.current;setBusy(true);setMessage('');setCapabilities(null);
  try{const value=await api.capabilities();if(mounted.current&&n===sequence.current&&current())setCapabilities(value);}
  catch(error){if(mounted.current&&n===sequence.current)failure(error);}
  finally{if(mounted.current&&n===sequence.current)setBusy(false);}
 }
 async function load(){
  const n=++sequence.current;setBusy(true);setMessage('');setProfile(null);setInput(null);setGuide(null);setDiagnostic(null);
  try{const value=await api.read(lookup);if(mounted.current&&n===sequence.current&&current()){setDiagnostic(null);setProfile(value);setInput({...value.input,expected_revision:value.revision});setEdited(false);}}
  catch(error){if(mounted.current&&n===sequence.current)failure(error);}
  finally{if(mounted.current&&n===sequence.current)setBusy(false);}
 }
 async function requirements(){
  if(!input)return;const uf=input.state,n=++sequence.current;setBusy(true);setMessage('');setGuide(null);
  try{const value=await api.requirements(uf);if(mounted.current&&n===sequence.current&&current())setGuide(value);}
  catch(error){if(mounted.current&&n===sequence.current)failure(error);}
  finally{if(mounted.current&&n===sequence.current)setBusy(false);}
 }
 async function save(){
  if(!input||!owner||pending||writing.current||!current())return;
  writing.current=true;setBusy(true);setMessage('');
  const attempt={input:{...input,operation_id:crypto.randomUUID()},scope};setPending(attempt);
  try{const value=await api.save(attempt.input);if(mounted.current&&current()){setDiagnostic(null);setProfile(value);setInput({...value.input,expected_revision:value.revision});setPending(null);setEdited(false);setMessage('saved');}}
  catch(error){if(mounted.current){
   // A definitive HTTP rejection permits correction. Missing/malformed responses
   // retain the original operation and block a second POST, including on logout.
   if(error instanceof FiscalCustomerError&&error.status>=400&&error.status<500&&current()){setPending(null);failure(error);}else setMessage('uncertain');
  }}finally{writing.current=false;if(mounted.current)setBusy(false);}
 }
 async function recover(){
  if(!pending||pending.scope!==scope||!current())return;
  setBusy(true);setMessage('');
  try{const value=await api.read(pending.input.profile_id,pending.input.expected_revision+1);
   if(mounted.current&&current()){
    if(!profileMatches(value,pending.input)){setMessage('conflict');return;}
    setDiagnostic(null);setProfile(value);setInput({...value.input,expected_revision:value.revision});setPending(null);setEdited(false);setMessage('saved');
   }
  }catch(error){if(mounted.current)failure(error);}finally{if(mounted.current)setBusy(false);}
 }
 function create(){if(!owner||!current()||pending)return;sequence.current++;const profile_id=crypto.randomUUID();setProfile(null);setGuide(null);setLookup(profile_id);setInput({operation_id:'',profile_id,expected_revision:0,cnpj:'',legal_name:'',state:'',municipality_ibge:'',state_registration:'',crt:'',model:'55',environment:'homologation',series:1});setEdited(true);setMessage('');void companyApi.read().then(company=>{if(mounted.current&&current())setInput(previous=>previous?.profile_id===profile_id?prefillFiscalProfile(previous,company.input):previous);}).catch(()=>{/* Manual entry remains available. */});}
 const blocked=busy||!!pending||!current();
 return <section className="brand-panel fiscal-customer" aria-labelledby="fiscal-customer-title">
  <h2 id="fiscal-customer-title">{fw.title}</h2><p>{fw.intro}</p><p><strong>{fw.draft}</strong></p><p>{fw.private}</p>
  <p>{fw.scope}: {scope}</p>{!owner&&<p>{fw.readonly}</p>}
  <div className="erp89-actions"><button type="button" className="brand-secondary" disabled={busy||!current()} onClick={()=>void refresh()}>{fw.refresh}</button></div>
  {capabilities&&<section aria-label={fw.installed}><h3>{fw.installed}</h3><dl>{(['transmission','nfse','sped'] as const).map((key,i)=><div key={key}><dt>{fw[key]}</dt><dd>{fw[[capabilities.transmission_available,capabilities.nfse_available,capabilities.sped_generation_available][i]?'yes':'no']}</dd></div>)}</dl></section>}
  <form onSubmit={e=>{e.preventDefault();void load();}} className="erp89-filters"><label>{fw.lookup}<input required pattern="[A-Za-z0-9][A-Za-z0-9_-]{0,127}" maxLength={128} value={lookup} disabled={blocked||edited} onChange={e=>setLookup(e.target.value)}/></label><button className="brand-secondary" disabled={blocked||edited}>{fw.load}</button><button type="button" className="brand-secondary" disabled={blocked||edited||!owner} onClick={create}>{fw.create}</button></form>
  {busy&&<p role="status">{fw.loading}</p>}{message&&<p role={message==='saved'?'status':'alert'}>{fw[message]||fw.request}</p>}
  {pending&&<section role="status"><p>{pending.scope!==scope?fw.session:fw.uncertain}</p><p>{fw.profile_id}: {pending.input.profile_id} · {pending.input.operation_id} · {fw.revision}: {pending.input.expected_revision+1}</p><button type="button" className="brand-secondary" disabled={busy||pending.scope!==scope||!current()} onClick={()=>void recover()}>{fw.recover}</button></section>}
  {input&&<form onSubmit={e=>{e.preventDefault();void save();}}><p>{fw.profile_id}: {input.profile_id} · {fw.revision}: {input.expected_revision}</p><fieldset disabled={blocked||!owner}><div className="erp89-form">
   {(['cnpj','legal_name','municipality_ibge','state_registration'] as const).map(key=><label key={key}>{fw[key]}<input required name={key} value={input[key]} maxLength={key==='legal_name'?180:key==='cnpj'?14:key==='municipality_ibge'?7:20} pattern={key==='cnpj'?'[A-Z0-9]{12}[0-9]{2}':key==='municipality_ibge'?'[0-9]{7}':undefined} onChange={e=>{setInput({...input,[key]:e.target.value});setEdited(true);}}/></label>)}
   {(['state','crt','model','environment'] as const).map(key=><label key={key}>{fw[key]}<select required name={key} value={input[key]} onChange={e=>{sequence.current++;setGuide(null);setInput({...input,[key]:e.target.value});setEdited(true);}}><option value="">{fw.choose}</option>{(key==='state'?states:key==='crt'?['1','2','3','4']:key==='model'?['55','65']:['homologation','production']).map(v=><option key={v} value={v}>{key==='environment'?fw[v]:v}</option>)}</select></label>)}
   <label>{fw.series}<input required type="number" min={0} max={889} step={1} value={input.series} onChange={e=>{setInput({...input,series:Number(e.target.value)});setEdited(true);}}/></label>
  </div><button className="brand-primary" disabled={!edited}>{fw.save}</button><button type="button" className="brand-secondary" disabled={!edited} onClick={()=>{setInput(profile?{...profile.input,expected_revision:profile.revision}:null);setEdited(false);setGuide(null);setMessage('');}}>{fw.discard}</button></fieldset><button type="button" className="brand-secondary" disabled={busy||!current()||!states.includes(input.state)} onClick={()=>void requirements()}>{fw.ufGuide}</button></form>}
  {guide&&<section aria-label={fw.ufGuide}><h3>{guide.uf} · {guide.reference_date}</h3><p>{fw.dated}</p><dl>{(['issuer','developer','limits'] as const).map((key,i)=><div key={key}><dt>{fw[key]}</dt><dd>{[guide.issuer_requirements,guide.developer_requirements,guide.limitations][i]}</dd></div>)}</dl>{guide.sources.filter(s=>{try{const u=new URL(s.url);return u.protocol==='https:'&&u.hostname.endsWith('.gov.br');}catch{return false;}}).map((s,i)=><p key={i}><a href={s.url} target="_blank" rel="noreferrer">{s.title}</a></p>)}</section>}
  {profile&&owner&&!edited&&profile.input.model==='65'&&<FiscalProviderCredentials key={[scope,profile.input.profile_id,profile.revision].join(':')} api={api} profile={profile} isCurrent={current} words={fw}/>}
  {profile&&<section aria-label={fw.onboardingTitle}><h3>{fw.onboardingTitle}</h3><p>{fw.onboardingIntro}</p><button type="button" className="brand-secondary" disabled={blocked||edited} onClick={()=>void diagnose()}>{fw.onboardingCheck}</button>
   {diagnostic&&!edited&&diagnostic.profile_revision===profile.revision&&<div role="status"><p>{fw.onboardingConfiguration}: {fw[diagnostic.configuration_valid?'yes':'no']}</p>{diagnostic.provider_configuration&&<dl>
    <div><dt>{fw.onboardingProvider}</dt><dd>Focus NFC-e</dd></div>
    <div><dt>{fw.environment}</dt><dd>{fw.homologation}</dd></div>
    <div><dt>{fw.onboardingCertificateType}</dt><dd>{diagnostic.provider_configuration.certificate_type}</dd></div>
    <div><dt>{fw.onboardingTokenPresent}</dt><dd>{fw[diagnostic.provider_configuration.token_present?'yes':'no']}</dd></div>
    <div><dt>{fw.onboardingCSCPresent}</dt><dd>{fw[diagnostic.provider_configuration.csc_present?'yes':'no']}</dd></div>
    <div><dt>{fw.onboardingLocalOnly}</dt><dd>{fw.onboardingNoRemoteVerification}</dd></div>
   </dl>}<ul>{diagnostic.blockers.map(code=><li key={code}>{fw[code]}</li>)}</ul></div>}
  </section>}
  {profile&&<section><h3>{fw.blockers}</h3><ul>{profile.readiness.blockers.map((code,i)=><li key={i}>{fw[code]||code}</li>)}</ul><p>{fw.draft}</p></section>}
 </section>;
}
