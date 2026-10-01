import { useState } from 'react';
import type { FormEvent } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { localErrorMessage, useLocalSession } from '../../../core/local/useLocalSession';

export default function LocalLogin() {
  const [identityID, setIdentityID] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const login = useLocalSession((state) => state.login);
  const notice = useLocalSession((state) => state.notice);
  const navigate = useNavigate();
  async function submit(event: FormEvent) {
    event.preventDefault(); if (busy) return;
    setBusy(true); setError('');
    try { await login(identityID.trim(), password); navigate('/local/catalog', { replace: true }); }
    catch (failure) { setError(localErrorMessage(failure)); }
    finally { setPassword(''); setBusy(false); }
  }
  return <main className="min-h-screen bg-slate-900 text-slate-100 flex items-center justify-center p-4">
    <section className="w-full max-w-md rounded-2xl border border-slate-700 bg-slate-800 p-6">
      <h1 className="text-2xl font-bold">TitanSystem — instalação local</h1>
      <p className="mt-3 text-sm text-slate-300">Entre com o ID do operador e a senha cadastrados nesta instalação.</p>
      <p className="mt-2 text-sm text-slate-400">A API local precisa estar em execução neste PC. Esta conexão ainda não permite vender no celular com o PC desligado.</p>
      {notice && <p role="status" className="mt-4 text-sm text-amber-300">{notice}</p>}
      <form onSubmit={submit} className="mt-6 space-y-4">
        <label className="block">ID do operador<input autoComplete="username" required disabled={busy} value={identityID} onChange={(event) => setIdentityID(event.target.value)} className="mt-1 block w-full rounded bg-slate-900 p-3" /></label>
        <label className="block">Senha<input type="password" autoComplete="current-password" required disabled={busy} value={password} onChange={(event) => setPassword(event.target.value)} className="mt-1 block w-full rounded bg-slate-900 p-3" /></label>
        {error && <p role="alert" className="text-sm text-red-300">{error}</p>}
        <button disabled={busy} className="w-full rounded bg-indigo-600 p-3 font-semibold disabled:opacity-50">{busy ? 'Verificando sessão…' : 'Entrar na instalação local'}</button>
      </form>
      <Link to="/login" className="mt-5 inline-block text-sm text-indigo-300">Acessar login da API online</Link>
    </section>
  </main>;
}
