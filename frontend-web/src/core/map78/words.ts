import pt from '../../i18n/map78/pt-BR.json';
import en from '../../i18n/map78/en.json';
import es from '../../i18n/map78/es.json';
import fr from '../../i18n/map78/fr.json';
import zh from '../../i18n/map78/zh-CN.json';
const dictionary:Record<string,typeof pt>={'pt-BR':pt,pt,en,es,fr,'zh-CN':zh,zh};
export function words78(language:string){return dictionary[language]||pt;}
