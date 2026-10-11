import {useRef,useState} from 'react';
import type {FormEvent} from 'react';
import {Link} from 'react-router-dom';
import {useTranslation} from 'react-i18next';
import {BrandShell} from '../../../shared/brand/BrandShell';
import {plans,planTotals,formatMoney} from './registrationModel.mjs';
import './registration.css';
const recommendation:Record<string,string>={retail:'pos',production:'production',staff:'staff',accounting:'accounting'};
const sizes=['mei','micro','small','medium','large'],activities=['retail','production','staff','accounting'],stores=['local','server','cloud','hybrid'];
export default function Registration() {
 const {t,i18n}=useTranslation(), money=(cents:number)=>formatMoney(cents,i18n.resolvedLanguage);
 const [step,setStep]=useState(0),[size,setSize]=useState('mei'),[activity,setActivity]=useState('retail'),[storage,setStorage]=useState('local'),[users,setUsers]=useState('2');
 const [selected,setSelected]=useState<string[]>(['pos']),[others,setOthers]=useState(false),[message,setMessage]=useState('');
 const [name,setName]=useState(''),[email,setEmail]=useState(''),[company,setCompany]=useState(''),[reviewed,setReviewed]=useState(false);
 const heading=useRef<HTMLHeadingElement>(null),steps=['profile','modules','data','review'];
 const totals=planTotals(selected),recommended=recommendation[activity],chosen=plans.filter(plan=>selected.includes(plan.id));
 function go(next:number){setStep(next);setMessage('');requestAnimationFrame(()=>heading.current?.focus());}
 function changeActivity(value:string){setActivity(value);setSelected(recommendation[value]?[recommendation[value]]:[]);setOthers(false);setReviewed(false);setMessage('');}
 function toggle(id:string){setSelected(value=>value.includes(id)?value.filter(item=>item!==id):[...value,id]);setReviewed(false);setMessage('');}
 function submit(event:FormEvent<HTMLFormElement>){
  event.preventDefault();
  if(step===3&&!reviewed){setMessage('errors.confirm');return;}
  if(!event.currentTarget.checkValidity()){setMessage('errors.fields');event.currentTarget.querySelector<HTMLElement>(':invalid')?.focus();return;}
  if(step===1&&!selected.length){setMessage('errors.modules');return;}
  if(step<3){go(step+1);return;}setMessage('register.done');
 }
 function cards(ids:string[]){
  return <div className="plan-grid">{plans.filter(plan=>ids.includes(plan.id)).map(plan=><button type="button" key={plan.id} className={`plan-card ${selected.includes(plan.id)?'selected':''}`} aria-pressed={selected.includes(plan.id)} onClick={()=>toggle(plan.id)}>
   <span className="plan-card-top"><span className="plan-badge">{t(plan.id===recommended?'modules.recommended':'modules.optional')}</span><span className="plan-check" aria-hidden="true">{selected.includes(plan.id)?'✓':'+'}</span></span>
   <h3>{t('modules.'+plan.id)}</h3><p className="plan-price">{money(plan.price)}<small>{t('modules.month')}</small></p>
   {plan.previous>plan.price&&<><p className="plan-previous">{t('modules.before',{price:money(plan.previous)})}</p><p className="plan-savings">{t('modules.savings',{price:money(plan.previous-plan.price)})}</p></>}
   <p className="plan-description">{t('modules.'+plan.id+'Help')}</p><span className="plan-selection">{t(selected.includes(plan.id)?'modules.selected':'modules.select')}</span>
  </button>)}</div>;
 }
 return <BrandShell><section className="registration">
  <div className="registration-intro"><Link className="brand-link registration-back-login" to="/local/login">← {t('register.backLogin')}</Link><p className="brand-eyebrow">{t('register.eyebrow')}</p><h1>{t('register.title')}</h1><p className="brand-state">{t('register.subtitle')}</p></div>
  <ol className="registration-progress" aria-label={t('register.steps')}>{steps.map((label,index)=><li key={label} aria-current={step===index?'step':undefined} className={index<step?'completed':index===step?'current':''}><span aria-hidden="true">{index<step?'✓':index+1}</span><span>{t('register.'+label)}</span></li>)}</ol>
  <div className="registration-meter" role="progressbar" aria-label={t('register.steps')} aria-valuemin={0} aria-valuemax={4} aria-valuenow={step+1}><span style={{width:`${(step+1)*25}%`}}/></div>
  <form noValidate onSubmit={submit}>
   <h2 ref={heading} tabIndex={-1} className="registration-heading">{step+1}. {t('register.'+steps[step])}</h2>
   {step===0&&<section className="brand-panel"><div className="catalog-form">
    <label>{t('register.size')}<select value={size} onChange={event=>{setSize(event.target.value);setReviewed(false);}}>{sizes.map(value=><option value={value} key={value}>{t('size.'+value)}</option>)}</select></label>
    <label>{t('register.activity')}<select value={activity} onChange={event=>changeActivity(event.target.value)}>{activities.map(value=><option value={value} key={value}>{t('activity.'+value)}</option>)}</select></label>
    <label>{t('register.storage')}<select value={storage} aria-describedby="storage-help" onChange={event=>{setStorage(event.target.value);setReviewed(false);}}>{stores.map(value=><option value={value} key={value}>{t('storage.'+value)}</option>)}</select><span id="storage-help" className="registration-field-help">{t('storage.'+storage+'Help')}</span></label>
    <label>{t('register.users')}<input required type="number" min="1" max="10000" step="1" value={users} onChange={event=>{setUsers(event.target.value);setReviewed(false);}}/><span className="registration-field-help">{t('register.usersHelp')}</span></label>
   </div>{size==='mei'&&<p className="brand-state">{t('register.meiHelp')}</p>}</section>}
   {step===1&&<><p className="brand-state">{t('register.recommend',{activity:t('activity.'+activity)})}</p>{cards([recommended])}<button type="button" className="brand-secondary registration-more" aria-expanded={others} aria-controls="other-modules" onClick={()=>setOthers(value=>!value)}>{t(others?'register.less':'register.more')} <span aria-hidden="true">{others?'−':'+'}</span></button><div id="other-modules" hidden={!others}>{others&&cards(plans.filter(plan=>plan.id!==recommended).map(plan=>plan.id))}</div><p className="brand-state">{t('register.optionalHelp')}</p></>}
   {step===2&&<section className="brand-panel"><p className="brand-state">{t('register.dataHelp')}</p><div className="catalog-form">
    <label>{t('register.name')}<input required autoComplete="name" maxLength={150} value={name} onChange={event=>{setName(event.target.value);setReviewed(false);}}/></label>
    <label>{t('menu.email')}<input required type="email" autoComplete="email" maxLength={254} value={email} onChange={event=>{setEmail(event.target.value);setReviewed(false);}}/></label>
    <label className="registration-full-field">{t('register.companyName')}<input required autoComplete="organization" maxLength={150} value={company} onChange={event=>{setCompany(event.target.value);setReviewed(false);}}/></label>
   </div></section>}
   {step===3&&<section className="brand-panel registration-review"><h3>{t('register.check')}</h3><dl>
    <div><dt>{t('register.company')}</dt><dd>{company} · {t('size.'+size)}</dd></div>
    <div><dt>{t('register.owner')}</dt><dd>{name} · {email}</dd></div>
    <div><dt>{t('register.configuration')}</dt><dd>{t('activity.'+activity)} · {t('storage.'+storage)} · {t('register.userCount',{count:Number(users)})}</dd></div>
    <div><dt>{t('register.modules')}</dt><dd>{chosen.map(plan=>t('modules.'+plan.id)).join(', ')}</dd></div>
    <div><dt>{t('register.total')}</dt><dd>{money(totals.monthly)}</dd></div>
   </dl><p className="brand-message">{t('register.unavailable')}</p><label className="registration-confirm"><input type="checkbox" required checked={reviewed} onChange={event=>setReviewed(event.target.checked)}/>{t('register.confirm')}</label></section>}
   {message&&<p className="brand-message" role="status">{t(message)}</p>}
   <div className="registration-sticky"><div className="registration-total" aria-live="polite"><span>{chosen.length===1?t('modules.'+chosen[0].id):t('register.chosen',{count:chosen.length})}</span><strong>{money(totals.monthly)}<small>{t('modules.month')}</small></strong>{totals.savings>0&&<span className="plan-savings">{t('modules.savings',{price:money(totals.savings)})}</span>}</div><div className="registration-sticky-actions">{step>0&&<button type="button" className="brand-secondary" onClick={()=>go(step-1)}>{t('register.back')}</button>}<button type="submit" className="brand-primary" disabled={step===1&&!selected.length}>{t(step===3?'register.finish':'register.continue')}<span aria-hidden="true"> →</span></button></div></div>
  </form>
 </section></BrandShell>;
}
