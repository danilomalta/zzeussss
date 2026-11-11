import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { useAuthStore } from '../../../core/auth/useAuthStore';
import { BrandLogo, ThemeToggle } from '../../../shared/brand/BrandShell';

export default function AdminDashboard() {
  const { t } = useTranslation();
  const user = useAuthStore((state) => state.user);
  const logout = useAuthStore((state) => state.logout);
  const logoutUncertain = useAuthStore((state) => state.logoutUncertain);
  const [busy, setBusy] = useState(false);
  async function onLogout() {
    if (busy) return;
    setBusy(true);
    try { await logout(); } finally { setBusy(false); }
  }
  return <div className="brand-page brand-app-page">
    <header className="brand-topbar"><BrandLogo /><ThemeToggle /></header>
    <main className="brand-app-content">
      <div className="brand-app-heading"><div><p className="brand-eyebrow">ÁREA DA CONTA</p>
        <h1>Olá, {user?.name || 'bem-vindo'}</h1><p className="brand-subtitle">Sua conta TitanSystem</p></div>
        <button type="button" className="brand-secondary" disabled={busy} onClick={() => void onLogout()}>{t('workspace.logout')}</button></div>
      {logoutUncertain && <p role="alert">{t('account.logoutUncertain')}</p>}
      <section className="brand-panel brand-feature-panel">
        <h2>Seu espaço está em construção</h2>
        <p>Cadastro de empresas, planos e pagamentos ainda precisam ser ligados a esta área. As operações locais desta fase usam o acesso da instalação.</p>
        <Link className="brand-feature-link" to="/local/login">Abrir acesso local</Link>
      </section>
    </main>
  </div>;
}
