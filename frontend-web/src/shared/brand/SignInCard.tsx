import { Link } from 'react-router-dom';
import { useState } from 'react';
import type { FormEvent, ReactNode } from 'react';
import { BrandLogo, BrandShell } from './BrandShell';

function Icon({ kind }: { kind: 'user' | 'lock' | 'eye' }) {
  return <svg aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
    {kind === 'user' ? <><circle cx="12" cy="7" r="4" /><path d="M4 21v-2a8 8 0 0 1 16 0v2" /></> :
      kind === 'lock' ? <><rect x="5" y="10" width="14" height="11" rx="2" /><path d="M8 10V7a4 4 0 0 1 8 0v3" /></> :
        <><path d="M2 12s3.5-6 10-6 10 6 10 6-3.5 6-10 6S2 12 2 12Z" /><circle cx="12" cy="12" r="2.5" /></>}
  </svg>;
}

interface SignInCardProps {
  credentialLabel: string;
  credentialPlaceholder: string;
  credentialType: 'email' | 'text';
  credentialHelp?: string;
  subtitle: string;
  onSubmit: (credential: string, password: string) => Promise<void>;
  errorMessage: (error: unknown) => string;
  notice?: string;
  secondary: ReactNode;
  signupHelp: string;
  recoveryHelp: string;
}

export default function SignInCard({ credentialLabel, credentialPlaceholder, credentialType, credentialHelp, subtitle, onSubmit, errorMessage, notice, secondary, recoveryHelp }: SignInCardProps) {
  const [credential, setCredential] = useState('');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [error, setError] = useState('');
  const [help, setHelp] = useState('');
  const [busy, setBusy] = useState(false);

  async function submit(event: FormEvent) {
    event.preventDefault();
    if (busy) return;
    setBusy(true); setError(''); setHelp('');
    try { await onSubmit(credential.trim(), password); }
    catch (failure) { setError(errorMessage(failure)); }
    finally { setPassword(''); setBusy(false); }
  }

  return <BrandShell>
    <section className="brand-card" aria-label="Entrar no TitanSystem">
      <div className="brand-card-head">
        <BrandLogo />
        <h1>Bem-vindo ao Titan</h1>
        <p className="brand-subtitle">{subtitle}</p>
      </div>
      {notice && <p className="brand-message" role="status">{notice}</p>}
      <form onSubmit={submit}>
        <label htmlFor="brand-credential" className="brand-field">{credentialLabel}</label>
        <div className="brand-input-wrap"><Icon kind="user" />
          <input id="brand-credential" type={credentialType} autoComplete="username" aria-describedby={credentialHelp ? 'credential-help' : undefined} required disabled={busy} placeholder={credentialPlaceholder} value={credential} onChange={(event) => setCredential(event.target.value)} />
        </div>
        {credentialHelp && <p id="credential-help" className="brand-input-help">{credentialHelp}</p>}
        <label htmlFor="brand-password" className="brand-field">Senha</label>
        <div className="brand-input-wrap"><Icon kind="lock" />
          <input id="brand-password" type={showPassword ? 'text' : 'password'} autoComplete="current-password" required disabled={busy} value={password} onChange={(event) => setPassword(event.target.value)} />
          <button type="button" className="brand-icon-button" aria-label={showPassword ? 'Ocultar senha' : 'Mostrar senha'} onClick={() => setShowPassword((value) => !value)}><Icon kind="eye" /></button>
        </div>
        <div className="brand-help"><button type="button" className="brand-text-button" onClick={() => setHelp(recoveryHelp)}>Esqueceu a senha?</button></div>
        {help && <p role="status" className="brand-message">{help}</p>}
        {error && <p role="alert" className="brand-message error">{error}</p>}
        <button className="brand-primary" type="submit" disabled={busy} aria-busy={busy}>{busy && <span className="brand-spinner" aria-hidden="true" />}{busy ? 'Verificando acesso…' : 'Entrar'}</button>
      </form>
      <div className="brand-card-foot">Ainda não tem conta? <Link viewTransition to="/register">Criar conta</Link></div>
    </section>
    <div className="auth-secondary">{secondary}</div>
  </BrandShell>;
}
