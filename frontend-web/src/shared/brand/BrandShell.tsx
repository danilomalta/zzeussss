import { useEffect, useState } from 'react';
import type { ReactNode } from 'react';
import './brand.css';
import ContactDrawer from './ContactDrawer';

type Theme = 'light' | 'dark';
const themeKey = 'titansystem-theme';

function readTheme(): Theme {
  try {
    const saved = localStorage.getItem(themeKey);
    if (saved === 'light' || saved === 'dark') return saved;
  } catch { /* Private browsing can disable storage. */ }
  return window.matchMedia?.('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

function applyTheme(theme: Theme) {
  document.documentElement.dataset.theme = theme;
  document.documentElement.style.colorScheme = theme;
  try { localStorage.setItem(themeKey, theme); } catch { /* Visual preference remains in memory. */ }
}

export function BrandLogo({ compact = false }: { compact?: boolean }) {
  return <span className="brand-logo" aria-label="TitanSystem">
    <span className="brand-mark" aria-hidden="true" />
    {!compact && <strong>TITAN</strong>}
  </span>;
}

export function ThemeToggle() {
  const [theme, setTheme] = useState<Theme>(() => readTheme());
  useEffect(() => { applyTheme(theme); }, [theme]);
  return <div className="brand-theme-toggle" role="group" aria-label="Aparência">
    <button type="button" aria-pressed={theme === 'light'} onClick={() => setTheme('light')} className={theme === 'light' ? 'selected' : ''}>
      <span aria-hidden="true">☼</span> Claro
    </button>
    <button type="button" aria-pressed={theme === 'dark'} onClick={() => setTheme('dark')} className={theme === 'dark' ? 'selected' : ''}>
      <span aria-hidden="true">☾</span> Escuro
    </button>
  </div>;
}

export function BrandShell({ children, footer = true }: { children: ReactNode; footer?: boolean }) {
  return <div className="brand-page">
    <div className="brand-wave brand-wave-left" aria-hidden="true" />
    <div className="brand-wave brand-wave-right" aria-hidden="true" />
    <header className="brand-topbar"><BrandLogo /><div className="brand-topbar-actions"><ContactDrawer /><ThemeToggle /></div></header>
    <main className="brand-content">{children}</main>
    {footer && <footer className="brand-footer"><span>Termos de uso</span><span aria-hidden="true">•</span><span>Privacidade</span></footer>}
  </div>;
}
