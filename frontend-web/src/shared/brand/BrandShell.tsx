import {useId} from 'react';
import type {ReactNode} from 'react';
import {Link,useLocation} from 'react-router-dom';
import {useLocalSession} from '../../core/local/useLocalSession';
import {useAuthStore} from '../../core/auth/useAuthStore';
import {logoDestination} from '../../core/local/navigationDestination.mjs';
import {useTranslation} from 'react-i18next';
import ContactDrawer from './ContactDrawer';
import {useTheme} from './theme';
import './brand.css';
import {contactConfig} from './contactConfig';
const symbolPath = 'M50 0Q51 0 53 1L92 22Q94 23 94 26V29Q94 31 92 32L82 37V29H18V37L8 32Q6 31 6 29V26Q6 23 8 22L47 1Q49 0 50 0Z M3 35Q0 33 0 37V82Q0 84 2 85L35 103Q38 105 38 101V52Z M97 35Q100 33 100 37V82Q100 84 98 85L65 103Q62 105 62 101V52Z';
export function BrandLogo({compact=false}:{compact?:boolean}) {
  const {t} = useTranslation();
  const id = useId().replace(/:/g,'');
  const {pathname}=useLocation();
  const session=useLocalSession(state=>state.session),token=useLocalSession(state=>state.token);
  const onlineActive=useAuthStore(state=>state.isAuthenticated);
  const localActive=!!token&&!!session&&session.expires_unix*1000>Date.now();
  const destination=logoDestination({pathname,localActive,onlineActive});
  return <Link to={destination} className="brand-logo" aria-label={t(localActive||onlineActive?'navigation.backToMenu':'login.home')}>
    <svg className="brand-mark" viewBox="0 0 100 104" aria-hidden="true">
      <defs><clipPath id={id+'clip'}><path d={symbolPath}/></clipPath><linearGradient id={id+'shine'}><stop offset="0" stopColor="var(--accent)" stopOpacity="0"/><stop offset=".4" stopColor="var(--accent)" stopOpacity=".9"/><stop offset=".55" stopColor="white" stopOpacity=".7"/><stop offset="1" stopColor="var(--accent)" stopOpacity="0"/></linearGradient></defs>
      <path d={symbolPath} fill="currentColor"/>
      <g clipPath={`url(#${id}clip)`}><g transform="rotate(-20 50 52)"><rect className="logo-shine" x="-140" y="-50" width="75" height="210" fill={`url(#${id}shine)`}/></g></g>
    </svg>
    {!compact && <strong>TITAN</strong>}
  </Link>;
}
export function ThemeToggle() {
  const {t} = useTranslation(), {theme,setTheme} = useTheme();
  return <div className="brand-theme-toggle" role="group" aria-label={t('theme.appearance')}>
    <button type="button" aria-pressed={theme==='light'} onClick={()=>setTheme('light')} className={theme==='light'?'selected':''}><span aria-hidden="true">☼</span>{t('theme.light')}</button>
    <button type="button" aria-pressed={theme==='dark'} onClick={()=>setTheme('dark')} className={theme==='dark'?'selected':''}><span aria-hidden="true">☾</span>{t('theme.dark')}</button>
  </div>;
}
export function BrandShell({children,footer=true}:{children:ReactNode;footer?:boolean}) {
  const {t} = useTranslation();
  return <div className="brand-page auth-surface">
    <header className="brand-topbar"><BrandLogo/><div className="brand-topbar-actions"><div className="header-theme"><ThemeToggle/></div><ContactDrawer/></div></header>
    <main className="brand-content">{children}</main>
    {footer && <footer className="brand-footer">{contactConfig.termsUrl?<a href={contactConfig.termsUrl}>{t('menu.terms')}</a>:<span>{t('menu.terms')}</span>}<span aria-hidden="true">·</span>{contactConfig.privacyUrl?<a href={contactConfig.privacyUrl}>{t('menu.privacy')}</a>:<span>{t('menu.privacy')}</span>}</footer>}
  </div>;
}
