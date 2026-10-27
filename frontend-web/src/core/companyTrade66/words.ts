import d0 from '../../i18n/companyTrade66/pt-BR.json';
import d1 from '../../i18n/companyTrade66/en.json';
import d2 from '../../i18n/companyTrade66/es.json';
import d3 from '../../i18n/companyTrade66/fr.json';
import d4 from '../../i18n/companyTrade66/zh-CN.json';
const dictionaries:Record<string,typeof d0>={'pt-BR':d0,pt:d0,en:d1,es:d2,fr:d3,'zh-CN':d4,zh:d4};
export function tradeWords(lang:string){return dictionaries[lang]||d0;}
