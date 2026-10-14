import pt from '../../i18n/settings80/pt-BR.json';
import en from '../../i18n/settings80/en.json';
import es from '../../i18n/settings80/es.json';
import fr from '../../i18n/settings80/fr.json';
import zh from '../../i18n/settings80/zh-CN.json';
const dictionary:Record<string,typeof pt>={'pt-BR':pt,pt,en,es,fr,'zh-CN':zh,zh};
export function words80(language:string){return dictionary[language]||pt;}
