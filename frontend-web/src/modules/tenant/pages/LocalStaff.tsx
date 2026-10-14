import { useEffect, useRef, useState } from 'react';
import { Link, useBlocker } from 'react-router-dom';
import { wordsFor } from '../../../core/hr71/words';
import { useLocalAccess } from '../../../core/local/LocalAccess';
import {
  localClient,
  localErrorMessage,
  useLocalSession,
} from '../../../core/local/useLocalSession';
import type { StaffMember } from '../../../core/local/localClient.mjs';
import { useTranslation } from 'react-i18next';
import './localStaff.css';
export default function LocalStaff() {
  const { t, i18n } = useTranslation();
  const hrWords = wordsFor(i18n.resolvedLanguage || i18n.language);
  const token = useLocalSession((s) => s.token),
    session = useLocalSession((s) => s.session);
  const { capabilities } = useLocalAccess();
  const [showPassword, setShowPassword] = useState(false);
  const [rows, setRows] = useState<StaffMember[]>([]),
    [loading, setLoading] = useState(true),
    [name, setName] = useState(''),
    [role, setRole] = useState('employee'),
    [password, setPassword] = useState(''),
    [busy, setBusy] = useState(false),
    [message, setMessage] = useState(''),
    [error, setError] = useState(''),
    [revision, setRevision] = useState(0);
  const [pending, setPending] = useState<{
    operation_id: string;
    identity_id: string;
    name: string;
    role: string;
  } | null>(null);
  const latch = useRef(false);
  const key = `titan-staff-pending:${session?.tenant_id}:${session?.store_id}:${session?.device_id}:${session?.identity_id}`;
  useEffect(() => {
    try {
      const raw = localStorage.getItem(key);
      if (raw) {
        const value = JSON.parse(raw);
        if (
          !value ||
          !['operation_id', 'identity_id', 'name', 'role'].every(
            (k) => typeof value[k] === 'string'
          ) ||
          Object.keys(value).length !== 4
        )
          throw new Error();
        setPending(value);
        setName(value.name);
        setRole(value.role);
      }
    } catch {
      setError(
        'Não foi possível conferir o cadastro pendente. Não envie outro cadastro antes da conferência.'
      );
      setPending({ operation_id: '', identity_id: '', name: '', role: '' });
    }
  }, [key]);
  useEffect(() => {
    if (!token) return;
    let live = true;
    setLoading(true);
    localClient
      .staff(token)
      .then((v) => {
        if (live) setRows(v);
      })
      .catch((e) => {
        if (live) setError(localErrorMessage(e));
      })
      .finally(() => {
        if (live) setLoading(false);
      });
    return () => {
      live = false;
    };
  }, [token, revision]);
  const roles =
    capabilities?.role === 'owner'
      ? ['employee', 'cashier', 'stock', 'production', 'accountant', 'manager', 'supplier']
      : ['employee', 'cashier', 'stock'];
  const roleName = (value: string) => t('staffUi.role.' + value, { defaultValue: value });
  async function save() {
    if (latch.current || !token || capabilities?.license.state !== 'active') return;
    latch.current = true;
    setBusy(true);
    setError('');
    setMessage('');
    const input = pending || {
      operation_id: crypto.randomUUID(),
      identity_id: crypto.randomUUID(),
      name: name.trim(),
      role,
    };
    try {
      if (!input.operation_id || !input.identity_id)
        throw new Error('Cadastro pendente inválido. Solicite conferência técnica.');
      if (
        new TextEncoder().encode(password).length < 12 ||
        new TextEncoder().encode(password).length > 72
      )
        throw new Error('A senha precisa ter entre 12 e 72 bytes.');
      if (!navigator.locks) throw new Error('Use navegador com proteção entre abas em localhost.');
      await navigator.locks.request(key, { ifAvailable: true }, async (lock) => {
        if (!lock) throw new Error('Outra aba está cadastrando um funcionário.');
        const existing = localStorage.getItem(key);
        if (existing && existing !== JSON.stringify(input))
          throw new Error('Há outro cadastro pendente. Atualize a página.');
        localStorage.setItem(key, JSON.stringify(input));
        setPending(input);
        await localClient.createStaff(token, { ...input, password });
        localStorage.removeItem(key);
        setPending(null);
        setName('');
        setMessage(t('staffUi.created'));
        setRevision((v) => v + 1);
      });
    } catch (e) {
      setError(e instanceof Error ? e.message : localErrorMessage(e));
    } finally {
      setPassword('');
      setBusy(false);
      latch.current = false;
    }
  }
  const unsaved =
    Boolean(token) && (busy || Boolean(password) || (!pending && Boolean(name.trim())));
  const leaveMessage = hrWords.unsaved;
  const blocker = useBlocker(
    ({ currentLocation, nextLocation }) =>
      unsaved && currentLocation.pathname !== nextLocation.pathname
  );
  useEffect(() => {
    if (blocker.state === 'blocked') {
      if (window.confirm(leaveMessage)) blocker.proceed();
      else blocker.reset();
    }
  }, [blocker, leaveMessage]);
  useEffect(() => {
    if (!unsaved) return;
    const warn = (event: BeforeUnloadEvent) => {
      event.preventDefault();
      event.returnValue = '';
    };
    window.addEventListener('beforeunload', warn);
    return () => window.removeEventListener('beforeunload', warn);
  }, [unsaved]);
  const bytes = new TextEncoder().encode(password).length;
  const strength = bytes === 0 ? 'empty' : bytes < 12 ? 'weak' : bytes < 24 ? 'medium' : 'strong';
  return (
    <main className="brand-app-content staff-page">
      <nav
        aria-label={hrWords.breadcrumb}
        style={{ display: 'flex', gap: 8, fontSize: 14, marginBottom: 16, alignItems: 'center' }}
      >
        <Link
          style={{ color: 'var(--text)', padding: '12px 0', textUnderlineOffset: 4 }}
          to="/local/hr"
        >
          {hrWords.overview}
        </Link>
        <span aria-hidden="true">›</span>
        <span>{hrWords['area.people']}</span>
      </nav>
      <header className="staff-heading">
        <h1>{t('staffUi.title')}</h1>
        <p>{t('staffUi.subtitle')}</p>
      </header>
      {error && (
        <p role="alert" className="brand-message error">
          {error}
        </p>
      )}
      {message && (
        <p role="status" className="brand-message">
          {message}
        </p>
      )}
      <section id="staff-form" className="brand-panel staff-panel">
        <h2>{t(pending ? 'staffUi.pending' : 'staffUi.add')}</h2>
        {pending && <p className="brand-state">{t('staffUi.retryHelp')}</p>}
        <form
          className="catalog-form"
          onSubmit={(e) => {
            e.preventDefault();
            void save();
          }}
        >
          <label>
            {t('staffUi.name')}
            <input
              required
              maxLength={255}
              value={name}
              placeholder={t('staffUi.namePlaceholder')}
              disabled={busy || !!pending}
              onChange={(e) => setName(e.target.value)}
            />
          </label>
          <label>
            {t('staffUi.profile')}
            <select
              value={role}
              disabled={busy || !!pending}
              onChange={(e) => setRole(e.target.value)}
            >
              {roles.map((r) => (
                <option key={r} value={r}>
                  {roleName(r)}
                </option>
              ))}
            </select>
          </label>
          <label className="catalog-wide">
            {t('staffUi.password')}
            <span className="staff-password-input">
              <input
                required
                type={showPassword ? 'text' : 'password'}
                autoComplete="new-password"
                value={password}
                placeholder={t('staffUi.passwordPlaceholder')}
                disabled={busy}
                onChange={(e) => setPassword(e.target.value)}
              />
              <button
                type="button"
                className="brand-secondary"
                onClick={() => setShowPassword((v) => !v)}
                aria-label={t(showPassword ? 'staffUi.hide' : 'staffUi.show')}
              >
                {t(showPassword ? 'staffUi.hide' : 'staffUi.show')}
              </button>
            </span>
            <span className={'staff-strength ' + strength} aria-live="polite">
              {t('staffUi.strength')}: {t('staffUi.' + strength)}
            </span>
          </label>
          <button
            className="brand-small-primary catalog-wide"
            disabled={busy || capabilities?.license.state !== 'active'}
          >
            {t(busy ? 'staffUi.saving' : pending ? 'staffUi.retry' : 'staffUi.add')}
          </button>
        </form>
      </section>
      <section className="brand-panel staff-panel">
        <h2>{t('staffUi.list')}</h2>
        {loading ? (
          <p role="status">{t('common.loading')}</p>
        ) : rows.length === 0 ? (
          <div className="staff-empty" role="status">
            <span aria-hidden="true">♧</span>
            <strong>{t('staffUi.noEmployees')}</strong>
            <p>{t('staffUi.emptyHelp')}</p>
            <a href="#staff-form">{t('staffUi.add')}</a>
          </div>
        ) : (
          <div className="brand-table-scroll">
            <table className="brand-table">
              <thead>
                <tr>
                  <th>{t('staffUi.name')}</th>
                  <th>{t('staffUi.profile')}</th>
                  <th>{t('staffUi.access')}</th>
                  <th>{t('staffUi.status')}</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((r) => (
                  <tr key={r.identity_id}>
                    <td>{r.name}</td>
                    <td>{roleName(r.role)}</td>
                    <td>
                      <details>
                        <summary>{t('staffUi.accessDetails')}</summary>
                        <code>{r.identity_id}</code>
                      </details>
                    </td>
                    <td>
                      <span
                        className={'staff-badge ' + (r.status === 'active' ? 'active' : 'inactive')}
                      >
                        {t(r.status === 'active' ? 'staffUi.active' : 'staffUi.inactive')}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </main>
  );
}
