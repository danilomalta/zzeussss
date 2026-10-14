import pt from '../../i18n/map79/pt-BR.json';
import en from '../../i18n/map79/en.json';
import es from '../../i18n/map79/es.json';
import fr from '../../i18n/map79/fr.json';
import zh from '../../i18n/map79/zh-CN.json';
const dictionary:Record<string,typeof pt>={'pt-BR':pt,pt,en,es,fr,'zh-CN':zh,zh};
export function words79(language:string){return dictionary[language]||pt;}
