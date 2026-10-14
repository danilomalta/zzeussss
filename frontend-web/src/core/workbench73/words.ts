import pt from '../../i18n/workbench73/pt-BR.json';import en from '../../i18n/workbench73/en.json';import es from '../../i18n/workbench73/es.json';import fr from '../../i18n/workbench73/fr.json';import zh from '../../i18n/workbench73/zh-CN.json';
const words:Record<string,typeof pt>={'pt-BR':pt,pt,en,es,fr,'zh-CN':zh,zh};
export const workbenchWords=(locale:string)=>words[locale]||pt;
