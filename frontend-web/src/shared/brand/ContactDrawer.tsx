import {useEffect,useRef,useState} from 'react';
import {createPortal} from 'react-dom';
import {useTranslation} from 'react-i18next';
import {languages} from '../../i18n';
import {useTheme} from './theme';
import {contactConfig as contacts} from './contactConfig';

export default function ContactDrawer() {
  const {t,i18n} = useTranslation(), {theme,setTheme} = useTheme();
  const [open,setOpen] = useState(false), [legal,setLegal] = useState(false);
  const trigger = useRef<HTMLButtonElement>(null), dialog = useRef<HTMLElement>(null), close = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    if (!open) return;
    const overflow = document.body.style.overflow; document.body.style.overflow='hidden'; close.current?.focus();
    function keydown(event:KeyboardEvent) {
      if (event.key==='Escape') {event.preventDefault();setOpen(false);}
      if (event.key!=='Tab') return;
      const elements=Array.from(dialog.current?.querySelectorAll<HTMLElement>('a[href],button:not([disabled]),select,[tabindex="0"]')??[]);
      const first=elements[0],last=elements[elements.length-1];
      if (event.shiftKey && (document.activeElement===first || !dialog.current?.contains(document.activeElement))) {event.preventDefault();last?.focus();}
      else if (!event.shiftKey && (document.activeElement===last || !dialog.current?.contains(document.activeElement))) {event.preventDefault();first?.focus();}
    }
    function focus(event:FocusEvent) {if (!dialog.current?.contains(event.target as Node)) close.current?.focus();}
    document.addEventListener('keydown',keydown);document.addEventListener('focusin',focus);
    return ()=>{document.removeEventListener('keydown',keydown);document.removeEventListener('focusin',focus);document.body.style.overflow=overflow;trigger.current?.focus();};
  },[open]);
  const whatsapp=/^\d{10,15}$/.test(contacts.whatsapp)?`https://wa.me/${contacts.whatsapp}`:null;
  return <>
    <button ref={trigger} type="button" className="brand-contact-trigger" aria-label={t('menu.open')} aria-haspopup="dialog" aria-expanded={open} onClick={()=>setOpen(true)}><svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" strokeWidth="1.7" aria-hidden="true"><path d="M3 6h18M3 12h18M3 18h18"/></svg><span className="menu-label">{t('menu.label')}</span></button>
    {open && createPortal(<div className="contact-overlay auth-surface" onClick={event=>{if(event.target===event.currentTarget)setOpen(false);}}>
      <section ref={dialog} className="contact-drawer" role="dialog" aria-modal="true" aria-labelledby="contact-title">
        <div className="contact-heading"><h2 id="contact-title">{t('menu.title')}</h2><button ref={close} type="button" className="brand-icon-button" onClick={()=>setOpen(false)} aria-label={t('menu.close')}>×</button></div>
        <section className="menu-section"><h3>{t('menu.language')}</h3><div className="language-list">{languages.map(language=><button key={language.id} type="button" lang={language.id} aria-pressed={i18n.resolvedLanguage===language.id} onClick={()=>void i18n.changeLanguage(language.id)}><span className="language-code">{language.code}</span><span>{language.name}</span><span className="language-check" aria-hidden="true">{i18n.resolvedLanguage===language.id?'✓':''}</span></button>)}</div></section>
        <section className="menu-section"><h3>{t('menu.theme')}</h3><div className="brand-theme-toggle" role="group" aria-label={t('theme.appearance')}>{(['light','dark'] as const).map(value=><button key={value} type="button" aria-pressed={theme===value} className={theme===value?'selected':''} onClick={()=>setTheme(value)}>{t('theme.'+value)}</button>)}</div></section>
        <section className="menu-section"><h3>{t('menu.contact')}</h3><dl className="contact-list">
          <div><dt>WhatsApp</dt><dd>{whatsapp?<a href={whatsapp} target="_blank" rel="noopener noreferrer">{t('menu.chat')}</a>:t('menu.soon')}</dd></div>
          <div><dt>{t('menu.email')}</dt><dd>{contacts.email?<a href={`mailto:${contacts.email}`}>{contacts.email}</a>:t('menu.soon')}</dd></div>
          <div><dt>{t('menu.phone')}</dt><dd>{contacts.phone?<a href={`tel:${contacts.phone.replace(/[^+\d]/g,'')}`}>{contacts.phone}</a>:t('menu.soon')}</dd></div>
          <div><dt>{t('menu.hours')}</dt><dd>{contacts.hours||t('menu.hoursPending')}</dd></div>
          <div><dt>TITAN</dt><dd><a href={contacts.website} target="_blank" rel="noopener noreferrer">titantecnology.com.br</a></dd></div>
        </dl>{whatsapp?<a className="brand-primary contact-whatsapp" href={whatsapp} target="_blank" rel="noopener noreferrer">{t('menu.whatsapp')}</a>:<button className="brand-primary" type="button" disabled>{t('menu.whatsappPending')}</button>}</section>
        <div className="menu-legal">{contacts.termsUrl?<a href={contacts.termsUrl}>{t('menu.terms')}</a>:<button type="button" className="brand-text-button" onClick={()=>setLegal(true)}>{t('menu.terms')}</button>}{contacts.privacyUrl?<a href={contacts.privacyUrl}>{t('menu.privacy')}</a>:<button type="button" className="brand-text-button" onClick={()=>setLegal(true)}>{t('menu.privacy')}</button>}</div>
        {legal&&<p role="status" className="brand-message">{t('menu.legalPending')}</p>}
      </section>
    </div>,document.body)}
  </>;
}
