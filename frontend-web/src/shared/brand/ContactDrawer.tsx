import {useEffect, useRef, useState} from 'react';
import {createPortal} from 'react-dom';
import {contactConfig as contacts} from './contactConfig';

export default function ContactDrawer() {
  const [open, setOpen] = useState(false);
  const trigger = useRef<HTMLButtonElement>(null);
  const dialog = useRef<HTMLElement>(null);
  const close = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    if (!open) return;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    close.current?.focus();
    function keydown(event: KeyboardEvent) {
      if (event.key === 'Escape') { event.preventDefault(); setOpen(false); }
      if (event.key !== 'Tab') return;
      const elements = Array.from(dialog.current?.querySelectorAll<HTMLElement>('a[href], button:not([disabled]), [tabindex="0"]') ?? []);
      const first = elements[0], last = elements[elements.length - 1];
      if (event.shiftKey && (document.activeElement === first || !dialog.current?.contains(document.activeElement))) { event.preventDefault(); last?.focus(); }
      else if (!event.shiftKey && (document.activeElement === last || !dialog.current?.contains(document.activeElement))) { event.preventDefault(); first?.focus(); }
    }
    function containFocus(event: FocusEvent) {
      if (!dialog.current?.contains(event.target as Node)) close.current?.focus();
    }
    document.addEventListener('keydown', keydown);
    document.addEventListener('focusin', containFocus);
    return () => {
      document.removeEventListener('keydown', keydown);
      document.removeEventListener('focusin', containFocus);
      document.body.style.overflow = previousOverflow;
      trigger.current?.focus();
    };
  }, [open]);
  const whatsapp = /^\d{10,15}$/.test(contacts.whatsapp) ? `https://wa.me/${contacts.whatsapp}` : null;
  return <>
    <button ref={trigger} type="button" className="brand-contact-trigger" aria-haspopup="dialog" aria-expanded={open} onClick={() => setOpen(true)}><span aria-hidden="true">✉</span> Fale conosco</button>
    {open && createPortal(<div className="contact-overlay" onClick={event => {if (event.target === event.currentTarget) setOpen(false);}}>
      <section ref={dialog} className="contact-drawer" role="dialog" aria-modal="true" aria-labelledby="contact-title">
        <div className="contact-heading"><h2 id="contact-title">Fale com a TITAN</h2><button ref={close} type="button" className="brand-icon-button" onClick={() => setOpen(false)} aria-label="Fechar contatos">×</button></div>
        <p className="brand-state">Escolha o melhor canal para falar com nossa equipe.</p>
        <dl className="contact-list">
          <div><dt>WhatsApp</dt><dd>{whatsapp ? <a href={whatsapp} target="_blank" rel="noopener noreferrer">Abrir conversa</a> : 'Em breve'}</dd></div>
          <div><dt>E-mail</dt><dd>{contacts.email ? <a href={`mailto:${contacts.email}`}>{contacts.email}</a> : 'Em breve'}</dd></div>
          <div><dt>Telefone</dt><dd>{contacts.phone ? <a href={`tel:${contacts.phone.replace(/[^+\d]/g, '')}`}>{contacts.phone}</a> : 'Em breve'}</dd></div>
          <div><dt>Atendimento</dt><dd>{contacts.hours || 'Horário a informar'}</dd></div>
          <div><dt>Site</dt><dd><a href={contacts.website} target="_blank" rel="noopener noreferrer">titantecnology.com.br</a></dd></div>
        </dl>
        {whatsapp ? <a className="brand-primary contact-whatsapp" href={whatsapp} target="_blank" rel="noopener noreferrer">Falar no WhatsApp</a> : <button type="button" className="brand-primary" disabled>WhatsApp em breve</button>}
      </section>
    </div>, document.body)}
  </>;
}
