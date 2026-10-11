import {Link} from 'react-router-dom';
import {useTranslation} from 'react-i18next';
import {useLocalAccess} from '../../../core/local/LocalAccess';
import './subscription.css';

const moduleNames:Record<string,string>={inventory:'catalog',pos:'pos',staff:'staff'};

export default function LocalSubscription(){
 const {capabilities,refresh}=useLocalAccess();
 const {t,i18n}=useTranslation();
 const license=capabilities?.license;
 const state=license?.state??'unavailable';
 const badge=state==='active'?'active':state==='expired'?'expired':state==='not_yet_valid'?'pending':'unknown';
 const expiry=license?.expires_unix?new Intl.DateTimeFormat(i18n.resolvedLanguage||'pt-BR',{dateStyle:'medium'}).format(new Date(license.expires_unix*1000)):null;
 return <main className="brand-app-content subscription-page">
  <div className="brand-app-heading"><div><p className="brand-eyebrow">{t('subscription.eyebrow')}</p><h1>{t('subscription.title')}</h1></div><button type="button" className="brand-secondary subscription-check" onClick={refresh}>{t('subscription.check')}</button></div>
  <div className="subscription-grid">
   <section className="brand-panel subscription-panel"><h2>{t('subscription.period')}</h2><span className={`subscription-badge ${badge}`}>{t('subscription.state.'+state)}</span><strong className="license-days">{state==='active'?t('subscription.days',{count:license?.remaining_days}):state==='not_installed'?t('subscription.noLicense'):t('subscription.state.'+state)}</strong>{expiry&&<p className="brand-state">{t('subscription.expires',{date:expiry})}</p>}</section>
   <section className="brand-panel subscription-panel"><h2>{t('subscription.billing')}</h2><strong className="subscription-statement">{t('subscription.billingUnavailable')}</strong><p className="brand-state">{t('subscription.billingHelp')}</p><Link className="brand-primary subscription-action" to="/register">{t('subscription.plans')}</Link></section>
   <section className="brand-panel subscription-panel subscription-modules"><h2>{t('subscription.modules')}</h2>{license?.modules.length?<ul>{license.modules.map(id=><li key={id}>{moduleNames[id]?t('workspace.'+moduleNames[id]):id}</li>)}</ul>:<div className="subscription-empty"><span aria-hidden="true">▦</span><p>{t('subscription.noModules')}</p><Link className="brand-secondary" to="/register">{t('subscription.choose')}</Link></div>}</section>
  </div>
 </main>;
}
