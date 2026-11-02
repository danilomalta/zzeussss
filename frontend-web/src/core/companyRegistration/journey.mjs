// Only persisted API revisions count. Editing a draft never completes a step.
export function companySetupStatus(record){
 const saved=!!record&&Number.isSafeInteger(record.revision)&&record.revision>0;
 const stage=record?.input?.stage;
 return Object.freeze({preSaved:saved&&['pre_registration','company_profile'].includes(stage),fullSaved:saved&&stage==='company_profile'});
}
export function setupLocale(locale){
 const code=typeof locale==='string'?locale.toLowerCase():'';
 return code.startsWith('en')?'en':code.startsWith('es')?'es':code.startsWith('fr')?'fr':code.startsWith('zh')?'zh-CN':'pt-BR';
}
