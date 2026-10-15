import pt from '../../i18n/operations83/pt-BR.json';
import en from '../../i18n/operations83/en.json';
import es from '../../i18n/operations83/es.json';
import fr from '../../i18n/operations83/fr.json';
import zh from '../../i18n/operations83/zh-CN.json';
export const words83=(language:string):typeof pt=>language.startsWith('en')?en:language.startsWith('es')?es:language.startsWith('fr')?fr:language.startsWith('zh')?zh:pt;
