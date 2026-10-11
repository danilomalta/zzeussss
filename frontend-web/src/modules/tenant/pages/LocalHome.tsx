import {Link} from 'react-router-dom';
import {useLocalAccess} from '../../../core/local/LocalAccess';
import {allowedAreas} from '../../../core/local/accessModel.mjs';
import {useTranslation} from 'react-i18next';
export default function LocalHome(){
 const {t}=useTranslation();
 const {capabilities}=useLocalAccess();const areas=allowedAreas(capabilities);
 const cards=[['catalog','/local/catalog'],['stock','/local/stock'],['pos','/local/pos'],['staff','/local/staff'],['plans','/local/subscription']];
 return <main className="brand-app-content"><p className="brand-eyebrow">{t('home.eyebrow')}</p><h1>{t('home.title')}</h1><p className="brand-state">{t('home.access')}</p>
 {capabilities?.license.state!=='active'&&<div role="status" className="brand-message"><p>{t('home.licenseNotice')}</p>{areas.includes('plans')&&<Link to="/local/subscription" className="brand-link">{t('home.viewLicense')}</Link>}</div>}
 <div className="plan-grid">{cards.filter(([id])=>areas.includes(id)||capabilities?.role==='owner').map(([id,path])=>areas.includes(id)?<Link className="brand-panel area-card" key={id} to={path}><h2>{t('home.'+id)}</h2><p>{t('home.'+id+'Help')}</p><span>{t('home.open')} →</span></Link>:<section className="brand-panel area-card area-card-locked" key={id} aria-label={t('home.'+id)}><h2>{t('home.'+id)}</h2><p>{t('home.'+id+'Help')}</p><span>{t('home.needsLicense')}</span></section>)}</div>
 {areas.includes('production') && <p className="brand-message">{t('home.productionPending')}</p>}{areas.includes('point') && <p className="brand-message">{t('home.pointPending')}</p>}</main>;
}
