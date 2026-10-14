import pt from '../../i18n/context75/pt-BR.json';import en from '../../i18n/context75/en.json';import es from '../../i18n/context75/es.json';import fr from '../../i18n/context75/fr.json';import zh from '../../i18n/context75/zh-CN.json';
const copy:Record<string,typeof pt>={'pt-BR':pt,pt,en,es,fr,'zh-CN':zh,zh};export const words75=(locale:string)=>copy[locale]||pt;
