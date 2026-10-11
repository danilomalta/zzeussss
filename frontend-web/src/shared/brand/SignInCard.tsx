import {Link} from 'react-router-dom';
import {useState} from 'react';
import type {FormEvent} from 'react';
import {useTranslation} from 'react-i18next';
import {BrandLogo,BrandShell} from './BrandShell';

function Icon({kind}:{kind:'user'|'lock'|'eye'}) {
  return <svg aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">{kind==='user'?<><circle cx="12" cy="7" r="4"/><path d="M4 21v-2a8 8 0 0 1 16 0v2"/></>:kind==='lock'?<><rect x="5" y="10" width="14" height="11" rx="2"/><path d="M8 10V7a4 4 0 0 1 8 0v3"/></>:<><path d="M2 12s3.5-6 10-6 10 6 10 6-3.5 6-10 6S2 12 2 12Z"/><circle cx="12" cy="12" r="2.5"/></>}</svg>;
}
interface Props {mode:'local'|'online';onSubmit:(credential:string,password:string)=>Promise<void>;notice?:boolean;}
export default function SignInCard({mode,onSubmit,notice}:Props) {
  const {t}=useTranslation();
  const [credential,setCredential]=useState(''),[password,setPassword]=useState(''),[show,setShow]=useState(false);
  const [error,setError]=useState(''),[help,setHelp]=useState(false),[busy,setBusy]=useState(false);
  async function submit(event:FormEvent) {
    event.preventDefault();if(busy)return;setError('');setHelp(false);
    if(!credential.trim()||!password){setError('errors.required');return;}
    if(mode==='online') {const field=document.createElement('input');field.type='email';field.value=credential.trim();if(!field.checkValidity()){setError('errors.invalidEmail');return;}}
    setBusy(true);
    try {await onSubmit(credential.trim(),password);}
    catch(failure) {
      const value=failure as {status?:number;response?:{status?:number}};
      const status=value?.status??value?.response?.status;
      setError([401,403,429].includes(status??0)?'errors.'+status:'errors.connection');
    } finally {setPassword('');setBusy(false);}
  }
  return <BrandShell><section className="brand-card" aria-labelledby="login-title">
    <div className="brand-card-head"><BrandLogo/><h1 id="login-title">{t('login.title')}</h1><p className="brand-subtitle">{t('login.subtitle')}</p></div>
    {notice&&<p role="status" className="brand-message">{t('errors.session')}</p>}
    <form noValidate onSubmit={submit}>
      <label htmlFor="brand-credential" className="brand-field">{t('login.credential')}</label><div className="brand-input-wrap"><Icon kind="user"/><input id="brand-credential" type="text" autoComplete="username" autoCapitalize="none" spellCheck={false} required disabled={busy} aria-describedby="credential-help" aria-invalid={!!error} placeholder={t('login.placeholder')} value={credential} onChange={event=>setCredential(event.target.value)}/></div>
      <p id="credential-help" className="brand-input-help">{t(mode==='local'?'login.localHelp':'login.onlineHelp')}</p>
      <label htmlFor="brand-password" className="brand-field">{t('login.password')}</label><div className="brand-input-wrap"><Icon kind="lock"/><input id="brand-password" type={show?'text':'password'} autoComplete="current-password" required disabled={busy} aria-invalid={!!error} aria-describedby={error?'login-error':undefined} value={password} onChange={event=>setPassword(event.target.value)}/><button type="button" className="brand-icon-button" aria-label={t(show?'login.hide':'login.show')} onClick={()=>setShow(value=>!value)}><Icon kind="eye"/></button></div>
      <div className="brand-help"><button type="button" className="brand-text-button" onClick={()=>setHelp(true)}>{t('login.forgot')}</button></div>
      {help&&<p role="status" className="brand-message">{t(mode==='local'?'login.recoveryLocal':'login.recoveryOnline')}</p>}
      {error&&<p id="login-error" role="alert" className="brand-message error">{t(error)}</p>}
      <button type="submit" className="brand-primary" disabled={busy} aria-busy={busy}>{busy&&<span className="brand-spinner" aria-hidden="true"/>}{t(busy?'login.loading':'login.enter')}</button>
    </form>
    <div className="brand-card-foot">{t('login.noAccount')} <Link to="/register">{t('login.create')}</Link></div>
  </section><div className="auth-secondary"><Link to={mode==='local'?'/login':'/local/login'}>{t(mode==='local'?'login.online':'login.local')}</Link></div></BrandShell>;
}
