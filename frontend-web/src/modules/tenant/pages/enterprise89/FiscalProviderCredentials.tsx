import {useEffect,useRef,useState} from 'react';
import {createFiscalCustomer,FiscalCustomerError} from '../../../../core/fiscalCustomer/client.mjs';
import type {CustomerCredentialStatus,Profile} from '../../../../core/fiscalCustomer/client.mjs';
type Props={api:ReturnType<typeof createFiscalCustomer>;profile:Profile;isCurrent:()=>boolean;words:Record<string,string>};
export function FiscalProviderCredentials({api,profile,isCurrent,words:fw}:Props){
 const [status,setStatus]=useState<CustomerCredentialStatus|null>(null),[token,setToken]=useState(''),[csc,setCSC]=useState('');
 const [busy,setBusy]=useState(false),[message,setMessage]=useState(''),[pending,setPending]=useState<string|null>(null);
 const generation=useRef(0),alive=useRef(true),writing=useRef(false);
 useEffect(()=>{alive.current=true;return()=>{alive.current=false;generation.current++;};},[]);
 useEffect(()=>{generation.current++;setToken('');setCSC('');setStatus(null);setPending(null);setBusy(false);setMessage('');},[api]);
 function error(e:unknown){setMessage(e instanceof FiscalCustomerError?e.code:'network');}
 async function read(){
  if(writing.current||!isCurrent())return;
  const n=++generation.current;setBusy(true);setMessage('');
  try{const value=await api.providerStatus(profile.input.profile_id);if(alive.current&&n===generation.current&&isCurrent()){
   setStatus(value);if(pending&&value.operation_id!==pending){setMessage('conflict');return;}setPending(null);
  }}catch(e){if(alive.current&&n===generation.current)error(e);}finally{if(alive.current&&n===generation.current)setBusy(false);}
 }
 async function save(){
  if(!status||busy||writing.current||pending||!isCurrent())return;
  generation.current++;writing.current=true;setBusy(true);setMessage('');const operation_id=crypto.randomUUID();setPending(operation_id);
  try{
   const result=api.saveProviderCredentials(profile.input.profile_id,{operation_id,expected_revision:status.revision,profile_revision:profile.revision,token,csc});
   setToken('');setCSC('');
   const value=await result;if(alive.current&&isCurrent()){setStatus(value);setPending(null);setMessage('saved');}
  }catch(e){if(alive.current&&isCurrent()){
   if(e instanceof FiscalCustomerError&&(e.code==='invalid'||(e.status>=400&&e.status<500))){setPending(null);setStatus(null);error(e);}else setMessage('uncertain');
  }}finally{writing.current=false;if(alive.current)setBusy(false);}
 }
 return <section aria-label={fw.providerCredentialsTitle}><h3>{fw.providerCredentialsTitle}</h3><p>{fw.providerCredentialsIntro}</p>
  <button type="button" className="brand-secondary" disabled={busy||!isCurrent()} onClick={()=>void read()}>{pending?fw.recover:fw.providerCredentialsRead}</button>
  {message&&<p role={message==='saved'?'status':'alert'}>{fw[message]||fw.request}</p>}
  {pending&&<p role="status">{fw.uncertain} · {pending}</p>}
  {status&&<dl><div><dt>{fw.revision}</dt><dd>{status.revision}</dd></div><div><dt>{fw.onboardingTokenPresent}</dt><dd>{fw[status.token_present?'yes':'no']}</dd></div><div><dt>{fw.onboardingCSCPresent}</dt><dd>{fw[status.csc_present?'yes':'no']}</dd></div></dl>}
  <form onSubmit={e=>{e.preventDefault();void save();}} autoComplete="off"><fieldset disabled={!status||busy||!!pending||!isCurrent()}>
   <label>{fw.providerToken}<input type="password" autoComplete="off" value={token} maxLength={512} required onChange={e=>setToken(e.target.value)}/></label>
   <label>{fw.providerCSC}<input type="password" autoComplete="off" value={csc} maxLength={128} onChange={e=>setCSC(e.target.value)}/></label>
   <p>{fw.providerReplaceWarning}</p><button type="submit" className="brand-primary">{fw.providerCredentialsSave}</button>
  </fieldset></form><p>{fw.onboarding_production_blocked}</p>
 </section>;
}
