import {Link} from 'react-router-dom';
import {companySetupStatus,setupLocale} from '../../../core/companyRegistration/journey.mjs';
import type {RegistrationRecord} from '../../../core/companyRegistration/client.mjs';
type Props={record:RegistrationRecord|null;words:Record<string,string>;locale:string};
export function CompanySetupSteps({record,words:cw,locale}:Props){
 const progress=companySetupStatus(record),lang=setupLocale(locale);
 return <section className="brand-panel" aria-labelledby="company-setup-title"><h2 id="company-setup-title">{cw.setupTitle}</h2><p role="note">{cw.setupScope}</p>
 <ol>
  <li><h3>{cw.setupStorage}</h3><p>{cw.dataDestination}</p><p>{cw.setupStorageHelp}</p><code>titan-local mode --deployment PROFILE_ABSOLUTE_PATH</code></li>
  <li><h3>{cw.keyCustody}</h3><p role="note">{cw.lostKey}</p><p>{cw.setupKeyHelp}</p>
   <details><summary>{cw.setupCommands}</summary><p>{cw.setupCommandHelp}</p>
    <pre><code>{`titan-fiscal vault-init --vault VAULT_ABSOLUTE_PATH --key-file KEY_ABSOLUTE_PATH --lang ${lang}`}</code></pre>
    <pre><code>{`titan-fiscal vault-export-key --key-file KEY_ABSOLUTE_PATH --out BACKUP_KEY_ABSOLUTE_PATH --lang ${lang}`}</code></pre>
    <pre><code>{`titan-fiscal vault-rotate --vault VAULT_ABSOLUTE_PATH --key-file KEY_ABSOLUTE_PATH --out NEW_KEY_ABSOLUTE_PATH --backup-confirmed --lang ${lang}`}</code></pre>
   </details><p>{cw.setupRestore}</p>
  </li>
  <li><h3>{cw.setupRegistration}</h3><p>{cw.setupSavedOnly}</p><dl>
   <div><dt>{cw.pre_registration}</dt><dd>{progress.preSaved?cw.setupPersisted:cw.setupNotConfirmed}</dd></div>
   <div><dt>{cw.company_profile}</dt><dd>{progress.fullSaved?cw.setupPersisted:cw.setupNotConfirmed}</dd></div>
  </dl><a className="brand-secondary" href="#company-registration-form">{cw.setupEditRegistration}</a></li>
  <li><h3>{cw.fiscal}</h3><p>{cw.setupFocus}</p><p>{cw.fiscalNote}</p><Link className="brand-secondary" to="/local/enterprise?area=fiscal">{cw.fiscal}</Link></li>
  <li><h3>{cw.staff}</h3><p>{cw.setupStaffHelp}</p><Link className="brand-secondary" to="/local/staff">{cw.staff}</Link></li>
 </ol></section>;
}
