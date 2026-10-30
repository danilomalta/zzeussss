import pt from '../../i18n/companyRegistration/pt-BR.json';
import en from '../../i18n/companyRegistration/en.json';
import es from '../../i18n/companyRegistration/es.json';
import fr from '../../i18n/companyRegistration/fr.json';
import zh from '../../i18n/companyRegistration/zh-CN.json';
export function companyWords(locale:string):Record<string,string>{return locale.startsWith('en')?en:locale.startsWith('es')?es:locale.startsWith('fr')?fr:locale.startsWith('zh')?zh:pt;}
