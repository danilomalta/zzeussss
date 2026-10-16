import {useEffect,useRef,useState} from 'react';
import {useTranslation} from 'react-i18next';
import {validCustomer93,type Customer93} from '../../../core/local/reconciliation93.mjs';
import {words93} from '../../../core/local/words93';
import './operation93.css';
export default function SaleCustomer93({value,onContinue}:{value:Customer93;onContinue:(v:Customer93)=>void}){
 const {i18n}=useTranslation(),w=words93(i18n.resolvedLanguage||i18n.language),ref=useRef<HTMLDialogElement>(null),[draft,setDraft]=useState(value),[error,setError]=useState(false);
 useEffect(()=>{const previous=document.activeElement as HTMLElement|null;ref.current?.showModal();return()=>{ref.current?.close();previous?.focus();};},[]);
 function submit(v:Customer93){const clean={...v,document:v.document.replace(/\D/g,'')};if(!validCustomer93(clean)){setError(true);return;}onContinue(clean);}
 return <dialog ref={ref} className="pos-payment-dialog customer93" aria-labelledby="customer93-title" onCancel={e=>{e.preventDefault();onContinue(value);}}><h2 id="customer93-title">{w.identify}</h2><p>{w.firstProduct}</p><form onSubmit={e=>{e.preventDefault();submit(draft);}}><label>{w.cpf}<input autoFocus name="cpf93" autoComplete="off" inputMode="numeric" maxLength={14} placeholder="000.000.000-00" value={draft.document} onChange={e=>{setDraft({...draft,document:e.target.value});setError(false);}}/></label><p className="customer93-note">{w.identityOnly}</p><fieldset><legend>{w.clubQuestion}</legend>{(['skip','member','join'] as const).map(choice=><label className="customer93-radio" key={choice}><input type="radio" name="club93" checked={draft.club===choice} onChange={()=>setDraft({...draft,club:choice})}/>{w[choice]}</label>)}</fieldset>{draft.club==='member'&&<p role="status">{w.clubUnavailable}</p>}{draft.club==='join'&&<p>{w.joinHelp}</p>}{error&&<p role="alert">{w.invalidCPF}</p>}<div className="customer93-actions"><button type="button" className="brand-secondary" onClick={()=>onContinue({document:'',club:'skip'})}>{w.without}</button><button className="brand-primary">{w.continueSale}</button></div></form></dialog>;
}
